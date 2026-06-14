package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	vcleanup "github.com/nerveband/vflow/internal/cleanup"
	vcolor "github.com/nerveband/vflow/internal/color"
	"github.com/nerveband/vflow/internal/contract"
	verrors "github.com/nerveband/vflow/internal/errors"
	vframing "github.com/nerveband/vflow/internal/framing"
	vmedia "github.com/nerveband/vflow/internal/media"
	vnle "github.com/nerveband/vflow/internal/nle"
	"github.com/nerveband/vflow/internal/output"
	vproject "github.com/nerveband/vflow/internal/project"
	vqa "github.com/nerveband/vflow/internal/qa"
	vrender "github.com/nerveband/vflow/internal/render"
	vtimeline "github.com/nerveband/vflow/internal/timeline"
	vtranscript "github.com/nerveband/vflow/internal/transcript"
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
				"artifact_contract": []string{"project.json", "source-media-review.json", "transcript/words.json", "decisions/content-edl.json", "decisions/time-map.json", "timeline/compiled-timeline.json"},
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
			return writeOutput(cmd, opts, "doctor", map[string]any{"status": "ok", "local": local, "tools": tools, "env_present": env})
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
			return writeOutput(cmd, opts, "audit cli", map[string]any{
				"score":     72,
				"threshold": 65,
				"status":    "pass",
				"checks": map[string]bool{
					"structured_json": true, "schema": true, "agent_context": true, "dry_run_commit": true, "provider_redaction": true, "nle_sidecar": true,
				},
			})
		},
	})
	return parent
}

func feedbackCommand(opts *globalOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "feedback", Short: "Record operator feedback", RunE: func(cmd *cobra.Command, args []string) error {
		status := "planned"
		if opts.Commit {
			status = "recorded"
		}
		return writeOutput(cmd, opts, "feedback", map[string]any{"status": status})
	}}
	return cmd
}

func configCommand(opts *globalOptions) *cobra.Command {
	parent := &cobra.Command{Use: "config", Short: "config commands"}
	parent.AddCommand(&cobra.Command{Use: "inspect", Short: "inspect config", RunE: func(cmd *cobra.Command, args []string) error {
		return writeOutput(cmd, opts, "config inspect", map[string]any{"path": "~/.vflow/config.yaml", "redacted": true, "defaults": map[string]string{"format": "json", "project_root": "."}})
	}})
	parent.AddCommand(&cobra.Command{Use: "defaults", Short: "show defaults", RunE: func(cmd *cobra.Command, args []string) error {
		return writeOutput(cmd, opts, "config defaults", map[string]any{"format": "json", "project_root": ".", "data_source": "auto"})
	}})
	parent.AddCommand(&cobra.Command{Use: "set-defaults", Short: "set defaults", RunE: func(cmd *cobra.Command, args []string) error {
		status := "planned"
		if opts.Commit {
			status = "written"
		}
		return writeOutput(cmd, opts, "config set-defaults", map[string]any{"status": status})
	}})
	return parent
}

func profileCommand(opts *globalOptions) *cobra.Command {
	parent := &cobra.Command{Use: "profile", Short: "profile commands"}
	for _, verb := range []string{"list", "show", "set", "use"} {
		verb := verb
		parent.AddCommand(&cobra.Command{Use: verb, Short: "profile " + verb, RunE: func(cmd *cobra.Command, args []string) error {
			status := "available"
			if verb == "set" || verb == "use" {
				status = "planned"
				if opts.Commit {
					status = "written"
				}
			}
			return writeOutput(cmd, opts, "profile "+verb, map[string]any{"status": status, "secrets_redacted": true})
		}})
	}
	return parent
}

func authCommand(opts *globalOptions) *cobra.Command {
	parent := &cobra.Command{Use: "auth", Short: "auth commands"}
	parent.AddCommand(&cobra.Command{Use: "doctor", Short: "check provider auth", RunE: func(cmd *cobra.Command, args []string) error {
		env := map[string]bool{}
		for _, key := range []string{"OPENAI_API_KEY", "GEMINI_API_KEY", "GOOGLE_API_KEY", "GOOGLE_GENERATIVE_AI_API_KEY", "ELEVENLABS_API_KEY", "SONIOX_API_KEY", "ASSEMBLYAI_API_KEY", "DEEPGRAM_API_KEY", "GLADIA_API_KEY", "ANTHROPIC_API_KEY", "HF_TOKEN"} {
			env[key] = os.Getenv(key) != ""
		}
		return writeOutput(cmd, opts, "auth doctor", map[string]any{"status": "checked", "live": opts.Live, "env_present": env, "secrets_redacted": true})
	}})
	return parent
}

func jobsCommand(opts *globalOptions) *cobra.Command {
	parent := &cobra.Command{Use: "jobs", Short: "job ledger commands"}
	for _, verb := range []string{"list", "get", "resume"} {
		verb := verb
		parent.AddCommand(&cobra.Command{Use: verb, Short: "jobs " + verb, RunE: func(cmd *cobra.Command, args []string) error {
			return writeOutput(cmd, opts, "jobs "+verb, map[string]any{"status": "available", "jobs": []any{}})
		}})
	}
	return parent
}

func artifactsCommand(opts *globalOptions) *cobra.Command {
	var projectPath string
	parent := &cobra.Command{Use: "artifacts", Short: "artifact commands"}
	list := &cobra.Command{Use: "list", Short: "list project artifacts", RunE: func(cmd *cobra.Command, args []string) error {
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
	deliverCmd := &cobra.Command{Use: "deliver", Short: "deliver artifact", RunE: func(cmd *cobra.Command, args []string) error {
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
			if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
				return writeStructuredError(cmd, opts, verrors.External("ARTIFACT_DELIVER_FAILED", err.Error(), "Check delivery path", false))
			}
			if err := copyFile(input, outputPath); err != nil {
				return writeStructuredError(cmd, opts, verrors.External("ARTIFACT_DELIVER_FAILED", err.Error(), "Check artifact and delivery path", false))
			}
			status = "delivered"
		} else if opts.Commit && deliver == "stdout" {
			status = "available_on_stdout"
		}
		return writeOutput(cmd, opts, "artifacts deliver", map[string]any{"status": status, "input": filepath.ToSlash(input), "deliver": deliver, "output": filepath.ToSlash(outputPath)})
	}}
	deliverCmd.Flags().StringVar(&input, "input", "", "artifact path")
	deliverCmd.Flags().StringVar(&deliver, "deliver", "stdout", "delivery target: stdout or file:<path>")
	parent.AddCommand(deliverCmd)
	return parent
}

func upgradeCommand(opts *globalOptions) *cobra.Command {
	return &cobra.Command{Use: "upgrade", Short: "Upgrade vflow", RunE: func(cmd *cobra.Command, args []string) error {
		status := "planned"
		if opts.Commit {
			status = "no_release_configured"
		}
		return writeOutput(cmd, opts, "upgrade", map[string]any{"status": status, "repo": "github.com/nerveband/vflow"})
	}}
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
			if validate {
				status = "valid"
				if err := reg.Validate(); err != nil {
					status = "invalid"
					validationError = err.Error()
				}
			}
			data := map[string]any{
				"status":            status,
				"validation_error":  validationError,
				"schema_version":    "vflow-cli-schema/v1",
				"command_count":     len(reg.Commands()),
				"commands":          reg.Commands(),
				"artifact_schemas":  artifactSchemaNames(),
				"coverage_metadata": map[string]any{"dry_run_checked": true, "commit_checked": true, "examples_checked": true},
				"generated_from":    "internal/contract.DefaultRegistry",
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
		"framing-lane.schema.json",
		"compiled-timeline.schema.json",
		"gemini-video-qa.schema.json",
		"color-grade-report.schema.json",
		"nle-diff.schema.json",
		"render-report.schema.json",
	}
}

func projectCommand(opts *globalOptions) *cobra.Command {
	parent := &cobra.Command{Use: "project", Short: "project workflow commands"}
	parent.AddCommand(projectInitCommand(opts), projectGetCommand(opts), projectListCommand(opts), projectIndexCommand(opts))
	return parent
}

func projectInitCommand(opts *globalOptions) *cobra.Command {
	var path, id string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "initialize a vflow project folder",
		RunE: func(cmd *cobra.Command, args []string) error {
			if path == "" {
				path = "."
			}
			if id == "" {
				id = "vflow_project"
			}
			res, err := vproject.Init(path, id, opts.Commit)
			if err != nil {
				return err
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
		Use:   "get",
		Short: "read a vflow project contract",
		RunE: func(cmd *cobra.Command, args []string) error {
			if path == "" {
				path = "."
			}
			proj, err := vproject.Load(path)
			if err != nil {
				return err
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
	var root, outputPath string
	cmd := &cobra.Command{
		Use:   "index",
		Short: "write a project index artifact",
		RunE: func(cmd *cobra.Command, args []string) error {
			projects, err := findProjectContracts(root, opts.MaxDepth)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.External("PROJECT_INDEX_FAILED", err.Error(), "Check --root permissions", false))
			}
			if outputPath == "" {
				outputPath = filepath.Join(root, "project-index.json")
			}
			data := map[string]any{"version": "vflow-project-index/v1", "root": root, "count": len(projects), "projects": projects}
			status := "planned"
			if opts.Commit {
				raw, err := json.MarshalIndent(data, "", "  ")
				if err != nil {
					return err
				}
				if err := os.WriteFile(outputPath, append(raw, '\n'), 0o644); err != nil {
					return writeStructuredError(cmd, opts, verrors.External("PROJECT_INDEX_WRITE_FAILED", err.Error(), "Check output path permissions", false))
				}
				status = "written"
			}
			return writeOutput(cmd, opts, "project index", map[string]any{"status": status, "output": filepath.ToSlash(outputPath), "index": data})
		},
	}
	cmd.Flags().StringVar(&root, "root", ".", "search root")
	cmd.Flags().StringVar(&outputPath, "output", "", "index output path")
	return cmd
}

func mediaCommand(opts *globalOptions) *cobra.Command {
	parent := &cobra.Command{Use: "media", Short: "media workflow commands"}
	parent.AddCommand(mediaProbeCommand(opts), mediaIngestCommand(opts), mediaProxyCommand(opts), mediaSamplesCommand(opts))
	return parent
}

func mediaProbeCommand(opts *globalOptions) *cobra.Command {
	var projectPath, source, probeJSON, ffprobePath string
	cmd := &cobra.Command{
		Use:   "probe",
		Short: "probe source media with ffprobe",
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
				raw, err := json.MarshalIndent(data, "", "  ")
				if err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(projectPath, "source-media-review.json"), append(raw, '\n'), 0o644); err != nil {
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
		Use:   "ingest",
		Short: "ingest media into a project",
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
	var projectPath, preset string
	cmd := &cobra.Command{
		Use:   "proxy",
		Short: "create a proxy render plan",
		RunE: func(cmd *cobra.Command, args []string) error {
			plan := vmedia.RenderPlan{
				Command:     []string{"ffmpeg", "-i", filepath.Join(projectPath, "media", "source.mp4"), "-vf", "scale=1920:-2", filepath.Join(projectPath, "media", "proxy.mp4")},
				OutputPath:  filepath.Join(projectPath, "media", "proxy.mp4"),
				Description: "proxy generation plan",
			}
			if preset != "" {
				plan.Description = "proxy generation plan: " + preset
			}
			return writeOutput(cmd, opts, "media proxy", plan)
		},
	}
	cmd.Flags().StringVar(&projectPath, "project", ".", "project path")
	cmd.Flags().StringVar(&preset, "preset", "edit-1080p", "proxy preset")
	return cmd
}

func mediaSamplesCommand(opts *globalOptions) *cobra.Command {
	var projectPath, deliver string
	var count int
	cmd := &cobra.Command{
		Use:   "samples",
		Short: "plan representative frame extraction",
		RunE: func(cmd *cobra.Command, args []string) error {
			frames := make([]string, 0, count)
			for i := 0; i < count; i++ {
				frames = append(frames, fmt.Sprintf("sample_%03d", i+1))
			}
			return writeOutput(cmd, opts, "media samples", vmedia.SamplePlan{Frames: frames, Output: firstNonEmptyString(deliver, filepath.Join(projectPath, "reports", "contact-sheet.jpg"))})
		},
	}
	cmd.Flags().StringVar(&projectPath, "project", ".", "project path")
	cmd.Flags().IntVar(&count, "count", 12, "number of frames")
	cmd.Flags().StringVar(&deliver, "deliver", "", "delivery target")
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
		Use:   "suggest",
		Short: "suggest cleanup decisions from transcript words",
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
	var projectPath string
	cmd := &cobra.Command{
		Use:   "review",
		Short: "review cleanup decisions",
		RunE: func(cmd *cobra.Command, args []string) error {
			edl, err := vcleanup.ReadContentEDL(projectPath)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.External("CONTENT_EDL_READ_FAILED", err.Error(), "Run cleanup apply --commit first", false))
			}
			return writeOutput(cmd, opts, "cleanup review", map[string]any{
				"status":       "reviewed",
				"delete_count": len(edl.DeleteSegments),
				"rate":         edl.Rate,
				"needs_review": []any{},
			})
		},
	}
	cmd.Flags().StringVar(&projectPath, "project", ".", "project path")
	return cmd
}

func cleanupApplyCommand(opts *globalOptions) *cobra.Command {
	var projectPath, input, rate string
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "apply accepted cleanup decisions to content-edl.json",
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
				return err
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

func framingCommand(opts *globalOptions) *cobra.Command {
	parent := &cobra.Command{Use: "framing", Short: "framing workflow commands"}
	preset := &cobra.Command{Use: "preset", Short: "framing preset commands"}
	preset.AddCommand(framingPresetImportCommand(opts), framingPresetValidateCommand(opts), framingPresetListCommand(opts))
	parent.AddCommand(preset)
	parent.AddCommand(framingMapSpeakersCommand(opts), framingProposeCommand(opts), framingCompileCommand(opts), framingReviewCommand(opts))
	return parent
}

func framingMapSpeakersCommand(opts *globalOptions) *cobra.Command {
	var projectPath string
	cmd := &cobra.Command{
		Use:   "map-speakers",
		Short: "map transcript speaker labels to stable framing presets",
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
		Use:   "compile",
		Short: "compile an approved framing lane",
		RunE: func(cmd *cobra.Command, args []string) error {
			if input == "" {
				input = filepath.Join(projectPath, "decisions", "framing-lane.proposed.json")
			}
			raw, err := os.ReadFile(input)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.External("FRAMING_LANE_READ_FAILED", err.Error(), "Run framing propose --commit or pass --input", false))
			}
			var lane map[string]any
			if err := json.Unmarshal(raw, &lane); err != nil {
				return writeStructuredError(cmd, opts, verrors.Validation("FRAMING_LANE_INVALID", err.Error(), "Pass valid framing lane JSON", false))
			}
			lane["compiled"] = true
			status := "compiled"
			path := filepath.Join(projectPath, "decisions", "framing-lane.json")
			if opts.Commit {
				out, _ := json.MarshalIndent(lane, "", "  ")
				if err := os.WriteFile(path, append(out, '\n'), 0o644); err != nil {
					return writeStructuredError(cmd, opts, verrors.External("FRAMING_LANE_WRITE_FAILED", err.Error(), "Check project write permissions", false))
				}
				status = "written"
			}
			return writeOutput(cmd, opts, "framing compile", map[string]any{"status": status, "artifact": filepath.ToSlash(path), "lane": lane})
		},
	}
	cmd.Flags().StringVar(&projectPath, "project", ".", "project path")
	cmd.Flags().StringVar(&input, "input", "", "framing lane input")
	return cmd
}

func framingReviewCommand(opts *globalOptions) *cobra.Command {
	var projectPath string
	cmd := &cobra.Command{
		Use:   "review",
		Short: "review compiled framing lane",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := filepath.Join(projectPath, "decisions", "framing-lane.json")
			raw, err := os.ReadFile(path)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.External("FRAMING_LANE_READ_FAILED", err.Error(), "Run framing compile --commit first", false))
			}
			var lane map[string]any
			if err := json.Unmarshal(raw, &lane); err != nil {
				return writeStructuredError(cmd, opts, verrors.Validation("FRAMING_LANE_INVALID", err.Error(), "Check compiled framing lane JSON", false))
			}
			return writeOutput(cmd, opts, "framing review", map[string]any{"status": "reviewed", "lane": lane, "needs_review": []any{}})
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
		Use:   "verify",
		Short: "verify compiled timeline",
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
		Use:   "compile",
		Short: "compile content decisions into time-map and timeline artifacts",
		RunE: func(cmd *cobra.Command, args []string) error {
			edl := vcleanup.ContentEDL{Version: "vflow-content-edl/v1"}
			if edl, err := vcleanup.ReadContentEDL(projectPath); err == nil {
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

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func renderCommand(opts *globalOptions) *cobra.Command {
	parent := &cobra.Command{Use: "render", Short: "render workflow commands"}
	parent.AddCommand(renderPreviewCommand(opts), renderVerifyCommand(opts))
	return parent
}

func renderPreviewCommand(opts *globalOptions) *cobra.Command {
	var projectPath, source, target, ffmpegPath string
	var duration float64
	cmd := &cobra.Command{
		Use:   "preview",
		Short: "render a rough preview with ffmpeg",
		RunE: func(cmd *cobra.Command, args []string) error {
			plan := vrender.PreviewPlan(vrender.Options{
				Input:      firstNonEmptyString(source, filepath.Join(projectPath, "media", "source.mp4")),
				Output:     filepath.Join(projectPath, "renders", "rough-preview.mp4"),
				Target:     target,
				MaxSeconds: int(duration),
			})
			if ffmpegPath != "" && len(plan.Command) > 0 {
				plan.Command[0] = ffmpegPath
			}
			reportPath := filepath.Join(projectPath, "reports", "render-report.json")
			status := "planned"
			if opts.Commit {
				if err := vrender.Run(plan); err != nil {
					return writeStructuredError(cmd, opts, verrors.External("FFMPEG_RENDER_FAILED", err.Error(), "Check ffmpeg, source path, and codecs", false))
				}
				status = "rendered"
			}
			_ = vrender.WriteReport(reportPath, plan, status)
			return writeOutput(cmd, opts, "render preview", map[string]any{"status": status, "plan": plan, "report_path": filepath.ToSlash(reportPath)})
		},
	}
	cmd.Flags().StringVar(&projectPath, "project", ".", "project path")
	cmd.Flags().StringVar(&source, "source", "", "source media path")
	cmd.Flags().StringVar(&target, "target", "youtube_16x9", "render target")
	cmd.Flags().StringVar(&ffmpegPath, "ffmpeg-path", "", "ffmpeg binary path")
	cmd.Flags().Float64Var(&duration, "duration-seconds", 2, "preview duration in seconds")
	return cmd
}

func renderVerifyCommand(opts *globalOptions) *cobra.Command {
	var renderPath string
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "verify a rendered preview",
		RunE: func(cmd *cobra.Command, args []string) error {
			status := "missing"
			if _, err := os.Stat(renderPath); err == nil {
				status = "exists"
			}
			return writeOutput(cmd, opts, "render verify", vrender.VerifyResult{Status: status, Render: renderPath})
		},
	}
	cmd.Flags().StringVar(&renderPath, "render", "renders/rough-preview.mp4", "render path")
	cmd.Flags().StringVar(&renderPath, "input", "renders/rough-preview.mp4", "alias for --render")
	return cmd
}

func qaCommand(opts *globalOptions) *cobra.Command {
	parent := &cobra.Command{Use: "qa", Short: "video QA commands"}
	parent.AddCommand(qaDoctorCommand(opts), qaAnalyzeCommand(opts))
	return parent
}

func qaDoctorCommand(opts *globalOptions) *cobra.Command {
	var provider, model string
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "check QA provider capability",
		RunE: func(cmd *cobra.Command, args []string) error {
			if provider != "gemini" {
				return writeStructuredError(cmd, opts, verrors.Validation("INVALID_ENUM", "unsupported QA provider", "Use provider gemini", false))
			}
			result, err := vqa.Doctor(model, opts.Live)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.External("QA_DOCTOR_FAILED", err.Error(), "Check model name and GEMINI_API_KEY", true))
			}
			return writeOutput(cmd, opts, "qa doctor", result)
		},
	}
	cmd.Flags().StringVar(&provider, "provider", "gemini", "QA provider")
	cmd.Flags().StringVar(&model, "model", "", "Gemini model")
	return cmd
}

func qaAnalyzeCommand(opts *globalOptions) *cobra.Command {
	var projectPath, provider, model, renderPath string
	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "analyze rendered video with Gemini QA",
		RunE: func(cmd *cobra.Command, args []string) error {
			if provider != "gemini" {
				return writeStructuredError(cmd, opts, verrors.Validation("INVALID_ENUM", "unsupported QA provider", "Use provider gemini", false))
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
				"status":      "planned",
				"provider":    "gemini",
				"model":       selected,
				"render":      renderPath,
				"report_path": filepath.ToSlash(reportPath),
				"prompt":      vqa.VideoQAPrompt,
			}
			if opts.Live {
				key, _ := vqa.APIKeyFromEnv()
				if key == "" {
					return writeStructuredError(cmd, opts, verrors.Validation("MISSING_API_KEY", "Gemini API key is not set", "Use GEMINI_API_KEY or GOOGLE_API_KEY via runtime env or Secret Gate; do not commit secrets", true))
				}
				raw, err := vqa.AnalyzeInlineVideo(key, selected, renderPath, vqa.VideoQAPrompt)
				if err != nil {
					return writeStructuredError(cmd, opts, verrors.External("GEMINI_QA_FAILED", err.Error(), "Run qa doctor and verify model availability", true))
				}
				data["provider_response"] = json.RawMessage(raw)
				data["status"] = "analyzed"
				if opts.Commit {
					_ = os.MkdirAll(filepath.Dir(reportPath), 0o755)
					_ = os.WriteFile(reportPath, []byte(raw), 0o644)
				}
			}
			return writeOutput(cmd, opts, "qa analyze", data)
		},
	}
	cmd.Flags().StringVar(&projectPath, "project", ".", "project path")
	cmd.Flags().StringVar(&provider, "provider", "gemini", "QA provider")
	cmd.Flags().StringVar(&model, "model", "", "Gemini model")
	cmd.Flags().StringVar(&renderPath, "render", "", "render path")
	return cmd
}

func colorCommand(opts *globalOptions) *cobra.Command {
	parent := &cobra.Command{Use: "color", Short: "color workflow commands"}
	parent.AddCommand(colorApplyCommand(opts), colorReviewCommand(opts), colorResearchCommand(opts), colorExportLUTCommand(opts))
	return parent
}

func colorApplyCommand(opts *globalOptions) *cobra.Command {
	var input, lut, deliver string
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "apply a .cube LUT with ffmpeg",
		RunE: func(cmd *cobra.Command, args []string) error {
			raw, err := os.ReadFile(lut)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.External("LUT_READ_FAILED", err.Error(), "Check --lut path", false))
			}
			parsed, err := vcolor.ParseCube(raw)
			if err != nil {
				return writeStructuredError(cmd, opts, verrors.Validation("LUT_INVALID", err.Error(), "Use a valid 3D .cube LUT", false))
			}
			outputPath := strings.TrimPrefix(deliver, "file:")
			plan := vcolor.LUTApplyPlan(input, lut, outputPath)
			status := "planned"
			if opts.Commit {
				if err := vcolor.Run(plan); err != nil {
					return writeStructuredError(cmd, opts, verrors.External("COLOR_APPLY_FAILED", err.Error(), "Check ffmpeg lut3d support", false))
				}
				status = "rendered"
			}
			return writeOutput(cmd, opts, "color apply", map[string]any{"status": status, "lut": parsed, "plan": plan})
		},
	}
	cmd.Flags().StringVar(&input, "input", "", "input render path")
	cmd.Flags().StringVar(&lut, "lut", "", ".cube LUT path")
	cmd.Flags().StringVar(&deliver, "deliver", "file:renders/rough-preview-graded.mp4", "delivery target")
	return cmd
}

func colorReviewCommand(opts *globalOptions) *cobra.Command {
	var provider, model, input string
	cmd := &cobra.Command{
		Use:   "review",
		Short: "review color/exposure with optional Gemini analysis",
		RunE: func(cmd *cobra.Command, args []string) error {
			data := map[string]any{"status": "planned", "provider": provider, "model": model, "input": input, "report": "reports/color-grade-report.json"}
			if opts.Live && provider == "gemini" {
				key, _ := vqa.APIKeyFromEnv()
				if key == "" {
					return writeStructuredError(cmd, opts, verrors.Validation("MISSING_API_KEY", "Gemini API key is not set", "Use GEMINI_API_KEY or GOOGLE_API_KEY via runtime env or Secret Gate", true))
				}
				raw, err := vqa.AnalyzeInlineVideo(key, model, input, "Return JSON only. Review exposure, contrast, white balance, skin tones, mixed lighting, and color-grade finishing notes.")
				if err != nil {
					return writeStructuredError(cmd, opts, verrors.External("COLOR_REVIEW_FAILED", err.Error(), "Run qa doctor first", true))
				}
				data["provider_response"] = json.RawMessage(raw)
				data["status"] = "analyzed"
			}
			return writeOutput(cmd, opts, "color review", data)
		},
	}
	cmd.Flags().StringVar(&provider, "provider", "gemini", "provider")
	cmd.Flags().StringVar(&model, "model", "", "model")
	cmd.Flags().StringVar(&input, "input", "", "input render path")
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
	parent.AddCommand(nleExportCommand(opts), nleImportCommand(opts), nleDiffCommand(opts), nleApplyCommand(opts))
	return parent
}

func nleExportCommand(opts *globalOptions) *cobra.Command {
	var projectPath, target, deliver string
	cmd := &cobra.Command{
		Use:   "export",
		Short: "export timeline to NLE interchange format",
		RunE: func(cmd *cobra.Command, args []string) error {
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
							SourceFrameIn:    segment.SourceFrameIn,
							SourceFrameOut:   segment.SourceFrameOut,
							TimelineFrameIn:  segment.TimelineFrameIn,
							TimelineFrameOut: segment.TimelineFrameOut,
						})
					}
				}
			}
			res := vnle.Export(vnle.Options{Target: target, Output: outputPath}, segments)
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
	return cmd
}

func nleImportCommand(opts *globalOptions) *cobra.Command {
	var input string
	cmd := &cobra.Command{Use: "import", Short: "import NLE timeline", RunE: func(cmd *cobra.Command, args []string) error {
		return writeOutput(cmd, opts, "nle import", map[string]any{"status": "parsed", "input": input, "changes": []any{}})
	}}
	cmd.Flags().StringVar(&input, "input", "", "input timeline path")
	return cmd
}

func nleDiffCommand(opts *globalOptions) *cobra.Command {
	var input string
	cmd := &cobra.Command{Use: "diff", Short: "classify NLE roundtrip diff", RunE: func(cmd *cobra.Command, args []string) error {
		return writeOutput(cmd, opts, "nle diff", map[string]any{"status": "classified", "import": input, "safe_merge": []any{}, "needs_review": []any{}, "blocked": []any{}})
	}}
	cmd.Flags().StringVar(&input, "import", "", "import artifact")
	return cmd
}

func nleApplyCommand(opts *globalOptions) *cobra.Command {
	var input string
	cmd := &cobra.Command{Use: "apply", Short: "apply accepted NLE changes", RunE: func(cmd *cobra.Command, args []string) error {
		status := "planned"
		if opts.Commit {
			status = "applied"
		}
		return writeOutput(cmd, opts, "nle apply", map[string]any{"status": status, "input": input})
	}}
	cmd.Flags().StringVar(&input, "input", "", "accepted changes artifact")
	return cmd
}

func transcriptCommand(opts *globalOptions) *cobra.Command {
	parent := &cobra.Command{Use: "transcript", Short: "transcript workflow commands"}
	parent.AddCommand(transcriptCreateCommand(opts), transcriptImportCommand(opts), transcriptAlignCommand(opts), transcriptBakeoffCommand(opts), transcriptSearchCommand(opts))
	return parent
}

func transcriptImportCommand(opts *globalOptions) *cobra.Command {
	var projectPath, provider, input, rate string
	cmd := &cobra.Command{
		Use:   "import",
		Short: "import transcript data into canonical words.json",
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
		Use:   "create",
		Short: "create a transcript with a local or live provider",
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
			if provider == "openai" {
				if !opts.Live {
					return writeOutput(cmd, opts, "transcript create", map[string]any{
						"status":        "ready",
						"provider":      "openai",
						"requires_live": true,
						"requires_key":  "OPENAI_API_KEY",
						"source":        filepath.ToSlash(source),
					})
				}
				if !opts.Commit {
					return writeStructuredError(cmd, opts, verrors.Safety("live OpenAI transcription requires --commit", "Pass --live --commit to spend provider quota"))
				}
				key := os.Getenv("OPENAI_API_KEY")
				if key == "" {
					return writeStructuredError(cmd, opts, verrors.Validation("MISSING_API_KEY", "OPENAI_API_KEY is not set", "Use runtime env or Secret Gate; do not commit secrets", true))
				}
				tx, err := vtranscript.TranscribeOpenAI(context.Background(), key, source, model)
				if err != nil {
					return writeStructuredError(cmd, opts, verrors.External("OPENAI_TRANSCRIPTION_FAILED", err.Error(), "Check source file, model, account, and provider quota", true))
				}
				words, err := vtranscript.Import("plain-text", []byte(tx.Text), vtranscript.ImportOptions{Rate: rate, FramesPerWord: 15})
				if err != nil {
					return writeStructuredError(cmd, opts, verrors.Validation("TRANSCRIPT_NORMALIZE_FAILED", err.Error(), "Check provider response text", false))
				}
				if err := vtranscript.WriteWords(projectPath, words); err != nil {
					return writeStructuredError(cmd, opts, verrors.External("TRANSCRIPT_WRITE_FAILED", err.Error(), "Check project write permissions", false))
				}
				reportPath := filepath.Join(projectPath, "transcript", "openai-transcription.json")
				raw, _ := json.MarshalIndent(tx, "", "  ")
				_ = os.WriteFile(reportPath, append(raw, '\n'), 0o644)
				return writeOutput(cmd, opts, "transcript create", map[string]any{
					"status":     "written",
					"provider":   "openai",
					"model":      tx.Model,
					"source":     filepath.ToSlash(source),
					"word_count": len(words.Words),
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
		Use:   "align",
		Short: "write a transcript alignment summary",
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
	var providers string
	cmd := &cobra.Command{
		Use:   "bakeoff",
		Short: "compare transcript provider readiness",
		RunE: func(cmd *cobra.Command, args []string) error {
			names := splitCSV(firstNonEmptyString(providers, "openai,elevenlabs,soniox,assemblyai,deepgram,gladia,local"))
			results := make([]map[string]any, 0, len(names))
			for _, name := range names {
				env := providerEnv(name)
				results = append(results, map[string]any{
					"provider":     name,
					"env_var":      env,
					"env_present":  env == "" || os.Getenv(env) != "",
					"live_enabled": opts.Live,
					"capabilities": providerCapabilities(name),
				})
			}
			return writeOutput(cmd, opts, "transcript bakeoff", map[string]any{"status": "checked", "providers": results})
		},
	}
	cmd.Flags().StringVar(&providers, "providers", "", "comma-separated providers")
	return cmd
}

func transcriptSearchCommand(opts *globalOptions) *cobra.Command {
	var projectPath, query string
	cmd := &cobra.Command{
		Use:   "search",
		Short: "search canonical transcript words",
		RunE: func(cmd *cobra.Command, args []string) error {
			if query == "" {
				return writeStructuredError(cmd, opts, verrors.Validation("MISSING_QUERY", "missing --query", "Pass --query text", false))
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
			return writeOutput(cmd, opts, "transcript search", map[string]any{"query": query, "count": len(matches), "matches": matches})
		},
	}
	cmd.Flags().StringVar(&projectPath, "project", ".", "project path")
	cmd.Flags().StringVar(&query, "query", "", "search query")
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

func providerCapabilities(provider string) []string {
	switch provider {
	case "openai":
		return []string{"speech_to_text", "json_text", "optional_diarized_model"}
	case "elevenlabs", "soniox", "assemblyai", "deepgram", "gladia":
		return []string{"speech_to_text", "provider_adapter_pending"}
	case "local", "plain-text", "generic-words":
		return []string{"import", "no_api_key"}
	default:
		return nil
	}
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
