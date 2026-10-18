package branch

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func setupTestGitRepo(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "msh-test-git-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// git init
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	// git config user.name and email
	_ = exec.Command("git", "-C", dir, "config", "user.name", "msh-test").Run()
	_ = exec.Command("git", "-C", dir, "config", "user.email", "test@msh.dev").Run()

	// Initial commit
	initFile := filepath.Join(dir, "README.md")
	if err := os.WriteFile(initFile, []byte("# Test Repo\n"), 0644); err != nil {
		t.Fatalf("failed to write initial file: %v", err)
	}
	_ = exec.Command("git", "-C", dir, "add", "README.md").Run()
	_ = exec.Command("git", "-C", dir, "commit", "-m", "initial commit").Run()

	return dir
}

func TestManagerLifecycle(t *testing.T) {
	repoDir := setupTestGitRepo(t)
	defer os.RemoveAll(repoDir)

	mgr, err := NewManager(repoDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// 1. Create shadow branch
	info, err := mgr.Create("test-hypo-1", "HEAD", "Trial refactor strategy A")
	if err != nil {
		t.Fatalf("Create branch failed: %v", err)
	}
	if info.Name != "test-hypo-1" {
		t.Errorf("expected branch name 'test-hypo-1', got '%s'", info.Name)
	}
	if info.Status != "active" {
		t.Errorf("expected status 'active', got '%s'", info.Status)
	}

	// 2. Prevent duplicate creation
	_, err = mgr.Create("test-hypo-1", "HEAD", "Duplicate")
	if err != ErrBranchExists {
		t.Errorf("expected ErrBranchExists, got %v", err)
	}

	// 3. List branches
	list, err := mgr.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 1 || list[0].Name != "test-hypo-1" {
		t.Errorf("unexpected list result: %+v", list)
	}

	// 4. Run command in shadow branch
	resp, err := mgr.Run("test-hypo-1", "echo 'hello from shadow' > file.txt", 10*time.Second, 100)
	if err != nil {
		t.Fatalf("Run in branch failed: %v", err)
	}
	if resp.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", resp.ExitCode)
	}

	// Verify main repo does NOT have file.txt
	if _, err := os.Stat(filepath.Join(repoDir, "file.txt")); err == nil {
		t.Errorf("main repo unexpectedly contains file.txt (isolation broken!)")
	}

	// Verify shadow branch HAS file.txt
	if _, err := os.Stat(filepath.Join(info.Path, "file.txt")); os.IsNotExist(err) {
		t.Errorf("shadow branch missing file.txt")
	}

	// 5. Diff in shadow branch
	diff, err := mgr.Diff("test-hypo-1")
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}
	if len(diff) == 0 {
		t.Errorf("expected non-empty diff, got empty string")
	}

	// 6. Abort shadow branch
	payload, err := mgr.Abort("test-hypo-1")
	if err != nil {
		t.Fatalf("Abort failed: %v", err)
	}
	if payload.Status != "aborted" {
		t.Errorf("expected aborted status, got %s", payload.Status)
	}
	if payload.ContextPruneTurns < 1 {
		t.Errorf("expected at least 1 turn to prune, got %d", payload.ContextPruneTurns)
	}

	// Verify worktree directory is removed
	if _, err := os.Stat(info.Path); err == nil {
		t.Errorf("shadow worktree dir was not cleaned up after abort")
	}
}

func TestManagerMerge(t *testing.T) {
	repoDir := setupTestGitRepo(t)
	defer os.RemoveAll(repoDir)

	mgr, err := NewManager(repoDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Create and write to branch
	_, err = mgr.Create("merge-test", "HEAD", "Testing merge back to main")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	_, err = mgr.Run("merge-test", "echo 'solution verified' > solution.txt", 10*time.Second, 100)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Merge branch
	err = mgr.Merge("merge-test", "msh: verified solution")
	if err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	// Check solution.txt now exists in main repo
	solPath := filepath.Join(repoDir, "solution.txt")
	if _, err := os.Stat(solPath); os.IsNotExist(err) {
		t.Fatalf("solution.txt was not merged into main working tree")
	}
	content, _ := os.ReadFile(solPath)
	if len(content) == 0 {
		t.Errorf("solution.txt is empty")
	}
}
