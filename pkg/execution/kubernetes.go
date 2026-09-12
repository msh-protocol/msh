package execution

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/aymanbagabas/go-pty"
	"github.com/msh-protocol/msh/pkg/protocol"
)

// KubernetesEngine implements the Engine interface using the local kubectl CLI.
type KubernetesEngine struct{}

func NewKubernetesEngine() *KubernetesEngine {
	return &KubernetesEngine{}
}

func (k *KubernetesEngine) Run(ctx context.Context, req protocol.ExecRequest, cwd string, env []string) (string, string, int, int, error) {
	if req.DockerImage == "" {
		return "", "", -1, 0, fmt.Errorf("docker_image is required for the kubernetes engine")
	}

	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return "", "", -1, 0, fmt.Errorf("kubectl executable not found in PATH")
	}

	// Generate a unique pod name based on the session ID or a timestamp
	podName := fmt.Sprintf("msh-exec-%d", time.Now().UnixNano())
	if req.SessionID != "" {
		podName = fmt.Sprintf("msh-exec-%s", req.SessionID)
	}

	// We use 'kubectl run' to spawn an ephemeral pod
	// e.g., kubectl run msh-exec-123 --image=ubuntu --restart=Never --rm -i --tty --command -- sh -c "command"
	args := []string{"run", podName, "--image=" + req.DockerImage, "--restart=Never", "--rm"}

	if req.KubernetesNamespace != "" {
		args = append(args, "-n", req.KubernetesNamespace)
	}

	// Add environment variables
	for _, e := range env {
		args = append(args, "--env="+e)
	}

	if req.UsePty {
		args = append(args, "-i", "--tty")
	} else {
		args = append(args, "-i") // still need stdin attached for execution
	}

	args = append(args, "--command", "--")

	if runtime.GOOS == "windows" {
		args = append(args, "cmd.exe", "/C", req.Command)
	} else {
		args = append(args, "sh", "-c", req.Command)
	}

	var stdoutBuf, stderrBuf bytes.Buffer

	if req.UsePty {
		ptmx, err := pty.New()
		if err != nil {
			return "", "", -1, 0, err
		}
		defer ptmx.Close()
		
		ptyCmd := ptmx.CommandContext(ctx, kubectlPath, args...)
		
		err = ptyCmd.Start()
		if err != nil {
			return "", "", -1, 0, err
		}
		
		done := make(chan struct{})
		go func() {
			_, _ = io.Copy(&stdoutBuf, ptmx)
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

		// Clean up the pod name from output if it says "pod XYZ deleted"
		out := stdoutBuf.String()
		if strings.Contains(out, "pod \""+podName+"\" deleted") {
			out = strings.ReplaceAll(out, "pod \""+podName+"\" deleted", "")
		}

		return out, "", exitCode, 0, err
	}

	cmd := exec.CommandContext(ctx, kubectlPath, args...)
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	
	err = cmd.Run()
	
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	out := stdoutBuf.String()
	if strings.Contains(out, "pod \""+podName+"\" deleted") {
		out = strings.ReplaceAll(out, "pod \""+podName+"\" deleted\n", "")
		out = strings.ReplaceAll(out, "pod \""+podName+"\" deleted", "")
	}

	return out, stderrBuf.String(), exitCode, 0, err
}
