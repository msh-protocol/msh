package guard

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// EvaluationResult contains the verdict of a pre-execution guard check.
type EvaluationResult struct {
	Allowed    bool        `json:"allowed"`
	Risk       RiskLevel   `json:"risk"`
	Violations []Violation `json:"violations,omitempty"`
	Reason     string      `json:"reason,omitempty"`
}

// Regex to detect path traversal or sensitive system root targeting in write/delete contexts
var (
	traversalEscapeRe = regexp.MustCompile(`(?i)(>\s*|>>\s*|\b(rm|rmdir|del|erase|move|mv|cp|copy|touch|chmod|chown)\s+.*)(\.\.[\\/]\.\.[\\/]|/etc/|/var/|/root/|[a-zA-Z]:[\\/]Windows[\\/]|[a-zA-Z]:[\\/]System32[\\/]|~[\\/]\.ssh)`)
)

// Evaluate scans a command line against the active guard config and constraints.
func Evaluate(command string, cwd string, cfg *Config, strictWorkspace bool, noGuard bool) *EvaluationResult {
	res := &EvaluationResult{
		Allowed: true,
		Risk:    RiskLow,
	}

	// 1. Explicit escape hatch
	if noGuard {
		return res
	}

	if cfg == nil {
		// Use default configuration if none provided
		cfg, _ = LoadConfig(cwd)
	}

	trimmedCmd := strings.TrimSpace(command)
	if (strings.HasPrefix(trimmedCmd, "\"") && strings.HasSuffix(trimmedCmd, "\"")) ||
		(strings.HasPrefix(trimmedCmd, "'") && strings.HasSuffix(trimmedCmd, "'")) {
		trimmedCmd = strings.TrimSpace(trimmedCmd[1 : len(trimmedCmd)-1])
	}
	if trimmedCmd == "" {
		return res
	}

	// 2. Evaluate pattern rules
	for _, rule := range cfg.Policies {
		if rule.re == nil {
			continue
		}

		if rule.re.MatchString(trimmedCmd) {
			violation := Violation{
				RuleID:  rule.ID,
				Risk:    rule.Risk,
				Action:  rule.Action,
				Message: rule.Message,
			}
			res.Violations = append(res.Violations, violation)

			// Escalate overall risk level
			if riskPrecedence(rule.Risk) > riskPrecedence(res.Risk) {
				res.Risk = rule.Risk
			}

			// Block if any triggering rule demands ActionBlock
			if rule.Action == ActionBlock {
				res.Allowed = false
				if res.Reason == "" {
					res.Reason = fmt.Sprintf("Blocked by guard rule [%s]: %s", rule.ID, rule.Message)
				}
			}
		}
	}

	// 3. Strict Workspace Boundary Check (Tier 3)
	if strictWorkspace || (cfg != nil && cfg.StrictWorkspace) {
		if traversalEscapeRe.MatchString(trimmedCmd) {
			violation := Violation{
				RuleID:  "strict-workspace-escape",
				Risk:    RiskHigh,
				Action:  ActionBlock,
				Message: "Command targets sensitive paths outside the workspace boundary.",
			}
			res.Violations = append(res.Violations, violation)
			res.Allowed = false
			if riskPrecedence(RiskHigh) > riskPrecedence(res.Risk) {
				res.Risk = RiskHigh
			}
			if res.Reason == "" {
				res.Reason = "Blocked by strict workspace boundary: destination escapes project root."
			}
		}

		// Also check explicit denied paths
		if cfg != nil && len(cfg.DeniedPaths) > 0 {
			normalizedCmd := strings.ReplaceAll(trimmedCmd, "\\", "/")
			for _, denied := range cfg.DeniedPaths {
				cleanDenied := strings.ReplaceAll(filepath.ToSlash(filepath.Clean(denied)), "\\", "/")
				if strings.Contains(normalizedCmd, cleanDenied) {
					res.Violations = append(res.Violations, Violation{
						RuleID:  "denied-path-access",
						Risk:    RiskHigh,
						Action:  ActionBlock,
						Message: fmt.Sprintf("Access to denied path %q is prohibited.", denied),
					})
					res.Allowed = false
					res.Risk = RiskHigh
					if res.Reason == "" {
						res.Reason = fmt.Sprintf("Access to denied path %q is prohibited.", denied)
					}
				}
			}
		}
	}

	return res
}

func riskPrecedence(r RiskLevel) int {
	switch r {
	case RiskCritical:
		return 4
	case RiskHigh:
		return 3
	case RiskModerate:
		return 2
	case RiskLow:
		return 1
	default:
		return 0
	}
}
