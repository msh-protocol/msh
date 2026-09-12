package execution

import (
	"bytes"
	"strings"
	"testing"
)

// TestStreamScannerFeedsAnswer verifies that a prompt detected mid-stream
// (without a trailing newline) is answered and the raw output is preserved.
func TestStreamScannerFeedsAnswer(t *testing.T) {
	var stdinBuf bytes.Buffer
	answerer := newPromptAnswerer(&stdinBuf, []string{"y"})

	var out bytes.Buffer
	scanner := &streamScanner{out: &out, answerer: answerer}

	if _, err := scanner.Write([]byte("Do you want to continue? [y/N]: ")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if _, err := scanner.Write([]byte("got:y\n")); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	if answerer.answeredCount() != 1 {
		t.Fatalf("expected 1 answer used, got %d", answerer.answeredCount())
	}
	if got := stdinBuf.String(); got != "y\n" {
		t.Fatalf("expected answer \"y\\n\" written to stdin, got %q", got)
	}
	if !strings.Contains(out.String(), "Do you want to continue? [y/N]: ") {
		t.Fatalf("output not preserved fully: %q", out.String())
	}
	if !strings.Contains(out.String(), "got:y") {
		t.Fatalf("output not preserved fully: %q", out.String())
	}
}

// TestStreamScannerMultipleAnswers verifies answers are fed in order across
// multiple prompts, including prompts that complete a line vs. sit at the end.
func TestStreamScannerMultipleAnswers(t *testing.T) {
	var stdinBuf bytes.Buffer
	answerer := newPromptAnswerer(&stdinBuf, []string{"yes", "no"})

	var out bytes.Buffer
	scanner := &streamScanner{out: &out, answerer: answerer}

	_, _ = scanner.Write([]byte("Proceed? [y/N] "))
	_, _ = scanner.Write([]byte("Continue? [y/N] "))
	_, _ = scanner.Write([]byte("\n"))
	_, _ = scanner.Write([]byte("first=yes second=no\n"))

	if answerer.answeredCount() != 2 {
		t.Fatalf("expected 2 answers used, got %d", answerer.answeredCount())
	}
	if got := stdinBuf.String(); got != "yes\nno\n" {
		t.Fatalf("expected answers \"yes\\nno\\n\" in order, got %q", got)
	}
}

// TestStreamScannerAnswersExhausted verifies that once all answers are
// consumed, no further writes occur even if another prompt appears.
func TestStreamScannerAnswersExhausted(t *testing.T) {
	var stdinBuf bytes.Buffer
	answerer := newPromptAnswerer(&stdinBuf, []string{"y"})

	var out bytes.Buffer
	scanner := &streamScanner{out: &out, answerer: answerer}

	_, _ = scanner.Write([]byte("Proceed? [y/N] "))
	_, _ = scanner.Write([]byte("Still running? [y/N] "))

	if answerer.answeredCount() != 1 {
		t.Fatalf("expected only 1 answer used, got %d", answerer.answeredCount())
	}
	if got := stdinBuf.String(); got != "y\n" {
		t.Fatalf("expected single answer, got %q", got)
	}
}

// TestStreamScannerNoPrompts verifies normal output produces no writes.
func TestStreamScannerNoPrompts(t *testing.T) {
	var stdinBuf bytes.Buffer
	answerer := newPromptAnswerer(&stdinBuf, []string{"y"})

	var out bytes.Buffer
	scanner := &streamScanner{out: &out, answerer: answerer}

	_, _ = scanner.Write([]byte("hello world\n"))
	_, _ = scanner.Write([]byte("plain output\n"))

	if answerer.answeredCount() != 0 {
		t.Fatalf("expected 0 answers used, got %d", answerer.answeredCount())
	}
	if stdinBuf.Len() != 0 {
		t.Fatalf("expected no stdin writes, got %q", stdinBuf.String())
	}
}