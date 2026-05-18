package cmd

import (
	"fmt"
	"os"
	"aikit/pkg/version"

	"github.com/spf13/cobra"
)

func Execute() {
	root := &cobra.Command{
		Use:     "aikit",
		Short:   "AI Hub — extensible AI-powered developer tools",
		Version: version.Version,
	}
	registerCommands(root)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
