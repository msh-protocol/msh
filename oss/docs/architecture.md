# msh Architecture

> Technical overview of the msh execution runtime internals.

## System Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                        msh Runtime                                  │
│                                                                     │
│  ┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────────┐  │
│  │ Protocol │───▶│ Executor │───▶│ Sanitize │───▶│  Structured  │  │
│  │  Parser  │    │  Engine  │    │ Pipeline │    │ JSON Output  │  │
│  └──────────┘    └──────────┘    └──────────┘    └──────────────┘  │
│       │               │               │                             │
│       │          ┌──────────┐    ┌──────────┐                      │
│       │          │ Session  │    │    FS    │                       │
│       └─────────▶│  State   │    │ Snapshot │                      │
│                  └──────────┘    └──────────┘                      │
└─────────────────────────────────────────────────────────────────────┘
```

## Package Responsibilities

### `pkg/protocol`
Defines the **execution contract** — the JSON types that form the interface between AI agents and the msh runtime.

- `ExecRequest` — What the agent sends (command, cwd, env, timeout, max lines)
- `ExecResponse` — What msh returns (status, exit code, sanitized stdout/stderr, files changed, duration)
- Status types: `success`, `error`, `timeout`, `blocked`

### `pkg/execution`
The **core engine** that spawns subprocesses and manages their lifecycle.

- **Executor** — Runs commands with timeout enforcement, captures output, coordinates sanitization and filesystem diffing
- **Session** — Maintains persistent state across commands:
  - Working directory tracking (survives `cd` commands)
  - Environment variable layering (session vars override host vars)
  - Cross-platform shell selection (`cmd /C` on Windows, `sh -c` on Unix)

### `pkg/sanitize`
The **token-saving pipeline** that cleans raw terminal output for AI consumption.

- **ANSI Stripping** — Removes all escape sequences (colors, cursor movement, OSC, 256-color)
- **Output Truncation** — Keeps first N/2 + last N/2 lines with a truncation marker. Prevents a 50MB error log from blowing out an agent's context window.
- **Prompt Detection** — Identifies interactive prompts (`[y/N]`, `password:`, `Are you sure`) that would hang an agent's execution loop

### `pkg/fs`
**Filesystem change detection** via lightweight stat-based snapshots.

- Pre-execution: snapshot all file metadata (mtime + size)
- Post-execution: re-snapshot and diff
- Returns list of added, modified, and deleted files
- Skips `node_modules/`, `.git/`, `dist/`, etc. via ignore patterns

## Execution Flow

```
1. Agent sends ExecRequest (JSON)
       │
2. Resolve CWD (session state or override)
       │
3. Build environment (host → session → per-command layers)
       │
4. [Optional] Take filesystem pre-snapshot
       │
5. Spawn subprocess with timeout context
       │
6. Capture stdout + stderr into buffers
       │
7. On completion/timeout/error:
   ├── Strip ANSI escape codes
   ├── Truncate output to max lines
   ├── Detect interactive prompts
   ├── [Optional] Take post-snapshot, compute diff
   └── Update session state (CWD changes)
       │
8. Package and return ExecResponse (JSON)
```

## Design Decisions

### Why `os/exec` instead of PTY in V0.1?
PTY (pseudo-terminal) support adds complexity, especially cross-platform. V0.1 uses direct subprocess spawning via `os/exec` which is simpler, faster, and sufficient for most agent use cases. PTY support (via `aymanbagabas/go-pty`) is planned for V0.2 to handle commands that require a terminal.

### Why stat-based diffing instead of content hashing?
Reading file contents to compute hashes (SHA-256, xxHash) is expensive for large workspaces. Comparing `mtime + size` via `os.Stat()` is orders of magnitude faster and catches >99% of real modifications. False positives (same size, updated mtime, no content change) are acceptable for the `files_changed` field.

### Why regex for ANSI stripping instead of a DFA?
V0.1 uses a compiled regex for simplicity and correctness. The regex handles CSI, OSC, and simple escape sequences. A hand-written DFA state machine would be faster for high-throughput streaming, but the regex adds <1ms overhead per command — negligible compared to subprocess execution time. DFA upgrade is planned for V0.2.
