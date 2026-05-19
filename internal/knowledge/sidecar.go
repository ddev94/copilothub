package knowledge

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// Sidecar manages the Python knowledge service subprocess.
type Sidecar struct {
	cmd  *exec.Cmd
	port int
}

// StartSidecar locates the Python script, picks a free port, starts uvicorn,
// waits until the health endpoint responds, and returns the base URL.
// Returns ("", nil) if the script or python3 cannot be found — caller treats
// knowledge as disabled rather than fatal.
func StartSidecar(chromaDir string) (*Sidecar, string, error) {
	scriptDir, err := findScriptDir()
	if err != nil {
		return nil, "", nil // graceful: no Python service available
	}

	python, err := findPython()
	if err != nil {
		fmt.Println("[knowledge] python3 not found — knowledge service disabled")
		return nil, "", nil
	}

	port, err := freePort()
	if err != nil {
		return nil, "", fmt.Errorf("knowledge sidecar: no free port: %w", err)
	}

	env := append(os.Environ(), fmt.Sprintf("PORT=%d", port))
	if chromaDir != "" {
		env = append(env, "CHROMA_DIR="+chromaDir)
	}

	cmd := exec.Command(
		python, "-m", "uvicorn", "main:app",
		"--host", "127.0.0.1",
		"--port", fmt.Sprintf("%d", port),
	)
	cmd.Dir = scriptDir
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, "", fmt.Errorf("knowledge sidecar start: %w", err)
	}

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	if err := waitHealthy(baseURL, 30*time.Second); err != nil {
		cmd.Process.Kill() //nolint:errcheck
		return nil, "", fmt.Errorf("knowledge sidecar did not become healthy: %w", err)
	}

	fmt.Printf("[knowledge] sidecar ready at %s\n", baseURL)
	return &Sidecar{cmd: cmd, port: port}, baseURL, nil
}

// Stop kills the sidecar process.
func (s *Sidecar) Stop() {
	if s != nil && s.cmd != nil && s.cmd.Process != nil {
		s.cmd.Process.Kill() //nolint:errcheck
	}
}

// --------------------------------------------------------------------------
// helpers
// --------------------------------------------------------------------------

func findScriptDir() (string, error) {
	// 1. Relative to the running executable (works for installed binary).
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "python", "knowledge_service")
		if fileExists(filepath.Join(candidate, "main.py")) {
			return candidate, nil
		}
	}
	// 2. Relative to the current working directory (works for `go run . open`).
	if cwd, err := os.Getwd(); err == nil {
		candidate := filepath.Join(cwd, "python", "knowledge_service")
		if fileExists(filepath.Join(candidate, "main.py")) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("knowledge_service/main.py not found")
}

func findPython() (string, error) {
	if scriptDir, err := findScriptDir(); err == nil {
		venvPython := filepath.Join(scriptDir, ".venv", "bin", "python")
		if runtime.GOOS == "windows" {
			venvPython = filepath.Join(scriptDir, ".venv", "Scripts", "python.exe")
		}
		if fileExists(venvPython) {
			return venvPython, nil
		}
	}

	names := []string{"python3", "python"}
	if runtime.GOOS == "windows" {
		names = []string{"python"}
	}
	for _, name := range names {
		if p, err := exec.LookPath(name); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("python not found in PATH")
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func waitHealthy(baseURL string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: time.Second}
	for time.Now().Before(deadline) {
		resp, err := client.Get(baseURL + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("health check timed out after %s", timeout)
}

func waitHealthyCtx(ctx context.Context, baseURL string, timeout time.Duration) error {
	return waitHealthy(baseURL, timeout)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
