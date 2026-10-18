// Package guard provides pre-execution semantic guardrails and policy enforcement
// for autonomous AI agents executing shell commands.
//
// Based on recent research in agent security (arXiv:2607.22569, arXiv:2512.12806),
// this package implements a three-tier defense model:
//   - Tier 1: Catastrophic Floor (always active by default)
//   - Tier 2: Configurable Project Policies (.msh/policies.yaml)
//   - Tier 3: Strict Workspace Boundary Enforcement (--strict-workspace)
package guard

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"gopkg.in/yaml.v3"
)

// RiskLevel represents the severity of a command or policy match.
type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskModerate RiskLevel = "moderate"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

// Action defines the enforcement behavior when a rule triggers.
type Action string

const (
	ActionBlock Action = "block"
	ActionWarn  Action = "warn"
)

// Rule defines a single pattern-matching guardrail rule.
type Rule struct {
	ID          string    `yaml:"id" json:"id"`
	Match       string    `yaml:"match" json:"match"`
	Action      Action    `yaml:"action" json:"action"`
	Risk        RiskLevel `yaml:"risk" json:"risk"`
	Message     string    `yaml:"message" json:"message"`
	Description string    `yaml:"description,omitempty" json:"description,omitempty"`
	re          *regexp.Regexp
}

// Config represents the `.msh/policies.yaml` configuration.
type Config struct {
	Version         string   `yaml:"version,omitempty" json:"version,omitempty"`
	StrictWorkspace bool     `yaml:"strict_workspace,omitempty" json:"strict_workspace,omitempty"`
	AllowedPaths    []string `yaml:"allowed_paths,omitempty" json:"allowed_paths,omitempty"`
	DeniedPaths     []string `yaml:"denied_paths,omitempty" json:"denied_paths,omitempty"`
	Policies        []Rule   `yaml:"policies" json:"policies"`
}

// Violation captures the details of an evaluated rule match.
type Violation struct {
	RuleID  string    `json:"rule_id"`
	Risk    RiskLevel `json:"risk"`
	Action  Action    `json:"action"`
	Message string    `json:"message"`
}

// DefaultRules returns the Tier 1 Catastrophic Floor rules that protect the host
// system against catastrophic failure modes, prompt injection, and destructive wipes.
func DefaultRules() []Rule {
	return []Rule{
		{
			ID:      "catastrophic-root-deletion",
			Match:   `(?i)\brm\s+.*(-[a-zA-Z]*r[a-zA-Z]*f|-[a-zA-Z]*f[a-zA-Z]*r|-[a-zA-Z]*r[a-zA-Z]*\s+.*-[a-zA-Z]*f|-[a-zA-Z]*f[a-zA-Z]*\s+.*-[a-zA-Z]*r|--recursive)\b.*\s+([~/]|\$HOME|\${HOME}|\/\*|\$ROOT)(/|\s|$)`,
			Action:  ActionBlock,
			Risk:    RiskCritical,
			Message: "Blocked destructive root or home directory deletion (rm -rf / or rm -rf ~).",
		},
		{
			ID:      "windows-root-wipe",
			Match:   `(?i)\brmdir\s+.*(/[sS]\s+/[qQ]|/[qQ]\s+/[sS]).*\s+[a-zA-Z]:(\\?|\s*$)`,
			Action:  ActionBlock,
			Risk:    RiskCritical,
			Message: "Blocked destructive drive root deletion (rmdir /s /q C:\\).",
		},
		{
			ID:      "shell-fork-bomb",
			Match:   `:\(\)\s*\{\s*:\|:&\s*\};:`,
			Action:  ActionBlock,
			Risk:    RiskCritical,
			Message: "Blocked classic shell fork bomb denial-of-service pattern.",
		},
		{
			ID:      "raw-disk-overwrite",
			Match:   `(?i)\bdd\s+.*of=/dev/(sd[a-z]|hd[a-z]|nvme[0-9]n[0-9]|disk[0-9])(\s|$)`,
			Action:  ActionBlock,
			Risk:    RiskCritical,
			Message: "Blocked raw block device overwrite via dd.",
		},
		{
			ID:      "raw-filesystem-format",
			Match:   `(?i)\b(mkfs(\.[a-z0-9]+)?|fdisk|parted)\s+/dev/`,
			Action:  ActionBlock,
			Risk:    RiskCritical,
			Message: "Blocked raw disk formatting or partition modification command.",
		},
		{
			ID:      "recursive-root-chmod",
			Match:   `(?i)\bchmod\s+.*-[a-zA-Z]*R[a-zA-Z]*.*\s+(777|000)\s+/(/|\s|$)`,
			Action:  ActionBlock,
			Risk:    RiskCritical,
			Message: "Blocked recursive root permission wipe (chmod -R 777 /).",
		},
	}
}

// compileRule compiles the regex of a rule and attaches it to the struct.
func compileRule(r *Rule) error {
	re, err := regexp.Compile(r.Match)
	if err != nil {
		return fmt.Errorf("invalid guard rule match %q (id: %s): %w", r.Match, r.ID, err)
	}
	r.re = re
	if r.Risk == "" {
		r.Risk = RiskHigh
	}
	if r.Action == "" {
		r.Action = ActionBlock
	}
	return nil
}

// LoadConfig loads `.msh/policies.yaml` from workspaceRoot. If missing, it returns
// a config initialized with default catastrophic protection rules.
func LoadConfig(workspaceRoot string) (*Config, error) {
	cfg := &Config{
		Version:  "1.0",
		Policies: []Rule{},
	}

	configPath := filepath.Join(workspaceRoot, ".msh", "policies.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Initialize with default Tier 1 rules
			for _, r := range DefaultRules() {
				_ = compileRule(&r)
				cfg.Policies = append(cfg.Policies, r)
			}
			return cfg, nil
		}
		return nil, fmt.Errorf("failed to read %s: %w", configPath, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", configPath, err)
	}

	// Compile user-configured rules
	for i := range cfg.Policies {
		if err := compileRule(&cfg.Policies[i]); err != nil {
			return nil, err
		}
	}

	// Ensure default Tier 1 rules are always present unless explicitly overridden by ID
	userRuleIDs := make(map[string]bool)
	for _, r := range cfg.Policies {
		if r.ID != "" {
			userRuleIDs[r.ID] = true
		}
	}

	for _, def := range DefaultRules() {
		if !userRuleIDs[def.ID] {
			_ = compileRule(&def)
			cfg.Policies = append(cfg.Policies, def)
		}
	}

	return cfg, nil
}

// TemplateYAML returns a well-commented sample .msh/policies.yaml configuration.
func TemplateYAML() string {
	return `# msh Agent Execution Policy Configuration
# Schema Version: 1.0 (msh v1.5.0+)
#
# Three-tier enforcement model:
#   Tier 1: Built-in catastrophic floor (always active: rm -rf /, fork bombs, disk wipes)
#   Tier 2: Custom project rules (defined below)
#   Tier 3: Strict workspace containment (strict_workspace: true or --strict-workspace)

version: "1.0"

# Enforce that all write operations, redirects, and file modifications remain
# strictly confined within the project workspace.
strict_workspace: false

# Custom allowed or denied directory paths for agent file modifications
# allowed_paths:
#   - "./"
#   - "./tmp"
# denied_paths:
#   - "../"
#   - "~/.ssh"
#   - "~/.aws"

policies:
  # Example 1: Prevent agent from running non-reproducible global package installations
  - id: "no-global-npm"
    match: 'npm\s+(i|install)\s+(-g|--global)'
    action: "block"
    risk: "high"
    message: "Global npm installations are prohibited; use local project dependencies or npx."
    description: "Enforces project isolation by preventing global pollution of host environment."

  # Example 2: Warn when agents run git force-pushes
  - id: "warn-git-force-push"
    match: 'git\s+push\s+.*(-f|--force)'
    action: "warn"
    risk: "moderate"
    message: "Force pushing may overwrite remote commits on shared branches."
    description: "Flags destructive git history rewrites."

  # Example 3: Block direct execution of arbitrary remote scripts via curl pipe to shell
  - id: "block-curl-pipe-sh"
    match: '(curl|wget)\s+.*\|\s*(bash|sh|zsh|powershell|cmd)'
    action: "block"
    risk: "critical"
    message: "Piping remote URLs directly to a shell interpreter is prohibited."
    description: "Prevents supply-chain script execution without local inspection."
`
}
