package execution

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/aymanbagabas/go-pty"
	"github.com/msh-protocol/msh/pkg/prompts"
	"github.com/msh-protocol/msh/pkg/protocol"
)

// KubernetesEngine implements the Engine interface using the local kubectl CLI.
type KubernetesEngine struct{}

func NewKubernetesEngine() *KubernetesEngine {
	return &KubernetesEngine{}
}

func (k *KubernetesEngine) Run(ctx context.Context, req protocol.ExecRequest, cwd string, env []string) (string, string, int, int, error) {
	return k.RunStreaming(ctx, req, cwd, env, nil, nil, nil)
}

// RunStreaming implements StreamingEngine: output streams to the sink in real
// time and interactive prompts inside the pod are answered by feeding the next
// answer to the pod stdin via `kubectl run -i`.
func (k *KubernetesEngine) RunStreaming(ctx context.Context, req protocol.ExecRequest, cwd string, env []string, sink OutputSink, live AnswerProvider, policy prompts.AnswerFunc) (string, string, int, int, error) {
	answers := req.PromptAnswers

	if req.UsePty {
		return k.runPTY(ctx, req, cwd, env, answers, live, policy, sink)
	}
	return k.runInteractive(ctx, req, cwd, env, answers, live, policy, sink)
}

// podNameFor derives a stable pod name for a request so cleanup can reference
// the exact ephemeral pod.
func podNameFor(req protocol.ExecRequest) string {
	if req.SessionID != "" {
		return fmt.Sprintf("msh-exec-%s", req.SessionID)
	}
	return fmt.Sprintf("msh-exec-%d", time.Now().UnixNano())
}

// buildKubectlArgs assembles the shared `kubectl run` arguments used by both
// the streaming-pipe and PTY modes.
func buildKubectlArgs(req protocol.ExecRequest, env []string, tty bool, podName string) ([]string, error) {
	if req.DockerImage == "" {
		return nil, fmt.Errorf("docker_image is required for the kubernetes engine")
	}

	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return nil, fmt.Errorf("kubectl executable not found in PATH")
	}

	args := []string{"run", podName, "--image=" + req.DockerImage, "--restart=Never", "--rm", "-i"}
	if tty {
		args = append(args, "--tty")
	}

	if req.KubernetesNamespace != "" {
		args = append(args, "-n", req.KubernetesNamespace)
	}

	for _, e := range env {
		args = append(args, "--env="+e)
	}

	args = append(args, "--command", "--")

	// Pods run Linux images; the in-container shell is sh regardless of host.
	args = append(args, "sh", "-c", req.Command)

	return append([]string{kubectlPath}, args...), nil
}

// cleanPodOutput removes kubectl's "pod deleted" cleanup chatter so the agent
// only sees the command output.
func cleanPodOutput(out, podName string) string {
	return strings.ReplaceAll(out, "pod \""+podName+"\" deleted\n", "")
}

// runInteractive executes the ephemeral pod with stdin attached so prompts
// detected in the output stream can be answered in real time.
func (k *KubernetesEngine) runInteractive(ctx context.Context, req protocol.ExecRequest, _ string, env []string, answers []string, live AnswerProvider, policy prompts.AnswerFunc, sink OutputSink) (string, string, int, int, error) {
	podName := podNameFor(req)
	argv, err := buildKubectlArgs(req, env, false, podName)
	if err != nil {
		return "", "", -1, 0, err
	}

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)

	// Attach a stdin pipe when answering prompts (or always, since kubectl
	// already requires -i for execution).
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
		_ = stdin.Close()
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

	return cleanPodOutput(stdoutBuf.String(), podName), cleanPodOutput(stderrBuf.String(), podName), exitCode, answersUsed, err
}

// runPTY executes the pod under a pseudo-terminal and feeds prompt answers to
// the PTY master in real time.
func (k *KubernetesEngine) runPTY(ctx context.Context, req protocol.ExecRequest, _ string, env []string, answers []string, live AnswerProvider, policy prompts.AnswerFunc, sink OutputSink) (string, string, int, int, error) {
	podName := podNameFor(req)
	argv, err := buildKubectlArgs(req, env, true, podName)
	if err != nil {
		return "", "", -1, 0, err
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

	ptyCmd := ptmx.CommandContext(ctx, argv[0], argv[1:]...)
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

	return cleanPodOutput(stdoutBuf.String(), podName), "", exitCode, answerer.answeredCount(), err
}