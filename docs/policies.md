# Pre-Execution Guardrails & Security Policies

> **Introduced in msh v1.5.0** (Phase 8)

Autonomous coding agents frequently generate and execute shell commands that can unintentionally cause irreversible damage—such as deleting root directories (`rm -rf /`), wiping operating system drives, executing fork bombs, formatting volumes, or escaping the current project workspace.

`msh` implements a **Three-Tier Pre-Execution Defense Architecture** that inspects, analyzes, and enforces security policies **before any OS process is spawned**, ensuring zero CPU execution time is wasted on malicious or destructive instructions.

---

## The Three-Tier Defense Architecture

```
                    Incoming Agent Shell Command
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────┐
│ TIER 1: Built-in Catastrophic Safety Floor                      │
│ Always Active · Non-Bypassable (without explicit --no-guard)    │
│ Blocks: rm -rf /, drive wipes, fork bombs, dd disk overwrites   │
└────────────────────────────────┬────────────────────────────────┘
                                 │ Pass
                                 ▼
┌─────────────────────────────────────────────────────────────────┐
│ TIER 2: Workspace Policy Engine (.msh/policies.yaml)            │
│ Team-Configured · Regex Matching · Risk Tiers · Custom Actions  │
│ Actions: "block" (exit -1, StatusBlocked) or "warn"             │
└────────────────────────────────┬────────────────────────────────┘
                                 │ Pass
                                 ▼
┌─────────────────────────────────────────────────────────────────┐
│ TIER 3: Strict Workspace Confinement (--strict-workspace)       │
│ Zero-Trust Isolation · Traversal Detection (../../) · Redirection│
│ Confines writes and modifications strictly to workspace root   │
└────────────────────────────────┬────────────────────────────────┘
                                 │ Pass
                                 ▼
                   Deterministic Process Execution
```

---

## Tier 1: Built-In Catastrophic Safety Floor

The Catastrophic Safety Floor is built directly into the `msh` core binary and is **always active by default**. No configuration is required.

The baseline floor blocks:

| Rule ID | Risk Level | Default Action | Prevented Danger |
| :--- | :--- | :--- | :--- |
| `catastrophic-root-deletion` | `CRITICAL` | `BLOCK` | Deleting the filesystem root or user home directory (`rm -rf /`, `rm -rf ~`, `rm -rf /*`). |
| `windows-root-wipe` | `CRITICAL` | `BLOCK` | Recursive drive root wipe on Windows (`rmdir /s /q C:\`). |
| `shell-fork-bomb` | `CRITICAL` | `BLOCK` | Classic bash/zsh fork bomb denial-of-service (`:(){ :|:& };:`). |
| `raw-disk-overwrite` | `CRITICAL` | `BLOCK` | Overwriting raw storage block devices via `dd` (`dd ... of=/dev/sd*`). |
| `raw-filesystem-format` | `CRITICAL` | `BLOCK` | Formatting disk partitions (`mkfs.*`, `Format-Volume`, `format C:`). |
| `recursive-root-chmod` | `CRITICAL` | `BLOCK` | Making root filesystem globally world-writable (`chmod -R 777 /`). |

When a Tier 1 rule is violated, `msh` intercepts the command immediately:
- The OS process is **never spawned**.
- `resp.Status` is set to `"blocked"`.
- `resp.ExitCode` is `-1`.
- Structured `policy_violations` and `risk_level` are returned to the agent.

---

## Tier 2: Custom Project Policies (`.msh/policies.yaml`)

Teams and enterprise repositories can define repository-specific execution boundaries by placing a `.msh/policies.yaml` file in the root of the project.

### Initializing Policy Configuration

Run the built-in initializer to generate a starter template:

```bash
msh guard init
```

This creates `.msh/policies.yaml`:

```yaml
version: "1.0"

# Enforce strict workspace containment by default for this repository
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
  # Example 1: Prevent agent from running global package installations
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
```

---

## Tier 3: Strict Workspace Confinement (`--strict-workspace`)

By default, developer commands like `npm run test` or `git status` may reference parent tools or global runtimes. When running completely autonomous agents or unvetted tasks, pass `--strict-workspace` (or configure `strict_workspace: true` in `.msh/policies.yaml`):

```bash
msh --strict-workspace "cat ../../etc/passwd"
# => Intercepted: Command targets sensitive paths outside workspace boundary
```

Strict workspace boundary enforcement checks:
1. **Path Traversal Escapes**: Flags write/delete operations attempting to escape the project directory via `../..`.
2. **System Root Access**: Blocks targeting `/etc/`, `/var/`, `C:\Windows\`, or `~/.ssh`.
3. **Explicit Denied Paths**: Blocks access to any path listed under `denied_paths` in `.msh/policies.yaml`.

---

## The `--no-guard` Escape Hatch

In rare development workflows or trusted administrative sessions where raw system access is deliberately required, developers can bypass guardrail evaluation with `--no-guard`:

```bash
msh --no-guard "sudo dd if=/dev/zero of=/dev/sdb bs=1M count=10"
```

In JSON requests, set `"no_guard": true`.

---

## CLI Subcommands

### 1. `msh guard check`
Simulates and evaluates any command string against active policies without executing:

```bash
msh guard check "rm -rf /"
msh guard check --json "npm i -g tool"
msh guard check --strict-workspace "cat ../../secret.env"
```

### 2. `msh guard list`
Inspects and displays all active policies, risk tiers, and actions:

```bash
msh guard list
```

### 3. `msh guard init`
Scaffolds a clean `.msh/policies.yaml` template in the workspace:

```bash
msh guard init
```

---

## Fleet UI Dashboard Integration

The `msh fleet` dashboard includes dedicated visual guardrail management:
- **Guardrails Tab**: View active safety tiers, inspect the security policies matrix, and test commands in real-time with the **Interactive Policy Simulator**.
- **History Docket**: Filter runs by ` Guardrails` to immediately isolate blocked commands and inspect security violation details and risk pills.
- **REST Endpoints**:
  - `GET /api/policies`: Fetches loaded policy configuration and rules.
  - `POST /api/guard/check`: Evaluates command payloads and returns verdict and risk classification.
