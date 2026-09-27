package guard

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultCatastrophicRules_Block(t *testing.T) {
	cfg, err := LoadConfig(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error loading default config: %v", err)
	}

	dangerousCommands := []struct {
		cmd        string
		expectedID string
	}{
		{"rm -rf /", "catastrophic-root-deletion"},
		{"rm -rf ~", "catastrophic-root-deletion"},
		{"rm -rf $HOME", "catastrophic-root-deletion"},
		{"rm -rf /*", "catastrophic-root-deletion"},
		{"rm -r -f /", "catastrophic-root-deletion"},
		{"rmdir /s /q C:\\", "windows-root-wipe"},
		{":(){ :|:& };:", "shell-fork-bomb"},
		{"dd if=/dev/zero of=/dev/sda bs=1M", "raw-disk-overwrite"},
		{"mkfs.ext4 /dev/sdb1", "raw-filesystem-format"},
		{"chmod -R 777 /", "recursive-root-chmod"},
	}

	for _, tc := range dangerousCommands {
		res := Evaluate(tc.cmd, "", cfg, false, false)
		if res.Allowed {
			t.Errorf("expected command %q to be blocked, but was allowed", tc.cmd)
		}
		if res.Risk != RiskCritical {
			t.Errorf("expected risk critical for %q, got %s", tc.cmd, res.Risk)
		}
		found := false
		for _, v := range res.Violations {
			if v.RuleID == tc.expectedID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected violation rule %q for %q, got violations: %+v", tc.expectedID, tc.cmd, res.Violations)
		}
	}
}

func TestBenignCommands_Allowed(t *testing.T) {
	cfg, err := LoadConfig(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error loading default config: %v", err)
	}

	benignCommands := []string{
		"git status",
		"git diff HEAD~1",
		"npm run build",
		"npm test -- --coverage",
		"pytest -v tests/",
		"cargo test --all",
		"go test -v ./...",
		"rm -rf node_modules",
		"rm -rf dist/",
		"rm ./tmp/cache.json",
		"ls -la",
		"echo 'hello world'",
	}

	for _, cmd := range benignCommands {
		res := Evaluate(cmd, "", cfg, false, false)
		if !res.Allowed {
			t.Errorf("expected benign command %q to be allowed, but was blocked: %s", cmd, res.Reason)
		}
		if len(res.Violations) > 0 {
			t.Errorf("expected 0 violations for %q, got %+v", cmd, res.Violations)
		}
	}
}

func TestCustomPolicyConfig(t *testing.T) {
	tmpDir := t.TempDir()
	mshDir := filepath.Join(tmpDir, ".msh")
	if err := os.MkdirAll(mshDir, 0755); err != nil {
		t.Fatal(err)
	}

	policyYAML := `version: "1.0"
strict_workspace: true
denied_paths:
  - "/sensitive/vault"
policies:
  - id: "block-git-force"
    match: "(?i)\\bgit\\s+push\\s+.*--force"
    action: "block"
    risk: "high"
    message: "Force pushing to git remote is prohibited."
  - id: "warn-npm-publish"
    match: "(?i)\\bnpm\\s+publish\\b"
    action: "warn"
    risk: "moderate"
    message: "Be cautious when publishing packages."
`
	if err := os.WriteFile(filepath.Join(mshDir, "policies.yaml"), []byte(policyYAML), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("failed to load custom config: %v", err)
	}

	// 1. Test block rule
	resForce := Evaluate("git push origin main --force", tmpDir, cfg, false, false)
	if resForce.Allowed {
		t.Error("expected git push --force to be blocked")
	}
	if resForce.Risk != RiskHigh {
		t.Errorf("expected risk high, got %s", resForce.Risk)
	}

	// 2. Test warn rule (should allow execution with warnings)
	resWarn := Evaluate("npm publish --access public", tmpDir, cfg, false, false)
	if !resWarn.Allowed {
		t.Error("expected npm publish with warn action to be allowed")
	}
	if len(resWarn.Violations) == 0 || resWarn.Violations[0].Action != ActionWarn {
		t.Errorf("expected warn violation, got %+v", resWarn.Violations)
	}

	// 3. Test denied path
	resDenied := Evaluate("cat /sensitive/vault/keys.txt", tmpDir, cfg, false, false)
	if resDenied.Allowed {
		t.Error("expected access to denied path to be blocked")
	}

	// 4. Test escape hatch (noGuard)
	resBypass := Evaluate("git push origin main --force", tmpDir, cfg, false, true)
	if !resBypass.Allowed {
		t.Error("expected noGuard=true to bypass policy checks")
	}
}

func TestStrictWorkspaceBoundary(t *testing.T) {
	cfg, _ := LoadConfig(t.TempDir())

	escapeCmds := []string{
		"echo 'evil' > ../../etc/passwd",
		"rm -rf ../../etc/shadow",
		"cp id_rsa ~/.ssh/authorized_keys",
		"touch C:\\Windows\\System32\\driver.sys",
	}

	for _, cmd := range escapeCmds {
		res := Evaluate(cmd, "", cfg, true, false)
		if res.Allowed {
			t.Errorf("expected strict workspace to block %q", cmd)
		}
	}
}
