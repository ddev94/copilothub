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

// ProgressBroadcaster tracks embedding model download state and fans out to SSE subscribers.
type ProgressBroadcaster struct {
	mu   sync.RWMutex
	cur  ModelProgress
	subs map[chan ModelProgress]struct{}
}

// EmbedProgress is the global download progress tracker for the cybertron model.
var EmbedProgress = &ProgressBroadcaster{
	cur:  ModelProgress{State: ModelStateUnknown},
	subs: make(map[chan ModelProgress]struct{}),
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
