// Package protocol defines the structured request and response contracts
// for the msh execution protocol. These types form the machine-readable
// interface between AI agents and the msh runtime.
//
// Every command execution in msh is represented as an ExecRequest/ExecResponse
// pair, replacing unstructured terminal text with deterministic JSON payloads.
package protocol

import (
	"encoding/json"
	"time"
)

// Status represents the outcome of a command execution.
type Status string

const (
	// StatusSuccess indicates the command completed with exit code 0.
	StatusSuccess Status = "success"

	// StatusError indicates the command completed with a non-zero exit code.
	StatusError Status = "error"

	// StatusTimeout indicates the command exceeded its execution time limit.
	StatusTimeout Status = "timeout"

	// StatusBlocked indicates the command is waiting for interactive input
	// (e.g., a [y/N] prompt) and cannot proceed without user intervention.
	StatusBlocked Status = "blocked"
)

// ExecRequest defines the input contract for executing a command through msh.
// AI agents construct this payload to request command execution.
type ExecRequest struct {
	// SessionID allows the client to persist state across multiple executions.
	// If empty, the execution runs in a new, ephemeral session.
	SessionID string `json:"session_id,omitempty"`

	// Command is the shell command string to execute.
	// Example: "npm run build", "ls -la", "go test ./..."
	Command string `json:"command"`

	// Cwd optionally overrides the working directory for this command.
	// If empty, the session's current working directory is used.
	Cwd string `json:"cwd,omitempty"`

	// Env provides additional environment variables for this command.
	// These are merged with (and override) the session's environment.
	Env map[string]string `json:"env,omitempty"`

	// EnvFile specifies a path to a .env file to load before execution.
	// Variables from this file are merged into Env.
	EnvFile string `json:"env_file,omitempty"`

	// Timeout sets the maximum execution duration for this command.
	// If the command exceeds this duration, it is killed and the response
	// status is set to "timeout". Zero means use the default (30s).
	Timeout time.Duration `json:"timeout,omitempty"`

	// MaxOutputLines limits the number of output lines returned in stdout/stderr.
	// If output exceeds this limit, it is truncated and the Truncated field
	// in ExecResponse is set to true. Zero means use the default (200).
	MaxOutputLines int `json:"max_output_lines,omitempty"`

	// DetectFiles enables filesystem change detection.
	// When true, msh takes pre and post execution snapshots to return FilesChanged.
	DetectFiles bool `json:"detect_files,omitempty"`

	// UsePty runs the command inside a pseudo-terminal (PTY) instead of standard pipes.
	// This is required for commands that demand a TTY (like interactive tools or SSH).
	// NOTE: When true, Stderr will be merged into Stdout.
	UsePty bool `json:"use_pty,omitempty"`

	// RedactSecrets enables secret redaction on command output. When true,
	// msh masks the exact values of known environment secrets and well-known
	// token formats (AWS keys, GitHub tokens, JWTs, private key blocks, etc.)
	// so they never leak into the agent's context window.
	// Defaults to true when nil.
	RedactSecrets *bool `json:"redact_secrets,omitempty"`

	// PromptAnswers provides answers to interactive prompts encountered
	// during execution (e.g., "[y/N]", "password:"). When a prompt is
	// detected, msh feeds the next answer to the process stdin instead of
	// blocking. Execution is only marked "blocked" if prompts remain
	// unanswered after all answers are consumed.
	PromptAnswers []string `json:"prompt_answers,omitempty"`

	// Engine specifies the execution engine to use (e.g., "subprocess", "docker", "kubernetes").
	// Defaults to "subprocess" if empty.
	Engine string `json:"engine,omitempty"`

	// DockerImage specifies the image to use if Engine is "docker".
	DockerImage string `json:"docker_image,omitempty"`

	// KubernetesNamespace specifies the namespace to use if Engine is "kubernetes".
	KubernetesNamespace string `json:"kubernetes_namespace,omitempty"`
}

// ExecResponse defines the structured output contract returned by msh
// after executing a command. This replaces raw, ANSI-polluted terminal
// streams with clean, machine-readable data.
type ExecResponse struct {
	// SessionID is the identifier of the session used for this execution.
	// Clients should pass this ID in subsequent requests to maintain state.
	SessionID string `json:"session_id"`

	// Status indicates the execution outcome.
	// One of: "success", "error", "timeout", "blocked".
	Status Status `json:"status"`

	// ExitCode is the process exit code. 0 typically indicates success.
	ExitCode int `json:"exit_code"`

	// Cwd is the working directory after command execution.
	// This captures any directory changes made by the command (e.g., cd).
	Cwd string `json:"cwd"`

	// Stdout contains the sanitized standard output of the command.
	// All ANSI escape codes are stripped. Output may be truncated.
	Stdout string `json:"stdout"`

	// Stderr contains the sanitized standard error of the command.
	// All ANSI escape codes are stripped. Output may be truncated.
	Stderr string `json:"stderr"`

	// Truncated indicates whether stdout or stderr was truncated
	// to stay within the MaxOutputLines limit.
	Truncated bool `json:"truncated"`

	// Redacted lists the secret names and token formats that were masked
	// out of the output by secret redaction (e.g., "API_KEY", "aws", "jwt").
	// Empty when redaction is disabled or nothing was masked.
	Redacted []string `json:"redacted,omitempty"`

	// FilesChanged lists relative paths of files that were added,
	// modified, or deleted during command execution.
	// Only populated when DetectFiles is true in the request.
	FilesChanged []string `json:"files_changed,omitempty"`

	// Hooks contains the execution results of any pre/post execution plugins.
	Hooks []HookResult `json:"hooks,omitempty"`

	// DurationMs is the wall-clock execution time in milliseconds.
	DurationMs int64 `json:"duration_ms"`

	// PromptDetected contains the text of any interactive prompt
	// detected during execution (e.g., "[y/N]", "password:").
	// Only set when Status is "blocked".
	PromptDetected string `json:"prompt_detected,omitempty"`

	// AnswersUsed is the number of PromptAnswers fed to the process
	// before execution completed. Zero when no answers were provided.
	AnswersUsed int `json:"answers_used,omitempty"`

	// Error contains a human-readable error description when the
	// command fails to execute (e.g., binary not found, permission denied).
	// This is distinct from stderr, which captures process output.
	Error string `json:"error,omitempty"`
}

// ToJSON serializes the ExecResponse to a compact JSON byte slice.
func (r *ExecResponse) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}

// ToPrettyJSON serializes the ExecResponse to an indented JSON byte slice.
func (r *ExecResponse) ToPrettyJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// ParseExecRequest deserializes an ExecRequest from a JSON byte slice.
func ParseExecRequest(data []byte) (*ExecRequest, error) {
	var req ExecRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, err
	}
	return &req, nil
}

// DefaultMaxOutputLines is the default number of output lines returned
// when MaxOutputLines is not specified in the request.
const DefaultMaxOutputLines = 200

// DefaultTimeout is the default execution timeout when Timeout is not
// specified in the request.
const DefaultTimeout = 30 * time.Second

// Version is the current version of the msh protocol.
const Version = "1.3.0"

// HookResult records the outcome of a pre/post execution hook.
type HookResult struct {
	Name     string `json:"name"`
	Type     string `json:"type"` // pre_exec, post_exec
	Command  string `json:"command"`
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
	ExitCode int    `json:"exit_code"`
}
