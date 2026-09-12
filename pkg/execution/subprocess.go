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
	"github.com/msh-protocol/msh/pkg/protocol"
)

// SubprocessEngine implements the Engine interface for local subprocess execution.
type SubprocessEngine struct{}

func NewSubprocessEngine() *SubprocessEngine {
	return &SubprocessEngine{}
}

func (s *SubprocessEngine) Run(ctx context.Context, req protocol.ExecRequest, cwd string, env []string) (string, string, int, int, error) {
	answers := req.PromptAnswers

	if req.UsePty {
		return s.runPTY(ctx, req, cwd, env, answers)
	}
	return s.runPipes(ctx, req, cwd, env, answers)
}

// runPipes executes the command with piped stdout/stderr/stdin and feeds
// prompt answers in real-time when provided.
func (s *SubprocessEngine) runPipes(ctx context.Context, req protocol.ExecRequest, cwd string, env []string, answers []string) (string, string, int, int, error) {
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
	if len(answers) > 0 {
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
		answerer = newPromptAnswerer(stdin, answers)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if answerer != nil {
			_, _ = io.Copy(&streamScanner{out: &stdoutBuf, answerer: answerer}, stdoutPipe)
		} else {
			_, _ = io.Copy(&stdoutBuf, stdoutPipe)
		}
	}()
	go func() {
		defer wg.Done()
		if answerer != nil {
			_, _ = io.Copy(&streamScanner{out: &stderrBuf, answerer: answerer}, stderrPipe)
		} else {
			_, _ = io.Copy(&stderrBuf, stderrPipe)
		}
	}()

	err = cmd.Wait()
	if stdin != nil {
		stdin.Close() // signal EOF once the process has exited
	}

	// Drain remaining buffered output.
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
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
// stdout, and feeds prompt answers to the PTY master in real-time.
func (s *SubprocessEngine) runPTY(ctx context.Context, req protocol.ExecRequest, cwd string, env []string, answers []string) (string, string, int, int, error) {
	ptmx, err := pty.New()
	if err != nil {
		return "", "", -1, 0, err
	}
	defer ptmx.Close()

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

	answerer := newPromptAnswerer(ptmx, answers)
	var stdoutBuf bytes.Buffer

	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&streamScanner{out: &stdoutBuf, answerer: answerer}, ptmx)
		close(done)
	}()

	err = ptyCmd.Wait()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
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