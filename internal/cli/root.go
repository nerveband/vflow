package cli

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	vaudit "github.com/nerveband/vflow/internal/audit"
	vcleanup "github.com/nerveband/vflow/internal/cleanup"
	vcolor "github.com/nerveband/vflow/internal/color"
	vconfig "github.com/nerveband/vflow/internal/config"
	"github.com/nerveband/vflow/internal/contract"
	verrors "github.com/nerveband/vflow/internal/errors"
	vframing "github.com/nerveband/vflow/internal/framing"
	vframingsession "github.com/nerveband/vflow/internal/framing/session"
	vindex "github.com/nerveband/vflow/internal/index"
	vjobs "github.com/nerveband/vflow/internal/jobs"
	vmedia "github.com/nerveband/vflow/internal/media"
	vnle "github.com/nerveband/vflow/internal/nle"
	"github.com/nerveband/vflow/internal/output"
	vproject "github.com/nerveband/vflow/internal/project"
	vqa "github.com/nerveband/vflow/internal/qa"
	vrender "github.com/nerveband/vflow/internal/render"
	vsyncmap "github.com/nerveband/vflow/internal/syncmap"
	vtimeline "github.com/nerveband/vflow/internal/timeline"
	vtranscript "github.com/nerveband/vflow/internal/transcript"
	vupdate "github.com/nerveband/vflow/internal/update"
	"github.com/spf13/cobra"
)

type globalOptions struct {
	Format      string
	FormatError string
	JSON        bool
	Quiet       bool
	Fields      string
	IDOnly      bool
	Count       bool
	Limit       int
	MaxDepth    int
	Transform   string
	Timeout     string
	Profile     string
	DryRun      bool
	Commit      bool
	Live        bool
	Wait        bool
	Overwrite   bool
}

type exitError struct {
	err  error
	code int
}

func (e exitError) Error() string {
	if e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e exitError) ExitCode() int {
	return e.code
}

func Execute() error {
	return NewRootCommand().Execute()
}

func NewRootCommand() *cobra.Command {
	opts := &globalOptions{Format: "human", FormatError: "human", Limit: 100, MaxDepth: 4, Timeout: "30s"}
	cmd := &cobra.Command{
		Use:           "vflow",
		Short:         "Agent-native local video workflow compiler",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if opts.JSON {
			opts.Format = "json"
			opts.FormatError = "json"
		}
		if os.Getenv("AI_AGENT") != "" {
			if opts.Format == "human" {
				opts.Format = "json"
			}
			if opts.FormatError == "human" {
				opts.FormatError = "json"
			}
		}
	}
	addGlobalFlags(cmd, opts)
	cmd.AddCommand(
		newVersionCommand(opts),
		schemaCommand(opts),
		agentContextCommand(opts),
		skillPathCommand(opts),
		doctorCommand(opts),
		auditCommand(opts),
		feedbackCommand(opts),
		configCommand(opts),
		profileCommand(opts),
		authCommand(opts),
		projectCommand(opts),
		mediaCommand(opts),
		transcriptCommand(opts),
		cleanupCommand(opts),
		cutCommand(opts),
		framingCommand(opts),
		timelineCommand(opts),
		renderCommand(opts),
		qaCommand(opts),
		colorCommand(opts),
		nleCommand(opts),
		jobsCommand(opts),
		artifactsCommand(opts),
		upgradeCommand(opts),
	)
	return cmd
}

func addGlobalFlags(cmd *cobra.Command, opts *globalOptions) {
	f := cmd.PersistentFlags()
	f.StringVar(&opts.Format, "format", "human", "output format: json, jsonl, yaml, raw, table, human")
	f.StringVar(&opts.FormatError, "format-error", "human", "error output format: json or human")
	f.BoolVar(&opts.JSON, "json", false, "shortcut for --format json --format-error json")
	f.BoolVar(&opts.Quiet, "quiet", false, "suppress non-essential output")
	f.StringVar(&opts.Fields, "fields", "", "comma-separated fields to include")
	f.BoolVar(&opts.IDOnly, "id-only", false, "emit only resource ids")
	f.BoolVar(&opts.Count, "count", false, "emit only a count")
	f.IntVar(&opts.Limit, "limit", 100, "maximum items to return")
	f.IntVar(&opts.MaxDepth, "max-depth", 4, "maximum nested traversal depth")
	f.StringVar(&opts.Transform, "transform", "", "gjson-style transform path")
	f.StringVar(&opts.Timeout, "timeout", "30s", "operation timeout")
	f.StringVar(&opts.Profile, "profile", "", "profile name")
	f.BoolVar(&opts.DryRun, "dry-run", false, "show intended changes without writing")
	f.BoolVar(&opts.Commit, "commit", false, "confirm writes, provider calls, or destructive work")
	f.BoolVar(&opts.Live, "live", false, "allow live provider calls")
	f.BoolVar(&opts.Wait, "wait", false, "wait for async work to finish")
	f.BoolVar(&opts.Overwrite, "overwrite", false, "allow overwriting existing artifacts with --commit")
}

func writeJSON(out io.Writer, command string, data interface{}) error {
	return output.WriteJSON(out, output.Envelope(command, data))
}

func writeOutput(cmd *cobra.Command, opts *globalOptions, command string, data interface{}) error {
	if opts.Format == "json" || opts.JSON {
		return writeJSON(cmd.OutOrStdout(), command, data)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s\n", command)
	return nil
}

func writeStructuredError(cmd *cobra.Command, opts *globalOptions, err *verrors.Error) error {
	if opts.FormatError == "json" || opts.Format == "json" || opts.JSON || os.Getenv("AI_AGENT") != "" {
		_ = output.WriteJSON(cmd.ErrOrStderr(), output.ErrorEnvelope(err))
	} else {
		fmt.Fprintln(cmd.ErrOrStderr(), err.Error())
	}
	return exitError{err: err, code: err.ExitCode}
}

func agentContextCommand(opts *globalOptions) *cobra.Command {
	var section string
	cmd := &cobra.Command{
		Use:   "agent-context",
		Short: "Emit concise context for coding agents",
		RunE: func(cmd *cobra.Command, args []string) error {
			data := map[string]any{
				"schema_version": "vflow-agent-context/v1",
				"section":        firstNonEmptyString(section, "all"),
				"safety": []string{
					"mutating commands support --dry-run and require --commit to write",
					"provider calls require --live and runtime env keys",
					"secrets are referenced by env var and never stored in project artifacts",
				},
				"commands": contract.DefaultRegistry().Commands(),
				"providers": map[string]any{
					"offline": []string{"ffprobe", "ffmpeg", "plain-text", "generic-words", "fcpxml", "edl", "mlt", "otio", "sidecar"},
					"live":    []string{"gemini via GEMINI_API_KEY", "openai via OPENAI_API_KEY", "optional STT provider env vars"},
				},
				"nle_targets":       []string{"resolve", "fcpxml", "premiere", "otio", "edl", "mlt", "sidecar"},
				"artifact_contract": []string{"project.json", "source-media-review.json", "transcript/words.json", "decisions/content-edl.json", "decisions/time-map.json", "timeline/compiled-timeline.json", "reports/provenance.json", "~/.vflow/index.sqlite"},
			}
			return writeOutput(cmd, opts, "agent-context", data)
		},
	}
	cmd.Flags().StringVar(&section, "section", "", "context section")
	return cmd
}

func skillPathCommand(opts *globalOptions) *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "skill-path",
		Short: "Print bundled skill paths",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths := map[string]string{
				"root":                 "SKILL.md",
				"vflow-video-workflow": "skills/vflow-video-workflow/SKILL.md",
			}
			if name != "" {
				return writeOutput(cmd, opts, "skill-path", map[string]any{"name": name, "path": paths[name]})
			}
			return writeOutput(cmd, opts, "skill-path", map[string]any{"skills": paths})
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "skill name")
	return cmd
}

func doctorCommand(opts *globalOptions) *cobra.Command {
	var local bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check local tools, providers, and project capabilities",
		RunE: func(cmd *cobra.Command, args []string) error {
			tools := map[string]any{}
			for _, name := range []string{"ffmpeg", "ffprobe", "python3"} {
				path, err := exec.LookPath(name)
				tools[name] = map[string]any{"available": err == nil, "path": path}
			}
			env := map[string]bool{}
			for _, key := range []string{"OPENAI_API_KEY", "GEMINI_API_KEY", "GOOGLE_API_KEY", "GOOGLE_GENERATIVE_AI_API_KEY", "ELEVENLABS_API_KEY", "SONIOX_API_KEY", "ASSEMBLYAI_API_KEY", "DEEPGRAM_API_KEY", "GLADIA_API_KEY", "ANTHROPIC_API_KEY", "HF_TOKEN"} {
				env[key] = os.Getenv(key) != ""
			}
			nle := map[string]any{
				"targets":                  []string{"resolve", "fcpxml", "premiere", "otio", "edl", "mlt", "sidecar"},
				"exports_sidecars":         true,
				"import_formats":           []string{"fcpxml", "premiere", "mlt", "otio", "edl"},
				"resolve_project_packages": "blocked_with_export_hint",
				"blocked_change_types":     []string{"color_grade", "complex_effect", "nested_timeline", "plugin_effect", "keyframed_transform", "missing_sidecar"},
				"real_editor_fixture_gap":  "requires exported timelines from Resolve/FCP/Premiere/Shotcut/OTIO for exhaustive compatibility proof",
			}
			return writeOutput(cmd, opts, "doctor", map[string]any{"status": "ok", "local": local, "tools": tools, "env_present": env, "nle": nle})
		},
	}
	cmd.Flags().BoolVar(&local, "local", false, "local-only checks")
	return cmd
}

func auditCommand(opts *globalOptions) *cobra.Command {
	parent := &cobra.Command{Use: "audit", Short: "Run vflow self-audits"}
	parent.AddCommand(&cobra.Command{
		Use:   "cli",
		Short: "Run CLI agent-readiness audit",
		RunE: func(cmd *cobra.Command, args []string) error {
			return writeOutput(cmd, opts, "audit cli", vaudit.Run("."))
		},
	})
	return parent
}

func feedbackCommand(opts *globalOptions) *cobra.Command {
	var projectPath, message, category, source, outputPath string
	cmd := &cobra.Command{Use: "feedback", Short: "Record operator feedback", RunE: func(cmd *cobra.Command, args []string) error {
		message = strings.TrimSpace(message)
		if message == "" {
			return writeStructuredError(cmd, opts, verrors.Validation("MISSING_FEEDBACK_MESSAGE", "missing --message", "Pass --message with the operator feedback to record", false))
		}
		category = strings.TrimSpace(category)
		if category == "" {
			category = "operator"
		}
		source = strings.TrimSpace(source)
		if source == "" {
			source = "operator"
		}
		if outputPath == "" {
			outputPath = filepath.Join("reports", "feedback.jsonl")
		}
		artifact := projectRelativePath(projectPath, outputPath)
		now := time.Now().UTC()
		entry := map[string]any{
			"version":    "vflow-feedback/v1",
			"id":         "feedback_" + now.Format("20060102T150405.000000000Z"),
			"created_at": now.Format(time.RFC3339Nano),
			"category":   category,
			"source":     source,
			"message":    message,
		}
		status := "planned"
		if opts.Commit {
			raw, err := json.Marshal(entry)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.External("FEEDBACK_ENCODE_FAILED", err.Error(), "Check feedback fields", false))
			}
			if err := appendJSONLine(artifact, raw); err != nil {
				return writeStructuredError(cmd, opts, verrors.External("FEEDBACK_WRITE_FAILED", err.Error(), "Check --project and --output permissions", false))
			}
			status = "recorded"
		}
		return writeOutput(cmd, opts, "feedback", map[string]any{
			"status":   status,
			"artifact": filepath.ToSlash(artifact),
			"entry":    entry,
			"commit":   opts.Commit,
		})
	}}
	cmd.Flags().StringVar(&projectPath, "project", ".", "project folder")
	cmd.Flags().StringVar(&message, "message", "", "feedback message to record")
	cmd.Flags().StringVar(&category, "category", "operator", "feedback category")
	cmd.Flags().StringVar(&source, "source", "operator", "feedback source")
	cmd.Flags().StringVar(&outputPath, "output", filepath.Join("reports", "feedback.jsonl"), "JSONL output path; relative paths are resolved under --project")
	return cmd
}

func configCommand(opts *globalOptions) *cobra.Command {
	parent := &cobra.Command{Use: "config", Short: "config commands"}
	parent.AddCommand(configInspectCommand(opts), configDefaultsCommand(opts), configSetDefaultsCommand(opts))
	return parent
}

func configInspectCommand(opts *globalOptions) *cobra.Command {
	return &cobra.Command{Use: "inspect", Short: "inspect config", RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := vconfig.Load()
		if err != nil {
			return writeStructuredError(cmd, opts, verrors.External("CONFIG_READ_FAILED", err.Error(), "Check VFLOW_CONFIG_PATH or ~/.vflow/config.yaml", false))
		}
		path, _ := vconfig.Path()
		return writeOutput(cmd, opts, "config inspect", map[string]any{"path": filepath.ToSlash(path), "redacted": true, "config": cfg.Redacted()})
	}}
}

func configDefaultsCommand(opts *globalOptions) *cobra.Command {
	return &cobra.Command{Use: "defaults", Short: "show defaults", RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := vconfig.Load()
		if err != nil {
			return writeStructuredError(cmd, opts, verrors.External("CONFIG_READ_FAILED", err.Error(), "Check VFLOW_CONFIG_PATH or ~/.vflow/config.yaml", false))
		}
		return writeOutput(cmd, opts, "config defaults", cfg.Defaults)
	}}
}

func configSetDefaultsCommand(opts *globalOptions) *cobra.Command {
	var format, projectRoot, dataSource string
	cmd := &cobra.Command{Use: "set-defaults", Short: "set defaults", RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := vconfig.Load()
		if err != nil {
			return writeStructuredError(cmd, opts, verrors.External("CONFIG_READ_FAILED", err.Error(), "Check VFLOW_CONFIG_PATH or ~/.vflow/config.yaml", false))
		}
		if format != "" {
			cfg.Defaults.Format = format
		}
		if projectRoot != "" {
			cfg.Defaults.ProjectRoot = projectRoot
		}
		if dataSource != "" {
			cfg.Defaults.DataSource = dataSource
		}
		status := "planned"
		if opts.Commit {
			if err := vconfig.Save(cfg); err != nil {
				return writeStructuredError(cmd, opts, verrors.External("CONFIG_WRITE_FAILED", err.Error(), "Check config path permissions", false))
			}
			status = "written"
		}
		return writeOutput(cmd, opts, "config set-defaults", map[string]any{"status": status, "defaults": cfg.Defaults})
	}}
	cmd.Flags().StringVar(&format, "output-format", "", "default output format")
	cmd.Flags().StringVar(&projectRoot, "project-root", "", "default project root")
	cmd.Flags().StringVar(&dataSource, "data-source", "", "default data source")
	return cmd
}

func profileCommand(opts *globalOptions) *cobra.Command {
	parent := &cobra.Command{Use: "profile", Short: "profile commands"}
	parent.AddCommand(profileListCommand(opts), profileShowCommand(opts), profileSetCommand(opts), profileUseCommand(opts))
	return parent
}

func profileListCommand(opts *globalOptions) *cobra.Command {
	return &cobra.Command{Use: "list", Short: "profile list", RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := vconfig.Load()
		if err != nil {
			return writeStructuredError(cmd, opts, verrors.External("CONFIG_READ_FAILED", err.Error(), "Check config path", false))
		}
		names := make([]string, 0, len(cfg.Profiles))
		for name := range cfg.Profiles {
			names = append(names, name)
		}
		return writeOutput(cmd, opts, "profile list", map[string]any{"default_profile": cfg.DefaultProfile, "profiles": names})
	}}
}

func profileShowCommand(opts *globalOptions) *cobra.Command {
	return &cobra.Command{Use: "show [name]", Short: "profile show", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := vconfig.Load()
		if err != nil {
			return writeStructuredError(cmd, opts, verrors.External("CONFIG_READ_FAILED", err.Error(), "Check config path", false))
		}
		name := cfg.DefaultProfile
		if len(args) > 0 {
			name = args[0]
		}
		profile, ok := cfg.Redacted().Profiles[name]
		if !ok {
			return writeStructuredError(cmd, opts, verrors.Validation("PROFILE_NOT_FOUND", "profile not found", "Run vflow profile list --format json", false))
		}
		return writeOutput(cmd, opts, "profile show", map[string]any{"name": name, "profile": profile, "secrets_redacted": true})
	}}
}

func profileSetCommand(opts *globalOptions) *cobra.Command {
	var name, provider, apiKeyEnv, apiKey string
	cmd := &cobra.Command{Use: "set", Short: "profile set", RunE: func(cmd *cobra.Command, args []string) error {
		if name == "" || provider == "" {
			return writeStructuredError(cmd, opts, verrors.Validation("MISSING_PROFILE_INPUT", "missing profile name or provider", "Pass --name and --provider", false))
		}
		cfg, err := vconfig.Load()
		if err != nil {
			return writeStructuredError(cmd, opts, verrors.External("CONFIG_READ_FAILED", err.Error(), "Check config path", false))
		}
		profile := cfg.Profiles[name]
		if profile.Providers == nil {
			profile.Providers = map[string]vconfig.Provider{}
		}
		profile.Providers[provider] = vconfig.Provider{APIKeyEnv: apiKeyEnv, APIKey: apiKey}
		cfg.Profiles[name] = profile
		status := "planned"
		if opts.Commit {
			if err := vconfig.Save(cfg); err != nil {
				return writeStructuredError(cmd, opts, verrors.External("CONFIG_WRITE_FAILED", err.Error(), "Check config path permissions", false))
			}
			status = "written"
		}
		return writeOutput(cmd, opts, "profile set", map[string]any{"status": status, "name": name, "provider": provider, "api_key_env": apiKeyEnv, "stored_value_redacted": apiKey != ""})
	}}
	cmd.Flags().StringVar(&name, "name", "", "profile name")
	cmd.Flags().StringVar(&provider, "provider", "", "provider name")
	cmd.Flags().StringVar(&apiKeyEnv, "api-key-env", "", "environment variable containing the key")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "raw API key value; prefer --api-key-env")
	return cmd
}

func profileUseCommand(opts *globalOptions) *cobra.Command {
	var name string
	cmd := &cobra.Command{Use: "use [name]", Short: "profile use", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if name == "" && len(args) > 0 {
			name = args[0]
		}
		if name == "" {
			return writeStructuredError(cmd, opts, verrors.Validation("MISSING_PROFILE", "missing profile name", "Pass vflow profile use NAME", false))
		}
		cfg, err := vconfig.Load()
		if err != nil {
			return writeStructuredError(cmd, opts, verrors.External("CONFIG_READ_FAILED", err.Error(), "Check config path", false))
		}
		if _, ok := cfg.Profiles[name]; !ok {
			return writeStructuredError(cmd, opts, verrors.Validation("PROFILE_NOT_FOUND", "profile not found", "Run vflow profile list --format json", false))
		}
		cfg.DefaultProfile = name
		status := "planned"
		if opts.Commit {
			if err := vconfig.Save(cfg); err != nil {
				return writeStructuredError(cmd, opts, verrors.External("CONFIG_WRITE_FAILED", err.Error(), "Check config path permissions", false))
			}
			status = "written"
		}
		return writeOutput(cmd, opts, "profile use", map[string]any{"status": status, "default_profile": name})
	}}
	cmd.Flags().StringVar(&name, "name", "", "profile name")
	return cmd
}

func authCommand(opts *globalOptions) *cobra.Command {
	parent := &cobra.Command{Use: "auth", Short: "auth commands"}
	var provider, model string
	doctor := &cobra.Command{Use: "doctor", Short: "check provider auth", RunE: func(cmd *cobra.Command, args []string) error {
		if opts.Live && !opts.Commit {
			return writeStructuredError(cmd, opts, verrors.Safety("live auth checks require --commit", "Pass --live --commit to call provider auth/model endpoints"))
		}
		results, err := authDoctorResults(provider, model, opts.Live)
		if err != nil {
			return writeStructuredError(cmd, opts, verrors.Validation("INVALID_ENUM", err.Error(), "Use --provider all, gemini, openai, elevenlabs, soniox, assemblyai, deepgram, gladia, anthropic, huggingface, or local", false))
		}
		status := "checked"
		for _, result := range results {
			if result["status"] == "missing_key" {
				status = "degraded"
				break
			}
		}
		return writeOutput(cmd, opts, "auth doctor", map[string]any{
			"status":           status,
			"live":             opts.Live,
			"provider":         provider,
			"providers":        results,
			"secrets_redacted": true,
		})
	}}
	doctor.Flags().StringVar(&provider, "provider", "all", "provider to check: all, gemini, openai, elevenlabs, soniox, assemblyai, deepgram, gladia, anthropic, huggingface, local")
	doctor.Flags().StringVar(&model, "model", "", "provider model to validate when supported")
	parent.AddCommand(doctor)
	return parent
}

func jobsCommand(opts *globalOptions) *cobra.Command {
	var projectPath string
	parent := &cobra.Command{Use: "jobs", Short: "job ledger commands"}
	parent.PersistentFlags().StringVar(&projectPath, "project", ".", "project path")
	parent.AddCommand(&cobra.Command{Use: "list", Short: "jobs list", RunE: func(cmd *cobra.Command, args []string) error {
		records, err := vjobs.List(projectPath)
		if err != nil {
			return writeStructuredError(cmd, opts, verrors.External("JOBS_LIST_FAILED", err.Error(), "Check project jobs directory", false))
		}
		return writeOutput(cmd, opts, "jobs list", map[string]any{"status": "available", "project": projectPath, "jobs": records})
	}})
	parent.AddCommand(&cobra.Command{Use: "get [job_id]", Short: "jobs get", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		record, err := vjobs.Get(projectPath, args[0])
		if err != nil {
			return writeStructuredError(cmd, opts, verrors.Validation("JOB_NOT_FOUND", err.Error(), "Run vflow jobs list --project PROJECT", false))
		}
		return writeOutput(cmd, opts, "jobs get", record)
	}})
	parent.AddCommand(&cobra.Command{Use: "resume [job_id]", Short: "jobs resume", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		record, err := vjobs.Get(projectPath, args[0])
		if err != nil {
			return writeStructuredError(cmd, opts, verrors.Validation("JOB_NOT_FOUND", err.Error(), "Run vflow jobs list --project PROJECT", false))
		}
		return writeOutput(cmd, opts, "jobs resume", record)
	}})
	return parent
}

func artifactsCommand(opts *globalOptions) *cobra.Command {
	var projectPath string
	parent := &cobra.Command{Use: "artifacts", Aliases: []string{"outputs"}, Short: "artifact commands"}
	list := &cobra.Command{Use: "list", Aliases: []string{"list-artifacts", "outputs"}, Short: "list project artifacts", RunE: func(cmd *cobra.Command, args []string) error {
		var artifacts []string
		_ = filepath.WalkDir(projectPath, func(path string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				if rel, relErr := filepath.Rel(projectPath, path); relErr == nil {
					artifacts = append(artifacts, filepath.ToSlash(rel))
				}
			}
			return nil
		})
		return writeOutput(cmd, opts, "artifacts list", map[string]any{"project": projectPath, "artifacts": artifacts})
	}}
	list.Flags().StringVar(&projectPath, "project", ".", "project path")
	parent.AddCommand(list)
	var input, deliver string
	deliverCmd := &cobra.Command{Use: "deliver", Aliases: []string{"publish-artifacts"}, Short: "deliver artifact", RunE: func(cmd *cobra.Command, args []string) error {
		if input == "" {
			return writeStructuredError(cmd, opts, verrors.Validation("MISSING_INPUT", "missing --input", "Pass --input artifact path", false))
		}
		if deliver == "" {
			deliver = "stdout"
		}
		status := "planned"
		outputPath := ""
		if opts.Commit && strings.HasPrefix(deliver, "file:") {
			outputPath = strings.TrimPrefix(deliver, "file:")
			res, err := output.DeliverFile(input, outputPath, opts.Overwrite)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.External("ARTIFACT_DELIVER_FAILED", err.Error(), "Check artifact and delivery path", false))
			}
			status = res.Status
		} else if opts.Commit && strings.HasPrefix(deliver, "webhook:") {
			outputPath = strings.TrimPrefix(deliver, "webhook:")
			res, err := output.DeliverWebhook(input, outputPath)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.External("ARTIFACT_WEBHOOK_FAILED", err.Error(), "Check webhook URL and receiver status", true))
			}
			return writeOutput(cmd, opts, "artifacts deliver", map[string]any{"status": res.Status, "input": filepath.ToSlash(input), "deliver": deliver, "output": outputPath, "http_status": res.HTTPStatus})
		} else if opts.Commit && deliver == "stdout" {
			status = "available_on_stdout"
		}
		return writeOutput(cmd, opts, "artifacts deliver", map[string]any{"status": status, "input": filepath.ToSlash(input), "deliver": deliver, "output": filepath.ToSlash(outputPath)})
	}}
	deliverCmd.Flags().StringVar(&input, "input", "", "artifact path")
	deliverCmd.Flags().StringVar(&deliver, "deliver", "stdout", "delivery target: stdout, file:<path>, or webhook:<url>")
	parent.AddCommand(deliverCmd)
	return parent
}

func upgradeCommand(opts *globalOptions) *cobra.Command {
	var repo, metadataURL, cacheDir, installDir string
	cmd := &cobra.Command{Use: "upgrade", Short: "Upgrade vflow", RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel, err := commandContext(opts.Timeout)
		if err != nil {
			return writeStructuredError(cmd, opts, verrors.Validation("INVALID_TIMEOUT", err.Error(), "Use a Go duration such as 30s, 5m, or 20m", false))
		}
		defer cancel()
		upgradeOpts := vupdate.Options{Repo: repo, MetadataURL: metadataURL, CacheDir: cacheDir, InstallDir: installDir, Current: Version, Commit: Commit}
		report, err := vupdate.Check(ctx, upgradeOpts)
		if err != nil {
			return writeStructuredError(cmd, opts, verrors.External("UPGRADE_CHECK_FAILED", err.Error(), "Check network access, GitHub release metadata, or --metadata-url", true))
		}
		if opts.Commit {
			report, err = vupdate.Stage(ctx, report, upgradeOpts)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.External("UPGRADE_STAGE_FAILED", err.Error(), "Check release asset URL, cache directory, and network access", true))
			}
		}
		return writeOutput(cmd, opts, "upgrade", report)
	}}
	cmd.Flags().StringVar(&repo, "repo", "github.com/nerveband/vflow", "GitHub repository")
	cmd.Flags().StringVar(&metadataURL, "metadata-url", "", "override latest release metadata URL")
	cmd.Flags().StringVar(&cacheDir, "cache-dir", "", "upgrade asset cache directory")
	cmd.Flags().StringVar(&installDir, "install-dir", "", "install verified binary into this directory after staging")
	return cmd
}

func schemaCommand(opts *globalOptions) *cobra.Command {
	var validate bool
	cmd := &cobra.Command{
		Use:   "schema",
		Short: "Inspect and validate command and artifact schemas",
		RunE: func(cmd *cobra.Command, args []string) error {
			reg := contract.DefaultRegistry()
			status := "available"
			var validationError string
			artifactValidation := map[string]any{"status": "not_checked", "checked": 0}
			if validate {
				status = "valid"
				errs := []string{}
				if err := reg.Validate(); err != nil {
					errs = append(errs, err.Error())
				}
				var artifactErr error
				artifactValidation, artifactErr = validateArtifactSchemas(artifactSchemaNames())
				if artifactErr != nil {
					errs = append(errs, artifactErr.Error())
				}
				if len(errs) > 0 {
					status = "invalid"
					validationError = strings.Join(errs, "; ")
				}
			}
			data := map[string]any{
				"status":                     status,
				"validation_error":           validationError,
				"schema_version":             "vflow-cli-schema/v1",
				"command_count":              len(reg.Commands()),
				"commands":                   reg.Commands(),
				"artifact_schemas":           artifactSchemaNames(),
				"artifact_schema_validation": artifactValidation,
				"coverage_metadata":          map[string]any{"dry_run_checked": true, "commit_checked": true, "examples_checked": true},
				"generated_from":             "internal/contract.DefaultRegistry",
			}
			return writeOutput(cmd, opts, "schema", data)
		},
	}
	cmd.Flags().BoolVar(&validate, "validate", false, "validate live command metadata against contract rules")
	cmd.AddCommand(&cobra.Command{
		Use:   "command [name]",
		Short: "Print a single command contract",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reg := contract.DefaultRegistry()
			c, ok := reg.Get(args[0])
			if !ok {
				return writeStructuredError(cmd, opts, verrors.Validation("NOT_FOUND", "command contract not found", "Run vflow schema --format json to list commands", false))
			}
			return writeOutput(cmd, opts, "schema command", c)
		},
	})
	return cmd
}

func artifactSchemaNames() []string {
	return []string{
		"cli.schema.json",
		"project.schema.json",
		"source-media-review.schema.json",
		"transcript.schema.json",
		"content-edl.schema.json",
		"time-map.schema.json",
		"framing-presets.schema.json",
		"speaker-map.schema.json",
		"framing-policy.schema.json",
		"framing-lane.schema.json",
		"review-queue.schema.json",
		"compiled-timeline.schema.json",
		"gemini-video-qa.schema.json",
		"provider-bakeoff.schema.json",
		"color-grade-report.schema.json",
		"nle-diff.schema.json",
		"nle-sidecar.schema.json",
		"provenance.schema.json",
		"render-report.schema.json",
		"audit-report.schema.json",
		"media-sync-map.schema.json",
		"source-range-manifest.schema.json",
		"transcript-proof.schema.json",
	}
}

func validateArtifactSchemas(names []string) (map[string]any, error) {
	root, err := findSchemaRoot()
	if err != nil {
		return map[string]any{"status": "invalid", "checked": 0, "error": err.Error()}, err
	}
	missing := []string{}
	invalid := []string{}
	for _, name := range names {
		raw, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			missing = append(missing, name)
			continue
		}
		var schema map[string]any
		if err := json.Unmarshal(raw, &schema); err != nil {
			invalid = append(invalid, name+": "+err.Error())
			continue
		}
		if !nonEmptySchemaString(schema, "$schema") || !nonEmptySchemaString(schema, "title") || !nonEmptySchemaString(schema, "type") {
			invalid = append(invalid, name+": missing $schema, title, or type")
		}
	}
	status := "valid"
	var errOut error
	if len(missing) > 0 || len(invalid) > 0 {
		status = "invalid"
		errOut = fmt.Errorf("artifact schema validation failed: missing=%v invalid=%v", missing, invalid)
	}
	return map[string]any{
		"status":  status,
		"root":    filepath.ToSlash(root),
		"checked": len(names),
		"missing": missing,
		"invalid": invalid,
	}, errOut
}

func findSchemaRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := cwd
	for i := 0; i < 8; i++ {
		candidate := filepath.Join(dir, "schemas")
		if stat, err := os.Stat(candidate); err == nil && stat.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("schemas directory not found from %s", cwd)
}

func nonEmptySchemaString(schema map[string]any, key string) bool {
	value, ok := schema[key].(string)
	return ok && strings.TrimSpace(value) != ""
}

func projectCommand(opts *globalOptions) *cobra.Command {
	parent := &cobra.Command{Use: "project", Short: "project workflow commands"}
	parent.AddCommand(projectInitCommand(opts), projectGetCommand(opts), projectListCommand(opts), projectIndexCommand(opts))
	return parent
}

func projectInitCommand(opts *globalOptions) *cobra.Command {
	var path, id string
	cmd := &cobra.Command{
		Use:     "init",
		Aliases: []string{"new-project", "create-project"},
		Short:   "initialize a vflow project folder",
		RunE: func(cmd *cobra.Command, args []string) error {
			if path == "" {
				path = "."
			}
			if id == "" {
				id = "vflow_project"
			}
			res, err := vproject.Init(path, id, opts.Commit)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.Validation("PROJECT_INVALID", err.Error(), "Use a stable project id such as client_event_v1", false))
			}
			return writeOutput(cmd, opts, "project init", res)
		},
	}
	cmd.Flags().StringVar(&path, "path", ".", "project path")
	cmd.Flags().StringVar(&id, "id", "", "project id")
	return cmd
}

func projectGetCommand(opts *globalOptions) *cobra.Command {
	var path string
	cmd := &cobra.Command{
		Use:     "get",
		Aliases: []string{"show", "inspect", "inspect-project"},
		Short:   "read a vflow project contract",
		RunE: func(cmd *cobra.Command, args []string) error {
			if path == "" {
				path = "."
			}
			proj, err := vproject.Load(path)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.Validation("PROJECT_INVALID", err.Error(), "Fix project.json or run project init in a clean folder", false))
			}
			return writeOutput(cmd, opts, "project get", proj)
		},
	}
	cmd.Flags().StringVar(&path, "path", ".", "project path")
	cmd.Flags().StringVar(&path, "project", ".", "project path")
	return cmd
}

func projectListCommand(opts *globalOptions) *cobra.Command {
	var root string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "list vflow projects under a root",
		RunE: func(cmd *cobra.Command, args []string) error {
			projects, err := findProjectContracts(root, opts.MaxDepth)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.External("PROJECT_LIST_FAILED", err.Error(), "Check --root permissions", false))
			}
			return writeOutput(cmd, opts, "project list", map[string]any{"root": root, "count": len(projects), "projects": projects})
		},
	}
	cmd.Flags().StringVar(&root, "root", ".", "search root")
	return cmd
}

func projectIndexCommand(opts *globalOptions) *cobra.Command {
	var root, path, outputPath, indexPath string
	cmd := &cobra.Command{
		Use:   "index",
		Short: "write a project index artifact",
		RunE: func(cmd *cobra.Command, args []string) error {
			if path == "" {
				path = root
			}
			if _, err := os.Stat(filepath.Join(path, "project.json")); err == nil {
				res, err := vindex.IndexProject(cmd.Context(), vindex.Options{ProjectPath: path, DBPath: indexPath, Commit: opts.Commit})
				if err != nil {
					return writeStructuredError(cmd, opts, verrors.External("PROJECT_INDEX_FAILED", err.Error(), "Check --path, --index, and project artifacts", false))
				}
				if outputPath != "" && opts.Commit {
					raw, err := json.MarshalIndent(res, "", "  ")
					if err != nil {
						return err
					}
					if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
						return writeStructuredError(cmd, opts, verrors.External("PROJECT_INDEX_WRITE_FAILED", err.Error(), "Check output path permissions", false))
					}
					if err := os.WriteFile(outputPath, append(raw, '\n'), 0o644); err != nil {
						return writeStructuredError(cmd, opts, verrors.External("PROJECT_INDEX_WRITE_FAILED", err.Error(), "Check output path permissions", false))
					}
				}
				return writeOutput(cmd, opts, "project index", map[string]any{"status": res.Status, "output": filepath.ToSlash(outputPath), "index": res})
			}

			projects, err := findProjectContracts(root, opts.MaxDepth)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.External("PROJECT_INDEX_FAILED", err.Error(), "Check --root permissions", false))
			}
			results := []vindex.Result{}
			for _, project := range projects {
				valid, _ := project["valid"].(bool)
				projectRoot, _ := project["root"].(string)
				if !valid || projectRoot == "" {
					continue
				}
				res, err := vindex.IndexProject(cmd.Context(), vindex.Options{ProjectPath: projectRoot, DBPath: indexPath, Commit: opts.Commit})
				if err != nil {
					return writeStructuredError(cmd, opts, verrors.External("PROJECT_INDEX_FAILED", err.Error(), "Check project artifacts", false))
				}
				results = append(results, res)
			}
			status := "planned"
			if opts.Commit {
				status = "written"
				if outputPath != "" {
					data := map[string]any{"version": vindex.ProjectIndexVersion, "root": root, "count": len(results), "projects": results}
					raw, err := json.MarshalIndent(data, "", "  ")
					if err != nil {
						return err
					}
					if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
						return writeStructuredError(cmd, opts, verrors.External("PROJECT_INDEX_WRITE_FAILED", err.Error(), "Check output path permissions", false))
					}
					if err := os.WriteFile(outputPath, append(raw, '\n'), 0o644); err != nil {
						return writeStructuredError(cmd, opts, verrors.External("PROJECT_INDEX_WRITE_FAILED", err.Error(), "Check output path permissions", false))
					}
				}
			}
			return writeOutput(cmd, opts, "project index", map[string]any{"status": status, "output": filepath.ToSlash(outputPath), "count": len(results), "projects": results})
		},
	}
	cmd.Flags().StringVar(&root, "root", ".", "search root")
	cmd.Flags().StringVar(&path, "path", "", "single project path to index")
	cmd.Flags().StringVar(&outputPath, "output", "", "index output path")
	cmd.Flags().StringVar(&indexPath, "index", "", "SQLite index path, default $VFLOW_INDEX_PATH or ~/.vflow/index.sqlite")
	return cmd
}

func mediaCommand(opts *globalOptions) *cobra.Command {
	parent := &cobra.Command{Use: "media", Short: "media workflow commands"}
	parent.AddCommand(mediaProbeCommand(opts), mediaIngestCommand(opts), mediaProxyCommand(opts), mediaSamplesCommand(opts), mediaSyncCommand(opts), mediaExtractRangesCommand(opts))
	return parent
}

func mediaSyncCommand(opts *globalOptions) *cobra.Command {
	var projectPath, outputPath, proofDir, reference, sourcesCSV, ffmpegPath, frameRate string
	var maxLag float64
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "calibrate source-camera sync from audio waveforms",
		RunE: func(cmd *cobra.Command, args []string) error {
			if reference == "" {
				return writeStructuredError(cmd, opts, verrors.Validation("MISSING_REFERENCE", "missing --reference-source-id", "Pass the source id to use as timeline reference", false))
			}
			sources, err := parseSourceInputs(sourcesCSV)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.Validation("SOURCE_LIST_INVALID", err.Error(), "Use id=/path/video.mp4 pairs separated by commas", false))
			}
			if outputPath == "" {
				outputPath = filepath.Join(projectPath, "calibration", "media-sync-map.json")
			} else if !filepath.IsAbs(outputPath) {
				outputPath = filepath.Join(projectPath, outputPath)
			}
			if proofDir == "" {
				proofDir = filepath.Join(projectPath, "calibration", "sync-proof")
			} else if !filepath.IsAbs(proofDir) {
				proofDir = filepath.Join(projectPath, proofDir)
			}
			ctx, cancel, err := commandContext(opts.Timeout)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.Validation("INVALID_TIMEOUT", err.Error(), "Use a Go duration such as 30s, 5m, or 20m", false))
			}
			defer cancel()
			report, err := vsyncmap.Calibrate(ctx, vsyncmap.CalibrationOptions{
				ProjectID: projectIDFromPath(projectPath), ReferenceSourceID: reference, Sources: sources,
				OutputPath: outputPath, ProofDir: proofDir, FFmpegPath: ffmpegPath, MaxLagSeconds: maxLag,
				FrameRate: frameRate, Commit: opts.Commit,
			})
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.External("MEDIA_SYNC_FAILED", err.Error(), "Check ffmpeg, source paths, audio streams, and timeout", true))
			}
			return writeOutput(cmd, opts, "media sync", report)
		},
	}
	cmd.Flags().StringVar(&projectPath, "project", ".", "project path")
	cmd.Flags().StringVar(&outputPath, "output", "", "sync map output path")
	cmd.Flags().StringVar(&proofDir, "proof-dir", "", "waveform proof directory")
	cmd.Flags().StringVar(&reference, "reference-source-id", "", "reference source id")
	cmd.Flags().StringVar(&sourcesCSV, "sources", "", "comma-separated id=path source list")
	cmd.Flags().StringVar(&ffmpegPath, "ffmpeg-path", "", "ffmpeg binary path")
	cmd.Flags().StringVar(&frameRate, "frame-rate", "24000/1001", "canonical frame rate")
	cmd.Flags().Float64Var(&maxLag, "max-lag-seconds", 90, "maximum correlation lag search")
	return cmd
}

func mediaExtractRangesCommand(opts *globalOptions) *cobra.Command {
	var projectPath, syncMapPath, rangesPath, outputDir, manifestPath, ffmpegPath string
	cmd := &cobra.Command{
		Use:   "extract-ranges",
		Short: "plan or extract transcript-mapped local source ranges",
		RunE: func(cmd *cobra.Command, args []string) error {
			if syncMapPath == "" {
				syncMapPath = filepath.Join(projectPath, "calibration", "media-sync-map.json")
			} else if !filepath.IsAbs(syncMapPath) {
				syncMapPath = projectRelativePath(projectPath, syncMapPath)
			}
			if rangesPath == "" {
				return writeStructuredError(cmd, opts, verrors.Validation("MISSING_RANGES", "missing --ranges", "Pass JSON ranges with source_id and transcript seconds", false))
			}
			if !filepath.IsAbs(rangesPath) {
				rangesPath = projectRelativePath(projectPath, rangesPath)
			}
			m, err := vsyncmap.Read(syncMapPath)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.External("SYNC_MAP_READ_FAILED", err.Error(), "Run media sync/transcript sync first", false))
			}
			ranges, err := vmedia.ReadTranscriptRanges(rangesPath)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.Validation("RANGES_INVALID", err.Error(), "Pass a JSON array or object with ranges", false))
			}
			if outputDir == "" {
				outputDir = filepath.Join(projectPath, "media", "ranges")
			} else if !filepath.IsAbs(outputDir) {
				outputDir = projectRelativePath(projectPath, outputDir)
			}
			manifest, err := vmedia.PlanSourceRanges(m, ranges, outputDir, ffmpegPath)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.Validation("RANGE_PLAN_INVALID", err.Error(), "Check source ids and transcript seconds", false))
			}
			manifest.SyncMap = filepath.ToSlash(syncMapPath)
			if manifestPath == "" {
				manifestPath = filepath.Join(projectPath, "calibration", "source-range-manifest.json")
			} else if !filepath.IsAbs(manifestPath) {
				manifestPath = projectRelativePath(projectPath, manifestPath)
			}
			status := "planned"
			if opts.Commit {
				ctx, cancel, err := commandContext(opts.Timeout)
				if err != nil {
					return writeStructuredError(cmd, opts, verrors.Validation("INVALID_TIMEOUT", err.Error(), "Use a Go duration such as 30s, 5m, or 20m", false))
				}
				defer cancel()
				if err := vmedia.RunSourceRanges(ctx, manifest); err != nil {
					return writeStructuredError(cmd, opts, verrors.External("RANGE_EXTRACT_FAILED", err.Error(), "Check ffmpeg, source media, destination space, and timeout", true))
				}
				if err := writeJSONFile(manifestPath, manifest); err != nil {
					return writeStructuredError(cmd, opts, verrors.External("RANGE_MANIFEST_WRITE_FAILED", err.Error(), "Check project write permissions", false))
				}
				status = "extracted"
			}
			return writeOutput(cmd, opts, "media extract-ranges", map[string]any{"status": status, "manifest_path": filepath.ToSlash(manifestPath), "manifest": manifest})
		},
	}
	cmd.Flags().StringVar(&projectPath, "project", ".", "project path")
	cmd.Flags().StringVar(&syncMapPath, "sync-map", "", "vflow-media-sync-map/v1 path")
	cmd.Flags().StringVar(&rangesPath, "ranges", "", "transcript range JSON")
	cmd.Flags().StringVar(&outputDir, "output-dir", "", "range clip output directory")
	cmd.Flags().StringVar(&manifestPath, "manifest", "", "source range manifest output path")
	cmd.Flags().StringVar(&ffmpegPath, "ffmpeg-path", "", "ffmpeg binary path")
	return cmd
}

func mediaProbeCommand(opts *globalOptions) *cobra.Command {
	var projectPath, source, probeJSON, ffprobePath string
	cmd := &cobra.Command{
		Use:     "probe",
		Aliases: []string{"inspect-media", "analyze-media", "metadata"},
		Short:   "probe source media with ffprobe",
		RunE: func(cmd *cobra.Command, args []string) error {
			if projectPath == "" {
				projectPath = "."
			}
			var reviews []vmedia.SourceReview
			if probeJSON != "" {
				raw, err := os.ReadFile(probeJSON)
				if err != nil {
					return writeStructuredError(cmd, opts, verrors.External("FFPROBE_JSON_READ_FAILED", err.Error(), "Check --ffprobe-json path", false))
				}
				review, err := vmedia.ParseFFProbe(raw, firstNonEmptyString(source, "media/source.mp4"))
				if err != nil {
					return writeStructuredError(cmd, opts, verrors.Validation("FFPROBE_JSON_INVALID", err.Error(), "Pass valid ffprobe JSON", false))
				}
				reviews = append(reviews, review)
			} else {
				sources, err := discoverMediaSources(projectPath, source)
				if err != nil {
					return writeStructuredError(cmd, opts, verrors.Validation("MEDIA_SOURCE_NOT_FOUND", err.Error(), "Pass --source, or place source files under media/source-4k", false))
				}
				for _, src := range sources {
					review, err := vmedia.ProbeFile(ffprobePath, src)
					if err != nil {
						return writeStructuredError(cmd, opts, verrors.External("FFPROBE_FAILED", err.Error(), "Check ffprobe, source path, and media codec support", false))
					}
					reviews = append(reviews, review)
				}
			}
			data := map[string]any{
				"status":       "planned",
				"project":      projectPath,
				"sources":      reviews,
				"review_path":  filepath.ToSlash(filepath.Join(projectPath, "source-media-review.json")),
				"write_commit": opts.Commit,
			}
			if len(reviews) == 1 {
				data["source"] = reviews[0]
			}
			if opts.Commit {
				if err := vmedia.WriteReviews(projectPath, reviews); err != nil {
					return writeStructuredError(cmd, opts, verrors.External("SOURCE_REVIEW_WRITE_FAILED", err.Error(), "Check project write permissions", false))
				}
				data["status"] = "written"
			}
			return writeOutput(cmd, opts, "media probe", data)
		},
	}
	cmd.Flags().StringVar(&projectPath, "project", ".", "project path")
	cmd.Flags().StringVar(&source, "source", "", "source media path")
	cmd.Flags().StringVar(&probeJSON, "ffprobe-json", "", "read ffprobe JSON from file instead of executing ffprobe")
	cmd.Flags().StringVar(&ffprobePath, "ffprobe-path", "", "ffprobe binary path")
	return cmd
}

func mediaIngestCommand(opts *globalOptions) *cobra.Command {
	var projectPath, source string
	var copySource bool
	cmd := &cobra.Command{
		Use:     "ingest",
		Aliases: []string{"add-media", "import-media"},
		Short:   "ingest media into a project",
		RunE: func(cmd *cobra.Command, args []string) error {
			if source == "" {
				return writeStructuredError(cmd, opts, verrors.Validation("MISSING_SOURCE", "missing --source", "Pass --source /path/to/media", false))
			}
			dest := filepath.Join(projectPath, "media", filepath.Base(source))
			data := map[string]any{
				"status":      "planned",
				"source":      source,
				"destination": dest,
				"copy":        copySource,
			}
			if opts.Commit {
				if !copySource {
					return writeStructuredError(cmd, opts, verrors.Validation("COPY_REQUIRED", "ingest currently requires --copy", "Pass --copy --commit", false))
				}
				if _, err := os.Stat(dest); err == nil && !opts.Overwrite {
					return writeStructuredError(cmd, opts, verrors.Safety("destination exists", "Pass --overwrite --commit to replace it"))
				}
				if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
					return err
				}
				if err := copyFile(source, dest); err != nil {
					return err
				}
				data["status"] = "copied"
			}
			return writeOutput(cmd, opts, "media ingest", data)
		},
	}
	cmd.Flags().StringVar(&projectPath, "project", ".", "project path")
	cmd.Flags().StringVar(&source, "source", "", "source media path")
	cmd.Flags().BoolVar(&copySource, "copy", false, "copy source into project media directory")
	return cmd
}

func mediaProxyCommand(opts *globalOptions) *cobra.Command {
	var projectPath, preset, source, ffmpegPath string
	cmd := &cobra.Command{
		Use:     "proxy",
		Aliases: []string{"make-proxy", "create-proxy", "transcode-proxy"},
		Short:   "create a proxy render plan",
		RunE: func(cmd *cobra.Command, args []string) error {
			if source == "" {
				source = filepath.Join(projectPath, "media", "source.mp4")
			}
			proxyOpts := vmedia.ProxyOptions{
				FFmpegPath: ffmpegPath,
				SourcePath: source,
				OutputPath: filepath.Join(projectPath, "media", "proxy.mp4"),
				Preset:     preset,
				Overwrite:  opts.Overwrite,
			}
			plan := vmedia.BuildProxyPlan(proxyOpts)
			if opts.Commit {
				ctx, cancel, err := commandContext(opts.Timeout)
				if err != nil {
					return writeStructuredError(cmd, opts, verrors.Validation("INVALID_TIMEOUT", err.Error(), "Use a Go duration such as 30s, 5m, or 20m", false))
				}
				defer cancel()
				plan, err = vmedia.RunProxy(ctx, proxyOpts)
				if err != nil {
					return writeStructuredError(cmd, opts, verrors.External("MEDIA_PROXY_FAILED", err.Error(), "Check ffmpeg, source path, and output permissions", true))
				}
			}
			return writeOutput(cmd, opts, "media proxy", plan)
		},
	}
	cmd.Flags().StringVar(&projectPath, "project", ".", "project path")
	cmd.Flags().StringVar(&preset, "preset", "edit-1080p", "proxy preset")
	cmd.Flags().StringVar(&source, "source", "", "source media path")
	cmd.Flags().StringVar(&ffmpegPath, "ffmpeg-path", "", "ffmpeg binary path")
	return cmd
}

func mediaSamplesCommand(opts *globalOptions) *cobra.Command {
	var projectPath, deliver, source, ffmpegPath string
	var count int
	cmd := &cobra.Command{
		Use:   "samples",
		Short: "plan representative frame extraction",
		RunE: func(cmd *cobra.Command, args []string) error {
			if source == "" {
				source = filepath.Join(projectPath, "media", "source.mp4")
			}
			outputPath := firstNonEmptyString(deliver, filepath.Join(projectPath, "reports", "contact-sheet.jpg"))
			if strings.HasPrefix(outputPath, "file:") {
				outputPath = strings.TrimPrefix(outputPath, "file:")
			}
			sampleOpts := vmedia.SampleOptions{
				FFmpegPath: ffmpegPath,
				SourcePath: source,
				OutputPath: outputPath,
				Count:      count,
				Overwrite:  opts.Overwrite,
			}
			plan := vmedia.BuildSamplePlan(sampleOpts)
			if opts.Commit {
				ctx, cancel, err := commandContext(opts.Timeout)
				if err != nil {
					return writeStructuredError(cmd, opts, verrors.Validation("INVALID_TIMEOUT", err.Error(), "Use a Go duration such as 30s, 5m, or 20m", false))
				}
				defer cancel()
				plan, err = vmedia.RunSamples(ctx, sampleOpts)
				if err != nil {
					return writeStructuredError(cmd, opts, verrors.External("MEDIA_SAMPLES_FAILED", err.Error(), "Check ffmpeg, source path, and output permissions", true))
				}
			}
			return writeOutput(cmd, opts, "media samples", plan)
		},
	}
	cmd.Flags().StringVar(&projectPath, "project", ".", "project path")
	cmd.Flags().IntVar(&count, "count", 12, "number of frames")
	cmd.Flags().StringVar(&deliver, "deliver", "", "delivery target")
	cmd.Flags().StringVar(&source, "source", "", "source media path")
	cmd.Flags().StringVar(&ffmpegPath, "ffmpeg-path", "", "ffmpeg binary path")
	return cmd
}

func cleanupCommand(opts *globalOptions) *cobra.Command {
	parent := &cobra.Command{Use: "cleanup", Short: "cleanup workflow commands"}
	parent.AddCommand(cleanupApplyCommand(opts), cleanupSuggestCommand(opts), cleanupReviewCommand(opts))
	return parent
}

func cleanupSuggestCommand(opts *globalOptions) *cobra.Command {
	var projectPath string
	cmd := &cobra.Command{
		Use:     "suggest",
		Aliases: []string{"cleanup-plan", "suggest-cleanup"},
		Short:   "suggest cleanup decisions from transcript words",
		RunE: func(cmd *cobra.Command, args []string) error {
			words, err := vtranscript.ReadWords(projectPath)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.External("TRANSCRIPT_READ_FAILED", err.Error(), "Create or import transcript/words.json first", false))
			}
			type suggested struct {
				Start      float64 `json:"start"`
				End        float64 `json:"end"`
				Reason     string  `json:"reason"`
				Confidence float64 `json:"confidence"`
			}
			suggestions := []suggested{}
			for _, word := range words.Words {
				text := strings.ToLower(word.Text)
				if text == "um" || text == "uh" || text == "erm" {
					suggestions = append(suggestions, suggested{Start: float64(word.StartFrame) / 30.0, End: float64(word.EndFrame) / 30.0, Reason: "filler", Confidence: 0.72})
				}
			}
			status := "suggested"
			artifact := filepath.Join(projectPath, "decisions", "delete_segments.proposed.json")
			if opts.Commit {
				if err := os.MkdirAll(filepath.Dir(artifact), 0o755); err != nil {
					return writeStructuredError(cmd, opts, verrors.External("CLEANUP_SUGGEST_WRITE_FAILED", err.Error(), "Check project write permissions", false))
				}
				raw, _ := json.MarshalIndent(suggestions, "", "  ")
				if err := os.WriteFile(artifact, append(raw, '\n'), 0o644); err != nil {
					return writeStructuredError(cmd, opts, verrors.External("CLEANUP_SUGGEST_WRITE_FAILED", err.Error(), "Check project write permissions", false))
				}
				status = "written"
			}
			return writeOutput(cmd, opts, "cleanup suggest", map[string]any{"status": status, "suggestion_count": len(suggestions), "artifact": filepath.ToSlash(artifact), "suggestions": suggestions})
		},
	}
	cmd.Flags().StringVar(&projectPath, "project", ".", "project path")
	return cmd
}

func cleanupReviewCommand(opts *globalOptions) *cobra.Command {
	var projectPath, deliver string
	cmd := &cobra.Command{
		Use:     "review",
		Aliases: []string{"review-cleanup"},
		Short:   "review cleanup decisions",
		RunE: func(cmd *cobra.Command, args []string) error {
			edl, err := vcleanup.ReadContentEDL(projectPath)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.External("CONTENT_EDL_READ_FAILED", err.Error(), "Run cleanup apply --commit first", false))
			}
			data := map[string]any{
				"status":       "reviewed",
				"delete_count": len(edl.DeleteSegments),
				"rate":         edl.Rate,
				"needs_review": []any{},
			}
			if opts.Commit && strings.HasPrefix(deliver, "file:") {
				path := strings.TrimPrefix(deliver, "file:")
				if err := writeCleanupReviewHTML(path, edl); err != nil {
					return writeStructuredError(cmd, opts, verrors.External("CLEANUP_REVIEW_WRITE_FAILED", err.Error(), "Check --deliver path", false))
				}
				data["artifact"] = filepath.ToSlash(path)
			}
			return writeOutput(cmd, opts, "cleanup review", data)
		},
	}
	cmd.Flags().StringVar(&projectPath, "project", ".", "project path")
	cmd.Flags().StringVar(&deliver, "deliver", "", "delivery target, e.g. file:review/cleanup-review.html")
	return cmd
}

func writeCleanupReviewHTML(path string, edl vcleanup.ContentEDL) error {
	var b strings.Builder
	b.WriteString("<!doctype html><meta charset=\"utf-8\"><title>vflow cleanup review</title><main>")
	b.WriteString("<h1>Cleanup Review</h1><table><thead><tr><th>ID</th><th>Frames</th><th>Reason</th><th>Confidence</th></tr></thead><tbody>")
	for _, del := range edl.DeleteSegments {
		b.WriteString("<tr><td>")
		b.WriteString(del.ID)
		b.WriteString("</td><td>")
		b.WriteString(fmt.Sprintf("%d-%d", del.StartFrame, del.EndFrame))
		b.WriteString("</td><td>")
		b.WriteString(del.Reason)
		b.WriteString("</td><td>")
		b.WriteString(fmt.Sprintf("%.2f", del.Confidence))
		b.WriteString("</td></tr>")
	}
	b.WriteString("</tbody></table></main>")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func cleanupApplyCommand(opts *globalOptions) *cobra.Command {
	var projectPath, input, rate string
	cmd := &cobra.Command{
		Use:     "apply",
		Aliases: []string{"apply-cleanup"},
		Short:   "apply accepted cleanup decisions to content-edl.json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if input == "" {
				return writeStructuredError(cmd, opts, verrors.Validation("MISSING_INPUT", "missing --input", "Pass --input delete_segments.json", false))
			}
			raw, err := os.ReadFile(input)
			if err != nil {
				return err
			}
			edl, err := vcleanup.ImportDeleteSegments(raw, rate)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.Validation("CONTENT_EDL_INVALID", err.Error(), "Fix delete segment ranges, confidence values, and input shape", false))
			}
			status := "planned"
			if opts.Commit {
				if err := vcleanup.WriteContentEDL(projectPath, edl); err != nil {
					return err
				}
				status = "written"
			}
			return writeOutput(cmd, opts, "cleanup apply", map[string]any{
				"status":       status,
				"delete_count": len(edl.DeleteSegments),
				"artifact":     filepath.ToSlash(filepath.Join(projectPath, "decisions", "content-edl.json")),
				"content_edl":  edl,
			})
		},
	}
	cmd.Flags().StringVar(&projectPath, "project", ".", "project path")
	cmd.Flags().StringVar(&input, "input", "", "input delete_segments.json")
	cmd.Flags().StringVar(&rate, "rate", "30000/1001", "frame rate")
	return cmd
}

func cutCommand(opts *globalOptions) *cobra.Command {
	parent := &cobra.Command{Use: "cut", Short: "cut decision workflow commands"}
	parent.AddCommand(cutCreateCommand(opts))
	return parent
}

func cutCreateCommand(opts *globalOptions) *cobra.Command {
	var projectPath, rangesPath, syncMapPath, outputPath string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "create a transcript cut decision from transcript ranges",
		RunE: func(cmd *cobra.Command, args []string) error {
			if rangesPath == "" {
				return writeStructuredError(cmd, opts, verrors.Validation("MISSING_RANGES", "missing --ranges", "Pass ranges JSON with source_id and transcript seconds", false))
			}
			if !filepath.IsAbs(rangesPath) {
				rangesPath = projectRelativePath(projectPath, rangesPath)
			}
			ranges, err := vmedia.ReadTranscriptRanges(rangesPath)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.Validation("RANGES_INVALID", err.Error(), "Pass a JSON array or object with ranges", false))
			}
			var m vsyncmap.SyncMap
			if syncMapPath != "" {
				if !filepath.IsAbs(syncMapPath) {
					syncMapPath = projectRelativePath(projectPath, syncMapPath)
				}
				m, err = vsyncmap.Read(syncMapPath)
				if err != nil {
					return writeStructuredError(cmd, opts, verrors.External("SYNC_MAP_READ_FAILED", err.Error(), "Check --sync-map path", false))
				}
			}
			cut := vrender.TranscriptCut{Version: "vflow-transcript-cut/v1"}
			for i, r := range ranges {
				id := r.ID
				if id == "" {
					id = fmt.Sprintf("cut_%06d", i+1)
				}
				seg := vrender.TranscriptCutSegment{
					ID: r.ID, SourceID: r.SourceID, TranscriptStartSeconds: r.Start, TranscriptEndSeconds: r.End,
					StartSeconds: r.Start, EndSeconds: r.End, Text: r.Text,
				}
				seg.ID = id
				if syncMapPath != "" {
					source, ok := m.Source(r.SourceID)
					if ok {
						seg.Source = source.Path
					}
					start, end, err := m.ResolveRange(r.SourceID, r.Start, r.End)
					if err != nil {
						return writeStructuredError(cmd, opts, verrors.Validation("SYNC_RANGE_INVALID", err.Error(), "Check source ids and transcript seconds", false))
					}
					seg.StartSeconds, seg.EndSeconds = start, end
					seg.SourceTimelineOffset = start - r.Start
					seg.ReferenceStartSeconds = m.ReferenceSeconds(r.Start)
					seg.ReferenceEndSeconds = m.ReferenceSeconds(r.End)
				}
				cut.Segments = append(cut.Segments, seg)
			}
			if outputPath == "" {
				outputPath = filepath.Join(projectPath, "decisions", "transcript-cut.json")
			} else if !filepath.IsAbs(outputPath) {
				outputPath = projectRelativePath(projectPath, outputPath)
			}
			status := "planned"
			if opts.Commit {
				if err := writeJSONFile(outputPath, cut); err != nil {
					return writeStructuredError(cmd, opts, verrors.External("CUT_WRITE_FAILED", err.Error(), "Check project write permissions", false))
				}
				status = "written"
			}
			return writeOutput(cmd, opts, "cut create", map[string]any{"status": status, "artifact": filepath.ToSlash(outputPath), "cut": cut})
		},
	}
	cmd.Flags().StringVar(&projectPath, "project", ".", "project path")
	cmd.Flags().StringVar(&rangesPath, "ranges", "", "transcript range JSON")
	cmd.Flags().StringVar(&syncMapPath, "sync-map", "", "optional sync map path")
	cmd.Flags().StringVar(&outputPath, "output", "", "transcript cut output path")
	return cmd
}

func framingCommand(opts *globalOptions) *cobra.Command {
	parent := &cobra.Command{Use: "framing", Short: "framing workflow commands"}
	preset := &cobra.Command{Use: "preset", Aliases: []string{"presets"}, Short: "framing preset commands"}
	preset.AddCommand(framingPresetImportCommand(opts), framingPresetValidateCommand(opts), framingPresetListCommand(opts))
	parent.AddCommand(preset)
	parent.AddCommand(framingCalibrateCommand(opts), framingMapSpeakersCommand(opts), framingProposeCommand(opts), framingCompileCommand(opts), framingReviewCommand(opts))
	return parent
}

func framingCalibrateCommand(opts *globalOptions) *cobra.Command {
	var projectPath, source, listen, sessionTimeout, shutdownToken string
	var openBrowser bool
	wait := true
	cmd := &cobra.Command{
		Use:     "calibrate",
		Aliases: []string{"crop", "zoom", "reframe", "frame", "crop-calibrate", "zoom-calibrate", "preset-calibrate"},
		Short:   "start a local crop/zoom framing calibration session",
		RunE: func(cmd *cobra.Command, args []string) error {
			timeout, err := time.ParseDuration(sessionTimeout)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.Validation("CALIBRATE_TIMEOUT_INVALID", err.Error(), "Use a Go duration such as 30m", false))
			}
			ctx := cmd.Context()
			server, result, verr := vframingsession.Start(ctx, vframingsession.Options{
				ProjectPath:   projectPath,
				Source:        source,
				Listen:        listen,
				Open:          openBrowser,
				Wait:          false,
				Timeout:       timeout,
				ShutdownToken: shutdownToken,
				CommitEnabled: opts.Commit,
			})
			if verr != nil {
				return writeStructuredError(cmd, opts, verr)
			}
			if err := writeOutput(cmd, opts, "framing calibrate", result); err != nil {
				_ = server.Shutdown(context.Background())
				return err
			}
			if wait {
				select {
				case <-server.Done():
				case <-time.After(timeout):
					_ = server.Shutdown(context.Background())
				case <-ctx.Done():
					_ = server.Shutdown(context.Background())
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&projectPath, "project", ".", "project path")
	cmd.Flags().StringVar(&source, "source", "", "project-relative source media path")
	cmd.Flags().StringVar(&listen, "listen", "127.0.0.1:0", "local listen address")
	cmd.Flags().BoolVar(&openBrowser, "open", false, "open the calibration UI in a browser")
	cmd.Flags().BoolVar(&wait, "wait", true, "wait until shutdown or timeout")
	cmd.Flags().StringVar(&sessionTimeout, "session-timeout", "30m", "session timeout")
	cmd.Flags().StringVar(&shutdownToken, "shutdown-token", "", "optional token required by POST /api/shutdown")
	return cmd
}

func framingMapSpeakersCommand(opts *globalOptions) *cobra.Command {
	var projectPath string
	cmd := &cobra.Command{
		Use:     "map-speakers",
		Aliases: []string{"speaker-map", "map-speaker", "assign-speakers"},
		Short:   "map transcript speaker labels to stable framing presets",
		RunE: func(cmd *cobra.Command, args []string) error {
			words, err := vtranscript.ReadWords(projectPath)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.External("TRANSCRIPT_READ_FAILED", err.Error(), "Create or import transcript/words.json first", false))
			}
			presets, err := vframing.ReadPresets(projectPath)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.External("FRAMING_PRESET_READ_FAILED", err.Error(), "Import framing presets first", false))
			}
			presetID := ""
			if len(presets.Presets) > 0 {
				presetID = presets.Presets[0].ID
			}
			mapping := map[string]string{}
			for _, word := range words.Words {
				if word.SpeakerLabel != "" {
					mapping[word.SpeakerLabel] = presetID
				}
			}
			data := map[string]any{"version": "vflow-speaker-map/v1", "status": "mapped", "speaker_count": len(mapping), "map": mapping}
			if opts.Commit {
				path := filepath.Join(projectPath, "calibration", "speaker-map.json")
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					return writeStructuredError(cmd, opts, verrors.External("SPEAKER_MAP_WRITE_FAILED", err.Error(), "Check project write permissions", false))
				}
				raw, _ := json.MarshalIndent(data, "", "  ")
				if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
					return writeStructuredError(cmd, opts, verrors.External("SPEAKER_MAP_WRITE_FAILED", err.Error(), "Check project write permissions", false))
				}
				data["artifact"] = filepath.ToSlash(path)
			}
			return writeOutput(cmd, opts, "framing map-speakers", data)
		},
	}
	cmd.Flags().StringVar(&projectPath, "project", ".", "project path")
	return cmd
}

func framingProposeCommand(opts *globalOptions) *cobra.Command {
	var projectPath string
	cmd := &cobra.Command{
		Use:   "propose",
		Short: "propose a framing lane from presets",
		RunE: func(cmd *cobra.Command, args []string) error {
			presets, err := vframing.ReadPresets(projectPath)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.External("FRAMING_PRESET_READ_FAILED", err.Error(), "Import framing presets first", false))
			}
			presetID := "wide"
			if len(presets.Presets) > 0 {
				presetID = presets.Presets[0].ID
			}
			lane := map[string]any{
				"version": "vflow-framing-lane/v1",
				"events": []map[string]any{{
					"id":          "fr_000001",
					"start_frame": 0,
					"end_frame":   0,
					"preset_id":   presetID,
					"reason":      "default framing proposal",
				}},
			}
			status := "proposed"
			path := filepath.Join(projectPath, "decisions", "framing-lane.proposed.json")
			if opts.Commit {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					return writeStructuredError(cmd, opts, verrors.External("FRAMING_PROPOSAL_WRITE_FAILED", err.Error(), "Check project write permissions", false))
				}
				raw, _ := json.MarshalIndent(lane, "", "  ")
				if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
					return writeStructuredError(cmd, opts, verrors.External("FRAMING_PROPOSAL_WRITE_FAILED", err.Error(), "Check project write permissions", false))
				}
				status = "written"
			}
			return writeOutput(cmd, opts, "framing propose", map[string]any{"status": status, "artifact": filepath.ToSlash(path), "lane": lane})
		},
	}
	cmd.Flags().StringVar(&projectPath, "project", ".", "project path")
	return cmd
}

func framingCompileCommand(opts *globalOptions) *cobra.Command {
	var projectPath, input string
	cmd := &cobra.Command{
		Use:     "compile",
		Aliases: []string{"apply-framing", "compile-framing", "build-framing"},
		Short:   "compile an approved framing lane",
		RunE: func(cmd *cobra.Command, args []string) error {
			if input != "" {
				return writeStructuredError(cmd, opts, verrors.Validation("FRAMING_INPUT_UNSUPPORTED", "--input is not used by the contract compiler", "Use calibration/framing-presets.json, calibration/speaker-map.json, policy/framing-policy.json, and transcript/words.json", false))
			}
			presets, err := vframing.ReadPresets(projectPath)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.External("FRAMING_PRESET_READ_FAILED", err.Error(), "Import framing presets first", false))
			}
			speakerMap, err := vframing.ReadSpeakerMap(projectPath)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.External("SPEAKER_MAP_READ_FAILED", err.Error(), "Create calibration/speaker-map.json first", false))
			}
			policy, err := vframing.ReadPolicy(projectPath)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.External("FRAMING_POLICY_READ_FAILED", err.Error(), "Fix policy/framing-policy.json or remove it to use defaults", false))
			}
			words, err := vtranscript.ReadWords(projectPath)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.External("TRANSCRIPT_READ_FAILED", err.Error(), "Create or import transcript/words.json first", false))
			}
			compiled, err := vframing.CompileLane(vframing.CompileInput{
				Presets:    presets,
				SpeakerMap: speakerMap,
				Policy:     policy,
				Words:      words,
			})
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.Validation("FRAMING_CONTRACT_INVALID", err.Error(), "Fix presets, speaker-map, policy, or transcript frame ranges", false))
			}
			status := "planned"
			artifacts := []string{
				filepath.ToSlash(filepath.Join(projectPath, "decisions", "framing-lane.json")),
				filepath.ToSlash(filepath.Join(projectPath, "review", "review-queue.json")),
			}
			if opts.Commit {
				if err := vframing.WriteCompiled(projectPath, compiled); err != nil {
					return writeStructuredError(cmd, opts, verrors.External("FRAMING_LANE_WRITE_FAILED", err.Error(), "Check project write permissions", false))
				}
				status = "written"
			}
			return writeOutput(cmd, opts, "framing compile", map[string]any{"status": status, "artifacts": artifacts, "lane": compiled.Lane, "review_queue": compiled.ReviewQueue})
		},
	}
	cmd.Flags().StringVar(&projectPath, "project", ".", "project path")
	cmd.Flags().StringVar(&input, "input", "", "deprecated framing lane input")
	return cmd
}

func framingReviewCommand(opts *globalOptions) *cobra.Command {
	var projectPath string
	cmd := &cobra.Command{
		Use:   "review",
		Short: "review compiled framing lane",
		RunE: func(cmd *cobra.Command, args []string) error {
			lanePath := filepath.Join(projectPath, "decisions", "framing-lane.json")
			raw, err := os.ReadFile(lanePath)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.External("FRAMING_LANE_READ_FAILED", err.Error(), "Run framing compile --commit first", false))
			}
			var lane vframing.Lane
			if err := json.Unmarshal(raw, &lane); err != nil {
				return writeStructuredError(cmd, opts, verrors.Validation("FRAMING_LANE_INVALID", err.Error(), "Check compiled framing lane JSON", false))
			}
			reviewQueue := vframing.ReviewQueue{Version: "vflow-review-queue/v1", Items: []vframing.ReviewItem{}}
			if reviewRaw, err := os.ReadFile(filepath.Join(projectPath, "review", "review-queue.json")); err == nil {
				if err := json.Unmarshal(reviewRaw, &reviewQueue); err != nil {
					return writeStructuredError(cmd, opts, verrors.Validation("REVIEW_QUEUE_INVALID", err.Error(), "Check review/review-queue.json", false))
				}
			}
			return writeOutput(cmd, opts, "framing review", map[string]any{"status": "reviewed", "lane": lane, "needs_review": reviewQueue.Items, "review_queue": reviewQueue})
		},
	}
	cmd.Flags().StringVar(&projectPath, "project", ".", "project path")
	return cmd
}

func framingPresetImportCommand(opts *globalOptions) *cobra.Command {
	var projectPath, input string
	cmd := &cobra.Command{
		Use:   "import",
		Short: "import approved framing presets",
		RunE: func(cmd *cobra.Command, args []string) error {
			raw, err := os.ReadFile(input)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.External("FRAMING_PRESET_READ_FAILED", err.Error(), "Check --input path", false))
			}
			presets, err := vframing.ParsePresets(raw)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.Validation("FRAMING_PRESET_PARSE_FAILED", err.Error(), "Check preset JSON", false))
			}
			if err := presets.Validate(); err != nil {
				return writeStructuredError(cmd, opts, verrors.Validation("FRAMING_PRESET_INVALID", err.Error(), "Use stable preset IDs and source-bounded crop boxes", false))
			}
			status := "planned"
			if opts.Commit {
				if err := vframing.WritePresets(projectPath, presets); err != nil {
					return writeStructuredError(cmd, opts, verrors.External("FRAMING_PRESET_WRITE_FAILED", err.Error(), "Check project write permissions", false))
				}
				status = "written"
			}
			return writeOutput(cmd, opts, "framing preset import", map[string]any{"status": status, "preset_count": len(presets.Presets), "artifact": filepath.ToSlash(filepath.Join(projectPath, "calibration", "framing-presets.json"))})
		},
	}
	cmd.Flags().StringVar(&projectPath, "project", ".", "project path")
	cmd.Flags().StringVar(&input, "input", "", "input framing-presets.json")
	return cmd
}

func framingPresetValidateCommand(opts *globalOptions) *cobra.Command {
	var projectPath string
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "validate approved framing presets",
		RunE: func(cmd *cobra.Command, args []string) error {
			presets, err := vframing.ReadPresets(projectPath)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.External("FRAMING_PRESET_READ_FAILED", err.Error(), "Import presets first", false))
			}
			if err := presets.Validate(); err != nil {
				return writeStructuredError(cmd, opts, verrors.Validation("FRAMING_PRESET_INVALID", err.Error(), "Use stable preset IDs and source-bounded crop boxes", false))
			}
			return writeOutput(cmd, opts, "framing preset validate", map[string]any{"status": "valid", "preset_count": len(presets.Presets)})
		},
	}
	cmd.Flags().StringVar(&projectPath, "project", ".", "project path")
	return cmd
}

func framingPresetListCommand(opts *globalOptions) *cobra.Command {
	var projectPath string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "list approved framing presets",
		RunE: func(cmd *cobra.Command, args []string) error {
			presets, err := vframing.ReadPresets(projectPath)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.External("FRAMING_PRESET_READ_FAILED", err.Error(), "Import presets first", false))
			}
			return writeOutput(cmd, opts, "framing preset list", map[string]any{"presets": presets.Presets})
		},
	}
	cmd.Flags().StringVar(&projectPath, "project", ".", "project path")
	return cmd
}

func timelineCommand(opts *globalOptions) *cobra.Command {
	parent := &cobra.Command{Use: "timeline", Short: "timeline workflow commands"}
	parent.AddCommand(timelineCompileCommand(opts))
	parent.AddCommand(timelineVerifyCommand(opts))
	return parent
}

func timelineVerifyCommand(opts *globalOptions) *cobra.Command {
	var projectPath string
	cmd := &cobra.Command{
		Use:     "verify",
		Aliases: []string{"verify-timeline", "check-timeline"},
		Short:   "verify compiled timeline",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := filepath.Join(projectPath, "timeline", "compiled-timeline.json")
			raw, err := os.ReadFile(path)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.External("TIMELINE_READ_FAILED", err.Error(), "Run timeline compile --commit first", false))
			}
			var tl vtimeline.CompiledTimeline
			if err := json.Unmarshal(raw, &tl); err != nil {
				return writeStructuredError(cmd, opts, verrors.Validation("TIMELINE_INVALID", err.Error(), "Check compiled timeline JSON", false))
			}
			issues := []map[string]any{}
			lastOut := 0
			for _, segment := range tl.Segments {
				if segment.TimelineFrameIn < lastOut {
					issues = append(issues, map[string]any{"code": "OVERLAP", "segment": segment.ID})
				}
				if segment.TimelineFrameOut < segment.TimelineFrameIn || segment.SourceFrameOut < segment.SourceFrameIn {
					issues = append(issues, map[string]any{"code": "NEGATIVE_DURATION", "segment": segment.ID})
				}
				lastOut = segment.TimelineFrameOut
			}
			status := "valid"
			if len(issues) > 0 {
				status = "invalid"
			}
			return writeOutput(cmd, opts, "timeline verify", map[string]any{"status": status, "segment_count": len(tl.Segments), "duration_frames": tl.DurationFrames, "issues": issues})
		},
	}
	cmd.Flags().StringVar(&projectPath, "project", ".", "project path")
	return cmd
}

func timelineCompileCommand(opts *globalOptions) *cobra.Command {
	var projectPath string
	var durationFrames int64
	cmd := &cobra.Command{
		Use:     "compile",
		Aliases: []string{"build-timeline", "make-timeline", "assemble"},
		Short:   "compile content decisions into time-map and timeline artifacts",
		RunE: func(cmd *cobra.Command, args []string) error {
			edl := vcleanup.ContentEDL{Version: "vflow-content-edl/v1"}
			if readEDL, err := vcleanup.ReadContentEDL(projectPath); err == nil {
				edl = readEDL
			} else if !os.IsNotExist(err) {
				return writeStructuredError(cmd, opts, verrors.Validation("CONTENT_EDL_INVALID", err.Error(), "Fix decisions/content-edl.json or remove it to compile an empty content lane", false))
			}
			compiled := vtimeline.Compile(edl, int(durationFrames))
			status := "planned"
			if opts.Commit {
				if err := vtimeline.WriteCompiled(projectPath, compiled); err != nil {
					return err
				}
				status = "written"
			}
			return writeOutput(cmd, opts, "timeline compile", map[string]any{
				"status":            status,
				"time_map":          compiled.TimeMap,
				"compiled_timeline": compiled,
				"artifacts": []string{
					filepath.ToSlash(filepath.Join(projectPath, "decisions", "time-map.json")),
					filepath.ToSlash(filepath.Join(projectPath, "timeline", "compiled-timeline.json")),
				},
			})
		},
	}
	cmd.Flags().StringVar(&projectPath, "project", ".", "project path")
	cmd.Flags().Int64Var(&durationFrames, "duration-frames", 900, "source duration in frames")
	return cmd
}

func discoverMediaSources(projectPath, source string) ([]string, error) {
	if source != "" {
		if filepath.IsAbs(source) {
			return []string{source}, nil
		}
		if _, err := os.Stat(source); err == nil {
			return []string{source}, nil
		}
		return []string{filepath.Join(projectPath, source)}, nil
	}
	var matches []string
	for _, pattern := range []string{
		filepath.Join(projectPath, "media", "source-4k", "*.MP4"),
		filepath.Join(projectPath, "media", "source-4k", "*.mp4"),
		filepath.Join(projectPath, "media", "source-4k", "*.MOV"),
		filepath.Join(projectPath, "media", "source-4k", "*.mov"),
		filepath.Join(projectPath, "media", "*.mp4"),
		filepath.Join(projectPath, "media", "*.MP4"),
	} {
		found, err := filepath.Glob(pattern)
		if err != nil {
			return nil, err
		}
		matches = append(matches, found...)
	}
	if len(matches) == 0 {
		return nil, os.ErrNotExist
	}
	return matches, nil
}

func findProjectContracts(root string, maxDepth int) ([]map[string]any, error) {
	if root == "" {
		root = "."
	}
	root = filepath.Clean(root)
	var projects []map[string]any
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == "bin" || name == "dist" {
				return filepath.SkipDir
			}
			if maxDepth > 0 {
				rel, relErr := filepath.Rel(root, path)
				if relErr == nil && rel != "." && strings.Count(rel, string(os.PathSeparator))+1 > maxDepth {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if d.Name() != "project.json" {
			return nil
		}
		proj, loadErr := vproject.Load(filepath.Dir(path))
		if loadErr != nil {
			projects = append(projects, map[string]any{
				"path":  filepath.ToSlash(path),
				"valid": false,
				"error": loadErr.Error(),
			})
			return nil
		}
		projects = append(projects, map[string]any{
			"id":    proj.ID,
			"root":  proj.Root,
			"path":  filepath.ToSlash(path),
			"valid": true,
		})
		return nil
	})
	return projects, err
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func appendJSONLine(path string, raw []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(append(raw, '\n')); err != nil {
		return err
	}
	return nil
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func trimText(value string, max int) string {
	value = strings.Join(strings.Fields(value), " ")
	if max <= 0 || len(value) <= max {
		return value
	}
	return value[:max] + "..."
}

func parseSourceInputs(value string) ([]vsyncmap.SourceInput, error) {
	if strings.TrimSpace(value) == "" {
		return nil, fmt.Errorf("sources are required")
	}
	var out []vsyncmap.SourceInput
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, path, ok := strings.Cut(part, "=")
		if !ok || strings.TrimSpace(id) == "" || strings.TrimSpace(path) == "" {
			return nil, fmt.Errorf("invalid source %q", part)
		}
		out = append(out, vsyncmap.SourceInput{ID: strings.TrimSpace(id), Path: strings.TrimSpace(path)})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("sources are required")
	}
	return out, nil
}

func projectIDFromPath(path string) string {
	if proj, err := vproject.Load(path); err == nil && proj.ID != "" {
		return proj.ID
	}
	return filepath.Base(filepath.Clean(path))
}

func writeJSONFile(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}

func renderCommand(opts *globalOptions) *cobra.Command {
	parent := &cobra.Command{Use: "render", Short: "render workflow commands"}
	parent.AddCommand(renderPreviewCommand(opts), renderTranscriptCutCommand(opts), renderVerifyCommand(opts), renderVerifyTranscriptCommand(opts))
	return parent
}

func renderPreviewCommand(opts *globalOptions) *cobra.Command {
	var projectPath, source, target, ffmpegPath, outputPath string
	var duration, startSeconds float64
	cmd := &cobra.Command{
		Use:     "preview",
		Aliases: []string{"make-preview", "render-sample"},
		Short:   "render a rough preview with ffmpeg",
		RunE: func(cmd *cobra.Command, args []string) error {
			output := outputPath
			if output == "" {
				output = filepath.Join(projectPath, "renders", "rough-preview.mp4")
			} else if !filepath.IsAbs(output) {
				output = filepath.Join(projectPath, output)
			}
			plan := vrender.PreviewPlan(vrender.Options{
				Input:        firstNonEmptyString(source, filepath.Join(projectPath, "media", "source.mp4")),
				Output:       output,
				Target:       target,
				MaxSeconds:   int(duration),
				StartSeconds: startSeconds,
			})
			if ffmpegPath != "" && len(plan.Command) > 0 {
				plan.Command[0] = ffmpegPath
			}
			reportPath := filepath.Join(projectPath, "reports", "render-report.json")
			status := "planned"
			if opts.Commit {
				if err := vrender.Run(plan); err != nil {
					_ = vjobs.Write(projectPath, vjobs.NewRecord(projectPath, "render preview", "failed", true))
					return writeStructuredError(cmd, opts, verrors.External("FFMPEG_RENDER_FAILED", err.Error(), "Check ffmpeg, source path, and codecs", false))
				}
				_ = vjobs.Write(projectPath, vjobs.NewRecord(projectPath, "render preview", "succeeded", true))
				status = "rendered"
				if err := vrender.WriteReport(reportPath, plan, status); err != nil {
					return writeStructuredError(cmd, opts, verrors.External("RENDER_REPORT_WRITE_FAILED", err.Error(), "Check project write permissions", false))
				}
			}
			return writeOutput(cmd, opts, "render preview", map[string]any{"status": status, "plan": plan, "report_path": filepath.ToSlash(reportPath)})
		},
	}
	cmd.Flags().StringVar(&projectPath, "project", ".", "project path")
	cmd.Flags().StringVar(&source, "source", "", "source media path")
	cmd.Flags().StringVar(&outputPath, "output", "", "output path; relative paths are resolved under --project")
	cmd.Flags().StringVar(&target, "target", "youtube_16x9", "render target")
	cmd.Flags().StringVar(&ffmpegPath, "ffmpeg-path", "", "ffmpeg binary path")
	cmd.Flags().Float64Var(&duration, "duration-seconds", 2, "preview duration in seconds")
	cmd.Flags().Float64Var(&startSeconds, "start-seconds", 0, "source start offset in seconds")
	return cmd
}

func renderTranscriptCutCommand(opts *globalOptions) *cobra.Command {
	var projectPath, inputPath, outputPath, target, ffmpegPath, syncMapPath string
	cmd := &cobra.Command{
		Use:   "transcript-cut",
		Short: "render transcript-selected segments into one social cut",
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputPath == "" {
				return writeStructuredError(cmd, opts, verrors.Validation("MISSING_INPUT", "missing --input", "Pass a vflow-transcript-cut/v1 JSON edit decision file", false))
			}
			edit, err := vrender.ReadTranscriptCut(inputPath)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.External("TRANSCRIPT_CUT_READ_FAILED", err.Error(), "Check --input path and JSON shape", false))
			}
			if syncMapPath != "" {
				m, err := vsyncmap.Read(syncMapPath)
				if err != nil {
					return writeStructuredError(cmd, opts, verrors.External("SYNC_MAP_READ_FAILED", err.Error(), "Check --sync-map path", false))
				}
				edit, err = vrender.ResolveTranscriptCutWithSyncMap(edit, m)
				if err != nil {
					return writeStructuredError(cmd, opts, verrors.Validation("SYNC_MAP_RESOLVE_FAILED", err.Error(), "Check source_id and transcript seconds in the cut", false))
				}
			}
			for i := range edit.Segments {
				if edit.Segments[i].Source != "" && !filepath.IsAbs(edit.Segments[i].Source) {
					edit.Segments[i].Source = filepath.Join(projectPath, edit.Segments[i].Source)
				}
			}
			output := outputPath
			if output == "" {
				output = filepath.Join(projectPath, "renders", "transcript-social-cut.mp4")
			} else if !filepath.IsAbs(output) {
				output = filepath.Join(projectPath, output)
			}
			result, err := vrender.TranscriptCutPlan(edit, output, target)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.Validation("TRANSCRIPT_CUT_INVALID", err.Error(), "Check segment source/start/end values", false))
			}
			if ffmpegPath != "" && len(result.Command) > 0 {
				result.Command[0] = ffmpegPath
			}
			reportPath := filepath.Join(projectPath, "reports", "transcript-cut-render-report.json")
			status := "planned"
			if opts.Commit {
				if err := vrender.Run(result.Plan); err != nil {
					_ = vjobs.Write(projectPath, vjobs.NewRecord(projectPath, "render transcript-cut", "failed", true))
					return writeStructuredError(cmd, opts, verrors.External("FFMPEG_RENDER_FAILED", err.Error(), "Check ffmpeg, segment sources, and codecs", false))
				}
				_ = vjobs.Write(projectPath, vjobs.NewRecord(projectPath, "render transcript-cut", "succeeded", true))
				status = "rendered"
			}
			_ = vrender.WriteTranscriptCutReport(reportPath, result, status)
			return writeOutput(cmd, opts, "render transcript-cut", map[string]any{
				"status":           status,
				"plan":             result.Plan,
				"segments":         result.Segments,
				"duration_seconds": result.DurationSeconds,
				"report_path":      filepath.ToSlash(reportPath),
			})
		},
	}
	cmd.Flags().StringVar(&projectPath, "project", ".", "project path")
	cmd.Flags().StringVar(&inputPath, "input", "", "vflow-transcript-cut/v1 JSON edit decision file")
	cmd.Flags().StringVar(&outputPath, "output", "", "output path; relative paths are resolved under --project")
	cmd.Flags().StringVar(&target, "target", "youtube_16x9", "render target")
	cmd.Flags().StringVar(&ffmpegPath, "ffmpeg-path", "", "ffmpeg binary path")
	cmd.Flags().StringVar(&syncMapPath, "sync-map", "", "optional vflow-media-sync-map/v1 for transcript-relative segments")
	return cmd
}

func renderVerifyTranscriptCommand(opts *globalOptions) *cobra.Command {
	var projectPath, renderPath, cutPath, outputPath string
	cmd := &cobra.Command{
		Use:   "verify-transcript",
		Short: "write transcript proof expectations for a rendered cut",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cutPath == "" {
				return writeStructuredError(cmd, opts, verrors.Validation("MISSING_CUT", "missing --cut", "Pass the transcript-cut JSON used for the render", false))
			}
			cut, err := vrender.ReadTranscriptCut(cutPath)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.External("CUT_READ_FAILED", err.Error(), "Check --cut path", false))
			}
			if renderPath == "" {
				renderPath = filepath.Join(projectPath, "renders", "transcript-social-cut.mp4")
			}
			if outputPath == "" {
				outputPath = filepath.Join(projectPath, "reports", "transcript-proof.json")
			}
			segments := []map[string]any{}
			for _, segment := range cut.Segments {
				segments = append(segments, map[string]any{
					"id": segment.ID, "source_id": segment.SourceID, "text": segment.Text,
					"transcript_start_seconds": segment.TranscriptStartSeconds,
					"transcript_end_seconds":   segment.TranscriptEndSeconds,
					"source_start_seconds":     segment.StartSeconds,
					"source_end_seconds":       segment.EndSeconds,
				})
			}
			proof := map[string]any{
				"version": "vflow-transcript-proof/v1", "status": "planned",
				"render": filepath.ToSlash(renderPath), "cut": filepath.ToSlash(cutPath),
				"provider_required": false, "segments": segments,
				"warnings": []string{"local proof records expected transcript coverage; run live STT separately when OPENAI_API_KEY is present"},
			}
			if opts.Commit {
				proof["status"] = "written"
				if err := writeJSONFile(outputPath, proof); err != nil {
					return writeStructuredError(cmd, opts, verrors.External("TRANSCRIPT_PROOF_WRITE_FAILED", err.Error(), "Check report path permissions", false))
				}
			}
			return writeOutput(cmd, opts, "render verify-transcript", map[string]any{"proof_path": filepath.ToSlash(outputPath), "proof": proof})
		},
	}
	cmd.Flags().StringVar(&projectPath, "project", ".", "project path")
	cmd.Flags().StringVar(&renderPath, "render", "", "render file path")
	cmd.Flags().StringVar(&cutPath, "cut", "", "transcript cut JSON")
	cmd.Flags().StringVar(&outputPath, "output", "", "transcript proof report path")
	return cmd
}

func renderVerifyCommand(opts *globalOptions) *cobra.Command {
	var projectPath, renderPath, ffprobePath, ffprobeJSON string
	var expectedWidth, expectedHeight int
	var expectedDuration float64
	cmd := &cobra.Command{
		Use:     "verify",
		Aliases: []string{"verify-render", "check-render", "qa-render"},
		Short:   "verify a rendered preview",
		RunE: func(cmd *cobra.Command, args []string) error {
			if renderPath != "" && !filepath.IsAbs(renderPath) {
				renderPath = projectRelativePath(projectPath, renderPath)
			}
			status := "missing"
			if ffprobeJSON != "" {
				raw, err := os.ReadFile(ffprobeJSON)
				if err != nil {
					return writeStructuredError(cmd, opts, verrors.External("FFPROBE_JSON_READ_FAILED", err.Error(), "Check --ffprobe-json path", false))
				}
				result, err := vrender.VerifyProbe(raw, vrender.VerifyOptions{Render: renderPath, ExpectedWidth: expectedWidth, ExpectedHeight: expectedHeight, ExpectedDurationSeconds: expectedDuration})
				if err != nil {
					return writeStructuredError(cmd, opts, verrors.Validation("FFPROBE_JSON_INVALID", err.Error(), "Pass valid ffprobe JSON", false))
				}
				return writeOutput(cmd, opts, "render verify", result)
			}
			if _, err := os.Stat(renderPath); err == nil {
				result, err := vrender.VerifyFile(ffprobePath, vrender.VerifyOptions{Render: renderPath, ExpectedWidth: expectedWidth, ExpectedHeight: expectedHeight, ExpectedDurationSeconds: expectedDuration})
				if err == nil {
					return writeOutput(cmd, opts, "render verify", result)
				}
				return writeOutput(cmd, opts, "render verify", vrender.VerifyResult{Status: "exists", Render: renderPath, Issues: []string{"ffprobe_failed: " + err.Error()}})
			}
			return writeOutput(cmd, opts, "render verify", vrender.VerifyResult{Status: status, Render: renderPath})
		},
	}
	cmd.Flags().StringVar(&projectPath, "project", ".", "project path")
	cmd.Flags().StringVar(&renderPath, "render", "renders/rough-preview.mp4", "render path")
	cmd.Flags().StringVar(&renderPath, "input", "renders/rough-preview.mp4", "alias for --render")
	cmd.Flags().StringVar(&ffprobePath, "ffprobe-path", "", "ffprobe binary path")
	cmd.Flags().StringVar(&ffprobeJSON, "ffprobe-json", "", "read ffprobe JSON from file instead of executing ffprobe")
	cmd.Flags().IntVar(&expectedWidth, "expected-width", 0, "expected render width")
	cmd.Flags().IntVar(&expectedHeight, "expected-height", 0, "expected render height")
	cmd.Flags().Float64Var(&expectedDuration, "expected-duration", 0, "expected duration in seconds")
	return cmd
}

func qaCommand(opts *globalOptions) *cobra.Command {
	parent := &cobra.Command{Use: "qa", Short: "video QA commands"}
	parent.AddCommand(qaDoctorCommand(opts), qaAnalyzeCommand(opts))
	return parent
}

func qaDoctorCommand(opts *globalOptions) *cobra.Command {
	var provider, model, keyEnv string
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "check QA provider capability",
		RunE: func(cmd *cobra.Command, args []string) error {
			if provider != "gemini" {
				return writeStructuredError(cmd, opts, verrors.Validation("INVALID_ENUM", "unsupported QA provider", "Use provider gemini", false))
			}
			key, source, err := geminiAPIKey(keyEnv)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.Validation("INVALID_KEY_ENV", err.Error(), "Pass an environment variable name such as GEMINI_API_KEY", false))
			}
			result, err := vqa.DoctorWithKey(model, opts.Live, key, source)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.External("QA_DOCTOR_FAILED", err.Error(), "Check model name and GEMINI_API_KEY", true))
			}
			return writeOutput(cmd, opts, "qa doctor", result)
		},
	}
	cmd.Flags().StringVar(&provider, "provider", "gemini", "QA provider")
	cmd.Flags().StringVar(&model, "model", "", "Gemini model")
	cmd.Flags().StringVar(&keyEnv, "key-env", "", "environment variable containing the Gemini API key")
	return cmd
}

func qaAnalyzeCommand(opts *globalOptions) *cobra.Command {
	var projectPath, provider, model, renderPath, uploadMode, keyEnv string
	var appendReviewQueue bool
	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "analyze rendered video with Gemini QA",
		RunE: func(cmd *cobra.Command, args []string) error {
			if provider != "gemini" {
				return writeStructuredError(cmd, opts, verrors.Validation("INVALID_ENUM", "unsupported QA provider", "Use provider gemini", false))
			}
			if uploadMode != "files" && uploadMode != "inline" {
				return writeStructuredError(cmd, opts, verrors.Validation("INVALID_ENUM", "unsupported Gemini upload mode", "Use --upload files or --upload inline", false))
			}
			selected, err := vqa.NormalizeModel(model)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.Validation("INVALID_MODEL", err.Error(), "Run vflow qa doctor --provider gemini", false))
			}
			if renderPath == "" {
				renderPath = filepath.Join(projectPath, "renders", "rough-preview.mp4")
			}
			reportPath := filepath.Join(projectPath, "reports", "gemini-video-qa.json")
			data := map[string]any{
				"version":     "vflow-gemini-video-qa/v1",
				"status":      "planned",
				"provider":    "gemini",
				"model":       selected,
				"render":      renderPath,
				"upload":      uploadMode,
				"report_path": filepath.ToSlash(reportPath),
				"prompt":      vqa.VideoQAPrompt,
			}
			if opts.Live {
				key, _, err := geminiAPIKey(keyEnv)
				if err != nil {
					return writeStructuredError(cmd, opts, verrors.Validation("INVALID_KEY_ENV", err.Error(), "Pass an environment variable name such as GEMINI_API_KEY", false))
				}
				if key == "" {
					return writeStructuredError(cmd, opts, verrors.Validation("MISSING_API_KEY", "Gemini API key is not set", "Use GEMINI_API_KEY or GOOGLE_API_KEY via runtime env or Secret Gate; do not commit secrets", true))
				}
				var raw string
				if uploadMode == "files" {
					var uploaded vqa.UploadedFile
					raw, uploaded, err = vqa.AnalyzeFileVideo(key, selected, renderPath, vqa.VideoQAPrompt)
					data["uploaded_file"] = uploaded
				} else {
					raw, err = vqa.AnalyzeInlineVideo(key, selected, renderPath, vqa.VideoQAPrompt)
				}
				if err != nil {
					hint := "Run qa doctor and verify model availability"
					if uploadMode == "files" {
						hint = "For small renders try --upload inline; otherwise verify Gemini Files API access for this key/project"
					}
					return writeStructuredError(cmd, opts, verrors.External("GEMINI_QA_FAILED", err.Error(), hint, true))
				}
				sanitized := vqa.SanitizeProviderResponse(raw)
				data["provider_response"] = sanitized
				data["status"] = "analyzed"
				if appendReviewQueue {
					items := reviewItemsFromQAResponse(sanitized)
					data["proposed_review_items"] = items
					data["review_queue_path"] = filepath.ToSlash(filepath.Join(projectPath, "review", "review-queue.json"))
					if opts.Commit && len(items) > 0 {
						if err := appendReviewQueueItems(projectPath, items); err != nil {
							return writeStructuredError(cmd, opts, verrors.External("REVIEW_QUEUE_WRITE_FAILED", err.Error(), "Check project review directory permissions", false))
						}
					}
				}
				if opts.Commit {
					if err := writeGeminiQAReport(reportPath, data); err != nil {
						return writeStructuredError(cmd, opts, verrors.External("GEMINI_QA_REPORT_WRITE_FAILED", err.Error(), "Check project report directory permissions", false))
					}
				}
			}
			if appendReviewQueue && data["proposed_review_items"] == nil {
				data["proposed_review_items"] = []vframing.ReviewItem{}
				data["review_queue_path"] = filepath.ToSlash(filepath.Join(projectPath, "review", "review-queue.json"))
			}
			return writeOutput(cmd, opts, "qa analyze", data)
		},
	}
	cmd.Flags().StringVar(&projectPath, "project", ".", "project path")
	cmd.Flags().StringVar(&provider, "provider", "gemini", "QA provider")
	cmd.Flags().StringVar(&model, "model", "", "Gemini model")
	cmd.Flags().StringVar(&renderPath, "render", "", "render path")
	cmd.Flags().StringVar(&uploadMode, "upload", "files", "Gemini video upload mode: files or inline")
	cmd.Flags().StringVar(&keyEnv, "key-env", "", "environment variable containing the Gemini API key")
	cmd.Flags().BoolVar(&appendReviewQueue, "append-review-queue", false, "append high-confidence QA observations to review/review-queue.json with --commit")
	return cmd
}

func geminiAPIKey(keyEnv string) (string, string, error) {
	if strings.TrimSpace(keyEnv) != "" {
		return vqa.APIKeyFromNamedEnv(keyEnv)
	}
	key, source := vqa.APIKeyFromEnv()
	return key, source, nil
}

func writeGeminiQAReport(reportPath string, data map[string]any) error {
	report := map[string]any{}
	for key, value := range data {
		report[key] = value
	}
	report["version"] = "vflow-gemini-video-qa/v1"
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(reportPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(reportPath, append(raw, '\n'), 0o644)
}

func reviewItemsFromQAResponse(raw json.RawMessage) []vframing.ReviewItem {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil
	}
	candidates := collectQAObservations(value)
	items := make([]vframing.ReviewItem, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.confidence < 0.75 {
			continue
		}
		endFrame := candidate.endFrame
		if endFrame < candidate.startFrame+1 {
			endFrame = candidate.startFrame + 1
		}
		items = append(items, vframing.ReviewItem{
			ID:         fmt.Sprintf("rev_%06d", len(items)+1),
			Code:       firstNonEmptyString(candidate.code, "qa_observation"),
			Severity:   "needs_human_review",
			Message:    firstNonEmptyString(candidate.message, "High-confidence QA observation needs review."),
			EventID:    "fr_000000",
			StartFrame: candidate.startFrame,
			EndFrame:   endFrame,
			PresetID:   "qa_video_output",
		})
	}
	return items
}

type qaObservation struct {
	code       string
	message    string
	confidence float64
	startFrame int64
	endFrame   int64
}

func collectQAObservations(value any) []qaObservation {
	out := []qaObservation{}
	switch typed := value.(type) {
	case map[string]any:
		if obs, ok := qaObservationFromMap(typed); ok {
			out = append(out, obs)
		}
		for key, child := range typed {
			if key == "text" {
				if text, ok := child.(string); ok {
					text = strings.TrimSpace(text)
					if strings.HasPrefix(text, "{") || strings.HasPrefix(text, "[") {
						var nested any
						if err := json.Unmarshal([]byte(text), &nested); err == nil {
							out = append(out, collectQAObservations(nested)...)
						}
					}
				}
			}
			out = append(out, collectQAObservations(child)...)
		}
	case []any:
		for _, child := range typed {
			out = append(out, collectQAObservations(child)...)
		}
	}
	return out
}

func qaObservationFromMap(value map[string]any) (qaObservation, bool) {
	confidence, ok := numberField(value, "confidence")
	if !ok {
		return qaObservation{}, false
	}
	message := firstNonEmptyString(stringField(value, "message"), stringField(value, "observation"), stringField(value, "issue"), stringField(value, "description"))
	if message == "" {
		return qaObservation{}, false
	}
	code := firstNonEmptyString(stringField(value, "code"), stringField(value, "type"), stringField(value, "category"))
	startFrame, startOK := intField(value, "start_frame")
	endFrame, endOK := intField(value, "end_frame")
	if !startOK {
		if seconds, ok := numberField(value, "start_seconds"); ok {
			startFrame = int64(seconds * 30)
		} else if seconds, ok := numberField(value, "time_seconds"); ok {
			startFrame = int64(seconds * 30)
		}
	}
	if !endOK {
		if seconds, ok := numberField(value, "end_seconds"); ok {
			endFrame = int64(seconds * 30)
		} else {
			endFrame = startFrame + 30
		}
	}
	if endFrame <= startFrame {
		endFrame = startFrame + 1
	}
	return qaObservation{code: sanitizeReviewCode(code), message: message, confidence: confidence, startFrame: startFrame, endFrame: endFrame}, true
}

func appendReviewQueueItems(projectPath string, items []vframing.ReviewItem) error {
	path := filepath.Join(projectPath, "review", "review-queue.json")
	queue := vframing.ReviewQueue{Version: "vflow-review-queue/v1", Items: []vframing.ReviewItem{}}
	if raw, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(raw, &queue); err != nil {
			return err
		}
		if queue.Version == "" {
			queue.Version = "vflow-review-queue/v1"
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	for _, item := range items {
		item.ID = fmt.Sprintf("rev_%06d", len(queue.Items)+1)
		queue.Items = append(queue.Items, item)
	}
	raw, err := json.MarshalIndent(queue, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}

func stringField(value map[string]any, key string) string {
	if raw, ok := value[key].(string); ok {
		return strings.TrimSpace(raw)
	}
	return ""
}

func numberField(value map[string]any, key string) (float64, bool) {
	switch raw := value[key].(type) {
	case float64:
		return raw, true
	case int:
		return float64(raw), true
	case json.Number:
		number, err := raw.Float64()
		return number, err == nil
	default:
		return 0, false
	}
}

func intField(value map[string]any, key string) (int64, bool) {
	number, ok := numberField(value, key)
	return int64(number), ok
}

func sanitizeReviewCode(code string) string {
	code = strings.ToLower(strings.TrimSpace(code))
	code = strings.ReplaceAll(code, " ", "_")
	code = strings.ReplaceAll(code, "-", "_")
	if code == "" {
		return "qa_observation"
	}
	return "qa_" + strings.TrimPrefix(code, "qa_")
}

func updateRenderReportColor(reportPath, ungradedPath, gradedPath, lutPath string, lutRaw []byte, plan vcolor.ApplyPlan, intent, qaReport string) error {
	report := map[string]any{
		"version": "vflow-render-report/v1",
		"status":  "color_applied",
	}
	warnings := []string{}
	if raw, err := os.ReadFile(reportPath); err == nil {
		if err := json.Unmarshal(raw, &report); err != nil {
			return err
		}
	} else if os.IsNotExist(err) {
		warnings = append(warnings, "render_report_created_from_color_apply_without_existing_preview_report")
	} else {
		return err
	}
	if report["version"] == nil {
		report["version"] = "vflow-render-report/v1"
	}
	if report["status"] == nil {
		report["status"] = "color_applied"
	}
	lutSum := sha256.Sum256(lutRaw)
	filtergraph := ""
	for i, token := range plan.Command {
		if token == "-vf" && i+1 < len(plan.Command) {
			filtergraph = plan.Command[i+1]
			break
		}
	}
	if intent == "" {
		intent = "preview"
	}
	colorData := map[string]any{
		"ungraded_render_path": filepath.ToSlash(ungradedPath),
		"graded_render_path":   filepath.ToSlash(gradedPath),
		"lut_path":             filepath.ToSlash(lutPath),
		"lut_sha256":           fmt.Sprintf("%x", lutSum),
		"ffmpeg_filtergraph":   filtergraph,
		"warnings":             warnings,
		"intent":               intent,
	}
	if qaReport != "" {
		colorData["qa_report_refs"] = []string{filepath.ToSlash(qaReport)}
	} else {
		colorData["qa_report_refs"] = []string{}
	}
	report["color"] = colorData
	if report["render_path"] == nil && ungradedPath != "" {
		report["render_path"] = filepath.ToSlash(ungradedPath)
	}
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(reportPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(reportPath, append(raw, '\n'), 0o644)
}

func colorCommand(opts *globalOptions) *cobra.Command {
	parent := &cobra.Command{Use: "color", Short: "color workflow commands"}
	parent.AddCommand(colorApplyCommand(opts), colorReviewCommand(opts), colorResearchCommand(opts), colorExportLUTCommand(opts))
	return parent
}

func colorApplyCommand(opts *globalOptions) *cobra.Command {
	var projectPath, input, lut, deliver, intent, qaReport, ffmpegPath string
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "apply a .cube LUT with ffmpeg",
		RunE: func(cmd *cobra.Command, args []string) error {
			intent = strings.TrimSpace(intent)
			if intent == "" {
				intent = "preview"
			}
			if intent != "preview" && intent != "final" {
				return writeStructuredError(cmd, opts, verrors.Validation("INVALID_ENUM", "unsupported color intent", "Use --intent preview or --intent final", false))
			}
			raw, err := os.ReadFile(lut)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.External("LUT_READ_FAILED", err.Error(), "Check --lut path", false))
			}
			parsed, err := vcolor.ParseCube(raw)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.Validation("LUT_INVALID", err.Error(), "Use a valid 3D .cube LUT", false))
			}
			outputPath := strings.TrimPrefix(deliver, "file:")
			if !filepath.IsAbs(outputPath) {
				outputPath = filepath.Join(projectPath, outputPath)
			}
			plan := vcolor.LUTApplyPlan(input, lut, outputPath)
			if ffmpegPath != "" && len(plan.Command) > 0 {
				plan.Command[0] = ffmpegPath
			}
			status := "planned"
			if opts.Commit {
				if err := vcolor.Run(plan); err != nil {
					return writeStructuredError(cmd, opts, verrors.External("COLOR_APPLY_FAILED", err.Error(), "Check ffmpeg lut3d support", false))
				}
				reportPath := filepath.Join(projectPath, "reports", "render-report.json")
				if err := updateRenderReportColor(reportPath, input, outputPath, lut, raw, plan, intent, qaReport); err != nil {
					return writeStructuredError(cmd, opts, verrors.External("RENDER_REPORT_COLOR_WRITE_FAILED", err.Error(), "Check project report permissions", false))
				}
				status = "rendered"
			}
			return writeOutput(cmd, opts, "color apply", map[string]any{
				"status":             status,
				"lut":                parsed,
				"plan":               plan,
				"render_report_path": filepath.ToSlash(filepath.Join(projectPath, "reports", "render-report.json")),
			})
		},
	}
	cmd.Flags().StringVar(&projectPath, "project", ".", "project path")
	cmd.Flags().StringVar(&input, "input", "", "input render path")
	cmd.Flags().StringVar(&lut, "lut", "", ".cube LUT path")
	cmd.Flags().StringVar(&deliver, "deliver", "file:renders/rough-preview-graded.mp4", "delivery target")
	cmd.Flags().StringVar(&intent, "intent", "preview", "color intent: preview or final")
	cmd.Flags().StringVar(&qaReport, "qa-report", "", "optional QA report reference to record in render-report.json")
	cmd.Flags().StringVar(&ffmpegPath, "ffmpeg-path", "", "ffmpeg binary path")
	return cmd
}

func colorReviewCommand(opts *globalOptions) *cobra.Command {
	var projectPath, provider, model, input, keyEnv string
	cmd := &cobra.Command{
		Use:   "review",
		Short: "review color/exposure with optional Gemini analysis",
		RunE: func(cmd *cobra.Command, args []string) error {
			if provider != "gemini" {
				return writeStructuredError(cmd, opts, verrors.Validation("INVALID_ENUM", "unsupported color review provider", "Use provider gemini", false))
			}
			reportPath := filepath.Join(projectPath, "reports", "color-grade-report.json")
			data := map[string]any{
				"version":    "vflow-color-grade-report/v1",
				"status":     "planned",
				"provider":   provider,
				"model":      model,
				"input":      input,
				"report":     filepath.ToSlash(reportPath),
				"confidence": 0,
				"observations": []map[string]any{{
					"code":       "manual_review_required",
					"message":    "Review exposure, contrast, white balance, skin tones, and mixed lighting before final delivery.",
					"confidence": 0.5,
				}},
			}
			if opts.Live {
				key, _, err := geminiAPIKey(keyEnv)
				if err != nil {
					return writeStructuredError(cmd, opts, verrors.Validation("INVALID_KEY_ENV", err.Error(), "Pass an environment variable name such as GEMINI_API_KEY", false))
				}
				if key == "" {
					return writeStructuredError(cmd, opts, verrors.Validation("MISSING_API_KEY", "Gemini API key is not set", "Use GEMINI_API_KEY or GOOGLE_API_KEY via runtime env or Secret Gate", true))
				}
				raw, err := vqa.AnalyzeInlineVideo(key, model, input, "Return JSON only. Review exposure, contrast, white balance, skin tones, mixed lighting, and color-grade finishing notes.")
				if err != nil {
					return writeStructuredError(cmd, opts, verrors.External("COLOR_REVIEW_FAILED", err.Error(), "Run qa doctor first", true))
				}
				data["provider_response"] = vqa.SanitizeProviderResponse(raw)
				data["status"] = "analyzed"
			}
			if opts.Commit {
				if data["status"] == "planned" {
					data["status"] = "written"
				}
				if err := os.MkdirAll(filepath.Dir(reportPath), 0o755); err != nil {
					return writeStructuredError(cmd, opts, verrors.External("COLOR_REPORT_WRITE_FAILED", err.Error(), "Check project write permissions", false))
				}
				raw, _ := json.MarshalIndent(data, "", "  ")
				if err := os.WriteFile(reportPath, append(raw, '\n'), 0o644); err != nil {
					return writeStructuredError(cmd, opts, verrors.External("COLOR_REPORT_WRITE_FAILED", err.Error(), "Check project write permissions", false))
				}
			}
			return writeOutput(cmd, opts, "color review", data)
		},
	}
	cmd.Flags().StringVar(&projectPath, "project", ".", "project path")
	cmd.Flags().StringVar(&provider, "provider", "gemini", "provider")
	cmd.Flags().StringVar(&model, "model", "", "model")
	cmd.Flags().StringVar(&input, "input", "", "input render path")
	cmd.Flags().StringVar(&keyEnv, "key-env", "", "environment variable containing the Gemini API key")
	cmd.Flags().String("mode", "best-practice", "review mode")
	return cmd
}

func colorResearchCommand(opts *globalOptions) *cobra.Command {
	return &cobra.Command{Use: "research", Short: "show color research status", RunE: func(cmd *cobra.Command, args []string) error {
		return writeOutput(cmd, opts, "color research", map[string]any{"status": "documented", "path": "docs/research/color-grading-github-study.md"})
	}}
}

func colorExportLUTCommand(opts *globalOptions) *cobra.Command {
	var input, outputPath string
	cmd := &cobra.Command{Use: "export-lut", Short: "export or generate a .cube LUT", RunE: func(cmd *cobra.Command, args []string) error {
		if outputPath == "" {
			outputPath = "exports/identity.cube"
		}
		var raw []byte
		var err error
		if input != "" {
			raw, err = os.ReadFile(input)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.External("LUT_READ_FAILED", err.Error(), "Check --input path", false))
			}
			if _, err := vcolor.ParseCube(raw); err != nil {
				return writeStructuredError(cmd, opts, verrors.Validation("LUT_INVALID", err.Error(), "Use a valid .cube LUT", false))
			}
		} else {
			raw = []byte("TITLE \"vflow identity\"\nLUT_3D_SIZE 2\n0 0 0\n0 0 1\n0 1 0\n0 1 1\n1 0 0\n1 0 1\n1 1 0\n1 1 1\n")
		}
		status := "planned"
		if opts.Commit {
			if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
				return writeStructuredError(cmd, opts, verrors.External("LUT_WRITE_FAILED", err.Error(), "Check output path permissions", false))
			}
			if err := os.WriteFile(outputPath, raw, 0o644); err != nil {
				return writeStructuredError(cmd, opts, verrors.External("LUT_WRITE_FAILED", err.Error(), "Check output path permissions", false))
			}
			status = "written"
		}
		return writeOutput(cmd, opts, "color export-lut", map[string]any{"status": status, "input": input, "output": filepath.ToSlash(outputPath)})
	}}
	cmd.Flags().StringVar(&input, "input", "", "source LUT path")
	cmd.Flags().StringVar(&outputPath, "output", "", "output .cube path")
	return cmd
}

func nleCommand(opts *globalOptions) *cobra.Command {
	parent := &cobra.Command{Use: "nle", Short: "NLE exchange commands"}
	parent.AddCommand(nleExportCommand(opts), nleImportCommand(opts), nleDiffCommand(opts), nleAcceptCommand(opts), nleApplyCommand(opts))
	return parent
}

func nleExportCommand(opts *globalOptions) *cobra.Command {
	var projectPath, target, deliver, syncMapPath string
	cmd := &cobra.Command{
		Use:     "export",
		Aliases: []string{"export-nle", "to-nle"},
		Short:   "export timeline to NLE interchange format",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !vnle.ValidTarget(target) {
				return writeStructuredError(cmd, opts, verrors.Validation("INVALID_ENUM", "unsupported NLE export target", "Use one of: edl, fcpxml, resolve, premiere, mlt, otio, sidecar", false))
			}
			outputPath := strings.TrimPrefix(deliver, "file:")
			if outputPath == "" || outputPath == deliver {
				outputPath = filepath.Join(projectPath, "exports", "timeline."+target)
			}
			segments := []vnle.Segment{{ID: "seg_000001", SourceFrameIn: 0, SourceFrameOut: 90, TimelineFrameIn: 0, TimelineFrameOut: 90}}
			if raw, err := os.ReadFile(filepath.Join(projectPath, "timeline", "compiled-timeline.json")); err == nil {
				var tl vtimeline.CompiledTimeline
				if err := json.Unmarshal(raw, &tl); err == nil && len(tl.Segments) > 0 {
					segments = make([]vnle.Segment, 0, len(tl.Segments))
					for _, segment := range tl.Segments {
						segments = append(segments, vnle.Segment{
							ID:               segment.ID,
							SyncMapRef:       syncMapPath,
							SourceFrameIn:    segment.SourceFrameIn,
							SourceFrameOut:   segment.SourceFrameOut,
							TimelineFrameIn:  segment.TimelineFrameIn,
							TimelineFrameOut: segment.TimelineFrameOut,
						})
					}
				}
			}
			res := vnle.Export(vnle.Options{Target: target, Output: outputPath, SyncMapRef: syncMapPath}, segments)
			status := "planned"
			if opts.Commit {
				if err := vnle.WriteExport(projectPath, res); err != nil {
					return writeStructuredError(cmd, opts, verrors.External("NLE_EXPORT_FAILED", err.Error(), "Check export path permissions", false))
				}
				status = "written"
			}
			return writeOutput(cmd, opts, "nle export", map[string]any{"status": status, "export": res})
		},
	}
	cmd.Flags().StringVar(&projectPath, "project", ".", "project path")
	cmd.Flags().StringVar(&target, "target", "fcpxml", "export target")
	cmd.Flags().StringVar(&deliver, "deliver", "", "delivery target")
	cmd.Flags().StringVar(&syncMapPath, "sync-map", "", "vflow-media-sync-map/v1 sidecar reference")
	return cmd
}

func nleImportCommand(opts *globalOptions) *cobra.Command {
	var projectPath, input string
	cmd := &cobra.Command{Use: "import", Aliases: []string{"import-nle", "from-nle"}, Short: "import NLE timeline", RunE: func(cmd *cobra.Command, args []string) error {
		if input == "" {
			return writeStructuredError(cmd, opts, verrors.Validation("MISSING_INPUT", "missing --input", "Pass --input timeline file", false))
		}
		inputPath := resolveProjectInputPath(projectPath, input)
		raw, err := os.ReadFile(inputPath)
		if err != nil {
			return writeStructuredError(cmd, opts, verrors.External("NLE_IMPORT_READ_FAILED", err.Error(), "Check --input path", false))
		}
		data, err := vnle.ParseImport(inputPath, raw)
		if err != nil {
			return writeStructuredError(cmd, opts, verrors.Validation("NLE_IMPORT_PARSE_FAILED", err.Error(), "Use a supported EDL, FCPXML, XMEML, MLT, or OTIO file", false))
		}
		if opts.Commit {
			path := filepath.Join(projectPath, "imports", "nle-import.json")
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return writeStructuredError(cmd, opts, verrors.External("NLE_IMPORT_WRITE_FAILED", err.Error(), "Check project write permissions", false))
			}
			data.Artifact = filepath.ToSlash(path)
			out, _ := json.MarshalIndent(data, "", "  ")
			if err := os.WriteFile(path, append(out, '\n'), 0o644); err != nil {
				return writeStructuredError(cmd, opts, verrors.External("NLE_IMPORT_WRITE_FAILED", err.Error(), "Check project write permissions", false))
			}
		}
		return writeOutput(cmd, opts, "nle import", data)
	}}
	cmd.Flags().StringVar(&projectPath, "project", ".", "project path")
	cmd.Flags().StringVar(&input, "input", "", "input timeline path")
	return cmd
}

func nleDiffCommand(opts *globalOptions) *cobra.Command {
	var projectPath, input, deliver string
	cmd := &cobra.Command{Use: "diff", Aliases: []string{"compare-nle", "nle-compare"}, Short: "classify NLE roundtrip diff", RunE: func(cmd *cobra.Command, args []string) error {
		if input == "" {
			return writeStructuredError(cmd, opts, verrors.Validation("MISSING_INPUT", "missing --import", "Pass --import timeline or nle-import.json", false))
		}
		importResult, err := loadNLEImport(projectPath, input)
		if err != nil {
			return writeStructuredError(cmd, opts, verrors.Validation("NLE_DIFF_PARSE_FAILED", err.Error(), "Use a supported import artifact or raw timeline file", false))
		}
		data := vnle.Classify(importResult)
		if strings.HasPrefix(deliver, "file:") {
			path := strings.TrimPrefix(deliver, "file:")
			if err := writeRoundtripReviewHTML(path, data); err != nil {
				return writeStructuredError(cmd, opts, verrors.External("NLE_DIFF_REVIEW_WRITE_FAILED", err.Error(), "Check --deliver path", false))
			}
			data.Artifact = filepath.ToSlash(path)
		}
		return writeOutput(cmd, opts, "nle diff", data)
	}}
	cmd.Flags().StringVar(&projectPath, "project", ".", "project path")
	cmd.Flags().StringVar(&input, "import", "", "import artifact")
	cmd.Flags().StringVar(&deliver, "deliver", "", "delivery target")
	return cmd
}

func loadNLEImport(projectPath, input string) (vnle.ImportResult, error) {
	path := resolveProjectInputPath(projectPath, input)
	raw, err := os.ReadFile(path)
	if err != nil {
		return vnle.ImportResult{}, err
	}
	var parsed vnle.ImportResult
	if err := json.Unmarshal(raw, &parsed); err == nil && parsed.Version == "vflow-nle-import/v1" {
		if parsed.Input == "" {
			parsed.Input = filepath.ToSlash(path)
		}
		return parsed, nil
	}
	return vnle.ParseImport(path, raw)
}

func loadNLEDiff(projectPath, input string) (vnle.DiffResult, error) {
	path := resolveProjectInputPath(projectPath, input)
	raw, err := os.ReadFile(path)
	if err != nil {
		return vnle.DiffResult{}, err
	}
	var diff vnle.DiffResult
	if err := json.Unmarshal(raw, &diff); err == nil && diff.Version == "vflow-nle-diff/v1" {
		return diff, nil
	}
	var parsed vnle.ImportResult
	if err := json.Unmarshal(raw, &parsed); err == nil && parsed.Version == "vflow-nle-import/v1" {
		return vnle.Classify(parsed), nil
	}
	importResult, err := vnle.ParseImport(path, raw)
	if err != nil {
		return vnle.DiffResult{}, err
	}
	return vnle.Classify(importResult), nil
}

func loadNLEAcceptedReview(projectPath, input string) (vnle.AcceptedReview, error) {
	path := resolveProjectInputPath(projectPath, input)
	raw, err := os.ReadFile(path)
	if err != nil {
		return vnle.AcceptedReview{}, err
	}
	var accepted vnle.AcceptedReview
	if err := json.Unmarshal(raw, &accepted); err != nil {
		return vnle.AcceptedReview{}, err
	}
	if accepted.Version != "vflow-nle-accepted-review/v1" {
		return vnle.AcceptedReview{}, fmt.Errorf("not an accepted NLE review artifact")
	}
	return accepted, nil
}

func resolveProjectInputPath(projectPath, input string) string {
	path := input
	if !filepath.IsAbs(path) {
		projectRelative := filepath.Join(projectPath, input)
		if _, err := os.Stat(projectRelative); err == nil {
			path = projectRelative
		}
	}
	return path
}

func writeRoundtripReviewHTML(path string, data vnle.DiffResult) error {
	var b strings.Builder
	b.WriteString("<!doctype html><meta charset=\"utf-8\"><title>vflow roundtrip review</title><main>")
	b.WriteString("<h1>Roundtrip Review</h1>")
	sections := []struct {
		name    string
		changes []vnle.Change
	}{
		{name: "safe_merge", changes: data.SafeMerge},
		{name: "needs_review", changes: data.NeedsReview},
		{name: "blocked", changes: data.Blocked},
		{name: "unclassified", changes: data.Unclassified},
	}
	for _, section := range sections {
		b.WriteString("<section><h2>")
		b.WriteString(section.name)
		b.WriteString("</h2><pre>")
		raw, _ := json.MarshalIndent(section.changes, "", "  ")
		b.Write(raw)
		b.WriteString("</pre></section>")
	}
	b.WriteString("</main>")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func nleAcceptCommand(opts *globalOptions) *cobra.Command {
	var projectPath, input, outputPath, reviewer, notes string
	var changeIDs []string
	var allNeedsReview bool
	cmd := &cobra.Command{Use: "accept", Short: "write an accepted NLE review artifact", RunE: func(cmd *cobra.Command, args []string) error {
		if input == "" {
			return writeStructuredError(cmd, opts, verrors.Validation("MISSING_INPUT", "missing --import", "Pass --import nle-import.json, nle-diff.json, or raw timeline", false))
		}
		diff, err := loadNLEDiff(projectPath, input)
		if err != nil {
			return writeStructuredError(cmd, opts, verrors.Validation("NLE_ACCEPT_PARSE_FAILED", err.Error(), "Use a supported import or diff artifact", false))
		}
		accepted, err := vnle.BuildAcceptedReview(diff, changeIDs, allNeedsReview, reviewer, notes)
		if err != nil {
			return writeStructuredError(cmd, opts, verrors.Validation("NLE_ACCEPT_SELECTION_FAILED", err.Error(), "Pass --change-id for reviewed changes or --all-needs-review", false))
		}
		if outputPath == "" {
			outputPath = filepath.Join(projectPath, "imports", "accepted-nle-changes.json")
		}
		status := "planned"
		if opts.Commit {
			if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
				return writeStructuredError(cmd, opts, verrors.External("NLE_ACCEPT_WRITE_FAILED", err.Error(), "Check output path permissions", false))
			}
			accepted.Artifact = filepath.ToSlash(outputPath)
			raw, _ := json.MarshalIndent(accepted, "", "  ")
			if err := os.WriteFile(outputPath, append(raw, '\n'), 0o644); err != nil {
				return writeStructuredError(cmd, opts, verrors.External("NLE_ACCEPT_WRITE_FAILED", err.Error(), "Check output path permissions", false))
			}
			status = "written"
		}
		return writeOutput(cmd, opts, "nle accept", map[string]any{"status": status, "artifact": filepath.ToSlash(outputPath), "accepted": accepted})
	}}
	cmd.Flags().StringVar(&projectPath, "project", ".", "project path")
	cmd.Flags().StringVar(&input, "import", "", "import, diff, or raw timeline artifact")
	cmd.Flags().StringVar(&outputPath, "output", "", "accepted review output path")
	cmd.Flags().StringArrayVar(&changeIDs, "change-id", nil, "needs-review change ID to accept; repeatable")
	cmd.Flags().BoolVar(&allNeedsReview, "all-needs-review", false, "accept every needs-review change in the diff")
	cmd.Flags().StringVar(&reviewer, "reviewer", "", "reviewer label")
	cmd.Flags().StringVar(&notes, "notes", "", "review notes")
	return cmd
}

func nleApplyCommand(opts *globalOptions) *cobra.Command {
	var projectPath, input string
	cmd := &cobra.Command{Use: "apply", Short: "apply accepted NLE changes", RunE: func(cmd *cobra.Command, args []string) error {
		if input == "" {
			return writeStructuredError(cmd, opts, verrors.Validation("MISSING_INPUT", "missing --input", "Pass --input accepted changes, nle-diff.json, or nle-import.json", false))
		}
		var plan vnle.ApplyPlan
		if accepted, err := loadNLEAcceptedReview(projectPath, input); err == nil {
			plan = vnle.PlanApplyAccepted(accepted)
		} else {
			diff, diffErr := loadNLEDiff(projectPath, input)
			if diffErr != nil {
				return writeStructuredError(cmd, opts, verrors.Validation("NLE_APPLY_PARSE_FAILED", diffErr.Error(), "Use a supported accepted changes artifact", false))
			}
			plan = vnle.PlanApply(diff, false)
		}
		if opts.Commit {
			switch plan.Status {
			case "blocked":
				return writeStructuredError(cmd, opts, verrors.Safety("blocked NLE changes cannot be applied", "Run nle diff, remove blocked changes, and provide an accepted changes artifact"))
			case "needs_review":
				return writeStructuredError(cmd, opts, verrors.Safety("NLE changes need accepted review before apply", "Accept reviewed changes explicitly before --commit"))
			default:
				plan.Status = "applied"
				path := filepath.Join(projectPath, "imports", "applied-nle-changes.json")
				plan.Artifact = filepath.ToSlash(path)
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					return writeStructuredError(cmd, opts, verrors.External("NLE_APPLY_WRITE_FAILED", err.Error(), "Check project write permissions", false))
				}
				raw, _ := json.MarshalIndent(plan, "", "  ")
				if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
					return writeStructuredError(cmd, opts, verrors.External("NLE_APPLY_WRITE_FAILED", err.Error(), "Check project write permissions", false))
				}
			}
		}
		return writeOutput(cmd, opts, "nle apply", map[string]any{"input": filepath.ToSlash(input), "plan": plan})
	}}
	cmd.Flags().StringVar(&projectPath, "project", ".", "project path")
	cmd.Flags().StringVar(&input, "input", "", "accepted changes artifact")
	return cmd
}

func transcriptCommand(opts *globalOptions) *cobra.Command {
	parent := &cobra.Command{Use: "transcript", Aliases: []string{"transcribe"}, Short: "transcript workflow commands"}
	parent.AddCommand(transcriptCreateCommand(opts), transcriptImportCommand(opts), transcriptAlignCommand(opts), transcriptBakeoffCommand(opts), transcriptSearchCommand(opts), transcriptSyncCommand(opts))
	return parent
}

func transcriptSyncCommand(opts *globalOptions) *cobra.Command {
	var projectPath, syncMapPath, outputPath, anchorID, method, text string
	var transcriptSeconds, referenceSeconds, confidence float64
	cmd := &cobra.Command{
		Use:     "sync",
		Aliases: []string{"sync-transcript-timing"},
		Short:   "record transcript-to-reference timing in a sync map",
		RunE: func(cmd *cobra.Command, args []string) error {
			if syncMapPath == "" {
				syncMapPath = filepath.Join(projectPath, "calibration", "media-sync-map.json")
			}
			if outputPath == "" {
				outputPath = syncMapPath
			}
			m, err := vsyncmap.Read(syncMapPath)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.External("SYNC_MAP_READ_FAILED", err.Error(), "Run media sync first or pass --sync-map", false))
			}
			if anchorID == "" {
				anchorID = "transcript_anchor"
			}
			if method == "" {
				method = "manual_text_anchor"
			}
			if confidence == 0 {
				confidence = 1
			}
			m = vsyncmap.ApplyTranscriptOffset(m, transcriptSeconds, referenceSeconds, anchorID, method, text, confidence)
			validation := m.Validate(vsyncmap.ValidationOptions{})
			status := "planned"
			if opts.Commit {
				if len(validation) > 0 {
					return writeStructuredError(cmd, opts, verrors.Validation("SYNC_MAP_INVALID", strings.Join(validation, "; "), "Fix sync map source metadata before writing", false))
				}
				if err := vsyncmap.Write(outputPath, m); err != nil {
					return writeStructuredError(cmd, opts, verrors.External("SYNC_MAP_WRITE_FAILED", err.Error(), "Check project write permissions", false))
				}
				status = "written"
			}
			return writeOutput(cmd, opts, "transcript sync", map[string]any{
				"status": status, "sync_map": filepath.ToSlash(outputPath), "transcript_to_reference_offset_seconds": m.TranscriptToReferenceOffsetSeconds,
				"validation_errors": validation, "warnings": m.ConfidenceWarnings(), "map": m,
			})
		},
	}
	cmd.Flags().StringVar(&projectPath, "project", ".", "project path")
	cmd.Flags().StringVar(&syncMapPath, "sync-map", "", "input sync map path")
	cmd.Flags().StringVar(&outputPath, "output", "", "output sync map path")
	cmd.Flags().Float64Var(&transcriptSeconds, "transcript-seconds", 0, "known transcript timestamp")
	cmd.Flags().Float64Var(&referenceSeconds, "reference-seconds", 0, "matching reference source timestamp")
	cmd.Flags().StringVar(&anchorID, "anchor-id", "", "anchor id")
	cmd.Flags().StringVar(&method, "method", "", "anchor method")
	cmd.Flags().StringVar(&text, "matched-text", "", "matched anchor text")
	cmd.Flags().Float64Var(&confidence, "confidence", 1, "anchor confidence")
	return cmd
}

func transcriptImportCommand(opts *globalOptions) *cobra.Command {
	var projectPath, provider, input, rate string
	cmd := &cobra.Command{
		Use:     "import",
		Aliases: []string{"load-transcript", "ingest-transcript"},
		Short:   "import transcript data into canonical words.json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !validProvider(provider) {
				return writeStructuredError(cmd, opts, verrors.Validation("INVALID_ENUM", "unsupported provider", "Use one of: plain-text, generic-words, local", false))
			}
			if input == "" {
				return writeStructuredError(cmd, opts, verrors.Validation("MISSING_INPUT", "missing --input", "Pass --input transcript file path", false))
			}
			raw, err := os.ReadFile(input)
			if err != nil {
				return err
			}
			words, err := vtranscript.Import(provider, raw, vtranscript.ImportOptions{Rate: rate, FramesPerWord: 15})
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.Validation("TRANSCRIPT_IMPORT_FAILED", err.Error(), "Check provider and input shape", false))
			}
			status := "planned"
			if opts.Commit {
				if err := vtranscript.WriteWords(projectPath, words); err != nil {
					return err
				}
				status = "written"
			}
			return writeOutput(cmd, opts, "transcript import", map[string]any{
				"status":     status,
				"provider":   provider,
				"word_count": len(words.Words),
				"artifact":   filepath.ToSlash(filepath.Join(projectPath, "transcript", "words.json")),
				"words":      words,
			})
		},
	}
	cmd.Flags().StringVar(&projectPath, "project", ".", "project path")
	cmd.Flags().StringVar(&provider, "provider", "generic-words", "input provider shape")
	cmd.Flags().StringVar(&input, "input", "", "input transcript path")
	cmd.Flags().StringVar(&rate, "rate", "30000/1001", "source frame rate")
	return cmd
}

func transcriptCreateCommand(opts *globalOptions) *cobra.Command {
	var projectPath, provider, source, model, rate string
	cmd := &cobra.Command{
		Use:     "create",
		Aliases: []string{"transcribe", "speech-to-text", "stt"},
		Short:   "create a transcript with a local or live provider",
		RunE: func(cmd *cobra.Command, args []string) error {
			if provider != "" && !validProvider(provider) {
				return writeStructuredError(cmd, opts, verrors.Validation(
					"INVALID_ENUM",
					"unsupported provider",
					"Use one of: local, plain-text, generic-words, elevenlabs, soniox, assemblyai, deepgram, gladia, openai",
					false,
				))
			}
			if source == "" {
				source = filepath.Join(projectPath, "media", "source.mp4")
			}
			if liveTranscriptProvider(provider) {
				env := providerEnv(provider)
				if !opts.Live {
					return writeOutput(cmd, opts, "transcript create", map[string]any{
						"status":        "ready",
						"provider":      provider,
						"requires_live": true,
						"requires_key":  env,
						"source":        filepath.ToSlash(source),
						"dry_run_payload": map[string]any{
							"source": source,
							"model":  firstNonEmptyString(model, defaultTranscriptModel(provider)),
							"writes": []string{"transcript/words.json", "transcript/" + provider + "-transcription.json"},
						},
					})
				}
				if !opts.Commit {
					return writeStructuredError(cmd, opts, verrors.Safety("live "+provider+" transcription requires --commit", "Pass --live --commit to spend provider quota"))
				}
				key := os.Getenv(env)
				if key == "" {
					return writeStructuredError(cmd, opts, verrors.Validation("MISSING_API_KEY", env+" is not set", "Use runtime env or Secret Gate; do not commit secrets", true))
				}
				ctx, cancel, err := commandContext(opts.Timeout)
				if err != nil {
					return writeStructuredError(cmd, opts, verrors.Validation("INVALID_TIMEOUT", err.Error(), "Use a Go duration such as 30s, 5m, or 20m", false))
				}
				defer cancel()
				tx, err := vtranscript.TranscribeProvider(ctx, provider, vtranscript.LiveTranscribeOptions{
					APIKey:        key,
					AudioPath:     source,
					Model:         model,
					Rate:          rate,
					SourceMediaID: "source",
					PollInterval:  2 * time.Second,
				})
				if err != nil {
					return writeStructuredError(cmd, opts, verrors.External(strings.ToUpper(provider)+"_TRANSCRIPTION_FAILED", err.Error(), "Check source file, model, account, provider quota, and --timeout", true))
				}
				if err := vtranscript.WriteWords(projectPath, tx.Words); err != nil {
					return writeStructuredError(cmd, opts, verrors.External("TRANSCRIPT_WRITE_FAILED", err.Error(), "Check project write permissions", false))
				}
				reportPath := filepath.Join(projectPath, "transcript", provider+"-transcription.json")
				raw, _ := json.MarshalIndent(tx, "", "  ")
				_ = os.MkdirAll(filepath.Dir(reportPath), 0o755)
				_ = os.WriteFile(reportPath, append(raw, '\n'), 0o644)
				return writeOutput(cmd, opts, "transcript create", map[string]any{
					"status":     "written",
					"provider":   provider,
					"model":      tx.Model,
					"job_id":     tx.JobID,
					"source":     filepath.ToSlash(source),
					"word_count": len(tx.Words.Words),
					"artifact":   filepath.ToSlash(filepath.Join(projectPath, "transcript", "words.json")),
					"report":     filepath.ToSlash(reportPath),
				})
			}
			env := providerEnv(provider)
			return writeOutput(cmd, opts, "transcript create", map[string]any{
				"status":      "provider_not_live_enabled",
				"provider":    provider,
				"source":      filepath.ToSlash(source),
				"env_var":     env,
				"env_present": env != "" && os.Getenv(env) != "",
				"hint":        "Use transcript import for local fixtures or --provider openai --live --commit for live STT in this build.",
			})
		},
	}
	cmd.Flags().StringVar(&projectPath, "project", ".", "project path")
	cmd.Flags().StringVar(&provider, "provider", "local", "transcript provider")
	cmd.Flags().StringVar(&source, "source", "", "audio or video source path")
	cmd.Flags().StringVar(&source, "audio", "", "alias for --source")
	cmd.Flags().StringVar(&model, "model", "", "provider model")
	cmd.Flags().StringVar(&rate, "rate", "30000/1001", "source frame rate")
	return cmd
}

func transcriptAlignCommand(opts *globalOptions) *cobra.Command {
	var projectPath string
	cmd := &cobra.Command{
		Use:     "align",
		Aliases: []string{"sync-transcript", "align-words", "word-align"},
		Short:   "write a transcript alignment summary",
		RunE: func(cmd *cobra.Command, args []string) error {
			words, err := vtranscript.ReadWords(projectPath)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.External("TRANSCRIPT_READ_FAILED", err.Error(), "Create or import transcript/words.json first", false))
			}
			data := map[string]any{
				"version":    "vflow-transcript-alignment/v1",
				"status":     "aligned",
				"rate":       words.Rate,
				"word_count": len(words.Words),
				"method":     "canonical-word-frames",
			}
			if opts.Commit {
				path := filepath.Join(projectPath, "transcript", "alignment.json")
				raw, _ := json.MarshalIndent(data, "", "  ")
				if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
					return writeStructuredError(cmd, opts, verrors.External("ALIGNMENT_WRITE_FAILED", err.Error(), "Check project write permissions", false))
				}
				data["artifact"] = filepath.ToSlash(path)
			}
			return writeOutput(cmd, opts, "transcript align", data)
		},
	}
	cmd.Flags().StringVar(&projectPath, "project", ".", "project path")
	return cmd
}

func transcriptBakeoffCommand(opts *globalOptions) *cobra.Command {
	var providers, projectPath, source, model, rate string
	cmd := &cobra.Command{
		Use:   "bakeoff",
		Short: "compare transcript provider readiness",
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.Live && !opts.Commit {
				return writeStructuredError(cmd, opts, verrors.Safety("live transcript bakeoff requires --commit", "Pass --live --commit to spend provider quota, or omit --live for dry-run readiness"))
			}
			if source == "" {
				source = filepath.Join(projectPath, "media", "source.mp4")
			}
			names := splitCSV(firstNonEmptyString(providers, "openai,elevenlabs,soniox,assemblyai,deepgram,gladia,local"))
			results := make([]map[string]any, 0, len(names))
			for _, name := range names {
				env := providerEnv(name)
				result := map[string]any{
					"provider":     name,
					"env_var":      env,
					"env_present":  env == "" || os.Getenv(env) != "",
					"live_enabled": opts.Live,
					"capabilities": providerCapabilities(name),
					"payload": map[string]any{
						"source": source,
						"model":  firstNonEmptyString(model, defaultTranscriptModel(name)),
					},
				}
				switch {
				case !validProvider(name):
					result["status"] = "invalid_provider"
				case !opts.Live:
					result["status"] = "ready"
				case !liveTranscriptProvider(name):
					result["status"] = "local_import_only"
				case env == "" || os.Getenv(env) == "":
					result["status"] = "skipped_missing_key"
				default:
					ctx, cancel, err := commandContext(opts.Timeout)
					if err != nil {
						return writeStructuredError(cmd, opts, verrors.Validation("INVALID_TIMEOUT", err.Error(), "Use a Go duration such as 30s, 5m, or 20m", false))
					}
					tx, err := vtranscript.TranscribeProvider(ctx, name, vtranscript.LiveTranscribeOptions{
						APIKey:        os.Getenv(env),
						AudioPath:     source,
						Model:         model,
						Rate:          rate,
						SourceMediaID: "source",
						PollInterval:  2 * time.Second,
					})
					cancel()
					if err != nil {
						result["status"] = "failed"
						result["error"] = err.Error()
						result["retryable"] = true
					} else {
						result["status"] = "completed"
						result["model"] = tx.Model
						result["job_id"] = tx.JobID
						result["word_count"] = len(tx.Words.Words)
						result["text_sample"] = trimText(tx.Text, 160)
					}
				}
				results = append(results, result)
			}
			data := map[string]any{
				"version":   "vflow-provider-bakeoff/v1",
				"status":    "checked",
				"live":      opts.Live,
				"source":    filepath.ToSlash(source),
				"providers": results,
			}
			if opts.Commit {
				reportPath := filepath.Join(projectPath, "reports", "provider-bakeoff.json")
				raw, _ := json.MarshalIndent(data, "", "  ")
				if err := os.MkdirAll(filepath.Dir(reportPath), 0o755); err != nil {
					return writeStructuredError(cmd, opts, verrors.External("BAKEOFF_REPORT_WRITE_FAILED", err.Error(), "Check project reports directory permissions", false))
				}
				if err := os.WriteFile(reportPath, append(raw, '\n'), 0o644); err != nil {
					return writeStructuredError(cmd, opts, verrors.External("BAKEOFF_REPORT_WRITE_FAILED", err.Error(), "Check project reports directory permissions", false))
				}
				data["report"] = filepath.ToSlash(reportPath)
			}
			return writeOutput(cmd, opts, "transcript bakeoff", data)
		},
	}
	cmd.Flags().StringVar(&providers, "providers", "", "comma-separated providers")
	cmd.Flags().StringVar(&projectPath, "project", ".", "project path")
	cmd.Flags().StringVar(&source, "source", "", "audio or video source path")
	cmd.Flags().StringVar(&source, "audio", "", "alias for --source")
	cmd.Flags().StringVar(&model, "model", "", "provider model")
	cmd.Flags().StringVar(&rate, "rate", "30000/1001", "source frame rate")
	return cmd
}

func transcriptSearchCommand(opts *globalOptions) *cobra.Command {
	var projectPath, query, dataSource, indexPath string
	cmd := &cobra.Command{
		Use:   "search",
		Short: "search canonical transcript words",
		RunE: func(cmd *cobra.Command, args []string) error {
			if query == "" {
				return writeStructuredError(cmd, opts, verrors.Validation("MISSING_QUERY", "missing --query", "Pass --query text", false))
			}
			if dataSource == "local" || dataSource == "index" {
				result, err := vindex.SearchTranscripts(cmd.Context(), vindex.SearchOptions{ProjectPath: projectPath, DBPath: indexPath, Query: query, Limit: opts.Limit})
				if err != nil {
					return writeStructuredError(cmd, opts, verrors.External("TRANSCRIPT_INDEX_SEARCH_FAILED", err.Error(), "Run vflow project index --path <project> --commit first", false))
				}
				return writeOutput(cmd, opts, "transcript search", result)
			}
			if dataSource != "" && dataSource != "project" && dataSource != "canonical" {
				return writeStructuredError(cmd, opts, verrors.Validation("INVALID_ENUM", "unsupported data source", "Use one of: project, canonical, local", false))
			}
			words, err := vtranscript.ReadWords(projectPath)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.External("TRANSCRIPT_READ_FAILED", err.Error(), "Create or import transcript/words.json first", false))
			}
			q := strings.ToLower(query)
			matches := []vtranscript.Word{}
			for _, word := range words.Words {
				if strings.Contains(strings.ToLower(word.Text), q) {
					matches = append(matches, word)
					if len(matches) >= opts.Limit {
						break
					}
				}
			}
			return writeOutput(cmd, opts, "transcript search", map[string]any{"query": query, "data_source": "project", "count": len(matches), "matches": matches})
		},
	}
	cmd.Flags().StringVar(&projectPath, "project", ".", "project path")
	cmd.Flags().StringVar(&query, "query", "", "search query")
	cmd.Flags().StringVar(&dataSource, "data-source", "project", "search data source: project, canonical, local")
	cmd.Flags().StringVar(&indexPath, "index", "", "SQLite index path, default $VFLOW_INDEX_PATH or ~/.vflow/index.sqlite")
	return cmd
}

func validProvider(provider string) bool {
	switch provider {
	case "local", "plain-text", "generic-words", "elevenlabs", "soniox", "assemblyai", "deepgram", "gladia", "openai":
		return true
	default:
		return false
	}
}

func providerEnv(provider string) string {
	switch provider {
	case "openai":
		return "OPENAI_API_KEY"
	case "elevenlabs":
		return "ELEVENLABS_API_KEY"
	case "soniox":
		return "SONIOX_API_KEY"
	case "assemblyai":
		return "ASSEMBLYAI_API_KEY"
	case "deepgram":
		return "DEEPGRAM_API_KEY"
	case "gladia":
		return "GLADIA_API_KEY"
	case "local", "plain-text", "generic-words":
		return ""
	default:
		return ""
	}
}

func liveTranscriptProvider(provider string) bool {
	switch provider {
	case "openai", "elevenlabs", "soniox", "assemblyai", "deepgram", "gladia":
		return true
	default:
		return false
	}
}

func defaultTranscriptModel(provider string) string {
	switch provider {
	case "openai":
		return vtranscript.DefaultOpenAITranscribeModel
	case "elevenlabs":
		return vtranscript.DefaultElevenLabsModel
	case "deepgram":
		return vtranscript.DefaultDeepgramModel
	case "assemblyai":
		return vtranscript.DefaultAssemblyAIModel
	case "gladia":
		return vtranscript.DefaultGladiaModel
	case "soniox":
		return vtranscript.DefaultSonioxModel
	default:
		return ""
	}
}

func providerCapabilities(provider string) []string {
	switch provider {
	case "openai":
		return []string{"speech_to_text", "json_text", "optional_diarized_model"}
	case "elevenlabs", "deepgram":
		return []string{"speech_to_text", "word_timestamps", "diarization", "live_adapter"}
	case "soniox", "assemblyai", "gladia":
		return []string{"speech_to_text", "word_timestamps", "diarization", "async_polling", "live_adapter"}
	case "local", "plain-text", "generic-words":
		return []string{"import", "no_api_key"}
	default:
		return nil
	}
}

func authDoctorResults(provider, model string, live bool) ([]map[string]any, error) {
	names, err := authProviderNames(provider)
	if err != nil {
		return nil, err
	}
	results := make([]map[string]any, 0, len(names))
	for _, name := range names {
		result := authProviderResult(name, model, live)
		results = append(results, result)
	}
	return results, nil
}

func authProviderNames(provider string) ([]string, error) {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "", "all":
		return []string{"openai", "elevenlabs", "soniox", "assemblyai", "deepgram", "gladia", "gemini", "anthropic", "huggingface", "local"}, nil
	case "hf", "huggingface":
		return []string{"huggingface"}, nil
	case "gemini", "openai", "elevenlabs", "soniox", "assemblyai", "deepgram", "gladia", "anthropic", "local", "plain-text", "generic-words":
		return []string{strings.ToLower(strings.TrimSpace(provider))}, nil
	default:
		return nil, fmt.Errorf("unsupported auth provider %q", provider)
	}
}

func authProviderResult(provider, model string, live bool) map[string]any {
	envVars := authProviderEnvVars(provider)
	envPresent := make(map[string]bool, len(envVars))
	keyPresent := len(envVars) == 0
	keySource := ""
	for _, env := range envVars {
		present := os.Getenv(env) != ""
		envPresent[env] = present
		if present && keySource == "" {
			keyPresent = true
			keySource = "env:" + env
		}
	}
	status := "ready"
	if !keyPresent {
		status = "missing_key"
	}
	result := map[string]any{
		"provider":      provider,
		"status":        status,
		"key_present":   keyPresent,
		"env_vars":      envVars,
		"env_present":   envPresent,
		"capabilities":  authProviderCapabilities(provider),
		"live_checked":  false,
		"quota_safe":    !live,
		"secret_policy": "env_reference_only",
	}
	if keySource != "" {
		result["key_source"] = keySource
	}
	if defaultModel := authProviderDefaultModel(provider); defaultModel != "" {
		result["default_model"] = defaultModel
	}
	if provider == "gemini" {
		key, source := vqa.APIKeyFromEnv()
		doctor, err := vqa.DoctorWithKey(model, live, key, source)
		if err != nil {
			result["status"] = "failed"
			result["error"] = err.Error()
			result["retryable"] = true
			return result
		}
		result["status"] = "ready"
		if !doctor.KeyPresent {
			result["status"] = "missing_key"
		}
		result["key_present"] = doctor.KeyPresent
		result["selected_model"] = doctor.SelectedModel
		result["model_available"] = doctor.ModelAvailable
		result["live_checked"] = live && doctor.KeyPresent
		if doctor.KeySource != "" {
			result["key_source"] = doctor.KeySource
		}
		if len(doctor.AvailableModels) > 0 {
			result["available_models"] = doctor.AvailableModels
		}
	}
	return result
}

func authProviderEnvVars(provider string) []string {
	switch provider {
	case "gemini":
		return []string{"GEMINI_API_KEY", "GOOGLE_API_KEY", "GOOGLE_GENERATIVE_AI_API_KEY"}
	case "anthropic":
		return []string{"ANTHROPIC_API_KEY"}
	case "huggingface":
		return []string{"HF_TOKEN", "HUGGINGFACE_TOKEN"}
	case "local", "plain-text", "generic-words":
		return nil
	default:
		if env := providerEnv(provider); env != "" {
			return []string{env}
		}
		return nil
	}
}

func authProviderCapabilities(provider string) []string {
	switch provider {
	case "gemini":
		return []string{"video_qa", "files_api", "model_listing", "color_review"}
	case "anthropic":
		return []string{"agent_suggestions", "text_review"}
	case "huggingface":
		return []string{"local_diarization_models", "gated_model_access"}
	default:
		return providerCapabilities(provider)
	}
}

func authProviderDefaultModel(provider string) string {
	switch provider {
	case "gemini":
		return vqa.DefaultFastModel
	default:
		return defaultTranscriptModel(provider)
	}
}

func commandContext(timeoutValue string) (context.Context, context.CancelFunc, error) {
	timeoutValue = strings.TrimSpace(timeoutValue)
	if timeoutValue == "" {
		ctx, cancel := context.WithCancel(context.Background())
		return ctx, cancel, nil
	}
	timeout, err := time.ParseDuration(timeoutValue)
	if err != nil {
		return nil, nil, err
	}
	if timeout <= 0 {
		return nil, nil, fmt.Errorf("timeout must be positive")
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	return ctx, cancel, nil
}

func projectRelativePath(projectPath, path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(projectPath, path)
}

func splitCSV(value string) []string {
	fields := strings.Split(value, ",")
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field != "" {
			out = append(out, field)
		}
	}
	return out
}
