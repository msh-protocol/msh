package execution

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/aymanbagabas/go-pty"
	"github.com/msh-protocol/msh/pkg/protocol"
)

// DockerEngine implements the Engine interface using the local Docker CLI.
type DockerEngine struct{}

func NewDockerEngine() *DockerEngine {
	return &DockerEngine{}
}

func (d *DockerEngine) Run(ctx context.Context, req protocol.ExecRequest, cwd string, env []string) (string, string, int, int, error) {
	if req.DockerImage == "" {
		return "", "", -1, 0, fmt.Errorf("docker_image is required for the docker engine")
	}

	// Verify docker is installed
	dockerPath, err := exec.LookPath("docker")
	if err != nil {
		return "", "", -1, 0, fmt.Errorf("docker executable not found in PATH")
	}

	// Build the docker command arguments
	// e.g., docker run --rm -w /app -v /host/path:/app ubuntu:latest sh -c "command"
	args := []string{"run", "--rm"}
	
	// Set working directory inside container
	args = append(args, "-w", cwd)

	// Mount the current host working directory into the container at the same path
	absCwd, err := filepath.Abs(cwd)
	if err == nil {
		args = append(args, "-v", fmt.Sprintf("%s:%s", absCwd, cwd))
	}

	// Add environment variables
	for _, e := range env {
		args = append(args, "-e", e)
	}

	if req.UsePty {
		args = append(args, "-it")
	}

	args = append(args, req.DockerImage)

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
		
		ptyCmd := ptmx.CommandContext(ctx, dockerPath, args...)
		
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

		return stdoutBuf.String(), "", exitCode, 0, err
	}

	cmd := exec.CommandContext(ctx, dockerPath, args...)
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

	return stdoutBuf.String(), stderrBuf.String(), exitCode, 0, err
}
