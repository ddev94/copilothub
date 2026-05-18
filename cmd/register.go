package cmd

import "github.com/spf13/cobra"

func registerCommands(root *cobra.Command) {
	root.AddCommand(newOpenCommand())
}
