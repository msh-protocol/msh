package execution

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/msh-protocol/msh/pkg/protocol"
)

// dockerAvailable reports whether a usable Docker daemon is reachable.
func dockerAvailable(t *testing.T) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "docker", "info").Run() == nil
}

// TestBuildDockerArgs verifies the docker command is assembled correctly for
// both interactive (prompt answering) and plain modes.
func TestBuildDockerArgs(t *testing.T) {
	if !dockerAvailable(t) {
		t.Skip("docker executable not available")
	}

	req := protocol.ExecRequest{
		Command:        "echo hi && read -p 'go? [y/N] ' x",
		DockerImage:    "alpine:latest",
		PromptAnswers:  []string{"n"},
		Cwd:            "/workspace",
		UsePty:         false,
	}

	argv, err := buildDockerArgs(req, "/workspace", []string{"FOO=bar"}, true)
	if err != nil {
		t.Fatalf("buildDockerArgs failed: %v", err)
	}
	if argv[0] == "docker" {
		t.Fatalf("expected full docker path as argv[0], got %q", argv[0])
	}
	if argv[1] != "run" {
		t.Fatalf("expected argv[1]=run, got %q", argv[1])
	}
	joined := strings.Join(argv, " ")
	if !strings.Contains(joined, " --rm ") || !strings.Contains(joined, " -i ") {
		t.Fatalf("expected --rm and -i in interactive mode, got %q", joined)
	}
	if !strings.Contains(joined, " -w /workspace ") {
		t.Fatalf("expected working dir mount, got %q", joined)
	}
	if !strings.Contains(joined, " -e FOO=bar ") {
		t.Fatalf("expected env passthrough, got %q", joined)
	}
	if !strings.Contains(joined, " alpine:latest sh -c ") {
		t.Fatalf("expected linux container shell invocation, got %q", joined)
	}
	if !strings.HasSuffix(joined, "-c "+req.Command) {
		t.Fatalf("expected command as final args, got %q", joined)
	}

	plain, err := buildDockerArgs(req, "/workspace", nil, false)
	if err != nil {
		t.Fatalf("buildDockerArgs(plain) failed: %v", err)
	}
	if strings.Contains(strings.Join(plain, " "), " -i ") {
		t.Fatalf("plain mode should not attach stdin, got %q", plain)
	}
}

// TestDockerAnswersPrompt runs a prompt-answering command inside a container
// and verifies the answer is fed back through docker run -i stdin. Skipped when
// no Docker daemon is reachable.
func TestDockerAnswersPrompt(t *testing.T) {
	if !dockerAvailable(t) {
		t.Skip("docker daemon not available")
	}

	session, err := NewSession("")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	req := protocol.ExecRequest{
		Engine:        "docker",
		Command:       "printf 'Proceed? [y/N] '; read a; printf 'Continue? [y/N] '; read b; echo first=$a second=$b",
		DockerImage:   "alpine:latest",
		PromptAnswers: []string{"yes", "no"},
		MaxOutputLines: 10,
		Cwd:           "/workspace",
	}

	executor := NewExecutor(session)
	resp := executor.Execute(req)

	if resp.Status == protocol.StatusBlocked {
		t.Fatalf("expected run to complete, got blocked on %q", resp.PromptDetected)
	}
	if resp.AnswersUsed != 2 {
		t.Fatalf("expected answers_used=2, got %d", resp.AnswersUsed)
	}
	if !strings.Contains(resp.Stdout, "first=yes") || !strings.Contains(resp.Stdout, "second=no") {
		t.Fatalf("expected both answers echoed back, got %q", resp.Stdout)
	}
}