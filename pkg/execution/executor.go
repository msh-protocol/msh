// Package execution implements the core command execution engine for msh.
//
// This package handles spawning subprocesses, capturing their output,
// enforcing timeouts, and packaging results into structured ExecResponse
// payloads. It works in conjunction with the sanitize package to strip
// ANSI codes and truncate output before returning to the agent.
package execution

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"github.com/msh-protocol/msh/pkg/fs"
	"github.com/msh-protocol/msh/pkg/hooks"
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
	return &Executor{
		session: session,
	}
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

	// Load .env file if specified
	if req.EnvFile != "" {
		envPath := req.EnvFile
		if !filepath.IsAbs(envPath) {
			envPath = filepath.Join(cwd, envPath)
		}
		
		if loadedEnv, err := godotenv.Read(envPath); err == nil {
			if req.Env == nil {
				req.Env = make(map[string]string)
			}
			// req.Env takes precedence over the .env file, so we only add keys that don't exist
			for k, v := range loadedEnv {
				if _, exists := req.Env[k]; !exists {
					req.Env[k] = v
				}
			}
		}
	}

	// Build environment
	env := e.session.BuildEnv(req.Env)

	// Build the initial response object
	resp := protocol.ExecResponse{
		SessionID: e.session.ID,
		Cwd:       cwd,
	}

	// 1. Load and Run Pre-Hooks
	cfg, err := hooks.LoadConfig(cwd)
	if err == nil && cfg != nil {
		preResults, preErr := hooks.RunPreHooks(context.Background(), cfg, req.Command, cwd)
		resp.Hooks = append(resp.Hooks, preResults...)
		if preErr != nil {
			resp.Status = protocol.StatusBlocked
			resp.ExitCode = -1
			resp.Error = preErr.Error()
			resp.DurationMs = time.Since(startTime).Milliseconds()
			return resp
		}
	}

	// 2. Start Hybrid Filesystem Watcher if file detection is enabled
	var watcher *fs.Watcher
	if req.DetectFiles {
		w, err := fs.NewWatcher(cwd, defaultIgnorePatterns())
		if err == nil {
			if startErr := w.Start(); startErr == nil {
				watcher = w
			}
		}
	}

	// Parse the command for cd detection (to update session state)
	cdTarget := parseCdCommand(req.Command)

	// Select the execution engine
	var engine Engine
	switch req.Engine {
	case "docker":
		engine = NewDockerEngine()
	case "kubernetes":
		engine = NewKubernetesEngine()
	default:
		engine = NewSubprocessEngine()
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Run command via engine
	stdoutStr, stderrStr, exitCode, answersUsed, err := engine.Run(ctx, req, cwd, env)
	
	duration := time.Since(startTime)

	// Populate the response (SessionID and Cwd were set earlier)
	resp.DurationMs = duration.Milliseconds()
	resp.AnswersUsed = answersUsed

	// Determine status and exit code
	if ctx.Err() == context.DeadlineExceeded {
		resp.Status = protocol.StatusTimeout
		resp.ExitCode = -1
		resp.Error = fmt.Sprintf("command timed out after %s", timeout)
	} else if err != nil {
		resp.Status = protocol.StatusError
		resp.ExitCode = exitCode
		resp.Error = err.Error()
	} else {
		resp.Status = protocol.StatusSuccess
		resp.ExitCode = exitCode
	}

	// Sanitize output — strip ANSI codes and truncate
	stdoutClean, stdoutTruncated := sanitize.CleanOutput(stdoutStr, maxLines)
	stderrClean, stderrTruncated := sanitize.CleanOutput(stderrStr, maxLines)

	resp.Stdout = strings.TrimRight(stdoutClean, "\n\r")
	resp.Stderr = strings.TrimRight(stderrClean, "\n\r")
	resp.Truncated = stdoutTruncated || stderrTruncated

	// Apply secret redaction before returning output to the agent.
	if req.RedactSecrets == nil || *req.RedactSecrets {
		secrets := secretEnvMap(env)
		var kinds []string
		resp.Stdout, kinds = sanitize.RedactSecrets(resp.Stdout, secrets)
		resp.Redacted = append(resp.Redacted, kinds...)
		resp.Stderr, kinds = sanitize.RedactSecrets(resp.Stderr, secrets)
		resp.Redacted = append(resp.Redacted, kinds...)
		resp.Redacted = uniqueStrings(resp.Redacted)
		resp.Hooks = redactHooks(resp.Hooks, secrets)
	}

	// Check for interactive prompts in the output. Skip the blocked upgrade
	// when answers were already fed to the process — the prompt was consumed,
	// so the run is a legitimate success/error, not a hang.
	if resp.AnswersUsed == 0 {
		for _, line := range strings.Split(resp.Stdout+"\n"+resp.Stderr, "\n") {
			if prompt, detected := sanitize.DetectPrompt(line); detected {
				resp.PromptDetected = prompt
				if resp.Status == protocol.StatusError || resp.Status == protocol.StatusTimeout {
					resp.Status = protocol.StatusBlocked
				}
				break
			}
		}
	}

	// Compute filesystem diff using the Watcher
	if watcher != nil {
		resp.FilesChanged = watcher.Stop()
	}

	// Update session state
	if cdTarget != "" && resp.Status == protocol.StatusSuccess {
		e.session.UpdateCwd(cdTarget, cwd)
	}
	resp.Cwd = e.session.Cwd

	// Run Post-Hooks
	if cfg != nil {
		postResults := hooks.RunPostHooks(context.Background(), cfg, req.Command, cwd)
		resp.Hooks = append(resp.Hooks, postResults...)
	}

	// Redact any secrets that leaked into post-hook output.
	if req.RedactSecrets == nil || *req.RedactSecrets {
		resp.Hooks = redactHooks(resp.Hooks, secretEnvMap(env))
	}

	return resp
}

// redactHooks masks secrets in pre/post hook output so hook results cannot
// leak API keys or tokens into the response payload.
func redactHooks(hooks []protocol.HookResult, secrets map[string]string) []protocol.HookResult {
	for i := range hooks {
		hooks[i].Stdout, _ = sanitize.RedactSecrets(hooks[i].Stdout, secrets)
		hooks[i].Stderr, _ = sanitize.RedactSecrets(hooks[i].Stderr, secrets)
	}
	return hooks
}

// secretEnvMap extracts the values of environment variables whose names
// look sensitive (contain TOKEN, KEY, SECRET, PASSWORD, CREDENTIAL, etc.)
// so their exact values can be masked out of the output.
func secretEnvMap(env []string) map[string]string {
	secrets := make(map[string]string)
	for _, entry := range env {
		parts := splitEnvVar(entry)
		if len(parts) == 2 && isSecretName(parts[0]) && parts[1] != "" {
			secrets[parts[0]] = parts[1]
		}
	}
	return secrets
}

// isSecretName reports whether an environment variable name looks sensitive.
func isSecretName(name string) bool {
	upper := strings.ToUpper(name)
	for _, pattern := range []string{"TOKEN", "KEY", "SECRET", "PASSWORD", "PASSWD", "PASS", "CREDENTIAL", "PRIVATE", "AUTH"} {
		if strings.Contains(upper, pattern) {
			return true
		}
	}
	return false
}

// uniqueStrings deduplicates a string slice while preserving order.
func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
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
