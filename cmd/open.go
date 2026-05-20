package cmd

import (
	"copilothub/internal/project"
	"copilothub/internal/server"
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"github.com/spf13/cobra"
)

func newOpenCommand() *cobra.Command {
	var port int
	cmd := &cobra.Command{
		Use:   "open",
		Short: "Start the AI Hub server",
		RunE: func(cmd *cobra.Command, args []string) error {
			dataDir := project.DefaultBaseDir()

			url := fmt.Sprintf("http://localhost:%d", port)
			fmt.Printf("copilothub running at %s\n", url)
			fmt.Printf("data directory: %s\n", dataDir)
			go func() {
				time.Sleep(500 * time.Millisecond)
				openBrowser(url)
			}()
			return server.Start(dataDir, fmt.Sprintf(":%d", port))
		},
	}
	cmd.Flags().IntVarP(&port, "port", "p", 3000, "Port to listen on")
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
