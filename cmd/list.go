package cmd

import (
	"copilothub/internal/features/specclarify"
	"copilothub/internal/hub"
	"fmt"

	"github.com/spf13/cobra"
)

func newListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all installed features",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Built-in features:")
			builtins := []hub.Feature{
				specclarify.New(),
			}
			for _, f := range builtins {
				m := f.Manifest()
				fmt.Printf("  %-20s %s - %s\n", m.ID, m.Version, m.Description)
			}

			reg, err := hub.LoadPluginRegistry()
			if err != nil || len(reg.Plugins) == 0 {
				fmt.Println("\nExternal features: (none installed)")
				fmt.Println("\nInstall with: copilothub install github.com/user/plugin-name")
				return nil
			}
			fmt.Printf("\nExternal features (%d installed):\n", len(reg.Plugins))
			for _, p := range reg.Plugins {
				m := p.Manifest
				fmt.Printf("  %-20s v%s - %s\n", m.ID, m.Version, m.Description)
			}
			return nil
		},
	}
}
