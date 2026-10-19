package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

const skillContent = `---
name: msh
description: Deterministic execution runtime for AI coding agents. Use whenever running shell commands, tests, builds, or multi-file changes so prompts are auto-handled, output is truncated and redacted, and changes can be undone.
---

# msh — Execution Protocol for AI Agents

msh sits between your agent execution harness and the operating system as a deterministic runtime, flight recorder, and safety boundary.

## Core Rules

1. Always Prefix Shell Commands with msh
   - Run commands via msh <command> (e.g., msh npm test, msh cargo build, msh pytest -v).
   - Do not run raw npm, pytest, cargo, git, or python commands directly.
   - Interactive prompts ([y/N], confirmations) are automatically answered or flagged.
   - Massive compiler dumps and stack traces are smartly truncated to avoid token window blowouts.
   - API keys and environment secrets are masked in memory before returning to context.

2. Instant Rollback via msh undo
   - If a command, script, or edit causes regressions or breaks workspace files, immediately run:
     msh undo
     # or target a specific run: msh undo <run-id>
   - Reverts file modifications and additions made by the execution cleanly.
   - Never guess ad-hoc reversal patches or run destructive git reset --hard when a surgical msh undo exists.

3. Isolate Risky Work with msh branch
   - For multi-file speculative refactors or unverified experiments:
     # 1. Spin up an isolated shadow worktree
     msh branch create <name>

     # 2. Run builds or tests inside the sandbox
     msh branch run <name> "<command>"

     # 3. Inspect changes
     msh branch diff <name>

     # 4. Merge verified solutions or abort cleanly
     msh branch merge <name> -m "verified feature"
     # or: msh branch abort <name>

4. Pre-Execution Guardrails via msh guard
   - For high-risk or potentially destructive operations, verify safety first:
     msh guard check "<command>"
   - Catastrophic root wipes, fork bombs, and directory traversal escapes are intercepted before OS process creation.
`

var globalFlag bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize msh agent skills and rules in the current workspace or globally",
	Long:  "Writes the official msh skill and rules for Antigravity, Cursor, Windsurf, OpenCode, and Claude Code.",
	RunE: func(cmd *cobra.Command, args []string) error {
		type targetInfo struct {
			path  string
			agent string
		}

		if globalFlag {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get user home directory: %w", err)
			}

			globalTargets := []targetInfo{
				{path: filepath.Join(homeDir, ".gemini", "config", "skills", "msh", "SKILL.md"), agent: "Global Antigravity Skill"},
				{path: filepath.Join(homeDir, ".gemini", "config", "rules", "msh.md"), agent: "Global Antigravity Rule"},
			}

			for _, target := range globalTargets {
				dir := filepath.Dir(target.path)
				if err := os.MkdirAll(dir, 0755); err != nil {
					return fmt.Errorf("failed to create directory %s: %w", dir, err)
				}
				if err := os.WriteFile(target.path, []byte(skillContent), 0644); err != nil {
					return fmt.Errorf("failed to write %s: %w", target.path, err)
				}
				fmt.Printf("✓ Installed %-45s (%s)\n", target.path, target.agent)
			}

			fmt.Println("\nSuccessfully installed msh globally for Antigravity!")
			fmt.Println("All existing and future Antigravity projects will now use msh automatically.")
			return nil
		}

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		targets := []targetInfo{
			{path: filepath.Join(cwd, "AGENTS.md"), agent: "Universal Agent Rule (Antigravity/OpenCode)"},
			{path: filepath.Join(cwd, ".agents", "skills", "msh", "SKILL.md"), agent: "Antigravity Skill"},
			{path: filepath.Join(cwd, ".cursor", "rules", "msh.mdc"), agent: "Cursor Rule"},
			{path: filepath.Join(cwd, ".windsurfrules"), agent: "Windsurf Rule"},
			{path: filepath.Join(cwd, ".opencode", "skills", "msh", "SKILL.md"), agent: "OpenCode Skill"},
			{path: filepath.Join(cwd, ".claude", "skills", "msh", "SKILL.md"), agent: "Claude Code Skill"},
		}

		createdCount := 0
		for _, target := range targets {
			dir := filepath.Dir(target.path)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", dir, err)
			}
			if err := os.WriteFile(target.path, []byte(skillContent), 0644); err != nil {
				return fmt.Errorf("failed to write %s: %w", target.path, err)
			}
			relPath, err := filepath.Rel(cwd, target.path)
			if err != nil {
				relPath = target.path
			}
			fmt.Printf("✓ Created %-45s (%s)\n", relPath, target.agent)
			createdCount++
		}

		fmt.Printf("\nSuccessfully initialized msh across %d agent ecosystems.\n", createdCount)
		fmt.Println("AI coding agents (Antigravity, Cursor, Windsurf, OpenCode, Claude Code) in this project will now use msh automatically.")
		return nil
	},
}

func init() {
	initCmd.Flags().BoolVarP(&globalFlag, "global", "g", false, "Install msh skill and rule globally for all Antigravity projects (~/.gemini/config)")
	rootCmd.AddCommand(initCmd)
}
