package cmd

import (
	"copilothub/internal/hub"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newUninstallCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall <id>",
		Short: "Uninstall an external feature",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			reg, err := hub.LoadPluginRegistry()
			if err != nil {
				return err
			}
			var plugin *hub.InstalledPlugin
			for _, p := range reg.Plugins {
				if p.ID == id {
					pp := p
					plugin = &pp
					break
				}
			}
			if plugin == nil {
				return fmt.Errorf("plugin %q is not installed", id)
			}
			if err := os.RemoveAll(plugin.InstallDir); err != nil {
				return fmt.Errorf("remove files: %w", err)
			}
			reg.Remove(id)
			if err := reg.Save(); err != nil {
				return err
			}
			fmt.Printf("✓ Uninstalled %s\n", id)
			return nil
		},
	}
}
