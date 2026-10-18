package evals

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/msh-protocol/msh/pkg/execution"
	"github.com/msh-protocol/msh/pkg/protocol"
	"github.com/msh-protocol/msh/pkg/sanitize"
)

// Benchmark & Evaluation Suite: Real-World Problematic CLI Scenarios for AI Coding Agents.
// Demonstrates that msh safely normalizes outputs where standard raw terminals fail.

func TestEval_ANSIColorStripping(t *testing.T) {
	evalCases := []struct {
		name     string
		rawInput string
		expected string
	}{
		{
			name:     "Jest / Pytest red failure header",
			rawInput: "\x1b[31mFAIL\x1b[39m \x1b[2mtests/\x1b[22m\x1b[1mindex.test.ts\x1b[22m",
			expected: "FAIL tests/index.test.ts",
		},
		{
			name:     "Webpack / Vite 256-color gradient and cursor movement",
			rawInput: "\x1b[38;5;208mvite v5.4.2\x1b[0m \x1b[32mready in 184ms\x1b[0m",
			expected: "vite v5.4.2 ready in 184ms",
		},
		{
			name:     "Git colored diff patch",
			rawInput: "\x1b[32m+const token = generateToken();\x1b[m\n\x1b[31m-const token = null;\x1b[m",
			expected: "+const token = generateToken();\n-const token = null;",
		},
	}

	for _, tc := range evalCases {
		t.Run(tc.name, func(t *testing.T) {
			cleaned := sanitize.StripANSIString(tc.rawInput)
			if strings.TrimSpace(cleaned) != strings.TrimSpace(tc.expected) {
				t.Errorf("expected clean output %q, got %q", tc.expected, cleaned)
			}
		})
	}
}

func TestEval_CompilerDumpContextBlowoutTruncation(t *testing.T) {
	// Simulate a 10,000-line pytest failure or C++ template compilation dump
	var bigDump strings.Builder
	for i := 1; i <= 10000; i++ {
		bigDump.WriteString(fmt.Sprintf("error: template instantiation failed at line %d: type mismatch in tensor_ops.hpp\n", i))
	}

	maxLines := 100
	truncated, wasTruncated := sanitize.TruncateOutput(bigDump.String(), maxLines)

	if !wasTruncated {
		t.Fatal("expected 10,000-line dump to be marked as truncated")
	}

	lines := strings.Split(truncated, "\n")
	if len(lines) > maxLines+5 {
		t.Errorf("output line count %d exceeded allowed limit %d", len(lines), maxLines+5)
	}

	if !strings.Contains(truncated, "[msh: truncated") {
		t.Error("expected truncation marker in output")
	}

	// First error should be visible
	if !strings.Contains(truncated, "line 1:") {
		t.Error("expected head of error log to be preserved")
	}
	// Last error should be visible
	if !strings.Contains(truncated, "line 10000:") {
		t.Error("expected tail of error log to be preserved")
	}
}

func TestEval_InteractivePromptInterception(t *testing.T) {
	prompts := []struct {
		name   string
		sample string
	}{
		{"npm init confirmation", "Is this OK? (yes) "},
		{"apt-get install prompt", "Do you want to continue? [Y/n] "},
		{"git reset confirmation", "Unstaged changes after reset? [y/N] "},
		{"terraform destroy confirmation", "Do you really want to destroy all resources? [yes/no]: "},
		{"npm package install prompt", "Need to install the following packages: create-vite? (y) "},
		{"generic sudo prompt", "[sudo] password for operator: "},
		{"ssh key passphrase", "Enter passphrase for key '/root/.ssh/id_ed25519': "},
	}

	for _, p := range prompts {
		t.Run(p.name, func(t *testing.T) {
			_, matched := sanitize.DetectPrompt(p.sample)
			if !matched {
				t.Errorf("failed to detect interactive prompt in %q", p.sample)
			}
		})
	}
}

func TestEval_SecretRedactionLeaks(t *testing.T) {
	leakCases := []struct {
		name       string
		leakOutput string
		shouldMask string
	}{
		{
			name:       "AWS Access Key ID in command logs",
			leakOutput: "Connected to S3 using AKIAIOSFODNN7EXAMPLE successfully.",
			shouldMask: "AKIAIOSFODNN7EXAMPLE",
		},
		{
			name:       "GitHub Personal Access Token in git clone output",
			leakOutput: "fatal: authentication failed for https://ghp_1234567890abcdefghijklmnopqrstuvwxyzAB@github.com",
			shouldMask: "ghp_1234567890abcdefghijklmnopqrstuvwxyzAB",
		},
		{
			name:       "OpenAI API Key printed by debug script",
			leakOutput: "OPENAI_API_KEY=sk-proj-abc1234567890defghijklmnopqrstuvwxyz1234567890",
			shouldMask: "sk-proj-abc1234567890defghijklmnopqrstuvwxyz1234567890",
		},
		{
			name:       "Private RSA Key block dump",
			leakOutput: "-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA0Y3...\n-----END RSA PRIVATE KEY-----",
			shouldMask: "-----BEGIN RSA PRIVATE KEY-----",
		},
	}

	for _, lc := range leakCases {
		t.Run(lc.name, func(t *testing.T) {
			redacted, list := sanitize.RedactSecrets(lc.leakOutput, nil)
			if strings.Contains(redacted, lc.shouldMask) {
				t.Errorf("secret %q was NOT masked: %s", lc.shouldMask, redacted)
			}
			if len(list) == 0 {
				t.Error("expected redacted secret list to record findings")
			}
		})
	}
}

func TestEval_EndToEndExecutionFidelity(t *testing.T) {
	session, err := execution.NewSession("")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	executor := execution.NewExecutor(session)

	// 1. Exit code fidelity
	resp := executor.Execute(protocol.ExecRequest{
		Command: "exit 42",
		Timeout: 5 * time.Second,
	})
	if resp.ExitCode != 42 {
		t.Errorf("expected exit code 42, got %d", resp.ExitCode)
	}

	// 2. Secret redaction during execution
	secretKey := "AKIAIOSFODNN7EXAMPLE"
	resp = executor.Execute(protocol.ExecRequest{
		Command: "echo " + secretKey,
		Timeout: 5 * time.Second,
	})
	if strings.Contains(resp.Stdout, secretKey) {
		t.Errorf("secret leaked into execution stdout: %s", resp.Stdout)
	}
	if !strings.Contains(resp.Stdout, "[REDACTED:") {
		t.Errorf("expected [REDACTED:...] in stdout, got: %s", resp.Stdout)
	}
}
