package sandbox

import (
	"context"
	"log"
	"sync"
	"time"
)

// TimeoutManager tracks sandbox lifetimes and automatically destroys
// sandboxes that exceed their configured timeout.
type TimeoutManager struct {
	mu      sync.Mutex
	entries map[string]*timeoutEntry
}

type timeoutEntry struct {
	timer   *time.Timer
	timeout time.Duration
	created time.Time
}

// NewTimeoutManager creates a new TimeoutManager.
func NewTimeoutManager() *TimeoutManager {
	return &TimeoutManager{
		entries: make(map[string]*timeoutEntry),
	}
}

// Register schedules automatic cleanup of a sandbox after the given duration.
// When the timeout expires, destroyFn is called to remove the container.
func (tm *TimeoutManager) Register(id string, timeout time.Duration, destroyFn func()) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Cancel any existing timer for this ID.
	if existing, ok := tm.entries[id]; ok {
		existing.timer.Stop()
	}

	timer := time.AfterFunc(timeout, func() {
		log.Printf("sandbox %s exceeded timeout (%s), auto-destroying", id, timeout)
		destroyFn()
		tm.mu.Lock()
		delete(tm.entries, id)
		tm.mu.Unlock()
	})

	tm.entries[id] = &timeoutEntry{
		timer:   timer,
		timeout: timeout,
		created: time.Now(),
	}
}

// Unregister cancels the timeout for a sandbox (e.g. on manual destroy).
func (tm *TimeoutManager) Unregister(id string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if e, ok := tm.entries[id]; ok {
		e.timer.Stop()
		delete(tm.entries, id)
	}
}

// Remaining returns the time remaining before a sandbox is auto-destroyed.
// Returns (0, false) if the sandbox has no registered timeout.
func (tm *TimeoutManager) Remaining(id string) (time.Duration, bool) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	e, ok := tm.entries[id]
	if !ok {
		return 0, false
	}
	remaining := e.timeout - time.Since(e.created)
	if remaining < 0 {
		remaining = 0
	}
	return remaining, true
}

// ActiveCount returns the number of sandboxes with active timeouts.
func (tm *TimeoutManager) ActiveCount() int {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	return len(tm.entries)
}

// Shutdown cancels all pending timeouts without destroying sandboxes.
func (tm *TimeoutManager) Shutdown() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	for id, e := range tm.entries {
		e.timer.Stop()
		delete(tm.entries, id)
	}
}

// DestroyAll cancels all pending timeouts and destroys all tracked sandboxes.
func (tm *TimeoutManager) DestroyAll(ctx context.Context, provider Provider) {
	tm.mu.Lock()
	ids := make([]string, 0, len(tm.entries))
	for id, e := range tm.entries {
		e.timer.Stop()
		ids = append(ids, id)
	}
	tm.entries = make(map[string]*timeoutEntry)
	tm.mu.Unlock()

	for _, id := range ids {
		sb, err := provider.Get(ctx, id)
		if err != nil {
			continue
		}
		_ = sb.Destroy(ctx)
	}
}
