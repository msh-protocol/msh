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

// promptCommand returns a shell command that prints a "[y/N]" prompt, reads a
// line from stdin, echoes it, and exits 0. Cross-platform via cmd.exe / sh.
func promptCommand(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		return writeBatch(t, "@echo off\r\nset /p ans=Do you want to continue? [y/N]: \r\necho got:%ans%\r\n")
	}
	return `printf 'Do you want to continue? [y/N]: '; read ans; echo "got:$ans"`
}

// doublePromptCommand prints two consecutive prompts and echoes both answers.
func doublePromptCommand(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		return writeBatch(t, "@echo off\r\nset /p a=Proceed? [y/N] \r\nset /p b=Continue? [y/N] \r\necho first=%a% second=%b%\r\n")
	}
	return `printf 'Proceed? [y/N] '; read a; printf 'Continue? [y/N] '; read b; echo "first=$a second=$b"`
}

// writeBatch writes a Windows batch file into a temp dir and returns its path.
func writeBatch(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "prompt.cmd")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write batch file: %v", err)
	}
	return path
}

// TestExecuteAnswersPrompt verifies that supplying PromptAnswers completes an
// interactive command instead of blocking it, via the real subprocess engine.
func TestExecuteAnswersPrompt(t *testing.T) {
	session, err := NewSession("")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	req := protocol.ExecRequest{
		Command:        promptCommand(t),
		PromptAnswers:  []string{"y"},
		MaxOutputLines: 10,
	}

	executor := NewExecutor(session)
	resp := executor.Execute(req)

	if resp.AnswersUsed != 1 {
		t.Fatalf("expected answers_used=1, got %d", resp.AnswersUsed)
	}
	if resp.Status == protocol.StatusBlocked {
		t.Fatalf("expected run to complete, got blocked on %q", resp.PromptDetected)
	}
	if !strings.Contains(resp.Stdout, "got:y") {
		t.Fatalf("expected answered output in stdout, got %q", resp.Stdout)
	}
	if resp.PromptDetected != "" {
		t.Fatalf("expected no lingering prompt, got %q", resp.PromptDetected)
	}
}

// TestExecuteAnswersMultiplePrompts verifies answers are consumed in order
// across several prompts in a single command.
func TestExecuteAnswersMultiplePrompts(t *testing.T) {
	session, err := NewSession("")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	req := protocol.ExecRequest{
		Command:        doublePromptCommand(t),
		PromptAnswers:  []string{"yes", "no"},
		MaxOutputLines: 10,
	}

	executor := NewExecutor(session)
	resp := executor.Execute(req)

	if resp.AnswersUsed != 2 {
		t.Fatalf("expected answers_used=2, got %d", resp.AnswersUsed)
	}
	if resp.Status == protocol.StatusBlocked {
		t.Fatalf("expected run to complete, got blocked on %q", resp.PromptDetected)
	}
	if !strings.Contains(resp.Stdout, "first=yes") || !strings.Contains(resp.Stdout, "second=no") {
		t.Fatalf("expected both answers echoed back, got %q", resp.Stdout)
	}
}

// TestExecutePromptWithoutAnswersDetected verifies that without PromptAnswers
// the existing prompt-detection behavior still applies: the prompt text is
// reported in the response and no answers are consumed. The process reads EOF
// (or times out), but the prompt is always flagged for the agent.
func TestExecutePromptWithoutAnswersDetected(t *testing.T) {
	session, err := NewSession("")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	req := protocol.ExecRequest{
		Command:       promptCommand(t),
		Timeout:       500 * time.Millisecond,
		MaxOutputLines: 10,
	}

	executor := NewExecutor(session)
	resp := executor.Execute(req)

	if resp.PromptDetected == "" {
		t.Fatalf("expected prompt_detected to be set, got %q", resp.PromptDetected)
	}
	if resp.AnswersUsed != 0 {
		t.Fatalf("expected answers_used=0, got %d", resp.AnswersUsed)
	}
}