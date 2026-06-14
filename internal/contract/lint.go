package contract

import (
	"fmt"
	"strings"
)

func (r *Registry) Validate() error {
	for _, cmd := range r.commands {
		fields := strings.Fields(cmd.Name)
		for _, field := range fields {
			if field == "info" {
				return fmt.Errorf("%s: use get or probe, not info", cmd.Name)
			}
		}
		if !cmd.ReadOnly && !cmd.SupportsDryRun {
			return fmt.Errorf("%s: mutating command must support --dry-run", cmd.Name)
		}
		if cmd.Destructive && !cmd.RequiresCommit {
			return fmt.Errorf("%s: destructive command must require --commit", cmd.Name)
		}
		if len(cmd.Examples) == 0 {
			return fmt.Errorf("%s: help examples missing", cmd.Name)
		}
	}
	return nil
}
