package fs

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const maxDiffLines = 500

// GenerateDiffs computes unified git-style diffs for all changed files.
// It prioritizes `git diff` if cwd is inside a git repository, and falls back
// to synthetic unified diffs for untracked/newly created files or non-git workspaces.
func GenerateDiffs(cwd string, changedFiles []string) map[string]string {
	if len(changedFiles) == 0 {
		return nil
	}

	diffs := make(map[string]string)
	isGit := isGitRepo(cwd)

	for _, relPath := range changedFiles {
		cleanRel := filepath.ToSlash(filepath.Clean(relPath))
		if cleanRel == "." || cleanRel == "" {
			continue
		}

		fullPath := filepath.Join(cwd, filepath.FromSlash(cleanRel))

		var diff string
		if isGit {
			diff = gitDiffForFile(cwd, cleanRel, fullPath)
		}

		// Fallback for non-git or if git did not produce diff
		if diff == "" {
			diff = syntheticDiffForFile(cleanRel, fullPath)
		}

		if diff != "" {
			diffs[cleanRel] = truncateDiff(diff, maxDiffLines)
		}
	}

	if len(diffs) == 0 {
		return nil
	}
	return diffs
}

// isGitRepo checks whether the directory is inside a git work tree.
func isGitRepo(cwd string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = cwd
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

// gitDiffForFile attempts to extract git diff for a specific relative file path.
func gitDiffForFile(cwd, relPath, fullPath string) string {
	// 1. Try unstaged + staged diff
	cmd := exec.Command("git", "diff", "--no-color", "-U3", "HEAD", "--", relPath)
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err == nil && len(bytes.TrimSpace(out)) > 0 {
		return string(out)
	}

	// 2. Try working tree diff without HEAD (e.g. if new repo with no commits)
	cmd2 := exec.Command("git", "diff", "--no-color", "-U3", "--", relPath)
	cmd2.Dir = cwd
	out2, err2 := cmd2.Output()
	if err2 == nil && len(bytes.TrimSpace(out2)) > 0 {
		return string(out2)
	}

	// 3. If file is untracked (newly created), git diff might be empty.
	// Check status:
	statusCmd := exec.Command("git", "status", "--porcelain", "--", relPath)
	statusCmd.Dir = cwd
	statusOut, sErr := statusCmd.Output()
	if sErr == nil && len(bytes.TrimSpace(statusOut)) > 0 {
		// If untracked or added, generate an additions diff
		return syntheticDiffForFile(relPath, fullPath)
	}

	return ""
}

// syntheticDiffForFile generates a unified diff for created or deleted files.
func syntheticDiffForFile(relPath, fullPath string) string {
	info, err := os.Stat(fullPath)
	if os.IsNotExist(err) {
		// File was deleted
		return fmt.Sprintf("--- a/%s\n+++ /dev/null\n@@ -1 +0,0 @@\n-[deleted]\n", relPath)
	}
	if err != nil || info.IsDir() {
		return ""
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return ""
	}

	// Quick binary check (NUL byte within first 8KB)
	checkLen := len(content)
	if checkLen > 8192 {
		checkLen = 8192
	}
	if bytes.IndexByte(content[:checkLen], 0) != -1 {
		return fmt.Sprintf("Binary file %s has been modified or created\n", relPath)
	}

	lines := strings.Split(string(content), "\n")
	// Handle trailing empty line from split
	totalLines := len(lines)
	if totalLines > 0 && lines[totalLines-1] == "" {
		lines = lines[:totalLines-1]
		totalLines--
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("--- /dev/null\n+++ b/%s\n@@ -0,0 +1,%d @@\n", relPath, totalLines))
	for _, l := range lines {
		sb.WriteString("+")
		sb.WriteString(l)
		sb.WriteString("\n")
	}

	return sb.String()
}

// truncateDiff limits diff output to maxLines preserving context.
func truncateDiff(diff string, maxLines int) string {
	lines := strings.Split(diff, "\n")
	if len(lines) <= maxLines {
		return diff
	}

	truncated := append(lines[:maxLines], fmt.Sprintf("\n[msh: diff truncated to %d lines to preserve token limit]", maxLines))
	return strings.Join(truncated, "\n")
}
