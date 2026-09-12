package execution

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/msh-protocol/msh/pkg/protocol"
)

// writePolicyFile drops a .msh/prompts.yaml into dir and returns its path.
func writePolicyFile(t *testing.T, dir, content string) {
	t.Helper()
	path := filepath.Join(dir, ".msh", "prompts.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// TestExecutePolicyAnswersPrompt verifies a prompt matching a committed
// .msh/prompts.yaml rule is answered without explicit PromptAnswers.
func TestExecutePolicyAnswersPrompt(t *testing.T) {
	dir := t.TempDir()
	writePolicyFile(t, dir, `prompts:
  - match: "continue.*\\[y/N\\]"
    answer: "y"
`)
	// The batch file lives in the session cwd so `set /p` resolves it.
	cmd := writeBatch(t, "@echo off\r\nset /p ans=Do you want to continue? [y/N]: \r\necho got:%ans%\r\n")
	if err := os.Rename(cmd, filepath.Join(dir, "prompt.cmd")); err != nil {
		t.Fatal(err)
	}

	session, err := NewSession(dir)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	req := protocol.ExecRequest{
		Command:       filepath.Join(dir, "prompt.cmd"),
		MaxOutputLines: 10,
	}

	executor := NewExecutor(session)
	resp := executor.Execute(req)

	if resp.AnswersUsed != 1 {
		t.Fatalf("expected answers_used=1 from policy, got %d", resp.AnswersUsed)
	}
	if resp.Status == protocol.StatusBlocked {
		t.Fatalf("expected run to complete, got blocked on %q", resp.PromptDetected)
	}
	if !strings.Contains(resp.Stdout, "got:y") {
		t.Fatalf("expected policy answer echoed back, got %q", resp.Stdout)
	}
}

// TestExecutePolicyWithoutMatchVerifiesUnmatched prompts still block when a
// policy file exists but has no matching rule.
func TestExecutePolicyWithoutMatch(t *testing.T) {
	dir := t.TempDir()
	writePolicyFile(t, dir, `prompts:
  - match: "unrelated"
    answer: "n"
`)
	cmd := writeBatch(t, "@echo off\r\nset /p ans=Proceed? [y/N]: \r\necho got:%ans%\r\n")
	if err := os.Rename(cmd, filepath.Join(dir, "prompt.cmd")); err != nil {
		t.Fatal(err)
	}

	session, err := NewSession(dir)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	req := protocol.ExecRequest{
		Command:       filepath.Join(dir, "prompt.cmd"),
		Timeout:       500 * 1000 * 1000, // 500ms; the command hangs for input
		MaxOutputLines: 10,
	}

	executor := NewExecutor(session)
	resp := executor.Execute(req)

	if resp.AnswersUsed != 0 {
		t.Fatalf("expected answers_used=0, got %d", resp.AnswersUsed)
	}
	if resp.PromptDetected == "" {
		t.Fatalf("expected unmatched prompt to be reported, got %q", resp.PromptDetected)
	}
}