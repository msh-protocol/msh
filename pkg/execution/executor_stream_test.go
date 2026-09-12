package execution

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/msh-protocol/msh/pkg/protocol"
)

// collectSink captures output chunks and prompt events for assertions.
type collectSink struct {
	mu       sync.Mutex
	streams  []string
	chunks   []string
	prompts  []string
	awaiting []bool
}

func (c *collectSink) OnOutput(stream string, chunk []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.streams = append(c.streams, stream)
	c.chunks = append(c.chunks, string(chunk))
}

func (c *collectSink) OnPrompt(prompt string, answered bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.prompts = append(c.prompts, prompt)
	c.awaiting = append(c.awaiting, answered)
}

func (c *collectSink) snapshot() (streams, chunks, prompts []string, awaiting []bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string{}, c.streams...), append([]string{}, c.chunks...),
		append([]string{}, c.prompts...), append([]bool{}, c.awaiting...)
}

// TestExecuteStreamingChunks verifies raw output chunks reach the sink in real
// time even when no answers are involved.
func TestExecuteStreamingChunks(t *testing.T) {
	session, err := NewSession("")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	sink := &collectSink{}
	req := protocol.ExecRequest{
		Command:        "echo hello stream",
		MaxOutputLines: 10,
	}

	executor := NewExecutor(session)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp := executor.ExecuteStreaming(ctx, req, sink, nil)

	if resp.Status != protocol.StatusSuccess {
		t.Fatalf("expected success, got %s (%s)", resp.Status, resp.Error)
	}
	streams, chunks, prompts, _ := sink.snapshot()
	if len(chunks) == 0 {
		t.Fatal("expected at least one output chunk to reach the sink")
	}
	joined := strings.Join(chunks, "")
	if !strings.Contains(joined, "hello stream") {
		t.Fatalf("expected streamed output to contain command result, got %q", joined)
	}
	if len(streams) == 0 || streams[0] != "stdout" {
		t.Fatalf("expected first stream to be stdout, got %v", streams)
	}
	if len(prompts) != 0 {
		t.Fatalf("expected no prompt events, got %v", prompts)
	}
}

// TestExecuteStreamingLiveAnswers verifies prompts detected mid-stream are
// answered from the live source once pre-supplied answers run out, and that
// the final response reports the consumed answers.
func TestExecuteStreamingLiveAnswers(t *testing.T) {
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
		Command:        doublePromptCommand(t),
		MaxOutputLines: 10,
	}

	executor := NewExecutor(session)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp := executor.ExecuteStreaming(ctx, req, sink, live)

	if resp.AnswersUsed != 2 {
		t.Fatalf("expected answers_used=2, got %d", resp.AnswersUsed)
	}
	if !strings.Contains(resp.Stdout, "first=yes") || !strings.Contains(resp.Stdout, "second=no") {
		t.Fatalf("expected both live answers echoed back, got %q", resp.Stdout)
	}
	if resp.Status == protocol.StatusBlocked {
		t.Fatalf("expected run to complete, got blocked on %q", resp.PromptDetected)
	}

	_, _, prompts, awaiting := sink.snapshot()
	// Each prompt reports an "awaiting" event then an "answered" event.
	if len(prompts) != 4 {
		t.Fatalf("expected 4 prompt events (awaiting+answered per prompt), got %v", prompts)
	}
	want := []bool{false, true, false, true}
	if len(awaiting) != len(want) {
		t.Fatalf("expected %d awaiting flags, got %v", len(want), awaiting)
	}
	for i, a := range awaiting {
		if a != want[i] {
			t.Fatalf("awaiting[%d] = %v, want %v (full: %v)", i, a, want[i], awaiting)
		}
	}
}

// TestExecuteStreamingAwaitingPrompt verifies a prompt with no live answer
// available is reported as awaiting, and the run proceeds without hanging.
func TestExecuteStreamingAwaitingPrompt(t *testing.T) {
	session, err := NewSession("")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	sink := &collectSink{}
	req := protocol.ExecRequest{
		Command:        promptCommand(t),
		MaxOutputLines: 10,
	}

	executor := NewExecutor(session)
	// ExecuteStreaming uses the caller's context (which is expected to carry
	// the request timeout); 500ms here because the process will hang waiting
	// for input that never comes.
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// live returns no answers, but is non-nil so interactive mode engages.
	live := func() (string, bool) { return "", false }

	resp := executor.ExecuteStreaming(ctx, req, sink, live)

	if resp.PromptDetected == "" {
		t.Fatalf("expected prompt_detected to be set, got %q", resp.PromptDetected)
	}
	_, _, prompts, _ := sink.snapshot()
	if len(prompts) == 0 {
		t.Fatal("expected at least one prompt event")
	}
	if resp.AnswersUsed != 0 {
		t.Fatalf("expected answers_used=0, got %d", resp.AnswersUsed)
	}
}