package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/msh-protocol/msh/pkg/guard"
	"github.com/spf13/cobra"
)

var (
	flagGuardCwd             string
	flagGuardStrictWorkspace bool
	flagGuardJSON            bool
)

var guardCmd = &cobra.Command{
	Use:   "guard",
	Short: "Inspect, initialize, and test pre-execution safety guardrails",
	Long: `msh guard provides policy inspection, evaluation, and initialization
for the pre-execution security guardrail engine (msh v1.5.0+).

Commands:
  msh guard check "<cmd>"   Evaluate a command string against active policies
  msh guard list            List all active rules (Tier 1 built-in and Tier 2 project)
  msh guard init            Create a starter .msh/policies.yaml configuration file`,
}

var guardCheckCmd = &cobra.Command{
	Use:   `check "command"`,
	Short: "Evaluate a command against active security policies without executing it",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		command := strings.Join(args, " ")
		targetDir := flagGuardCwd
		if targetDir == "" {
			var err error
			targetDir, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current working directory: %w", err)
			}
		}

		cfg, err := guard.LoadConfig(targetDir)
		if err != nil {
			return fmt.Errorf("failed to load policies: %w", err)
		}

		result := guard.Evaluate(command, targetDir, cfg, flagGuardStrictWorkspace, false)

		if flagGuardJSON {
			data, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal evaluation result: %w", err)
			}
			fmt.Println(string(data))
			if !result.Allowed {
				os.Exit(1)
			}
			return nil
		}

		// Pretty terminal output
		fmt.Printf("\n--- msh Guardrail Evaluation ---\n")
		fmt.Printf("Command:          %s\n", command)
		fmt.Printf("Workspace:        %s\n", targetDir)
		fmt.Printf("Strict Workspace: %t\n", flagGuardStrictWorkspace)
		fmt.Printf("Overall Risk:     %s\n", strings.ToUpper(string(result.Risk)))

		if !result.Allowed {
			fmt.Printf("Verdict:          ⛔ BLOCKED\n\n")
		} else if len(result.Violations) > 0 {
			fmt.Printf("Verdict:          ⚠️  WARNING\n\n")
		} else {
			fmt.Printf("Verdict:          ✅ ALLOWED (Clean)\n\n")
		}

		if len(result.Violations) > 0 {
			fmt.Println("Violations Detected:")
			for i, v := range result.Violations {
				actionIcon := "⚠️"
				if v.Action == guard.ActionBlock {
					actionIcon = "⛔"
				}
				fmt.Printf("  %d. %s [%s] %s\n", i+1, actionIcon, strings.ToUpper(string(v.Risk)), v.RuleID)
				fmt.Printf("     Message: %s\n", v.Message)
				fmt.Printf("     Action:  %s\n", strings.ToUpper(string(v.Action)))
			}
			fmt.Println()
		}

		if !result.Allowed {
			os.Exit(1)
		}
		return nil
	},
}

var guardListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all active guardrail rules and policies",
	RunE: func(cmd *cobra.Command, args []string) error {
		targetDir := flagGuardCwd
		if targetDir == "" {
			var err error
			targetDir, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current working directory: %w", err)
			}
		}

		cfg, err := guard.LoadConfig(targetDir)
		if err != nil {
			return fmt.Errorf("failed to load policies: %w", err)
		}

		configPath := filepath.Join(targetDir, ".msh", "policies.yaml")
		hasCustom := false
		if _, err := os.Stat(configPath); err == nil {
			hasCustom = true
		}

		fmt.Printf("\n--- Active msh Security Policies (Workspace: %s) ---\n", targetDir)
		if hasCustom {
			fmt.Printf("Custom Config:    %s\n", configPath)
		} else {
			fmt.Printf("Custom Config:    (none found, using default Tier 1 protection)\n")
		}
		fmt.Printf("Strict Workspace: %t\n", cfg.StrictWorkspace)
		fmt.Printf("Total Rules:      %d\n\n", len(cfg.Policies))

		fmt.Printf("%-24s %-8s %-10s %s\n", "RULE ID", "ACTION", "RISK", "DESCRIPTION")
		fmt.Println(strings.Repeat("-", 80))
		for _, r := range cfg.Policies {
			desc := r.Description
			if desc == "" {
				desc = r.Message
			}
			fmt.Printf("%-24s %-8s %-10s %s\n", r.ID, strings.ToUpper(string(r.Action)), strings.ToUpper(string(r.Risk)), desc)
		}
		fmt.Println()
		return nil
	},
}

var guardInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a starter .msh/policies.yaml in the workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		targetDir := flagGuardCwd
		if targetDir == "" {
			var err error
			targetDir, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current working directory: %w", err)
			}
		}

		dotMsh := filepath.Join(targetDir, ".msh")
		if err := os.MkdirAll(dotMsh, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dotMsh, err)
		}

		policiesPath := filepath.Join(dotMsh, "policies.yaml")
		if _, err := os.Stat(policiesPath); err == nil {
			fmt.Printf("[msh:guard] configuration file already exists: %s\n", policiesPath)
			return nil
		}

		template := guard.TemplateYAML()
		if err := os.WriteFile(policiesPath, []byte(template), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", policiesPath, err)
		}

		fmt.Printf("[msh:guard] created policy file: %s\n", policiesPath)
		fmt.Println("Edit this file to configure project-specific boundaries and custom rules.")
		return nil
	},
}

func init() {
	guardCheckCmd.Flags().SetInterspersed(false)
	guardCheckCmd.Flags().StringVar(&flagGuardCwd, "cwd", "", "Workspace root directory")
	guardCheckCmd.Flags().BoolVar(&flagGuardStrictWorkspace, "strict-workspace", false, "Simulate strict workspace boundary enforcement")
	guardCheckCmd.Flags().BoolVar(&flagGuardJSON, "json", false, "Output evaluation as JSON")

	guardListCmd.Flags().StringVar(&flagGuardCwd, "cwd", "", "Workspace root directory")
	guardInitCmd.Flags().StringVar(&flagGuardCwd, "cwd", "", "Workspace root directory")

	guardCmd.AddCommand(guardCheckCmd)
	guardCmd.AddCommand(guardListCmd)
	guardCmd.AddCommand(guardInitCmd)

	rootCmd.AddCommand(guardCmd)
}
