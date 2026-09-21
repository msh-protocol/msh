package fs

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// RollbackSingleFile reverts filesystem changes for a single file from an execution.
func RollbackSingleFile(cwd string, relPath string, diff string) error {
	if relPath == "" {
		return fmt.Errorf("file path cannot be empty")
	}
	normPath := filepath.ToSlash(filepath.Clean(relPath))
	return rollbackOne(cwd, normPath, diff, isGitRepo(cwd))
}

// RollbackExecution reverts the filesystem changes made by a specific execution.
// It uses git reverse patch application where possible, and handles created/deleted files directly.
func RollbackExecution(cwd string, filesChanged []string, fileDiffs map[string]string) ([]string, error) {
	if len(filesChanged) == 0 && len(fileDiffs) == 0 {
		return nil, nil
	}

	// Merge all target files
	targets := make(map[string]bool)
	for _, f := range filesChanged {
		if f != "" {
			targets[filepath.ToSlash(filepath.Clean(f))] = true
		}
	}
	for f := range fileDiffs {
		if f != "" {
			targets[filepath.ToSlash(filepath.Clean(f))] = true
		}
	}

	var reverted []string
	var skippedBinary []string
	var errs []string
	isGit := isGitRepo(cwd)

	for relPath := range targets {
		diff := fileDiffs[relPath]
		if isBinaryAsset(relPath, diff) {
			// Try git checkout if tracked in git HEAD
			if isGit {
				cmd := exec.Command("git", "checkout", "HEAD", "--", relPath)
				cmd.Dir = cwd
				if err := cmd.Run(); err == nil {
					reverted = append(reverted, relPath)
					continue
				}
			}
			skippedBinary = append(skippedBinary, relPath)
			continue
		}

		if err := rollbackOne(cwd, relPath, diff, isGit); err != nil {
			errs = append(errs, err.Error())
		} else {
			reverted = append(reverted, relPath)
		}
	}

	if len(reverted) == 0 {
		if len(skippedBinary) > 0 && len(errs) == 0 {
			return nil, fmt.Errorf("rollback unavailable for binary asset(s): %s (compiled payloads cannot be reverse-patched)", strings.Join(skippedBinary, ", "))
		}
		if len(errs) > 0 {
			return nil, fmt.Errorf("rollback failed: %s", strings.Join(errs, "; "))
		}
	}

	return reverted, nil
}

func rollbackOne(cwd, relPath, diff string, isGit bool) error {
	fullPath := filepath.Join(cwd, filepath.FromSlash(relPath))

	// Binary assets cannot be reverse-patched textually
	if isBinaryAsset(relPath, diff) {
		if isGit {
			cmd := exec.Command("git", "checkout", "HEAD", "--", relPath)
			cmd.Dir = cwd
			if err := cmd.Run(); err == nil {
				return nil
			}
		}
		if strings.Contains(diff, "--- /dev/null") && fileExists(fullPath) {
			if err := os.Remove(fullPath); err == nil {
				return nil
			}
		}
		return fmt.Errorf("binary asset %s cannot be reverse-patched: compiled payloads and untracked binaries lack textual diff history", relPath)
	}

	// Scenario A: Newly created / added file (diff shows --- /dev/null)
	if strings.Contains(diff, "--- /dev/null") || (!fileExists(fullPath) && isGit) {
		if fileExists(fullPath) {
			if err := os.Remove(fullPath); err != nil {
				return fmt.Errorf("failed to delete created file %s: %w", relPath, err)
			}
			return nil
		}
	}

	// Scenario B: Deleted file (diff shows +++ /dev/null)
	if strings.Contains(diff, "+++ /dev/null") {
		// Restore deleted file from diff lines
		content := extractDeletedContent(diff)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err == nil {
			if err := os.WriteFile(fullPath, []byte(content), 0644); err == nil {
				return nil
			}
		}
	}

	// Scenario C: Modified file inside Git repo
	if isGit {
		// First try git apply --reverse with the stored diff
		if diff != "" {
			cmd := exec.Command("git", "apply", "--reverse", "--whitespace=nowarn")
			cmd.Dir = cwd
			cmd.Stdin = strings.NewReader(diff)
			if err := cmd.Run(); err == nil {
				return nil
			}
		}

		// Fallback: restore file to HEAD via git checkout
		cmd := exec.Command("git", "checkout", "HEAD", "--", relPath)
		cmd.Dir = cwd
		if err := cmd.Run(); err == nil {
			return nil
		}
	}

	// Scenario D: Synthetic rollback for non-git workspaces
	if diff != "" {
		if ok := applySyntheticReverse(fullPath, diff); ok {
			return nil
		}
	}

	return fmt.Errorf("unable to cleanly rollback %s", relPath)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// extractDeletedContent reconstructs original file content from a deletion diff.
func extractDeletedContent(diff string) string {
	var lines []string
	for _, l := range strings.Split(diff, "\n") {
		if strings.HasPrefix(l, "-") && !strings.HasPrefix(l, "---") {
			lines = append(lines, strings.TrimPrefix(l, "-"))
		}
	}
	return strings.Join(lines, "\n")
}

// applySyntheticReverse attempts to reverse an additions-only or deletions-only diff without git.
func applySyntheticReverse(fullPath, diff string) bool {
	if strings.Contains(diff, "--- /dev/null") {
		// Was added, so delete it
		return os.Remove(fullPath) == nil
	}
	if strings.Contains(diff, "+++ /dev/null") {
		// Was deleted, restore lines
		content := extractDeletedContent(diff)
		return os.WriteFile(fullPath, []byte(content), 0644) == nil
	}
	return false
}

// isBinaryAsset checks if a relative path or diff corresponds to a binary payload.
func isBinaryAsset(relPath, diff string) bool {
	lowerDiff := strings.ToLower(diff)
	if strings.Contains(lowerDiff, "binary file") || strings.Contains(lowerDiff, "binary files") {
		return true
	}
	ext := strings.ToLower(filepath.Ext(relPath))
	switch ext {
	case ".exe", ".dll", ".so", ".dylib", ".bin", ".iso", ".img", ".png", ".jpg", ".jpeg", ".gif", ".ico", ".webp", ".pdf", ".zip", ".tar", ".gz", ".wasm":
		return true
	}
	return false
}
