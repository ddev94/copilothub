package knowledge

import "sync"

type ModelState string

const (
	ModelStateUnknown     ModelState = "unknown"
	ModelStateReady       ModelState = "ready"
	ModelStateDownloading ModelState = "downloading"
	ModelStateError       ModelState = "error"
)

type ModelProgress struct {
	State   ModelState `json:"state"`
	Message string     `json:"message"`
	Bytes   int64      `json:"bytes"`
	Total   int64      `json:"total"`
	Percent int        `json:"percent"`
}

// IngestState tracks the progress of a document ingestion.
type IngestState string

const (
	IngestIdle       IngestState = "idle"
	IngestConverting IngestState = "converting" // PDF → MD conversion
	IngestRunning    IngestState = "running"    // Embedding chunks
	IngestGraphing   IngestState = "graphing"   // Building knowledge graph
	IngestCompleted  IngestState = "completed"
	IngestFailed     IngestState = "failed"
)

type IngestProgress struct {
	State       IngestState `json:"state"`
	DocID       string      `json:"docId,omitempty"`
	FileName    string      `json:"fileName,omitempty"`
	Message     string      `json:"message"`
	ChunksDone  int         `json:"chunksDone"`
	ChunksTotal int         `json:"chunksTotal"`
	Percent     int         `json:"percent"`
}

// ProgressBroadcaster tracks embedding model download state and fans out to SSE subscribers.
type ProgressBroadcaster struct {
	mu   sync.RWMutex
	cur  ModelProgress
	subs map[chan ModelProgress]struct{}
}

// IngestBroadcaster tracks document ingestion progress.
type IngestBroadcaster struct {
	mu   sync.RWMutex
	cur  IngestProgress
	subs map[chan IngestProgress]struct{}
}

// EmbedProgress is the global download progress tracker for the cybertron model.
var EmbedProgress = &ProgressBroadcaster{
	cur:  ModelProgress{State: ModelStateUnknown},
	subs: make(map[chan ModelProgress]struct{}),
}

// IngestProgressTracker is the global ingestion progress tracker.
var IngestProgressTracker = &IngestBroadcaster{
	cur:  IngestProgress{State: IngestIdle},
	subs: make(map[chan IngestProgress]struct{}),
}

func (b *ProgressBroadcaster) Set(p ModelProgress) {
	b.mu.Lock()
	b.cur = p
	for ch := range b.subs {
		select {
		case ch <- p:
		default:
		}
	}
	b.mu.Unlock()
}

func (b *ProgressBroadcaster) Get() ModelProgress {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.cur
}

func (b *ProgressBroadcaster) Subscribe() chan ModelProgress {
	ch := make(chan ModelProgress, 8)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *ProgressBroadcaster) Unsubscribe(ch chan ModelProgress) {
	b.mu.Lock()
	delete(b.subs, ch)
	b.mu.Unlock()
	close(ch)
}

func (b *IngestBroadcaster) Set(p IngestProgress) {
	b.mu.Lock()
	b.cur = p
	for ch := range b.subs {
		select {
		case ch <- p:
		default:
		}
	}
	b.mu.Unlock()
}

func (b *IngestBroadcaster) Get() IngestProgress {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.cur
}

func (b *IngestBroadcaster) Subscribe() chan IngestProgress {
	ch := make(chan IngestProgress, 8)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *IngestBroadcaster) Unsubscribe(ch chan IngestProgress) {
	b.mu.Lock()
	delete(b.subs, ch)
	b.mu.Unlock()
	close(ch)
}

// ── Repo code index progress ────────────────────────────────────────

// RepoIndexProgress tracks the progress of a repo code indexing operation.
type RepoIndexProgress struct {
	ProjectID   string `json:"projectId"`
	RepoID      string `json:"repoId"`
	State       string `json:"state"` // "indexing" | "indexed" | "error"
	Message     string `json:"message"`
	TotalFiles  int    `json:"totalFiles"`
	TotalChunks int    `json:"totalChunks"`
	DoneChunks  int    `json:"doneChunks"`
	Percent     int    `json:"percent"`
}

// RepoIndexBroadcaster fans out repo indexing progress to SSE subscribers.
type RepoIndexBroadcaster struct {
	mu   sync.RWMutex
	cur  RepoIndexProgress
	subs map[chan RepoIndexProgress]struct{}
}

// RepoIndexTracker is the global repo index progress tracker.
var RepoIndexTracker = &RepoIndexBroadcaster{
	subs: make(map[chan RepoIndexProgress]struct{}),
}

func (b *RepoIndexBroadcaster) Set(p RepoIndexProgress) {
	b.mu.Lock()
	b.cur = p
	for ch := range b.subs {
		select {
		case ch <- p:
		default:
		}
	}
	b.mu.Unlock()
}

func (b *RepoIndexBroadcaster) Get() RepoIndexProgress {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.cur
}

func (b *RepoIndexBroadcaster) Subscribe() chan RepoIndexProgress {
	ch := make(chan RepoIndexProgress, 8)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *RepoIndexBroadcaster) Unsubscribe(ch chan RepoIndexProgress) {
	b.mu.Lock()
	delete(b.subs, ch)
	b.mu.Unlock()
	close(ch)
}
