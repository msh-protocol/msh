package fs

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

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
	var errs []string
	isGit := isGitRepo(cwd)

	for relPath := range targets {
		fullPath := filepath.Join(cwd, filepath.FromSlash(relPath))
		diff := fileDiffs[relPath]

		// Scenario A: Newly created / added file (diff shows --- /dev/null)
		if strings.Contains(diff, "--- /dev/null") || (!fileExists(fullPath) && isGit) {
			if fileExists(fullPath) {
				if err := os.Remove(fullPath); err != nil {
					errs = append(errs, fmt.Sprintf("failed to delete created file %s: %v", relPath, err))
					continue
				}
				reverted = append(reverted, relPath)
				continue
			}
		}

		// Scenario B: Deleted file (diff shows +++ /dev/null)
		if strings.Contains(diff, "+++ /dev/null") {
			// Restore deleted file from diff lines
			content := extractDeletedContent(diff)
			if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err == nil {
				if err := os.WriteFile(fullPath, []byte(content), 0644); err == nil {
					reverted = append(reverted, relPath)
					continue
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
					reverted = append(reverted, relPath)
					continue
				}
			}

			// Fallback: restore file to HEAD via git checkout
			cmd := exec.Command("git", "checkout", "HEAD", "--", relPath)
			cmd.Dir = cwd
			if err := cmd.Run(); err == nil {
				reverted = append(reverted, relPath)
				continue
			}
		}

		// Scenario D: Synthetic rollback for non-git workspaces
		if diff != "" {
			if ok := applySyntheticReverse(fullPath, diff); ok {
				reverted = append(reverted, relPath)
				continue
			}
		}

		errs = append(errs, fmt.Sprintf("unable to cleanly rollback %s", relPath))
	}

	if len(reverted) == 0 && len(errs) > 0 {
		return nil, fmt.Errorf("rollback failed: %s", strings.Join(errs, "; "))
	}

	return reverted, nil
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
