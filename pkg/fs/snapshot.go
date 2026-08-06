// Package fs provides filesystem change detection for the msh runtime.
//
// After every command execution, agents need to know which files were
// added, modified, or deleted. This package implements fast snapshot-based
// diffing using file metadata (mtime + size) instead of content hashing,
// keeping overhead minimal for small-to-medium workspaces.
package fs

import (
	"os"
	"path/filepath"
	"strings"
)

// FileChange represents a detected filesystem mutation.
type FileChange struct {
	// Path is the relative path from the workspace root.
	Path string `json:"path"`

	// Type indicates the kind of change: "added", "modified", or "deleted".
	Type ChangeType `json:"type"`
}

// ChangeType represents the kind of filesystem change.
type ChangeType string

const (
	ChangeAdded    ChangeType = "added"
	ChangeModified ChangeType = "modified"
	ChangeDeleted  ChangeType = "deleted"
)

// fileEntry stores lightweight metadata for a single file.
// We intentionally avoid content hashing for speed — stat calls
// are orders of magnitude faster than file reads.
type fileEntry struct {
	Size    int64
	ModTime int64 // Unix timestamp in nanoseconds
	IsDir   bool
}

// Snapshot represents the filesystem state at a point in time.
// It maps relative file paths to their metadata.
type Snapshot struct {
	Root    string
	Entries map[string]fileEntry
}

// TakeSnapshot walks the workspace directory and records metadata
// for every file, skipping directories that match ignore patterns.
//
// For workspaces under ~5000 files, this completes in <10ms.
// For larger workspaces, consider using fsnotify-based watching instead.
func TakeSnapshot(root string, ignorePatterns []string) (*Snapshot, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	snap := &Snapshot{
		Root:    absRoot,
		Entries: make(map[string]fileEntry),
	}

	err = filepath.Walk(absRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Skip files/dirs we can't read
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(absRoot, path)
		if err != nil {
			return nil
		}

		// Skip root itself
		if relPath == "." {
			return nil
		}

		// Normalize to forward slashes for consistency
		relPath = filepath.ToSlash(relPath)

		// Check ignore patterns
		if shouldIgnore(relPath, info.IsDir(), ignorePatterns) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip directories from entries (we only track files)
		if info.IsDir() {
			return nil
		}

		snap.Entries[relPath] = fileEntry{
			Size:    info.Size(),
			ModTime: info.ModTime().UnixNano(),
			IsDir:   info.IsDir(),
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return snap, nil
}

// DiffSnapshots compares two snapshots and returns a list of changes.
// It detects added, modified, and deleted files by comparing metadata.
//
// A file is considered "modified" if its size or modification time changed.
// This avoids expensive content hashing while catching the vast majority
// of real modifications.
func DiffSnapshots(before, after *Snapshot) []FileChange {
	var changes []FileChange

	// Check for modified and deleted files
	for path, beforeEntry := range before.Entries {
		afterEntry, exists := after.Entries[path]
		if !exists {
			changes = append(changes, FileChange{
				Path: path,
				Type: ChangeDeleted,
			})
		} else if beforeEntry.Size != afterEntry.Size ||
			beforeEntry.ModTime != afterEntry.ModTime {
			changes = append(changes, FileChange{
				Path: path,
				Type: ChangeModified,
			})
		}
	}

	// Check for added files
	for path := range after.Entries {
		if _, exists := before.Entries[path]; !exists {
			changes = append(changes, FileChange{
				Path: path,
				Type: ChangeAdded,
			})
		}
	}

	return changes
}

// shouldIgnore checks if a path should be excluded from snapshots.
func shouldIgnore(relPath string, isDir bool, patterns []string) bool {
	// Get the base name for pattern matching
	base := filepath.Base(relPath)

	for _, pattern := range patterns {
		// Direct name match (e.g., "node_modules", ".git")
		if base == pattern {
			return true
		}

		// Check if any path component matches the pattern
		parts := strings.Split(filepath.ToSlash(relPath), "/")
		for _, part := range parts {
			if part == pattern {
				return true
			}
		}
	}

	// Skip hidden files/directories (starting with .)
	// except for important config files
	if strings.HasPrefix(base, ".") && base != ".env" && base != ".env.msh" {
		return true
	}

	return false
}
