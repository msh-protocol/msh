package execution

import (
	"strings"
	"testing"

	"github.com/msh-protocol/msh/pkg/protocol"
)

// TestExecuteAnswersPromptUsePty verifies the use_pty + prompt_answers path:
// a command demanding a TTY still receives answers and echoes them back.
func TestExecuteAnswersPromptUsePty(t *testing.T) {
	session, err := NewSession("")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	req := protocol.ExecRequest{
		Command:       promptCommand(t),
		PromptAnswers: []string{"y"},
		UsePty:        true,
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
		t.Fatalf("expected answered output in PTY stdout, got %q", resp.Stdout)
	}
}

// TestExecuteAnswersMultiplePromptsUsePty verifies answers are consumed in
// order across several prompts inside a single PTY session.
func TestExecuteAnswersMultiplePromptsUsePty(t *testing.T) {
	session, err := NewSession("")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	req := protocol.ExecRequest{
		Command:       doublePromptCommand(t),
		PromptAnswers: []string{"yes", "no"},
		UsePty:        true,
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

// TestExecuteStreamingLiveAnswersUsePty verifies the streaming path answers
// prompts mid-flight from the live source when running under a PTY.
func TestExecuteStreamingLiveAnswersUsePty(t *testing.T) {
	session, err := NewSession("")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	liveCh := make(chan string, 2)
	liveCh <- "yes"
	liveCh <- "no"
	live := func() (string, bool) {
		select {
		case a := <-liveCh:
			return a, true
		default:
			return "", false
		}
	}

	sink := &collectSink{}
	req := protocol.ExecRequest{
		Command:       doublePromptCommand(t),
		UsePty:        true,
		MaxOutputLines: 10,
	}

	executor := NewExecutor(session)
	resp := executor.ExecuteStreaming(t.Context(), req, sink, live)

	if resp.AnswersUsed != 2 {
		t.Fatalf("expected answers_used=2, got %d", resp.AnswersUsed)
	}
	if resp.Status == protocol.StatusBlocked {
		t.Fatalf("expected run to complete, got blocked on %q", resp.PromptDetected)
	}
	if !strings.Contains(resp.Stdout, "first=yes") || !strings.Contains(resp.Stdout, "second=no") {
		t.Fatalf("expected both live answers echoed back, got %q", resp.Stdout)
	}

	_, _, prompts, awaiting := sink.snapshot()
	if len(prompts) == 0 {
		t.Fatal("expected prompt events over the PTY stream")
	}
	for i, a := range awaiting {
		if i%2 == 0 && a {
			t.Fatalf("expected awaiting=false before answer at index %d, got true (full: %v)", i, awaiting)
		}
	}
}