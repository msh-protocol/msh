package main

import (
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestIsInternalSubcommand(t *testing.T) {
	subcommands := []string{
		"exec", "wrap", "fleet", "daemon", "mcp", "serve", "plugin", "version", "help", "completion",
		"EXEC", "Wrap", "FLEET",
	}
	for _, cmd := range subcommands {
		if !IsInternalSubcommand(cmd) {
			t.Errorf("expected %q to be recognized as internal subcommand", cmd)
		}
	}

	passthroughs := []string{
		"git", "npm", "cargo", "go", "python", "node", "docker", "ls", "cat", "echo", "pytest",
	}
	for _, cmd := range passthroughs {
		if IsInternalSubcommand(cmd) {
			t.Errorf("expected %q to NOT be recognized as internal subcommand", cmd)
		}
	}
}

func TestReconstructCommand(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		contains string
	}{
		{
			name:     "simple args",
			args:     []string{"git", "status"},
			contains: "git status",
		},
		{
			name:     "args with spaces",
			args:     []string{"git", "commit", "-m", "feat: initial commit"},
			contains: "feat: initial commit",
		},
		{
			name:     "args with special characters",
			args:     []string{"echo", "hello & goodbye"},
			contains: "hello & goodbye",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := ReconstructCommand(tt.args)
			if !strings.Contains(res, tt.contains) {
				t.Errorf("ReconstructCommand(%v) = %q, expected to contain %q", tt.args, res, tt.contains)
			}
		})
	}
}

func TestQuoteWindowsArg(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("skipping Windows-specific quoting test on non-windows")
	}

	// Empty string should be quoted
	if got := QuoteWindowsArg(""); got != `""` {
		t.Errorf("expected empty string to be quoted as \"\", got %q", got)
	}

	// Simple string should remain unquoted
	if got := QuoteWindowsArg("status"); got != "status" {
		t.Errorf("expected unquoted string, got %q", got)
	}

	// String with spaces should be quoted
	if got := QuoteWindowsArg("hello world"); got != `"hello world"` {
		t.Errorf("expected quoted string, got %q", got)
	}

	// String with quotes should escape them
	if got := QuoteWindowsArg(`say "hello"`); got != `"say \"hello\""` {
		t.Errorf("expected escaped quotes, got %q", got)
	}
}

func TestQuoteUnixArg(t *testing.T) {
	// Empty string
	if got := QuoteUnixArg(""); got != "''" {
		t.Errorf("expected empty string to be quoted as '', got %q", got)
	}

	// Simple string
	if got := QuoteUnixArg("status"); got != "status" {
		t.Errorf("expected unquoted string, got %q", got)
	}

	// String with spaces
	if got := QuoteUnixArg("hello world"); got != "'hello world'" {
		t.Errorf("expected single-quoted string, got %q", got)
	}

	// String with single quote
	if got := QuoteUnixArg("don't"); got != "'don'\\''t'" {
		t.Errorf("expected escaped single quote, got %q", got)
	}
}

func TestParsePassthroughArgs(t *testing.T) {
	// 1. Direct passthrough
	flags, cmd, isPass := ParsePassthroughArgs([]string{"git", "status"})
	if !isPass {
		t.Fatal("expected isPassthrough to be true")
	}
	if !strings.Contains(cmd, "git") || !strings.Contains(cmd, "status") {
		t.Errorf("unexpected cmd: %q", cmd)
	}
	if flags.MaxLines != 500 {
		t.Errorf("expected default max lines 500, got %d", flags.MaxLines)
	}

	// 2. Flags preceding passthrough command
	flags, cmd, isPass = ParsePassthroughArgs([]string{
		"--max-lines", "100",
		"--timeout", "1m",
		"--pty",
		"npm", "run", "build",
	})
	if !isPass {
		t.Fatal("expected isPassthrough to be true")
	}
	if flags.MaxLines != 100 {
		t.Errorf("expected max lines 100, got %d", flags.MaxLines)
	}
	if flags.Timeout != time.Minute {
		t.Errorf("expected timeout 1m, got %v", flags.Timeout)
	}
	if !flags.Pty {
		t.Error("expected Pty to be true")
	}
	if !strings.Contains(cmd, "npm") || !strings.Contains(cmd, "build") {
		t.Errorf("unexpected cmd: %q", cmd)
	}

	// 3. Flags with = syntax
	flags, cmd, isPass = ParsePassthroughArgs([]string{
		"--max-lines=250",
		"--timeout=30s",
		"go", "test", "./...",
	})
	if !isPass {
		t.Fatal("expected isPassthrough to be true")
	}
	if flags.MaxLines != 250 {
		t.Errorf("expected max lines 250, got %d", flags.MaxLines)
	}
	if flags.Timeout != 30*time.Second {
		t.Errorf("expected timeout 30s, got %v", flags.Timeout)
	}

	// 4. Internal subcommands should NOT be passthrough
	internalCases := [][]string{
		{"exec", "echo 1"},
		{"wrap", "echo 1"},
		{"fleet", "start"},
		{"daemon", "start"},
		{"mcp"},
		{"serve", "--port", "8080"},
		{"version"},
		{"--help"},
		{"-h"},
		{"--version"},
		{},
	}
	for _, args := range internalCases {
		_, _, isPass := ParsePassthroughArgs(args)
		if isPass {
			t.Errorf("expected args %v to NOT be passthrough", args)
		}
	}
}
