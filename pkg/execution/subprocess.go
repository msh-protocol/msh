package execution

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/aymanbagabas/go-pty"
	"github.com/msh-protocol/msh/pkg/prompts"
	"github.com/msh-protocol/msh/pkg/protocol"
)

// SubprocessEngine implements the Engine interface for local subprocess execution.
type SubprocessEngine struct{}

func NewSubprocessEngine() *SubprocessEngine {
	return &SubprocessEngine{}
}

func (s *SubprocessEngine) Run(ctx context.Context, req protocol.ExecRequest, cwd string, env []string) (string, string, int, int, error) {
	return s.RunStreaming(ctx, req, cwd, env, nil, nil, nil)
}

// RunStreaming implements StreamingEngine. When answers are provided it feeds
// them to the process stdin in real time; when sink is set it pushes raw
// output chunks as they are read instead of only at completion.
func (s *SubprocessEngine) RunStreaming(ctx context.Context, req protocol.ExecRequest, cwd string, env []string, sink OutputSink, live AnswerProvider, policy prompts.AnswerFunc) (string, string, int, int, error) {
	answers := req.PromptAnswers

	if req.UsePty {
		return s.runPTY(ctx, req, cwd, env, answers, live, policy, sink)
	}
	return s.runPipes(ctx, req, cwd, env, answers, live, policy, sink)
}

// sinkWriter relays chunks to an underlying writer and optionally to an
// OutputSink for live streaming.
type sinkWriter struct {
	out    io.Writer
	sink   OutputSink
	stream string
}

func (w *sinkWriter) Write(chunk []byte) (int, error) {
	n, err := w.out.Write(chunk)
	if w.sink != nil {
		w.sink.OnOutput(w.stream, chunk)
	}
	return n, err
}

// sinkOf wraps a destination writer with live-output streaming when a sink is
// provided, otherwise returning the writer unchanged.
func sinkOf(out io.Writer, sink OutputSink, stream string) io.Writer {
	if sink == nil {
		return out
	}
	return &sinkWriter{out: out, sink: sink, stream: stream}
}

// ptyLineEnd returns the answer terminator used when feeding a pseudo
// terminal: PTYs on Windows expect a carriage return for Enter to register.
func ptyLineEnd() string {
	if runtime.GOOS == "windows" {
		return "\r\n"
	}
	return "\n"
}

// buildAnswerer creates the prompt answerer used across a streaming run,
// wiring in the optional live answer source, committed answer policy, and
// sink notification.
func buildAnswerer(stdin io.Writer, answers []string, live AnswerProvider, policy prompts.AnswerFunc, sink OutputSink) *promptAnswerer {
	a := newPromptAnswerer(stdin, answers)
	if live != nil {
		a.setLive(live)
	}
	if policy != nil {
		a.setPolicy(policy)
	}
	if sink != nil {
		a.setOnPrompt(sink.OnPrompt)
	}
	return a
}

// runPipes executes the command with piped stdout/stderr/stdin, feeding
// prompt answers (pre-supplied or live) in real time.
func (s *SubprocessEngine) runPipes(ctx context.Context, req protocol.ExecRequest, cwd string, env []string, answers []string, live AnswerProvider, policy prompts.AnswerFunc, sink OutputSink) (string, string, int, int, error) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd.exe", "/C", req.Command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", req.Command)
	}
	cmd.Dir = cwd
	cmd.Env = env

	// Attach a stdin pipe only when answering prompts, so the child sees
	// real input instead of EOF. Otherwise fall back to the original
	// /dev/null behavior.
	var stdin io.WriteCloser
	if len(answers) > 0 || live != nil || policy != nil {
		pw, err := cmd.StdinPipe()
		if err != nil {
			return "", "", -1, 0, err
		}
		stdin = pw
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", "", -1, 0, err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return "", "", -1, 0, err
	}

	if err := cmd.Start(); err != nil {
		return "", "", -1, 0, err
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	var answerer *promptAnswerer
	if stdin != nil {
		answerer = buildAnswerer(stdin, answers, live, policy, sink)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		var out io.Writer = &stdoutBuf
		if answerer != nil {
			out = &streamScanner{out: &stdoutBuf, answerer: answerer}
		}
		_, _ = io.Copy(sinkOf(out, sink, "stdout"), stdoutPipe)
	}()
	go func() {
		defer wg.Done()
		var out io.Writer = &stderrBuf
		if answerer != nil {
			out = &streamScanner{out: &stderrBuf, answerer: answerer}
		}
		_, _ = io.Copy(sinkOf(out, sink, "stderr"), stderrPipe)
	}()

	// Drain remaining buffered output before cmd.Wait() closes the pipes (per os/exec doc).
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		err = cmd.Wait()
	case <-time.After(2 * time.Second):
		err = cmd.Wait()
		<-done
	}

	if stdin != nil {
		_ = stdin.Close() // signal EOF once the process has exited
	}

	answersUsed := 0
	if answerer != nil {
		answersUsed = answerer.answeredCount()
	}

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	return stdoutBuf.String(), stderrBuf.String(), exitCode, answersUsed, err
}

// runPTY executes the command inside a pseudo-terminal, merging stderr into
// stdout, and feeds prompt answers to the PTY master in real time.
func (s *SubprocessEngine) runPTY(ctx context.Context, req protocol.ExecRequest, cwd string, env []string, answers []string, live AnswerProvider, policy prompts.AnswerFunc, sink OutputSink) (string, string, int, int, error) {
	ptmx, err := pty.New()
	if err != nil {
		return "", "", -1, 0, err
	}
	var closeOnce sync.Once
	closePtmx := func() {
		closeOnce.Do(func() {
			_ = ptmx.Close()
		})
	}
	defer closePtmx()

	var ptyCmd *pty.Cmd
	if runtime.GOOS == "windows" {
		cmdPath, _ := exec.LookPath("cmd.exe")
		if cmdPath == "" {
			cmdPath = "cmd.exe"
		}
		ptyCmd = ptmx.CommandContext(ctx, cmdPath, "/C", req.Command)
	} else {
		shPath, _ := exec.LookPath("sh")
		if shPath == "" {
			shPath = "/bin/sh"
		}
		ptyCmd = ptmx.CommandContext(ctx, shPath, "-c", req.Command)
	}
	ptyCmd.Dir = cwd
	ptyCmd.Env = env

	err = ptyCmd.Start()
	if err != nil {
		return "", "", -1, 0, err
	}

	answerer := buildAnswerer(ptmx, answers, live, policy, sink)
	answerer.setLineEnd(ptyLineEnd())
	var stdoutBuf bytes.Buffer

	done := make(chan struct{})
	go func() {
		scanner := &streamScanner{out: &stdoutBuf, answerer: answerer}
		_, _ = io.Copy(sinkOf(scanner, sink, "pty"), ptmx)
		close(done)
	}()

	err = ptyCmd.Wait()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		closePtmx()
		<-done
	}

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	return stdoutBuf.String(), "", exitCode, answerer.answeredCount(), err
}