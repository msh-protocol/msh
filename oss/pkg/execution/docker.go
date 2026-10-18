package execution

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/aymanbagabas/go-pty"
	"github.com/msh-protocol/msh/pkg/prompts"
	"github.com/msh-protocol/msh/pkg/protocol"
)

// DockerEngine implements the Engine interface using the local Docker CLI.
type DockerEngine struct{}

func NewDockerEngine() *DockerEngine {
	return &DockerEngine{}
}

func (d *DockerEngine) Run(ctx context.Context, req protocol.ExecRequest, cwd string, env []string) (string, string, int, int, error) {
	return d.RunStreaming(ctx, req, cwd, env, nil, nil, nil)
}

// RunStreaming implements StreamingEngine: output streams to the sink in real
// time and interactive prompts inside the container are answered by feeding
// the next answer to the container stdin via `docker run -i`.
func (d *DockerEngine) RunStreaming(ctx context.Context, req protocol.ExecRequest, cwd string, env []string, sink OutputSink, live AnswerProvider, policy prompts.AnswerFunc) (string, string, int, int, error) {
	answers := req.PromptAnswers

	if req.UsePty {
		return d.runPTY(ctx, req, cwd, env, answers, live, policy, sink)
	}
	return d.runInteractive(ctx, req, cwd, env, answers, live, policy, sink)
}

// buildDockerArgs assembles the shared `docker run` arguments: working dir,
// bind-mount of the host cwd, environment, and image selection.
func buildDockerArgs(req protocol.ExecRequest, cwd string, env []string, interactive bool) ([]string, error) {
	if req.DockerImage == "" {
		return nil, fmt.Errorf("docker_image is required for the docker engine")
	}

	dockerPath, err := exec.LookPath("docker")
	if err != nil {
		return nil, fmt.Errorf("docker executable not found in PATH")
	}

	args := []string{"run", "--rm"}
	if interactive {
		// Keep stdin open so prompts can be answered; closed below once the
		// container exits.
		args = append(args, "-i")
	}

	// Set working directory inside container
	args = append(args, "-w", cwd)

	// Mount the current host working directory into the container at the same path
	if absCwd, aerr := filepath.Abs(cwd); aerr == nil {
		args = append(args, "-v", fmt.Sprintf("%s:%s", absCwd, cwd))
	}

	// Add environment variables
	for _, e := range env {
		args = append(args, "-e", e)
	}

	args = append(args, req.DockerImage)

	// Containers are assumed to run Linux (the Docker Desktop default); the
	// in-container shell is therefore sh regardless of the host platform.
	args = append(args, "sh", "-c", req.Command)

	return append([]string{dockerPath}, args...), nil
}

// runInteractive executes the container with a stdin pipe so prompts detected
// in the output stream can be answered in real time.
func (d *DockerEngine) runInteractive(ctx context.Context, req protocol.ExecRequest, cwd string, env []string, answers []string, live AnswerProvider, policy prompts.AnswerFunc, sink OutputSink) (string, string, int, int, error) {
	interactive := len(answers) > 0 || live != nil || policy != nil
	argv, err := buildDockerArgs(req, cwd, env, interactive)
	if err != nil {
		return "", "", -1, 0, err
	}

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)

	// Attach a stdin pipe only when answering prompts, so the container sees
	// real input instead of EOF.
	var stdin io.WriteCloser
	if interactive {
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
		_ = stdin.Close() // signal EOF once the container has exited
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

// runPTY executes the container under a pseudo-terminal (docker run -it) and
// feeds prompt answers to the PTY master in real time.
func (d *DockerEngine) runPTY(ctx context.Context, req protocol.ExecRequest, cwd string, env []string, answers []string, live AnswerProvider, policy prompts.AnswerFunc, sink OutputSink) (string, string, int, int, error) {
	if req.DockerImage == "" {
		return "", "", -1, 0, fmt.Errorf("docker_image is required for the docker engine")
	}

	dockerPath, err := exec.LookPath("docker")
	if err != nil {
		return "", "", -1, 0, fmt.Errorf("docker executable not found in PATH")
	}

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

	args := []string{"run", "--rm", "-it", "-w", cwd}
	if absCwd, aerr := filepath.Abs(cwd); aerr == nil {
		args = append(args, "-v", fmt.Sprintf("%s:%s", absCwd, cwd))
	}
	for _, e := range env {
		args = append(args, "-e", e)
	}
	args = append(args, req.DockerImage)
	args = append(args, "sh", "-c", req.Command)

	ptyCmd := ptmx.CommandContext(ctx, dockerPath, args...)
	if err := ptyCmd.Start(); err != nil {
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