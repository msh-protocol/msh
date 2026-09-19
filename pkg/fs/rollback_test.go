package fs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRollbackExecution_AddedFile(t *testing.T) {
	tempDir := t.TempDir()
	createdFile := filepath.Join(tempDir, "created.txt")
	if err := os.WriteFile(createdFile, []byte("hello world\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	diffs := map[string]string{
		"created.txt": "--- /dev/null\n+++ b/created.txt\n@@ -0,0 +1 @@\n+hello world\n",
	}

	reverted, err := RollbackExecution(tempDir, []string{"created.txt"}, diffs)
	if err != nil {
		t.Fatalf("unexpected error during rollback: %v", err)
	}
	if len(reverted) != 1 || reverted[0] != "created.txt" {
		t.Errorf("expected created.txt to be reverted, got: %v", reverted)
	}

	if _, err := os.Stat(createdFile); !os.IsNotExist(err) {
		t.Errorf("expected created.txt to be removed after rollback")
	}
}

func TestRollbackExecution_DeletedFile(t *testing.T) {
	tempDir := t.TempDir()
	deletedFile := filepath.Join(tempDir, "deleted.txt")

	diffs := map[string]string{
		"deleted.txt": "--- a/deleted.txt\n+++ /dev/null\n@@ -1 +0,0 @@\n-restored line\n",
	}

	reverted, err := RollbackExecution(tempDir, []string{"deleted.txt"}, diffs)
	if err != nil {
		t.Fatalf("unexpected error during rollback: %v", err)
	}
	if len(reverted) != 1 || reverted[0] != "deleted.txt" {
		t.Errorf("expected deleted.txt to be restored, got: %v", reverted)
	}

	content, err := os.ReadFile(deletedFile)
	if err != nil {
		t.Fatalf("expected deleted.txt to exist after rollback: %v", err)
	}
	if string(content) != "restored line" {
		t.Errorf("expected 'restored line', got: '%s'", string(content))
	}
}
