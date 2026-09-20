# msh: sudo for AI Agents

> The local, machine-readable execution layer for LLMs and autonomous coding agents.

[![CI](https://github.com/msh-protocol/msh/actions/workflows/ci.yml/badge.svg)](https://github.com/msh-protocol/msh/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/msh-protocol/msh)](https://github.com/msh-protocol/msh/releases)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

When an AI agent runs a terminal command, raw shells break:
1. **Interactive Prompts Hang Forever**: Command asks `[y/N]`, `password:`, or `npm init` confirmation → the agent loop freezes indefinitely.
2. **Context Window Blowouts**: A compiler error dump or pytest trace outputs 30,000 lines → blows through model context limits and bankrupts token budgets.
3. **Secret Leaks**: Scripts print environment variables, AWS keys, or GitHub tokens → secrets are permanently leaked into prompt histories and model providers.
4. **ANSI Corruption**: Terminal colors, progress bars, and cursor movements pollute model embeddings and cause hallucinated outputs.

`msh` sits between your AI agent and the operating system as a protective, deterministic execution runtime.

---

## Quick Start

### Install

#### macOS / Linux
```bash
curl -fsSL https://raw.githubusercontent.com/msh-protocol/msh/main/scripts/install.sh | bash
```

#### Windows (PowerShell)
```powershell
irm https://raw.githubusercontent.com/msh-protocol/msh/main/scripts/install.ps1 | iex
```

#### Go Install
```bash
go install github.com/msh-protocol/msh/cmd/msh@latest
```

---

### 1. Universal Command Prefix (`msh <command>`)
Run any command through `msh` simply by prefixing it — no wrapping keywords or outer quotes required:

```bash
msh git status
msh npm run build
msh pytest -v
msh cargo test
```

Outputs are automatically sanitized (ANSI stripped), truncated (prevents token blowouts), secret-redacted, and interactive prompts are detected and answered.

Optional flags:
```bash
msh --max-lines 500 git log
msh --timeout 1m python script.py
msh --pty npm test
```

---

### 2. Model Context Protocol (MCP) Server
`msh` natively exposes an MCP server over standard I/O for Claude Desktop, Cursor, Claude Code, and Windsurf:

```json
{
  "mcpServers": {
    "msh": {
      "command": "msh",
      "args": ["mcp"]
    }
  }
}
```
Your agent immediately receives a safe, structured `execute_command` tool that cannot hang on `[y/N]` prompts or leak secrets.

---

### 3. Python SDK (`pip install msh-protocol`)

```python
from msh import exec

# Execute with automatic secret masking & token truncation
res = exec("npm test", max_output_lines=200)

if res.status == "blocked":
    print(f"Command requested confirmation: {res.prompt_detected}")
elif res.ok:
    print(res.stdout)
else:
    print(f"Failed with exit code {res.exit_code}: {res.stderr}")
```

#### 15-Line Drop-in Tool for LangChain / Claude:
```python
from msh import exec

def safe_shell(command: str) -> str:
    """Execute command safely without ANSI garbage, prompt hangs, or secret leaks."""
    res = exec(command, max_output_lines=300)
    if not res.ok:
        return f"Error (exit {res.exit_code}):\n{res.stderr or res.stdout}"
    return res.stdout
```

---

### 4. TypeScript / Node SDK (`npm install @msh-protocol/client`)

```typescript
import { exec } from '@msh-protocol/client';

const res = await exec('git status', { maxOutputLines: 200 });
console.log(res.stdout, res.exit_code, res.truncated);
```

---

### 5. Structured JSON Output (`msh exec`)
For direct integration with CLI pipelines and JSON agents:

```bash
msh exec "npm run build" --max-lines 500
```

Returns:
```json
{
  "session_id": "session-8b1c4e...",
  "status": "success",
  "exit_code": 0,
  "cwd": "/workspace/app",
  "stdout": "Build completed successfully.",
  "stderr": "",
  "truncated": false,
  "redacted": ["AWS_SECRET_ACCESS_KEY"],
  "files_changed": ["dist/main.js"],
  "duration_ms": 420
}
```

---

## Core Features

| Feature | Raw Terminal | `msh` |
| :--- | :--- | :--- |
| **Interactive Prompts (`[y/N]`)** | ❌ Hangs forever | ✅ Auto-answers or signals `blocked` |
| **Compiler Error Dumps** | ❌ Blows 100k token context | ✅ Smart truncation (keeps head + tail) |
| **API Keys & Tokens in Logs** | ❌ Leaked to LLM provider | ✅ Automatically masked |
| **ANSI Escape Codes** | ❌ Unstructured garbage | ✅ Clean plain text |
| **Exit Code Fidelity** | ⚠️ Shell-dependent | ✅ Preserved strictly |
| **Tool Calling Contract** | ❌ Raw unstructured text | ✅ Structured `ExecResponse` JSON |

---

## Protocol Specification

### Request Fields (`ExecRequest`)

| Field | Type | Default | Description |
|---|---|---|---|
| `command` | `string` | *required* | Shell command line to execute |
| `cwd` | `string` | current dir | Directory to execute the command in |
| `env` | `object` | `{}` | Key-value environment variables to inject |
| `timeout` | `string` | `30s` | Maximum duration before SIGKILL (e.g. `30s`, `5m`) |
| `max_output_lines` | `integer` | `200` | Max lines to return; preserves head and tail |
| `detect_files` | `boolean` | `true` | Tracks created/modified/deleted files via snapshots |
| `use_pty` | `boolean` | `false` | Runs inside pseudo-terminal for TTY-demanding tools |
| `redact_secrets` | `boolean` | `true` | Masks known tokens (AWS, GitHub, Slack, OpenAI) |
| `prompt_answers` | `string[]` | `[]` | Sequential answers fed to interactive prompts |

### Response Fields (`ExecResponse`)

| Field | Type | Description |
|---|---|---|
| `status` | `string` | `success`, `error`, `timeout`, or `blocked` |
| `exit_code` | `integer` | Exact process exit code |
| `cwd` | `string` | Working directory after command completed |
| `stdout` | `string` | Sanitized stdout with ANSI stripped |
| `stderr` | `string` | Sanitized stderr with ANSI stripped |
| `truncated` | `boolean` | `true` if output exceeded `max_output_lines` |
| `redacted` | `string[]` | Types of detected secrets that were masked |
| `files_changed` | `string[]` | Paths of files created, modified, or deleted |
| `file_diffs` | `object` | Map of file paths to unified git-style diffs |
| `error_root_cause` | `object` | Isolated compiler/runtime root cause (type, message, file, line) |
| `run_hash` | `string` | Cryptographic SHA-256 fingerprint for run verification |
| `duration_ms` | `integer` | Execution duration in milliseconds |
| `prompt_detected` | `string` | Text of prompt if command was blocked |

---

### 6. Atomic Workspace Rollback (`msh undo`)
The "Undo" button for AI agents. Surgically reverts per-command diff patches, restores modified/deleted files, and removes created files without wiping unrelated work:

```bash
msh undo          # Rollback files modified by the most recent execution
msh undo 42       # Rollback files modified by execution #42
```

---

### 7. Cryptographic Run Verification (`msh verify`)
Prove deterministic execution reproducibility for benchmarks and enterprise compliance:

```bash
msh verify 42     # Re-execute #42 and evaluate bit-for-bit reproducibility
```

---

## Observability & Fleet Hub (Optional)

`msh` includes an optional lightweight registry hub for monitoring background processes and distributed worker nodes:
```bash
msh fleet start
```
Starts an embedded web dashboard (`http://localhost:9000`) with live multi-terminal streaming, execution replay, visual git-style file diff drawer, atomic rollback (`Undo Changes ↺`), cryptographic run verification, and health metrics.

---

## Evaluation Suite

`msh` includes an automated benchmark evaluating real-world problematic CLI scenarios (`evals/cli_evals_test.go`):
- ANSI color and progress bar stripping
- 10,000-line compiler dumps safely truncated
- Interactive prompt detection (`[y/N]`, password, `npm init`)
- Secret leakage masking (AWS, GitHub PAT, OpenAI keys, RSA private keys)
- Exit code fidelity preservation

Run the evaluations:
```bash
msh go test -v ./evals/...
```

---

## Status

**`v1.3.0` — Production Ready.** Active use across agentic workflows and developer toolchains.

---

## License

Apache 2.0 — See [LICENSE](LICENSE) for details.
