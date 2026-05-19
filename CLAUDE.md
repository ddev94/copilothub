# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

### Build

```bash
make build          # build frontend (Vite) then Go binary → bin/copilothub
make build-frontend # build only the Vite frontend → internal/ui/dist
make install        # build + sudo cp bin/copilothub /usr/local/bin
make clean          # remove bin/, internal/ui/dist/, node_modules, dist
make deps           # go mod download && go mod tidy
make fmt            # go fmt ./...
```

### Development (two terminals)

```bash
# Terminal 1 – hot-reload frontend (Vite dev server, proxies API to :3000)
cd web && npm run dev

# Terminal 2 – Go API server (serves embedded UI in production, CORS open in dev)
go run . open
```

### Frontend type-check

```bash
cd web && npx vue-tsc --noEmit
```

There are no automated test suites in this repository.

## Architecture

### Go backend

`main.go` → `cmd/` (cobra CLI) → `internal/server/` → feature handlers.

**Entry point**: `cmd/open.go` resolves the repo path, starts `server.Start(repoPath, addr)`, then opens the browser after a 500 ms delay.

**Server** (`internal/server/`):
- `server.go` — creates the `http.ServeMux`, attaches a CORS middleware, mounts the embedded frontend at `/`.
- `routes.go` — wires everything together: creates `config.Store`, `ai.SDKProvider`, a `hub.Registry`, mounts all feature routes under `/api/features/{id}/`, and adds hub-level routes (`/api/hub/features`, `/api/repo`, `/api/config`, `/api/auth/status`).

**Hub / plugin system** (`internal/hub/`):
- `Feature` interface: `ID()`, `Manifest()`, `Init(FeatureContext)`, `RegisterRoutes(*http.ServeMux)`.
- `Registry.RegisterRoutes` strips the `/api/features/{id}` prefix and hands each feature its own sub-mux.
- `ExternalFeature` wraps a third-party binary: it spawns it as a subprocess on a random port, waits for the port to open, and reverse-proxies both API (`/api/features/{id}/`) and frontend (`/features/{id}/`) traffic to it.
- External plugins are described by a `copilothub.json` manifest (fetched from GitHub) and tracked in `~/.copilothub/registry.json`.

**Built-in feature — Spec Designer** (`internal/features/specdesigner/`):
- `SpecHandler` — CRUD for specs stored as JSON files in `.copilothub/specs/` inside the target repo.
- `AIHandler` — three AI endpoints: `POST /ai/clarify` (returns disambiguation questions), `POST /ai/suggest` (generates user stories for a section), `POST /ai/generate-spec` (creates a full `Spec` with user stories, acceptance criteria, and test cases).
- All AI prompts ask the model to first explore the workspace via tool use, then respond in Vietnamese.

**AI layer** (`internal/ai/`):
- `Provider` interface with a single `Complete(ctx, []Message) (string, error)` method.
- `SDKProvider` uses `github.com/github/copilot-sdk/go`. The client is lazily initialised (sync.Once) and authenticates via the `gh` CLI token. An optional `GITHUB_TOKEN` env var overrides the token. CLI path discovery checks `COPILOT_CLI_PATH`, `PATH`, and known VS Code extension locations.
- Config (`internal/config/`) stores `AI.Token` and `AI.Model` in `.copilothub/config.json` within the target repository.

**Repo scanner** (`internal/repo/scanner.go`): detects repo name, remote URL, branch, and tech stack from the filesystem; used to add codebase context to AI prompts.

### Vue 3 frontend (`web/`)

Stack: Vue 3 + Vite + TypeScript + Pinia + vue-router + Tailwind CSS + reka-ui.

The frontend is compiled by `make build-frontend` into `internal/ui/dist/`, then embedded via `//go:embed` in `internal/ui/embed.go` and served by `internal/server/frontend.go`. In development the Vite dev server proxies API calls to the Go backend.

Key layout:
- `web/src/main.ts` — bootstraps Pinia, vue-router, mounts `App.vue`.
- `web/src/components/` — `AIPanel`, `DocList`, `SectionEditor`, `WelcomeScreen`, `ConfigDialog`.
- `web/src/pages/` — `EditorPage` (three-column layout: doc list · editor · AI panel).
- `web/src/stores/` — Pinia stores for spec state, AI interactions, and repo info.
- `web/src/api/` — typed fetch wrappers for all backend endpoints.

### CLI commands (`cmd/`)

| Command | Description |
|---|---|
| `open` | Start the server and open the browser |
| `install` | Install an external plugin by GitHub URL |
| `uninstall` | Remove an installed plugin |
| `list` | List installed plugins |

### Data storage

Specs are stored as JSON files in `.copilothub/specs/<id>.json` within the target repository. Plugin registry lives globally at `~/.copilothub/registry.json`.
