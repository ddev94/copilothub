package cmd

import (
	"fmt"
	"os"
	"copilothub/pkg/version"

	"github.com/spf13/cobra"
)

func Execute() {
	root := &cobra.Command{
		Use:     "copilothub",
		Short:   "Copilot Hub — extensible AI-powered developer tools",
		Version: version.Version,
	}
	registerCommands(root)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
