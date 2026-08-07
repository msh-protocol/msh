package fs

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// Watcher provides real-time filesystem monitoring during command execution.
type Watcher struct {
	watcher *fsnotify.Watcher
	cwd     string
	ignores []string

	mu           sync.Mutex
	filesChanged map[string]bool
	done         chan struct{}
}

// NewWatcher creates and initializes a new filesystem watcher.
func NewWatcher(cwd string, ignores []string) (*Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		watcher:      fw,
		cwd:          cwd,
		ignores:      ignores,
		filesChanged: make(map[string]bool),
		done:         make(chan struct{}),
	}

	return w, nil
}

// Start begins listening for file events. It recursively adds all non-ignored
// subdirectories to the fsnotify watcher.
func (w *Watcher) Start() error {
	// Walk the directory tree to add subdirectories
	err := filepath.WalkDir(w.cwd, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Check if the current path should be ignored
		if w.isIgnored(path) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// We only need to add directories to fsnotify; it watches files inside them automatically.
		if d.IsDir() {
			if err := w.watcher.Add(path); err != nil {
				// Some OS limits (e.g. max user watches) might be hit here.
				// For now, we continue adding what we can.
			}
		}

		return nil
	})

	if err != nil {
		w.watcher.Close()
		return fmt.Errorf("failed to initialize watcher: %w", err)
	}

	// Start the event listening goroutine
	go w.listen()

	return nil
}

// listen processes fsnotify events and records modified files.
func (w *Watcher) listen() {
	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return // Channel closed
			}
			
			// We care about Create, Write, Remove, and Rename.
			// We ignore Chmod (permissions).
			if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) != 0 {
				if !w.isIgnored(event.Name) {
					// Get path relative to the working directory
					relPath, err := filepath.Rel(w.cwd, event.Name)
					if err == nil {
						// Normalize slashes for JSON response
						relPath = filepath.ToSlash(relPath)
						
						w.mu.Lock()
						w.filesChanged[relPath] = true
						w.mu.Unlock()
					}
				}
			}

		case <-w.watcher.Errors:
			// Ignore errors during execution to avoid crashing the session
		case <-w.done:
			return
		}
	}
}

// Stop halts the watcher and returns the list of changed files.
func (w *Watcher) Stop() []string {
	close(w.done)
	w.watcher.Close()

	w.mu.Lock()
	defer w.mu.Unlock()

	var changes []string
	for file := range w.filesChanged {
		changes = append(changes, file)
	}

	return changes
}

// isIgnored checks if the given path matches any of the ignore patterns.
func (w *Watcher) isIgnored(path string) bool {
	// Normalize slashes for checking
	normPath := filepath.ToSlash(path)
	
	// Check if any part of the path matches an ignore pattern
	parts := strings.Split(normPath, "/")
	
	for _, part := range parts {
		for _, ignore := range w.ignores {
			if part == ignore {
				return true
			}
		}
	}
	
	return false
}
