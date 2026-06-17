package contract

import "sort"

type Registry struct {
	commands map[string]Command
}

func NewRegistry() *Registry {
	return &Registry{commands: map[string]Command{}}
}

func (r *Registry) Add(cmd Command) {
	if cmd.Name == "" {
		return
	}
	if !cmd.Canonical {
		cmd.Canonical = true
	}
	r.commands[cmd.Name] = cmd
}

func (r *Registry) Command(name string) (Command, bool) {
	cmd, ok := r.commands[name]
	return cmd, ok
}

func (r *Registry) Get(name string) (Command, bool) {
	return r.Command(name)
}

func (r *Registry) Commands() []Command {
	out := make([]Command, 0, len(r.commands))
	for _, cmd := range r.commands {
		out = append(out, cmd)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func DefaultRegistry() *Registry {
	reg := NewRegistry()
	for _, spec := range []struct {
		name     string
		readOnly bool
		mutates  bool
		scope    string
	}{
		{"version", true, false, "system"},
		{"schema", true, false, "system"},
		{"agent-context", true, false, "system"},
		{"skill-path", true, false, "system"},
		{"doctor", true, false, "system"},
		{"suggest", true, false, "finish"},
		{"verify", true, false, "finish"},
		{"audit cli", true, false, "system"},
		{"feedback", false, true, "system"},
		{"config inspect", true, false, "config"},
		{"config defaults", true, false, "config"},
		{"config set-defaults", false, true, "config"},
		{"profile list", true, false, "config"},
		{"profile show", true, false, "config"},
		{"profile set", false, true, "config"},
		{"profile use", false, true, "config"},
		{"auth doctor", true, false, "providers"},
		{"project init", false, true, "project"},
		{"project create-project", false, true, "project"},
		{"project new-project", false, true, "project"},
		{"project get", true, false, "project"},
		{"project inspect", true, false, "project"},
		{"project inspect-project", true, false, "project"},
		{"project show", true, false, "project"},
		{"project list", true, false, "project"},
		{"project index", false, true, "project"},
		{"media ingest", false, true, "media"},
		{"media add-media", false, true, "media"},
		{"media import-media", false, true, "media"},
		{"media probe", false, true, "media"},
		{"media analyze-media", false, true, "media"},
		{"media inspect-media", false, true, "media"},
		{"media metadata", false, true, "media"},
		{"media proxy", false, true, "media"},
		{"media create-proxy", false, true, "media"},
		{"media make-proxy", false, true, "media"},
		{"media transcode-proxy", false, true, "media"},
		{"media samples", false, true, "media"},
		{"media sync", false, true, "media"},
		{"media extract-ranges", false, true, "media"},
		{"transcript create", false, true, "transcript"},
		{"transcript speech-to-text", false, true, "transcript"},
		{"transcript stt", false, true, "transcript"},
		{"transcript transcribe", false, true, "transcript"},
		{"transcript import", false, true, "transcript"},
		{"transcript ingest-transcript", false, true, "transcript"},
		{"transcript load-transcript", false, true, "transcript"},
		{"transcript align", false, true, "transcript"},
		{"transcript align-words", false, true, "transcript"},
		{"transcript sync-transcript", false, true, "transcript"},
		{"transcript word-align", false, true, "transcript"},
		{"transcript bakeoff", false, true, "transcript"},
		{"transcript search", true, false, "transcript"},
		{"transcript sync", false, true, "transcript"},
		{"transcript sync-transcript-timing", false, true, "transcript"},
		{"cut create", false, true, "cut"},
		{"cleanup suggest", false, true, "cleanup"},
		{"cleanup cleanup-plan", false, true, "cleanup"},
		{"cleanup suggest-cleanup", false, true, "cleanup"},
		{"cleanup review", false, true, "cleanup"},
		{"cleanup review-cleanup", false, true, "cleanup"},
		{"cleanup apply", false, true, "cleanup"},
		{"cleanup apply-cleanup", false, true, "cleanup"},
		{"framing calibrate", false, true, "framing"},
		{"framing crop", false, true, "framing"},
		{"framing crop-calibrate", false, true, "framing"},
		{"framing frame", false, true, "framing"},
		{"framing preset-calibrate", false, true, "framing"},
		{"framing reframe", false, true, "framing"},
		{"framing zoom", false, true, "framing"},
		{"framing zoom-calibrate", false, true, "framing"},
		{"framing preset", false, true, "framing"},
		{"framing presets", false, true, "framing"},
		{"framing map-speakers", false, true, "framing"},
		{"framing assign-speakers", false, true, "framing"},
		{"framing map-speaker", false, true, "framing"},
		{"framing speaker-map", false, true, "framing"},
		{"framing propose", false, true, "framing"},
		{"framing compile", false, true, "framing"},
		{"framing apply-framing", false, true, "framing"},
		{"framing build-framing", false, true, "framing"},
		{"framing compile-framing", false, true, "framing"},
		{"framing review", true, false, "framing"},
		{"timeline compile", false, true, "timeline"},
		{"timeline assemble", false, true, "timeline"},
		{"timeline build-timeline", false, true, "timeline"},
		{"timeline make-timeline", false, true, "timeline"},
		{"timeline verify", true, false, "timeline"},
		{"timeline check-timeline", true, false, "timeline"},
		{"timeline verify-timeline", true, false, "timeline"},
		{"render preview", false, true, "render"},
		{"render make-preview", false, true, "render"},
		{"render render-sample", false, true, "render"},
		{"render transcript-cut", false, true, "render"},
		{"render verify", true, false, "render"},
		{"render check-render", true, false, "render"},
		{"render qa-render", true, false, "render"},
		{"render verify-render", true, false, "render"},
		{"render verify-transcript", false, true, "render"},
		{"qa doctor", true, false, "qa"},
		{"qa analyze", false, true, "qa"},
		{"color research", true, false, "color"},
		{"color apply", false, true, "color"},
		{"color review", false, true, "color"},
		{"color export-lut", false, true, "color"},
		{"nle export", false, true, "nle"},
		{"nle export-nle", false, true, "nle"},
		{"nle to-nle", false, true, "nle"},
		{"nle import", false, true, "nle"},
		{"nle from-nle", false, true, "nle"},
		{"nle import-nle", false, true, "nle"},
		{"nle diff", true, false, "nle"},
		{"nle compare-nle", true, false, "nle"},
		{"nle nle-compare", true, false, "nle"},
		{"nle accept", false, true, "nle"},
		{"nle apply", false, true, "nle"},
		{"jobs list", true, false, "jobs"},
		{"jobs get", true, false, "jobs"},
		{"jobs resume", false, true, "jobs"},
		{"artifacts list", true, false, "artifacts"},
		{"artifacts list-artifacts", true, false, "artifacts"},
		{"artifacts outputs", true, false, "artifacts"},
		{"artifacts deliver", false, true, "artifacts"},
		{"artifacts publish-artifacts", false, true, "artifacts"},
		{"upgrade", false, true, "system"},
	} {
		reg.Add(Command{
			Name:           spec.name,
			Description:    spec.name,
			Canonical:      true,
			ReadOnly:       spec.readOnly,
			Idempotent:     true,
			SupportsDryRun: spec.mutates,
			RequiresCommit: spec.mutates,
			Scope:          spec.scope,
			Examples:       []string{"vflow " + spec.name + " --format json"},
		})
	}
	for _, contract := range writeInputContracts() {
		if cmd, ok := reg.commands[contract.Name]; ok {
			if contract.Description != "" {
				cmd.Description = contract.Description
			}
			cmd.InputSchema = contract.InputSchema
			cmd.Produces = contract.Produces
			cmd.Example = contract.Example
			cmd.ValidationHint = contract.ValidationHint
			if len(contract.Examples) > 0 {
				cmd.Examples = contract.Examples
			}
			reg.commands[contract.Name] = cmd
		}
	}
	for name, cmd := range reg.commands {
		if !cmd.RequiresCommit || cmd.InputSchema != nil {
			continue
		}
		cmd.InputSchema = fallbackInputSchema(cmd.Name)
		cmd.Example = fallbackExample(cmd.Name)
		cmd.ValidationHint = "Run vflow " + cmd.Name + " --dry-run --format json before adding --commit"
		reg.commands[name] = cmd
	}
	return reg
}

func writeInputContracts() []Command {
	return []Command{
		{
			Name:        "media ingest",
			Description: "record source media by copy, link, or external reference",
			InputSchema: objectSchema("media ingest options", []string{"source"}, map[string]any{
				"source":       stringSchema("absolute or project-relative media path"),
				"copy":         map[string]any{"type": "boolean", "description": "copy bytes into project media directory"},
				"reference":    map[string]any{"type": "boolean", "description": "record external path without copying bytes"},
				"link":         map[string]any{"type": "boolean", "description": "symlink source into project media directory"},
				"ffprobe_json": stringSchema("optional ffprobe JSON fixture for deterministic source review"),
			}),
			Produces:       "source-media-review.schema.json",
			Example:        map[string]any{"source": "/Volumes/Shams Drive/session-01-9mm.mp4", "reference": true},
			ValidationHint: "Pass exactly one of --copy, --reference, or --link; use --reference for large external 4K sources",
			Examples:       []string{"vflow media ingest --source /Volumes/Shams\\ Drive/session-01-9mm.mp4 --reference --commit --format json"},
		},
		{
			Name:        "media sync",
			Description: "calibrate source-camera sync from waveforms or transcript word anchors",
			InputSchema: objectSchema("media sync options", []string{"reference_source_id"}, map[string]any{
				"method":              map[string]any{"type": "string", "enum": []string{"audio_xcorr_envelope", "transcript"}},
				"reference_source_id": stringSchema("timeline reference source id"),
				"sources":             stringSchema("comma-separated id=media path list for audio_xcorr_envelope"),
				"sync_windows":        map[string]any{"type": "integer", "minimum": 1, "description": "number of waveform windows to sample and median-vote"},
				"reference_words":     stringSchema("reference vflow-words/v1 JSON for transcript method"),
				"source_words":        stringSchema("comma-separated id=words.json list for transcript method"),
				"frame_rate":          stringSchema("canonical project frame rate"),
			}),
			Produces:       "media-sync-map.schema.json",
			Example:        map[string]any{"method": "transcript", "reference_source_id": "atem", "reference_words": "transcript/atem.words.json", "source_words": "9mm=transcript/9mm.words.json", "frame_rate": "24000/1001"},
			ValidationHint: "Transcript sync requires at least 3 shared unique words and refuses commit below confidence 0.65; waveform sync should use multiple windows for long multicam sources",
			Examples:       []string{"vflow media sync --method transcript --reference-source-id atem --reference-words transcript/atem.words.json --source-words 9mm=transcript/9mm.words.json --format json", "vflow media sync --reference-source-id atem --sources atem=/proxy/atem.mp4,9mm=/proxy/9mm.mp4 --sync-windows 3 --format json"},
		},
		{
			Name:        "cut create",
			Description: "create a vflow-transcript-cut/v1 decision from transcript-relative ranges",
			InputSchema: objectSchema("cut create ranges input", []string{"ranges"}, map[string]any{
				"ranges": map[string]any{
					"type":     "array",
					"minItems": 1,
					"items": objectSchema("transcript range", []string{"source_id", "transcript_start_seconds", "transcript_end_seconds"}, map[string]any{
						"id":                       stringSchema("stable segment id"),
						"source_id":                stringSchema("source id from media-sync-map"),
						"transcript_start_seconds": numberSchema("inclusive transcript start in seconds"),
						"transcript_end_seconds":   numberSchema("exclusive transcript end in seconds"),
						"start":                    numberSchema("alias accepted by humans; prefer transcript_start_seconds"),
						"end":                      numberSchema("alias accepted by humans; prefer transcript_end_seconds"),
						"text":                     stringSchema("selected transcript text"),
						"speaker_id":               stringSchema("speaker id from transcript words or speaker map"),
						"reason":                   stringSchema("editorial reason for the cut"),
					}),
				},
			}),
			Produces: "transcript-cut.schema.json",
			Example: map[string]any{"ranges": []map[string]any{{
				"id": "hook", "source_id": "9mm", "transcript_start_seconds": 152.90, "transcript_end_seconds": 168.04,
				"text": "This is the moment the story turns.", "speaker_id": "s1", "reason": "opening hook",
			}}},
			ValidationHint: "Run vflow cut create --ranges @file.json --dry-run --format json before --commit",
			Examples:       []string{"vflow cut create --ranges @ranges.json --sync-map media/media-sync-map.json --format json"},
		},
		{
			Name:        "render transcript-cut",
			Description: "plan or render a transcript-selected social cut from vflow-transcript-cut/v1",
			InputSchema: objectSchema("render transcript-cut input", []string{"version", "segments"}, map[string]any{
				"version": map[string]any{"const": "vflow-transcript-cut/v1"},
				"summary": stringSchema("editorial summary"),
				"segments": map[string]any{
					"type":     "array",
					"minItems": 1,
					"items": objectSchema("transcript cut segment", []string{"id", "source", "start_seconds", "end_seconds"}, map[string]any{
						"id": stringSchema("stable segment id"), "source": stringSchema("project-relative or absolute media path"),
						"source_id": stringSchema("source id for sync-map resolution"), "start_seconds": numberSchema("source start seconds"),
						"end_seconds": numberSchema("source end seconds"), "transcript_start_seconds": numberSchema("original transcript start seconds"),
						"transcript_end_seconds": numberSchema("original transcript end seconds"), "speaker": stringSchema("speaker label"),
						"text": stringSchema("selected transcript text"), "reason": stringSchema("editorial reason"),
					}),
				},
			}),
			Produces: "render-report.schema.json",
			Example: map[string]any{"version": "vflow-transcript-cut/v1", "summary": "30s donor hook", "segments": []map[string]any{{
				"id": "hook", "source": "media/9mm.mp4", "source_id": "9mm", "start_seconds": 305.80, "end_seconds": 321.04,
				"transcript_start_seconds": 152.90, "transcript_end_seconds": 168.04, "speaker": "s1", "text": "This is the moment the story turns.",
			}}},
			ValidationHint: "Run vflow render transcript-cut --input @file.json --dry-run --format json before --commit",
			Examples:       []string{"vflow render transcript-cut --input decisions/transcript-cut.json --output renders/social.mp4 --format json"},
		},
		{
			Name:        "media extract-ranges",
			Description: "plan source media range extraction from transcript-relative ranges and a sync map",
			InputSchema: objectSchema("media extract-ranges input", []string{"ranges"}, map[string]any{
				"ranges": map[string]any{"type": "array", "minItems": 1, "items": objectSchema("source extraction range", []string{"source_id", "transcript_start_seconds", "transcript_end_seconds"}, map[string]any{
					"id": stringSchema("stable range id"), "source_id": stringSchema("source id from media-sync-map"),
					"transcript_start_seconds": numberSchema("inclusive transcript start in seconds"), "transcript_end_seconds": numberSchema("exclusive transcript end in seconds"),
					"output": stringSchema("optional output media path"),
				})},
			}),
			Produces:       "source-range-manifest.schema.json",
			Example:        map[string]any{"ranges": []map[string]any{{"id": "breakaway", "source_id": "12mm", "transcript_start_seconds": 2415.20, "transcript_end_seconds": 2432.10, "output": "media/ranges/breakaway-12mm.mp4"}}},
			ValidationHint: "Run vflow media extract-ranges --ranges @file.json --sync-map media/media-sync-map.json --dry-run --format json before --commit",
		},
		{
			Name:           "timeline compile",
			Description:    "compile canonical timeline artifacts from content decisions",
			InputSchema:    objectSchema("timeline compile options", []string{"duration_frames"}, map[string]any{"duration_frames": map[string]any{"type": "integer", "minimum": 1}, "delete_segments": map[string]any{"type": "string", "description": "path to vflow-content-edl/v1 delete_segments JSON"}}),
			Produces:       "compiled-timeline.schema.json",
			Example:        map[string]any{"duration_frames": 43200, "delete_segments": "decisions/delete_segments.json"},
			ValidationHint: "Run vflow timeline compile --duration-frames N --dry-run --format json before --commit",
		},
		{
			Name:           "transcript create",
			Description:    "create canonical transcript words from local or live speech-to-text providers",
			InputSchema:    objectSchema("transcript create options", []string{"provider", "source"}, map[string]any{"provider": stringSchema("deepgram, elevenlabs, openai, or local importer provider"), "source": stringSchema("source media or audio path"), "rate": stringSchema("frame rate; defaults from source-media-review.json when omitted"), "diarize": map[string]any{"type": "boolean", "description": "request speaker diarization when provider supports it"}, "keyterms": stringSchema("path to glossary/keyterms file for proper nouns"), "timeout": stringSchema("provider-aware timeout such as 20m")}),
			Produces:       "transcript.schema.json",
			Example:        map[string]any{"provider": "elevenlabs", "source": "media/session-01-9mm.mp4", "rate": "24000/1001", "diarize": true, "keyterms": "transcript/keyterms.txt"},
			ValidationHint: "Use --live only for provider calls; run --dry-run first to inspect provider, source, rate, diarization, and keyterms choices",
		},
		{
			Name:           "qa analyze",
			Description:    "analyze preview video with provider QA and optional editorial context",
			InputSchema:    objectSchema("qa analyze options", []string{"render"}, map[string]any{"render": stringSchema("rendered video path"), "mode": map[string]any{"type": "string", "enum": []string{"visual", "editorial"}}, "prompt": stringSchema("custom prompt file"), "transcript": stringSchema("transcript context file"), "timeout": stringSchema("provider-aware timeout such as 20m")}),
			Produces:       "gemini-video-qa.schema.json",
			Example:        map[string]any{"render": "renders/rough-preview.mp4", "mode": "editorial", "prompt": "qa/editorial-prompt.md", "transcript": "transcript/words.json", "timeout": "20m"},
			ValidationHint: "Run vflow qa analyze --render file.mp4 --mode editorial --dry-run --format json before live provider review",
		},
	}
}

func objectSchema(title string, required []string, properties map[string]any) map[string]any {
	schema := map[string]any{
		"$schema":              "https://json-schema.org/draft/2020-12/schema",
		"title":                title,
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func stringSchema(description string) map[string]any {
	return map[string]any{"type": "string", "minLength": 1, "description": description}
}

func numberSchema(description string) map[string]any {
	return map[string]any{"type": "number", "minimum": 0, "description": description}
}

func fallbackInputSchema(commandName string) map[string]any {
	return objectSchema(commandName+" options", nil, map[string]any{
		"project": map[string]any{"type": "string", "description": "project path when supported"},
		"input":   map[string]any{"type": "string", "description": "input artifact path when supported; @file.json is accepted by JSON payload commands"},
		"output":  map[string]any{"type": "string", "description": "output artifact path when supported"},
		"commit":  map[string]any{"type": "boolean", "description": "required to write artifacts, call live providers, or mutate local state"},
		"dry_run": map[string]any{"type": "boolean", "description": "inspect planned changes without writing"},
	})
}

func fallbackExample(commandName string) map[string]any {
	return map[string]any{
		"command": "vflow " + commandName + " --dry-run --format json",
		"commit":  false,
	}
}
