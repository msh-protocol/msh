package execution

import (
	"strings"
	"testing"

	"github.com/msh-protocol/msh/pkg/protocol"
)

// TestExecuteRedactsEnvSecret verifies that a secret injected via Env whose
// value appears in command output is masked in the response and reported in
// the Redacted field. This runs through the real subprocess engine.
func TestExecuteRedactsEnvSecret(t *testing.T) {
	session, err := NewSession("")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	secretValue := "gkx_secretvalue123"
	req := protocol.ExecRequest{
		Command:        "echo " + secretValue,
		Env:            map[string]string{"MY_API_KEY": secretValue},
		MaxOutputLines: 10,
	}

	executor := NewExecutor(session)
	resp := executor.Execute(req)

	if strings.Contains(resp.Stdout, secretValue) {
		t.Fatalf("secret leaked into stdout: %q", resp.Stdout)
	}
	if !containsStr(resp.Redacted, "MY_API_KEY") {
		t.Fatalf("expected MY_API_KEY in Redacted, got: %v", resp.Redacted)
	}
}

// TestExecuteRedactsKnownPattern verifies that known token formats are masked
// even without any environment secret.
func TestExecuteRedactsKnownPattern(t *testing.T) {
	session, err := NewSession("")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	const awsKey = "AKIAIOSFODNN7EXAMPLE"
	req := protocol.ExecRequest{
		Command:        "echo " + awsKey,
		MaxOutputLines: 10,
	}

	executor := NewExecutor(session)
	resp := executor.Execute(req)

	if strings.Contains(resp.Stdout, awsKey) {
		t.Fatalf("AWS key leaked into stdout: %q", resp.Stdout)
	}
	if !containsStr(resp.Redacted, "aws") {
		t.Fatalf("expected 'aws' in Redacted, got: %v", resp.Redacted)
	}
}

// TestExecuteRedactionDisabled verifies that explicitly passing
// redact_secrets=false disables masking.
func TestExecuteRedactionDisabled(t *testing.T) {
	session, err := NewSession("")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	secretValue := "gkx_secretvalue123"
	disabled := false
	req := protocol.ExecRequest{
		Command:        "echo " + secretValue,
		Env:            map[string]string{"MY_API_KEY": secretValue},
		MaxOutputLines: 10,
		RedactSecrets:  &disabled,
	}

	executor := NewExecutor(session)
	resp := executor.Execute(req)

	if !strings.Contains(resp.Stdout, secretValue) {
		t.Fatalf("expected secret to remain when redaction disabled: %q", resp.Stdout)
	}
	if len(resp.Redacted) != 0 {
		t.Fatalf("expected no redactions, got: %v", resp.Redacted)
	}
}

func containsStr(list []string, item string) bool {
	for _, e := range list {
		if e == item {
			return true
		}
	}
	return false
}