// Package execution implements the core command execution engine for msh.
//
// This package handles spawning subprocesses, capturing their output,
// enforcing timeouts, and packaging results into structured ExecResponse
// payloads. It works in conjunction with the sanitize package to strip
// ANSI codes and truncate output before returning to the agent.
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

	"github.com/msh-protocol/msh/pkg/fs"
	"github.com/msh-protocol/msh/pkg/protocol"
	"github.com/msh-protocol/msh/pkg/sanitize"
)

// Executor handles command execution within an msh session.
// It manages the subprocess lifecycle, output capture, and
// result packaging.
type Executor struct {
	session *Session
}

// NewExecutor creates a new Executor bound to the given session.
func NewExecutor(session *Session) *Executor {
	return &Executor{session: session}
}

// Execute runs a command and returns a structured ExecResponse.
// This is the core function of the msh runtime:
//
//  1. Resolve working directory and environment
//  2. Optionally snapshot filesystem state (for files_changed detection)
//  3. Spawn the subprocess with timeout enforcement
//  4. Capture and sanitize stdout/stderr
//  5. Detect interactive prompts
//  6. Compute filesystem diff
//  7. Update session state (CWD, env)
//  8. Return structured response
func (e *Executor) Execute(req protocol.ExecRequest) protocol.ExecResponse {
	startTime := time.Now()

	// Resolve timeout
	timeout := req.Timeout
	if timeout == 0 {
		timeout = protocol.DefaultTimeout
	}

	// Resolve max output lines
	maxLines := req.MaxOutputLines
	if maxLines == 0 {
		maxLines = protocol.DefaultMaxOutputLines
	}

	// Resolve working directory
	cwd := e.session.Cwd
	if req.Cwd != "" {
		cwd = req.Cwd
	}

	// Build environment
	env := e.session.BuildEnv(req.Env)

	// Take pre-execution filesystem snapshot if file detection is enabled
	var preSnapshot *fs.Snapshot
	if req.DetectFiles {
		snap, err := fs.TakeSnapshot(cwd, defaultIgnorePatterns())
		if err == nil {
			preSnapshot = snap
		}
	}

	// Parse the command for cd detection (to update session state)
	cdTarget := parseCdCommand(req.Command)

	// Build the OS command
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Capture stdout and stderr
	var stdoutBuf, stderrBuf bytes.Buffer

	var err error
	if req.UsePty {
		var ptmx pty.Pty
		ptmx, err = pty.New()
		if err == nil {
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
			if err == nil {
				// For PTY, stdout and stderr are merged into the PTY stream.
				// Read the output in the background.
				done := make(chan struct{})
				go func() {
					_, _ = io.Copy(&stdoutBuf, ptmx)
					close(done)
				}()
				
				// Wait for the command to finish
				err = ptyCmd.Wait()
				
				// Small delay to allow io.Copy to finish reading the remaining buffer
				select {
				case <-done:
				case <-time.After(100 * time.Millisecond):
				}
			}
		}
	} else {
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
		err = cmd.Run()
	}

	duration := time.Since(startTime)

	// Build the response
	resp := protocol.ExecResponse{
		DurationMs: duration.Milliseconds(),
		Cwd:        cwd,
	}

	// Determine status and exit code
	if ctx.Err() == context.DeadlineExceeded {
		resp.Status = protocol.StatusTimeout
		resp.ExitCode = -1
		resp.Error = fmt.Sprintf("command timed out after %s", timeout)
	} else if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			resp.Status = protocol.StatusError
			resp.ExitCode = exitErr.ExitCode()
		} else {
			resp.Status = protocol.StatusError
			resp.ExitCode = -1
			resp.Error = err.Error()
		}
	} else {
		resp.Status = protocol.StatusSuccess
		resp.ExitCode = 0
	}

	// Sanitize output — strip ANSI codes and truncate
	stdoutClean, stdoutTruncated := sanitize.CleanOutput(stdoutBuf.String(), maxLines)
	stderrClean, stderrTruncated := sanitize.CleanOutput(stderrBuf.String(), maxLines)

	resp.Stdout = strings.TrimRight(stdoutClean, "\n\r")
	resp.Stderr = strings.TrimRight(stderrClean, "\n\r")
	resp.Truncated = stdoutTruncated || stderrTruncated

	// Check for interactive prompts in the output
	for _, line := range strings.Split(resp.Stdout+"\n"+resp.Stderr, "\n") {
		if prompt, detected := sanitize.DetectPrompt(line); detected {
			resp.PromptDetected = prompt
			if resp.Status == protocol.StatusError || resp.Status == protocol.StatusTimeout {
				resp.Status = protocol.StatusBlocked
			}
			break
		}
	}

	// Compute filesystem diff
	if req.DetectFiles && preSnapshot != nil {
		postSnapshot, err := fs.TakeSnapshot(cwd, defaultIgnorePatterns())
		if err == nil {
			changes := fs.DiffSnapshots(preSnapshot, postSnapshot)
			resp.FilesChanged = make([]string, len(changes))
			for i, c := range changes {
				resp.FilesChanged[i] = c.Path
			}
		}
	}

	// Update session state
	if cdTarget != "" && resp.Status == protocol.StatusSuccess {
		e.session.UpdateCwd(cdTarget, cwd)
	}
	resp.Cwd = e.session.Cwd

	return resp
}

// parseCdCommand extracts the target directory from a cd command.
// Returns empty string if the command is not a cd command.
func parseCdCommand(command string) string {
	trimmed := strings.TrimSpace(command)

	// Handle standalone cd commands
	if trimmed == "cd" {
		return "~"
	}

	// Handle "cd <dir>" and "cd <dir> && ..." patterns
	if strings.HasPrefix(trimmed, "cd ") {
		parts := strings.Fields(trimmed)
		if len(parts) >= 2 {
			target := parts[1]
			// Stop at && or ; (compound commands)
			if target == "&&" || target == ";" || target == "||" {
				return ""
			}
			return target
		}
	}

	return ""
}

// defaultIgnorePatterns returns the default list of directory patterns
// to ignore during filesystem change detection.
func defaultIgnorePatterns() []string {
	return []string{
		"node_modules",
		".git",
		"dist",
		"build",
		"vendor",
		"__pycache__",
		".next",
		".cache",
		"target",
	}
}
