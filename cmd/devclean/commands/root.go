package commands

import "github.com/spf13/cobra"

func Root() *cobra.Command {
	root := &cobra.Command{
		Use:   "devclean",
		Short: "Developer system intelligence — find what has no owner",
		Long: `DevClean scans your system and classifies every trace as owned, orphaned,
or unknown. Orphaned traces from uninstalled apps and unknown silent installs
are flagged for review and safe removal.`,
	}

	root.AddCommand(newScanCmd())

	return root
}
