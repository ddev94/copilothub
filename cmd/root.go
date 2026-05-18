package cmd

import (
	"fmt"
	"os"
	"spec-designer/pkg/version"

	"github.com/spf13/cobra"
)

func Execute() {
	root := &cobra.Command{
		Use:     "spec-designer",
		Short:   "AI-powered SRS designer for your repository",
		Version: version.Version,
	}
	registerCommands(root)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
