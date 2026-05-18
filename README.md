# copilothub

AI-powered Software Requirements Document (SRD) designer for your repository. Runs as a local CLI that opens a browser-based editor, keeping all specs stored in your project.

![demo](assets/demo.png)

## Features

- Create and manage multiple SRDs per repository
- AI-assisted generation using GitHub Copilot
- Three-column layout: document list · markdown editor · AI chat panel
- Specs stored locally in `.copilothub/specs/` as JSON files

## Prerequisites

- [Go](https://golang.org/) 1.21+
- [Node.js](https://nodejs.org/) 18+
- [GitHub CLI (`gh`)](https://cli.github.com/) — authenticated (`gh auth login`) for Copilot access

## Installation

### From source

```bash
git clone https://github.com/your-org/copilothub
cd copilothub
make install          # builds frontend + Go binary, copies to /usr/local/bin
```

### Build only (no install)

```bash
make build            # outputs bin/copilothub
```

## Usage

```bash
# Open the UI for the current repository
copilothub open

# Specify a port (default: 3000)
copilothub open --port 8080

# Target a different directory
copilothub open --workdir /path/to/repo
```

The browser opens automatically at `http://localhost:3000`.

## Development

Run the frontend dev server and Go backend in separate terminals:

```bash
# Terminal 1 — Vite hot-reload
cd web && npm run dev

# Terminal 2 — Go API server
go run . open
```

## Project structure

```
.
├── cmd/                  CLI commands (open, register)
├── internal/
│   ├── ai/               GitHub Copilot integration
│   ├── config/           Per-repo config (.copilothub/config.json)
│   ├── handler/          HTTP handlers (spec, AI, config, repo)
│   ├── repo/             Git repository scanner
│   ├── server/           HTTP server + embedded frontend
│   └── spec/             Spec model and file store
├── web/                  Vue 3 frontend (Vite + Tailwind + reka-ui)
│   └── src/
│       ├── components/   AIPanel, DocList, SectionEditor, WelcomeScreen, ConfigDialog
│       ├── pages/        EditorPage
│       ├── stores/       Pinia stores (spec, ai, repo)
│       └── api/          Typed API client
└── Makefile
```

## Configuration

Settings are stored in `.copilothub/config.json` within the repository. You can also override the AI token via environment variable:

```bash
GITHUB_TOKEN=ghp_xxx copilothub open
```

## Makefile targets

| Target             | Description                                |
|--------------------|--------------------------------------------|
| `make build`       | Build frontend + Go binary                 |
| `make build-frontend` | Build only the Vite frontend            |
| `make install`     | Build and install to `/usr/local/bin`      |
| `make clean`       | Remove build artifacts                     |
| `make deps`        | Download Go dependencies                   |
| `make fmt`         | Format Go source files                     |
