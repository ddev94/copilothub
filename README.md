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

Run the frontend dev server, Go backend, and knowledge sidecar in separate terminals:

```bash
# Terminal 1 — Vite hot-reload
cd web && npm run dev

# Terminal 2 — Go API server
go run . open

# Terminal 3 — Knowledge sidecar (LangChain + ChromaDB)
make knowledge-deps
make dev-knowledge
```

If you do not run the knowledge sidecar, SRD generation still works, but uploaded knowledge retrieval will be unavailable.

## Knowledge base (PDF/MD/DOCX)

You can upload project knowledge files from the Spec Designer UI (Knowledge panel):
- Supported file types: `.pdf`, `.md`, `.docx`
- Uploaded files are copied into `.spec-designer/knowledge/files/` inside the target repository
- Embeddings are generated using `sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2`
- Vectors are stored in ChromaDB by the local Python sidecar (`python/knowledge_service`)

Knowledge service default URL is `http://localhost:8001` and can be overridden via:

```bash
KNOWLEDGE_SERVICE_URL=http://localhost:8001 copilothub open
```

Chroma persistence directory can be overridden when starting sidecar:

```bash
CHROMA_DIR=/path/to/chroma_db make dev-knowledge
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

| Target                  | Description                                         |
|-------------------------|-----------------------------------------------------|
| `make build`            | Build frontend + Go binary                          |
| `make build-frontend`   | Build only the Vite frontend                        |
| `make install`          | Build and install to `/usr/local/bin`               |
| `make knowledge-deps`   | Install Python knowledge sidecar dependencies       |
| `make dev-knowledge`    | Run knowledge sidecar on port 8001                  |
| `make clean`            | Remove build artifacts                              |
| `make deps`             | Download Go dependencies                            |
| `make fmt`              | Format Go source files                              |
