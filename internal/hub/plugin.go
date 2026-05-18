package hub

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// ExternalManifest is the copilothub.json file in an external plugin repo.
type ExternalManifest struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Category    string `json:"category"`
	Author      string `json:"author"`
	Releases    struct {
		BaseURL string            `json:"baseURL"`
		Assets  map[string]string `json:"assets"`
	} `json:"releases"`
}

// InstalledPlugin is an entry in the local registry file.
type InstalledPlugin struct {
	ID         string           `json:"id"`
	InstallDir string           `json:"installDir"`
	Binary     string           `json:"binary"`
	Manifest   ExternalManifest `json:"manifest"`
}

// PluginRegistry manages the ~/.copilothub/registry.json file.
type PluginRegistry struct {
	path    string
	Plugins []InstalledPlugin `json:"plugins"`
}

func HubDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".copilothub")
}

func LoadPluginRegistry() (*PluginRegistry, error) {
	path := filepath.Join(HubDir(), "registry.json")
	r := &PluginRegistry{path: path}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return r, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, r); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *PluginRegistry) Save() error {
	if err := os.MkdirAll(filepath.Dir(r.path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.path, data, 0600)
}

func (r *PluginRegistry) Add(p InstalledPlugin) {
	for i, existing := range r.Plugins {
		if existing.ID == p.ID {
			r.Plugins[i] = p
			return
		}
	}
	r.Plugins = append(r.Plugins, p)
}

func (r *PluginRegistry) Remove(id string) bool {
	for i, p := range r.Plugins {
		if p.ID == id {
			r.Plugins = append(r.Plugins[:i], r.Plugins[i+1:]...)
			return true
		}
	}
	return false
}

// CurrentPlatform returns "darwin/arm64", "linux/amd64", etc.
func CurrentPlatform() string {
	return runtime.GOOS + "/" + runtime.GOARCH
}

// ExternalFeature wraps an installed external plugin, running it as a subprocess.
type ExternalFeature struct {
	plugin  InstalledPlugin
	port    int
	cmd     *exec.Cmd
	proxy   *httputil.ReverseProxy
	mu      sync.Mutex
	started bool
}

func NewExternalFeature(p InstalledPlugin) *ExternalFeature {
	return &ExternalFeature{plugin: p}
}

func (f *ExternalFeature) ID() string { return f.plugin.ID }

func (f *ExternalFeature) Manifest() Manifest {
	m := f.plugin.Manifest
	return Manifest{
		ID:            m.ID,
		Name:          m.Name,
		Version:       m.Version,
		Description:   m.Description,
		Icon:          m.Icon,
		Category:      m.Category,
		Author:        m.Author,
		Type:          "external",
		FrontendRoute: "/features/" + m.ID,
	}
}

func (f *ExternalFeature) Init(ctx FeatureContext) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.started {
		return nil
	}
	port, err := freePort()
	if err != nil {
		return err
	}
	f.port = port
	binaryPath := filepath.Join(f.plugin.InstallDir, f.plugin.Binary)
	f.cmd = exec.Command(binaryPath,
		"--port", fmt.Sprintf("%d", port),
		"--workdir", ctx.WorkDir,
	)
	f.cmd.Stdout = os.Stdout
	f.cmd.Stderr = os.Stderr
	if err := f.cmd.Start(); err != nil {
		return fmt.Errorf("start plugin %s: %w", f.plugin.ID, err)
	}
	// Wait for the plugin to be ready
	target, _ := url.Parse(fmt.Sprintf("http://localhost:%d", port))
	f.proxy = httputil.NewSingleHostReverseProxy(target)
	if err := waitForPort(port, 5*time.Second); err != nil {
		f.cmd.Process.Kill() //nolint:errcheck
		return fmt.Errorf("plugin %s did not start in time: %w", f.plugin.ID, err)
	}
	f.started = true
	return nil
}

func (f *ExternalFeature) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		f.proxy.ServeHTTP(w, r)
	})
}

func (f *ExternalFeature) Stop() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.cmd != nil && f.cmd.Process != nil {
		f.cmd.Process.Kill() //nolint:errcheck
	}
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func waitForPort(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("port %d not open after %s", port, timeout)
}

// FetchExternalManifest downloads copilothub.json from a GitHub repo.
// repoURL is like "github.com/user/repo"
func FetchExternalManifest(repoURL string) (*ExternalManifest, error) {
	repoURL = strings.TrimPrefix(repoURL, "https://")
	repoURL = strings.TrimPrefix(repoURL, "http://")
	if !strings.HasPrefix(repoURL, "github.com/") {
		return nil, fmt.Errorf("only github.com repositories are supported")
	}
	parts := strings.Split(repoURL, "/")
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid repository URL: %s", repoURL)
	}
	rawURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/copilothub.json", parts[1], parts[2])
	resp, err := http.Get(rawURL) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("fetch manifest: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("manifest not found at %s (status %d)", rawURL, resp.StatusCode)
	}
	var m ExternalManifest
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	return &m, nil
}

// DownloadPlugin downloads the binary for the current platform and saves it to installDir.
func DownloadPlugin(m *ExternalManifest, installDir string) (string, error) {
	platform := CurrentPlatform()
	binaryName, ok := m.Releases.Assets[platform]
	if !ok {
		return "", fmt.Errorf("no binary available for platform %s", platform)
	}
	version := m.Version
	downloadURL := strings.ReplaceAll(m.Releases.BaseURL, "{version}", version)
	downloadURL = strings.TrimSuffix(downloadURL, "/") + "/" + binaryName

	if err := os.MkdirAll(installDir, 0755); err != nil {
		return "", err
	}
	destPath := filepath.Join(installDir, binaryName)

	resp, err := http.Get(downloadURL) //nolint:gosec
	if err != nil {
		return "", fmt.Errorf("download binary: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed (status %d): %s", resp.StatusCode, downloadURL)
	}
	f, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return "", err
	}
	return binaryName, nil
}
