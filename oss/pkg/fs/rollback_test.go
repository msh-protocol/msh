package fs

import (
	"os"
	"path/filepath"
	"strings"
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

func TestRollbackSingleFile(t *testing.T) {
	tempDir := t.TempDir()
	createdFile := filepath.Join(tempDir, "sample.txt")
	if err := os.WriteFile(createdFile, []byte("surgical line\n"), 0644); err != nil {
		t.Fatalf("failed to create sample file: %v", err)
	}

	diff := "--- /dev/null\n+++ b/sample.txt\n@@ -0,0 +1 @@\n+surgical line\n"

	if err := RollbackSingleFile(tempDir, "sample.txt", diff); err != nil {
		t.Fatalf("unexpected error during single file rollback: %v", err)
	}

	if _, err := os.Stat(createdFile); !os.IsNotExist(err) {
		t.Errorf("expected sample.txt to be removed after rollback")
	}
}

func TestRollbackSingleFile_BinaryAsset(t *testing.T) {
	tempDir := t.TempDir()
	binFile := filepath.Join(tempDir, "app.exe")
	if err := os.WriteFile(binFile, []byte{0x4d, 0x5a, 0x90, 0x00}, 0755); err != nil {
		t.Fatalf("failed to create binary test file: %v", err)
	}

	diff := "Binary file app.exe has been modified or created\n"

	// Non-git directory with modified binary file should fail with diagnostic message
	err := RollbackSingleFile(tempDir, "app.exe", diff)
	if err == nil {
		t.Fatalf("expected error when rolling back binary asset, got nil")
	}
	if !strings.Contains(err.Error(), "binary asset") {
		t.Errorf("expected error message to mention binary asset, got: %v", err)
	}
}

func TestRollbackExecution_BinaryAssetGraceful(t *testing.T) {
	tempDir := t.TempDir()
	txtFile := filepath.Join(tempDir, "doc.txt")
	if err := os.WriteFile(txtFile, []byte("text content\n"), 0644); err != nil {
		t.Fatalf("failed to create doc.txt: %v", err)
	}

	diffs := map[string]string{
		"doc.txt": "--- /dev/null\n+++ b/doc.txt\n@@ -0,0 +1 @@\n+text content\n",
		"app.exe": "Binary file app.exe has been modified or created\n",
	}

	// Execution has both text file and binary asset; text file should be rolled back and binary skipped
	reverted, err := RollbackExecution(tempDir, []string{"doc.txt", "app.exe"}, diffs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reverted) != 1 || reverted[0] != "doc.txt" {
		t.Errorf("expected doc.txt to be reverted, got: %v", reverted)
	}
}

