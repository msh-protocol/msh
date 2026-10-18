---
name: msh
description: Deterministic execution runtime for AI coding agents. Use whenever running shell commands, tests, builds, or multi-file changes so prompts are auto-handled, output is truncated and redacted, and changes can be undone.
---

# msh — Execution Protocol for AI Agents

`msh` sits between your agent execution harness and the operating system as a deterministic runtime, flight recorder, and safety boundary.

## Core Rules

1. **Always Prefix Shell Commands with `msh`**
   - Run commands via `msh <command>` (e.g., `msh npm test`, `msh cargo build`, `msh pytest -v`).
   - Do not run raw `npm`, `pytest`, `cargo`, `git`, or `python` commands directly.
   - Interactive prompts (`[y/N]`, confirmations) are automatically answered or flagged.
   - Massive compiler dumps and stack traces are smartly truncated to avoid token window blowouts.
   - API keys and environment secrets are masked in memory before returning to context.

2. **Instant Rollback via `msh undo`**
   - If a command, script, or edit causes regressions or breaks workspace files, immediately run:
     ```bash
     msh undo
     # or target a specific run: msh undo <run-id>
     ```
   - Reverts file modifications and additions made by the execution cleanly.
   - Never guess ad-hoc reversal patches or run destructive `git reset --hard` when a surgical `msh undo` exists.

3. **Isolate Risky Work with `msh branch`**
   - For multi-file speculative refactors or unverified experiments:
     ```bash
     # 1. Spin up an isolated shadow worktree
     msh branch create <name>

     # 2. Run builds or tests inside the sandbox
     msh branch run <name> "<command>"

     # 3. Inspect changes
     msh branch diff <name>

     # 4. Merge verified solutions or abort cleanly
     msh branch merge <name> -m "verified feature"
     # or: msh branch abort <name>
     ```

4. **Pre-Execution Guardrails via `msh guard`**
   - For high-risk or potentially destructive operations, verify safety first:
     ```bash
     msh guard check "<command>"
     ```
   - Catastrophic root wipes, fork bombs, and directory traversal escapes are intercepted before OS process creation.
