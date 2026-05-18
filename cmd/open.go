package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"aikit/internal/server"
	"time"

	"github.com/spf13/cobra"
)

func newOpenCommand() *cobra.Command {
	var port int
	var workDir string
	cmd := &cobra.Command{
		Use:   "open",
		Short: "Start the spec designer UI for the current repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			var repoPath string
			if workDir != "" {
				repoPath = workDir
			} else {
				var err error
				repoPath, err = os.Getwd()
				if err != nil {
					return err
				}
			}
			url := fmt.Sprintf("http://localhost:%d", port)
			fmt.Printf("spec-designer running at %s\n", url)
			go func() {
				time.Sleep(500 * time.Millisecond)
				openBrowser(url)
			}()
			return server.Start(repoPath, fmt.Sprintf(":%d", port))
		},
	}
	cmd.Flags().IntVarP(&port, "port", "p", 3000, "Port to listen on")
	cmd.Flags().StringVarP(&workDir, "workdir", "w", "", "Working directory (default: current directory)")
	return cmd
}

func openBrowser(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd, args = "open", []string{url}
	case "windows":
		cmd, args = "rundll32", []string{"url.dll,FileProtocolHandler", url}
	default:
		cmd, args = "xdg-open", []string{url}
	}
	exec.Command(cmd, args...).Start() //nolint:errcheck
}
