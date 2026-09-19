package execution

import (
	"testing"

	"github.com/msh-protocol/msh/pkg/protocol"
)

func TestVerifyExecution_DeterministicEcho(t *testing.T) {
	req := protocol.ExecRequest{
		Command: "echo hello verification",
	}

	session, err := NewSession("")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	executor := NewExecutor(session)
	origResp := executor.Execute(req)

	result, err := VerifyExecution(req, origResp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsDeterministic {
		t.Errorf("expected deterministic result for echo command, got drift: %s", result.DriftSummary)
	}
	if !result.ExitCodeMatch {
		t.Errorf("expected exit codes to match")
	}
	if !result.OutputMatch {
		t.Errorf("expected outputs to match")
	}
	if result.SimilarityScore < 1.0 {
		t.Errorf("expected 1.0 similarity, got: %f", result.SimilarityScore)
	}
}
