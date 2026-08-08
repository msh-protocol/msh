package execution

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"runtime"
	"time"

	"github.com/aymanbagabas/go-pty"
	"github.com/msh-protocol/msh/pkg/protocol"
)

// SubprocessEngine implements the Engine interface for local subprocess execution.
type SubprocessEngine struct{}

func NewSubprocessEngine() *SubprocessEngine {
	return &SubprocessEngine{}
}

func (s *SubprocessEngine) Run(ctx context.Context, req protocol.ExecRequest, cwd string, env []string) (string, string, int, error) {
	var stdoutBuf, stderrBuf bytes.Buffer

	if req.UsePty {
		ptmx, err := pty.New()
		if err != nil {
			return "", "", -1, err
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
			return "", "", -1, err
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

		return stdoutBuf.String(), "", exitCode, err
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd.exe", "/C", req.Command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", req.Command)
	}
	cmd.Dir = cwd
	cmd.Env = env
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	
	err := cmd.Run()
	
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	return stdoutBuf.String(), stderrBuf.String(), exitCode, err
}
