package execution

import (
	"context"

	"github.com/msh-protocol/msh/pkg/prompts"
	"github.com/msh-protocol/msh/pkg/protocol"
)

// OutputSink receives raw output chunks and prompt events in real time while
// a command is running. Implementations must return quickly: a slow sink
// would otherwise stall command execution.
type OutputSink interface {
	// OnOutput is called with each chunk as it is read from the process.
	// stream is "stdout", "stderr", or "pty" (PTY merges both into one).
	OnOutput(stream string, chunk []byte)

	// OnPrompt is called when an interactive prompt is detected in the
	// stream. answered is true if an answer (pre-supplied or live) was fed
	// to the process; false if the run is now waiting on the caller.
	OnPrompt(prompt string, answered bool)
}

// AnswerProvider returns the next answer to feed a detected prompt. It is
// consulted only after pre-supplied PromptAnswers are exhausted, and may
// block until a live answer arrives. Return ("", false) when no answer is
// available and the run must proceed without one.
type AnswerProvider func() (string, bool)

// StreamingEngine is implemented by engines that can stream output and
// accept runtime answers. The subprocess, docker, and kubernetes engines
// implement it; unknown/custom engines fall back to a buffered run.
type StreamingEngine interface {
	Engine

	// RunStreaming behaves like Run but pushes raw output chunks to sink as
	// they are read and requests extra answers from live when pre-supplied
	// answers run out. policy resolves reusable answers from a committed
	// prompts file (.msh/prompts.yaml) and is consulted before live.
	RunStreaming(ctx context.Context, req protocol.ExecRequest, cwd string, env []string, sink OutputSink, live AnswerProvider, policy prompts.AnswerFunc) (stdout, stderr string, exitCode, answersUsed int, err error)
}