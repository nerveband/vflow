package cli

import "github.com/spf13/cobra"

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

func newVersionCommand(opts *globalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print vflow version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			return writeOutput(cmd, opts, "version", map[string]any{
				"version": Version,
				"commit":  Commit,
				"date":    Date,
			})
		},
	}
}
