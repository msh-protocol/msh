package execution

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

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

	cmdStr := `printf 'Do you want to continue? [y/N]: '; read ans; echo "got:$ans"`
	if runtime.GOOS == "windows" {
		cmd := writeBatch(t, "@echo off\r\nset /p ans=Do you want to continue? [y/N]: \r\necho got:%ans%\r\n")
		target := filepath.Join(dir, "prompt.cmd")
		if err := os.Rename(cmd, target); err != nil {
			t.Fatal(err)
		}
		cmdStr = target
	}

	session, err := NewSession(dir)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	req := protocol.ExecRequest{
		Command:        cmdStr,
		MaxOutputLines: 10,
	}

	executor := NewExecutor(session)
	resp := executor.Execute(req)

	if resp.AnswersUsed != 1 {
		t.Fatalf("expected answers_used=1 from policy, got %d (stdout: %q, stderr: %q)", resp.AnswersUsed, resp.Stdout, resp.Stderr)
	}
	if resp.Status == protocol.StatusBlocked {
		t.Fatalf("expected run to complete, got blocked on %q", resp.PromptDetected)
	}
	if !strings.Contains(resp.Stdout, "got:y") {
		t.Fatalf("expected policy answer echoed back, got %q", resp.Stdout)
	}
}

// TestExecutePolicyWithoutMatch verifies unmatched prompts still block when a
// policy file exists but has no matching rule.
func TestExecutePolicyWithoutMatch(t *testing.T) {
	dir := t.TempDir()
	writePolicyFile(t, dir, `prompts:
  - match: "unrelated"
    answer: "n"
`)

	cmdStr := `printf 'Proceed? [y/N]: '; read ans; echo "got:$ans"`
	if runtime.GOOS == "windows" {
		cmd := writeBatch(t, "@echo off\r\nset /p ans=Proceed? [y/N]: \r\necho got:%ans%\r\n")
		target := filepath.Join(dir, "prompt.cmd")
		if err := os.Rename(cmd, target); err != nil {
			t.Fatal(err)
		}
		cmdStr = target
	}

	session, err := NewSession(dir)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	req := protocol.ExecRequest{
		Command:        cmdStr,
		Timeout:        500 * time.Millisecond,
		MaxOutputLines: 10,
	}

	executor := NewExecutor(session)
	resp := executor.Execute(req)

	if resp.AnswersUsed != 0 {
		t.Fatalf("expected answers_used=0, got %d", resp.AnswersUsed)
	}
	if resp.PromptDetected == "" {
		t.Fatalf("expected unmatched prompt to be reported, got %q (stdout: %q, stderr: %q)", resp.PromptDetected, resp.Stdout, resp.Stderr)
	}
}