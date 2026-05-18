package cmd

import (
	"aikit/internal/hub"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newInstallCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "install <github-repo>",
		Short:   "Install an external feature from a GitHub repository",
		Example: "  aikit install github.com/user/deep-wiki",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoURL := args[0]
			fmt.Printf("Fetching manifest from %s...\n", repoURL)

			manifest, err := hub.FetchExternalManifest(repoURL)
			if err != nil {
				return fmt.Errorf("fetch manifest: %w", err)
			}
			fmt.Printf("Found: %s v%s\n", manifest.Name, manifest.Version)

			installDir := filepath.Join(hub.HubDir(), "plugins", manifest.ID)
			fmt.Printf("Downloading binary for %s...\n", hub.CurrentPlatform())

			binaryName, err := hub.DownloadPlugin(manifest, installDir)
			if err != nil {
				return fmt.Errorf("download: %w", err)
			}

			reg, err := hub.LoadPluginRegistry()
			if err != nil {
				reg = &hub.PluginRegistry{}
			}
			reg.Add(hub.InstalledPlugin{
				ID:         manifest.ID,
				InstallDir: installDir,
				Binary:     binaryName,
				Manifest:   *manifest,
			})
			if err := reg.Save(); err != nil {
				return fmt.Errorf("save registry: %w", err)
			}

			fmt.Printf("✓ Installed %s v%s\n", manifest.Name, manifest.Version)
			return nil
		},
	}
}
