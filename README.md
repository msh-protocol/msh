# msh (Machine Shell)

> The Deterministic Execution Protocol & Runtime for AI Agents.

`msh` is an open-source execution layer built specifically for autonomous coding agents, LLM tool-use systems, and agentic workflows. Instead of forcing AI models to parse unstructured, ANSI-polluted terminal streams, `msh` provides a machine-readable execution environment with strict state preservation, automated log sanitization, and structured JSON output.

## Why msh?

Standard shells (`bash`, `zsh`) were designed for human eyes in 1989. When AI agents execute commands via traditional subprocesses, they encounter:

- **Terminal Locks:** Interactive prompts (`[y/N]`) hang the execution loop indefinitely.
- **Context Pollution:** Raw ANSI escape codes (`\033[0;31m`) waste token context.
- **Token Blowouts:** Massive error dumps exhaust model context windows.
- **State Loss:** Sequential commands lose relative directory and environment context.

`msh` solves the execution layer so agent builders can focus entirely on the intelligence layer.

## Quick Start

```bash
# Install
go install github.com/msh-protocol/msh/cmd/msh@latest

# Execute a command with structured output
msh exec "echo hello world"

# Pretty-print the JSON response
msh exec "ls -la" --pretty

# Set a timeout
msh exec "npm run build" --timeout 60s

# Limit output lines (saves tokens)
msh exec "cat huge_log.txt" --max-lines 100
```

## The Structured Output Contract

Every execution in `msh` yields a machine-readable payload:

```json
{
  "status": "success",
  "exit_code": 0,
  "cwd": "/workspace/app",
  "stdout": "Build completed successfully.",
  "stderr": "",
  "truncated": false,
  "files_changed": ["dist/main.js"],
  "duration_ms": 420
}
```

### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `status` | string | `success`, `error`, `timeout`, or `blocked` |
| `exit_code` | int | Process exit code |
| `cwd` | string | Working directory after execution |
| `stdout` | string | Sanitized standard output (ANSI stripped) |
| `stderr` | string | Sanitized standard error (ANSI stripped) |
| `truncated` | bool | Whether output was truncated to save tokens |
| `files_changed` | []string | Files added, modified, or deleted |
| `duration_ms` | int64 | Execution time in milliseconds |
| `prompt_detected` | string | Interactive prompt text if execution was blocked |

## Architecture

```
+-------------------+           +-----------------------+           +----------------------+
|                   |  JSON     |                       |  Syscall  |                      |
|  AI Agent / LLM   | --------> |     msh Runtime       | --------> |   Operating System   |
| (Gemini/Claude)   | <-------- |  (Go Execution Engine)| <-------- |   (POSIX / Windows)  |
|                   |  Response |                       |  Results  |                      |
+-------------------+           +-----------------------+           +----------------------+
```

## Project Structure

```
msh/
├── cmd/
│   └── msh/              # CLI entrypoint
├── pkg/
│   ├── protocol/          # JSON payload schemas & contracts
│   ├── execution/         # Process spawning & state management
│   ├── sanitize/          # ANSI stripping, log truncation, prompt detection
│   └── fs/                # Filesystem change detection
├── go.mod
├── LICENSE
└── README.md
```

## Status

`msh` is currently under active development. V0.1 (core execution engine) is in progress.

## License

Apache 2.0 — See [LICENSE](LICENSE) for details.
