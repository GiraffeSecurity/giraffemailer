package archive

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Progress tracks the live state of a sync run for one account.
type Progress struct {
	mu sync.RWMutex

	AccountID string `json:"account_id"`

	// Phase 1 (indexing)
	IndexedTotal   int64 `json:"indexed_total"`
	IndexedCount   int64 `json:"indexed_count"`
	IndexedPercent float64 `json:"indexed_pct"`

	// Phase 2 (archiving)
	ArchiveTotal    int64   `json:"archive_total"`
	ArchivedCount   int64   `json:"archived_count"`
	ArchivedPercent float64 `json:"archived_pct"`
	BytesDownloaded int64   `json:"bytes_downloaded"`

	Status string `json:"status"` // idle | indexing | archiving | done | error
	Error  string `json:"error,omitempty"`
}

func newProgress(accountID string) *Progress {
	return &Progress{AccountID: accountID, Status: "idle"}
}

func (p *Progress) setIndexing(total int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.IndexedTotal = total
	p.Status = "indexing"
}

func (p *Progress) incIndexed(n int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.IndexedCount += n
	if p.IndexedTotal > 0 {
		p.IndexedPercent = float64(p.IndexedCount) / float64(p.IndexedTotal) * 100
	}
}

func (p *Progress) setArchiving(total int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ArchiveTotal = total
	p.Status = "archiving"
}

func (p *Progress) incArchived(count, bytes int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ArchivedCount += count
	p.BytesDownloaded += bytes
	if p.ArchiveTotal > 0 {
		p.ArchivedPercent = float64(p.ArchivedCount) / float64(p.ArchiveTotal) * 100
	}
}

func (p *Progress) setDone() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Status = "done"
}

func (p *Progress) setError(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Status = "error"
	p.Error = err.Error()
}

func (p *Progress) snapshot() Progress {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return *p
}

// Tracker holds live progress for all accounts.
type Tracker struct {
	mu    sync.RWMutex
	state map[string]*Progress
}

func NewTracker() *Tracker {
	return &Tracker{state: make(map[string]*Progress)}
}

func (t *Tracker) get(accountID string) *Progress {
	t.mu.Lock()
	defer t.mu.Unlock()
	p, ok := t.state[accountID]
	if !ok {
		p = newProgress(accountID)
		t.state[accountID] = p
	}
	return p
}

func (t *Tracker) Snapshot(accountID string) *Progress {
	t.mu.RLock()
	p := t.state[accountID]
	t.mu.RUnlock()
	if p == nil {
		return &Progress{AccountID: accountID, Status: "idle"}
	}
	s := p.snapshot()
	return &s
}

// SSEHandler streams live progress for one account as Server-Sent Events.
// The client can close the connection to stop the stream.
func (t *Tracker) SSEHandler(accountID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		ctx := r.Context()
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				snap := t.Snapshot(accountID)
				data, err := json.Marshal(snap)
				if err == nil {
					fmt.Fprintf(w, "data: %s\n\n", data)
					flusher.Flush()
				}
				if snap.Status == "done" || snap.Status == "error" {
					return
				}
			}
		}
	}
}
