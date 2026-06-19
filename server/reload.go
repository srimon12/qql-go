package server

import (
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
)

// PolicyReloader watches a policy file and atomically swaps the policy engine
// on change. Zero downtime — in-flight requests use the old policy until the
// swap completes.
type PolicyReloader struct {
	path    string
	engine  atomic.Pointer[PolicyEngine]
	watcher *fsnotify.Watcher
	stopCh  chan struct{}
}

// NewPolicyReloader creates a reloader that watches the given policy file.
// The initial policy is loaded immediately. Returns an error if the file
// cannot be parsed.
func NewPolicyReloader(path string) (*PolicyReloader, error) {
	engine, err := NewPolicyEngine(path)
	if err != nil {
		return nil, fmt.Errorf("initial policy load failed: %w", err)
	}

	r := &PolicyReloader{
		path:   path,
		stopCh: make(chan struct{}),
	}
	r.engine.Store(engine)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("fsnotify init failed: %w", err)
	}
	if err := watcher.Add(path); err != nil {
		watcher.Close()
		return nil, fmt.Errorf("watch %s failed: %w", path, err)
	}
	r.watcher = watcher

	go r.loop()

	fmt.Fprintf(os.Stderr, "policy: watching %s for changes\n", path)
	return r, nil
}

// Engine returns the current policy engine. Safe for concurrent use.
func (r *PolicyReloader) Engine() *PolicyEngine {
	return r.engine.Load()
}

// Stop closes the file watcher.
func (r *PolicyReloader) Stop() {
	close(r.stopCh)
	r.watcher.Close()
}

func (r *PolicyReloader) loop() {
	// Debounce: editors often write multiple times in rapid succession.
	var timer *time.Timer

	for {
		select {
		case event, ok := <-r.watcher.Events:
			if !ok {
				return
			}
			// Only react to writes and creates (not chmod).
			if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
				continue
			}
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(500*time.Millisecond, r.reload)

		case err, ok := <-r.watcher.Errors:
			if !ok {
				return
			}
			fmt.Fprintf(os.Stderr, "policy watcher error: %v\n", err)

		case <-r.stopCh:
			return
		}
	}
}

func (r *PolicyReloader) reload() {
	newEngine, err := NewPolicyEngine(r.path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "policy reload failed (keeping previous): %v\n", err)
		return
	}
	r.engine.Store(newEngine)
	fmt.Fprintf(os.Stderr, "policy: reloaded %s (%d rules)\n", r.path, len(newEngine.config.Rules))
}
