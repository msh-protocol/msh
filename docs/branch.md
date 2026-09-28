# Speculative Execution & Shadow Worktrees (`msh branch`)

> **Academic Foundation**: Transactional state isolation for multi-step AI agent tool use (arXiv:2512.12806, arXiv:2605.22781) and Semantic Context Synchronization (arXiv:2608.03836).

When autonomous AI coding agents explore alternative refactoring strategies, architectural redesigns, or run dangerous test suites, running commands directly in the primary workspace risks corrupting git state and polluting the developer's working directory.

`msh branch` provisions **sub-second shadow git worktrees** (`.msh/branches/<name>`), allowing agents to trial speculative code changes, execute commands in total isolation, and either atomically merge verified solutions or cleanly abort failed hypotheses.

---

## Architecture & How It Works

```
┌───────────────────────────────────────────────────────────────┐
│                     Primary Working Tree                      │
│                  (Protected Developer State)                  │
└──────────────────────────────┬────────────────────────────────┘
                               │
               git worktree add (sub-second)
                               ▼
┌───────────────────────────────────────────────────────────────┐
│              Shadow Worktree: .msh/branches/hypo-1            │
│                 (Isolated Agent Playground)                   │
├───────────────────────────────────────────────────────────────┤
│ • Execution:  msh branch run hypo-1 "npm test"                │
│ • State:      Isolated HEAD, untracked delta monitoring       │
│ • Inspect:    msh branch diff hypo-1                          │
└──────────────┬────────────────────────────────┬───────────────┘
               │                                │
    Verified Solution?                   Failed Attempt?
               ▼                                ▼
┌──────────────────────────────┐ ┌──────────────────────────────┐
│       msh branch merge       │ │       msh branch abort       │
│  Atomic merge into main      │ │  Prune worktree, wipe branch,│
│  Prunes shadow worktree      │ │  emit context rewind turns   │
└──────────────────────────────┘ └──────────────────────────────┘
```

1. **Sub-Second Provisioning**: Leverages native Git worktrees (`git worktree add -b msh-shadow-<name> .msh/branches/<name> <ref>`). No Docker containers, no VM boot times, zero network dependency.
2. **True State Isolation**: Files written, modified, or deleted within a shadow worktree never leak into the primary repository or trigger local file watchers.
3. **Semantic Context Synchronization (arXiv:2608.03836)**: When an exploratory branch is aborted, `msh` emits a `context_prune_turns` counter instructing agent harnesses how many turns of hallucinated execution history to prune from LLM memory.

---

## CLI Reference

### 1. Create a Shadow Branch (`msh branch create`)

Spins up an isolated shadow worktree anchored to the current commit or a specified ref:

```bash
# Basic creation
msh branch create opt-cache

# With custom base ref and description
msh branch create try-ast --from HEAD~2 --desc "Trial AST transform instead of regex"

# Structured JSON output for agent harnesses
msh branch create trial-1 --json
```

### 2. List Active Shadow Worktrees (`msh branch list`)

Displays all active speculative branches, live modification counts, run counts, and base commit hashes:

```bash
msh branch list
```

Example output:
```
NAME         STATUS   RUNS   MODIFIED   CREATED    BASE COMMIT   DESCRIPTION
opt-cache    active   3      2 files    2m ago     99c36b20      Trial caching layer
try-ast      active   1      clean      45s ago    99c36b20      Trial AST transform
```

### 3. Run Commands in Isolation (`msh branch run`)

Executes shell commands strictly within the designated shadow worktree without touching your main workspace:

```bash
# Run tests inside shadow worktree
msh branch run opt-cache "npm test"

# Run builds and capture modified files
msh branch run opt-cache "go build -v ./..."
```

### 4. Inspect Speculative Diffs (`msh branch diff`)

Generates a unified git diff of all modifications made within the shadow worktree against its base reference:

```bash
msh branch diff opt-cache
```

### 5. Merge Verified Solutions (`msh branch merge`)

Atomically applies the verified solution back into your primary working tree and automatically cleans up the shadow worktree and git branch:

```bash
msh branch merge opt-cache -m "perf: implement high-throughput caching layer"
```

### 6. Abort & Prune Failed Hypotheses (`msh branch abort`)

Discards speculative changes cleanly, deletes the worktree and branch, and outputs the semantic pruning payload:

```bash
msh branch abort try-ast
```

Output:
```
✓ Speculative hypothesis 'try-ast' aborted.
  • Status:               Cleaned & Pruned
  • Context Prune Turns:  3 (suggested LLM history rewind, arXiv:2608.03836)
  • Reverted Files (2):   parser.go, ast.go
  Main workspace remains 100% clean.
```

---

## Fleet UI Speculation Dashboard

The `msh fleet` web control plane includes a dedicated **Branches** tab:
- **Real-Time Worktree Matrix**: Monitor all parallel hypotheses and modified file counters.
- **In-Browser Command Runner**: Dispatch commands directly to shadow worktrees with real-time output streams.
- **Side-by-Side Diff Inspector**: Inspect code diffs before merging.
- **One-Click Merge & Abort**: One-touch controls with prompt rewind guidance banners.
- **Strict Theme Adherence**: Styled in the vintage Bookbinder Editorial theme using stroke-calibrated SVG vector icons.

---

## REST Endpoints for Agent Swarms

All branch capabilities are accessible via HTTP for agent swarms:

| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/api/branches` | List all active shadow worktrees and status |
| `POST` | `/api/branch/create` | Spin up a new shadow worktree (`{"name", "from", "desc"}`) |
| `POST` | `/api/branch/run` | Execute command in shadow worktree (`{"name", "command"}`) |
| `GET` | `/api/branch/diff?name=<name>` | Get unified git diff for branch |
| `POST` | `/api/branch/merge` | Merge shadow branch into primary repo (`{"name", "message"}`) |
| `POST` | `/api/branch/abort` | Discard worktree and return prune turns (`{"name"}`) |
