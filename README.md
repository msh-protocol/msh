# msh (Machine Shell)

> The Deterministic Execution Protocol & Runtime for AI Agents.

`msh` is an open-source execution layer built specifically for autonomous coding agents, LLM tool-use systems, and agentic workflows. Instead of forcing AI models to parse unstructured, ANSI-polluted terminal streams, `msh` provides a machine-readable execution environment with strict state preservation, automated log sanitization, and structured JSON output.

## Why msh?

When an AI agent runs `npm run build`, it usually receives a raw stream of ANSI escape codes, progress bars, and unstructured text. If the command asks a `[y/N]` question, the agent hangs forever.

`msh` acts as a protective runtime between the agent and the OS:
- **Returns Structured JSON** — Everything is an `ExecResponse`.
- **Sanitizes Output** — Strips all ANSI codes and progress bars.
- **Prevents Context Blowouts** — Automatically truncates massive error dumps (keeps first 100 + last 100 lines).
- **Detects Prompts** — Kills the process and returns `"status": "blocked"` if a `[y/N]` or password prompt appears.
- **Network Daemon** — Run `msh serve` to expose an HTTP REST API, allowing remote agents to manage stateful execution sessions over the network.
- **Filesystem Diffing** — Returns exactly which files were added, modified, or deleted during the command.

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

#### 2. HTTP Daemon (Stateful Network Sessions)

Start the server:
```bash
msh serve --port 8080
```

Agents can now submit JSON requests over HTTP. By passing a `session_id`, `msh` remembers the working directory across multiple REST requests!

**Request 1 (Create Session & Change Directory):**
```bash
curl -X POST http://127.0.0.1:8080/execute \
  -d '{"command": "cd src", "session_id": "agent-123"}'
```
Returns: `{"session_id":"agent-123", "cwd":"/project/src", ...}`

**Request 2 (Execute in that Directory):**
```bash
curl -X POST http://127.0.0.1:8080/execute \
  -d '{"command": "ls", "session_id": "agent-123"}'
```
Returns: `{"session_id":"agent-123", "cwd":"/project/src", "stdout":"main.go...", ...}`

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
