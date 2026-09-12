package prompts

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadConfigMissingFile verifies a missing prompts file yields an empty
// no-op config rather than an error.
func TestLoadConfigMissingFile(t *testing.T) {
	cfg, err := LoadConfig(t.TempDir())
	if err != nil {
		t.Fatalf("missing file should not error, got %v", err)
	}
	if len(cfg.Prompts) != 0 {
		t.Fatalf("expected no rules, got %d", len(cfg.Prompts))
	}
	if resolver := cfg.Resolver(); resolver != nil {
		t.Fatalf("expected nil resolver for empty config")
	}
}

// TestLoadConfigParseAndMatch verifies the YAML is parsed and regex rules
// answer matching prompts in order.
func TestLoadConfigParseAndMatch(t *testing.T) {
	dir := t.TempDir()
	content := `prompts:
  - match: "(?i)(proceed|continue).*\\[y/N\\]"
    answer: "y"
  - match: "install"
    answer: "n"
  - match: ".*password.*"
    answer: "hunter2"
`
	path := filepath.Join(dir, ".msh", "prompts.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if len(cfg.Prompts) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(cfg.Prompts))
	}

	cases := []struct{ prompt, want string }{
		{"Do you want to continue? [y/N] (y/n) [n]:", "y"},
		{"Proceed? [y/N]: ", "y"},
		{"Is it ok to install x? Ok to proceed? (y/n)", "n"},
		{"Enter password: ", "hunter2"},
	}
	for _, c := range cases {
		got, ok := cfg.AnswerFor(c.prompt)
		if !ok {
			t.Fatalf("expected match for %q", c.prompt)
		}
		if got != c.want {
			t.Fatalf("AnswerFor(%q) = %q, want %q", c.prompt, got, c.want)
		}
	}

	if _, ok := cfg.AnswerFor("Unrelated line of text here"); ok {
		t.Fatal("expected no match for unrelated text")
	}
}

// TestLoadConfigInvalidRegex verifies a malformed regex fails loudly.
func TestLoadConfigInvalidRegex(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".msh", "prompts.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("prompts:\n  - match: \"[unclosed\"\n    answer: \"n\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadConfig(dir); err == nil {
		t.Fatal("expected error for invalid regex")
	}
}