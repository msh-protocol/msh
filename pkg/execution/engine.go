package execution

import (
	"context"

	"github.com/msh-protocol/msh/pkg/protocol"
)

// Engine defines the interface for running commands in isolated environments
// (e.g., local subprocess, Docker container, Kubernetes pod).
type Engine interface {
	// Run executes the command specified in the request.
	// It returns the raw stdout and stderr output, the exit code (if applicable), and any error.
	// The executor handles timeouts, sanitization, and hooks.
	Run(ctx context.Context, req protocol.ExecRequest, cwd string, env []string) (stdout, stderr string, exitCode int, err error)
}
