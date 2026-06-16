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
	return reg
}
