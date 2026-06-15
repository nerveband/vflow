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
		{"config inspect", true, false, "config"},
		{"config defaults", true, false, "config"},
		{"config set-defaults", false, true, "config"},
		{"profile list", true, false, "config"},
		{"profile show", true, false, "config"},
		{"profile set", false, true, "config"},
		{"profile use", false, true, "config"},
		{"auth doctor", true, false, "providers"},
		{"project init", false, true, "project"},
		{"project get", true, false, "project"},
		{"project list", true, false, "project"},
		{"project index", false, true, "project"},
		{"media ingest", false, true, "media"},
		{"media probe", false, true, "media"},
		{"media proxy", false, true, "media"},
		{"media samples", false, true, "media"},
		{"media sync", false, true, "media"},
		{"media extract-ranges", false, true, "media"},
		{"transcript create", false, true, "transcript"},
		{"transcript import", false, true, "transcript"},
		{"transcript align", false, true, "transcript"},
		{"transcript bakeoff", false, true, "transcript"},
		{"transcript search", true, false, "transcript"},
		{"transcript sync", false, true, "transcript"},
		{"cut create", false, true, "cut"},
		{"cleanup suggest", false, true, "cleanup"},
		{"cleanup review", false, true, "cleanup"},
		{"cleanup apply", false, true, "cleanup"},
		{"framing preset", false, true, "framing"},
		{"framing map-speakers", false, true, "framing"},
		{"framing compile", false, true, "framing"},
		{"timeline compile", false, true, "timeline"},
		{"timeline verify", true, false, "timeline"},
		{"render preview", false, true, "render"},
		{"render transcript-cut", false, true, "render"},
		{"render verify", true, false, "render"},
		{"render verify-transcript", false, true, "render"},
		{"qa doctor", true, false, "qa"},
		{"qa analyze", false, true, "qa"},
		{"color research", true, false, "color"},
		{"color apply", false, true, "color"},
		{"color review", false, true, "color"},
		{"color export-lut", false, true, "color"},
		{"nle export", false, true, "nle"},
		{"nle import", false, true, "nle"},
		{"nle diff", true, false, "nle"},
		{"nle accept", false, true, "nle"},
		{"nle apply", false, true, "nle"},
		{"jobs list", true, false, "jobs"},
		{"jobs get", true, false, "jobs"},
		{"jobs resume", false, true, "jobs"},
		{"artifacts list", true, false, "artifacts"},
		{"artifacts deliver", false, true, "artifacts"},
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
