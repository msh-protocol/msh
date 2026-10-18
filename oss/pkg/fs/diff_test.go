package fs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateDiffs_SyntheticAdded(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(filePath, []byte("line1\nline2\nline3\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	diffs := GenerateDiffs(tempDir, []string{"test.txt"})
	if diffs == nil {
		t.Fatal("expected diffs, got nil")
	}

	diff, exists := diffs["test.txt"]
	if !exists {
		t.Fatal("expected diff for test.txt")
	}

	if !strings.Contains(diff, "+++ b/test.txt") {
		t.Errorf("expected diff to contain +++ b/test.txt, got: %s", diff)
	}
	if !strings.Contains(diff, "+line1") || !strings.Contains(diff, "+line2") {
		t.Errorf("expected diff to contain added lines, got: %s", diff)
	}
}

func TestGenerateDiffs_SyntheticDeleted(t *testing.T) {
	tempDir := t.TempDir()
	diffs := GenerateDiffs(tempDir, []string{"nonexistent.txt"})
	if diffs == nil {
		t.Fatal("expected diffs, got nil")
	}

	diff, exists := diffs["nonexistent.txt"]
	if !exists {
		t.Fatal("expected diff for nonexistent.txt")
	}

	if !strings.Contains(diff, "--- a/nonexistent.txt") || !strings.Contains(diff, "+++ /dev/null") {
		t.Errorf("expected deletion diff, got: %s", diff)
	}
}
