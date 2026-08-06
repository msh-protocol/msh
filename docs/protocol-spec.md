# msh Protocol Specification v0.1

> The machine-readable execution contract for AI agents.

This document defines the JSON protocol that AI agents use to communicate with the msh runtime. Any client implementing this specification can execute commands through msh and receive structured results.

## ExecRequest

An agent sends an `ExecRequest` to execute a command.

```json
{
  "command": "npm run build",
  "cwd": "/workspace/app",
  "env": {
    "NODE_ENV": "production"
  },
  "timeout": 60000000000,
  "max_output_lines": 100,
  "detect_files": true
}
```

### Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `command` | string | ✅ | — | Shell command to execute |
| `cwd` | string | ❌ | Session CWD | Working directory override |
| `env` | object | ❌ | `{}` | Additional environment variables |
| `timeout` | integer | ❌ | `30s` | Max execution time (nanoseconds) |
| `max_output_lines` | integer | ❌ | `200` | Output truncation limit |
| `detect_files` | boolean | ❌ | `false` | Enable filesystem change detection |

## ExecResponse

The msh runtime returns an `ExecResponse` after command execution.

```json
{
  "status": "success",
  "exit_code": 0,
  "cwd": "/workspace/app",
  "stdout": "Build completed successfully.",
  "stderr": "",
  "truncated": false,
  "files_changed": ["dist/main.js", "dist/main.css"],
  "duration_ms": 4200
}
```

### Fields

| Field | Type | Always Present | Description |
|-------|------|----------------|-------------|
| `status` | string | ✅ | Execution outcome (see Status Codes) |
| `exit_code` | integer | ✅ | Process exit code (`-1` for timeout/system errors) |
| `cwd` | string | ✅ | Working directory after execution |
| `stdout` | string | ✅ | Sanitized standard output (ANSI stripped) |
| `stderr` | string | ✅ | Sanitized standard error (ANSI stripped) |
| `truncated` | boolean | ✅ | Whether output was truncated |
| `files_changed` | string[] | ❌ | Files added/modified/deleted (only when `detect_files: true`) |
| `duration_ms` | integer | ✅ | Wall-clock execution time in milliseconds |
| `prompt_detected` | string | ❌ | Interactive prompt text (only when `status: "blocked"`) |
| `error` | string | ❌ | System-level error description (binary not found, permission denied) |

## Status Codes

| Status | Meaning | Exit Code |
|--------|---------|-----------|
| `success` | Command completed with exit code 0 | `0` |
| `error` | Command completed with non-zero exit code | `1-255` |
| `timeout` | Command exceeded execution time limit | `-1` |
| `blocked` | Command waiting for interactive input | varies |

## Output Sanitization

All output returned in `stdout` and `stderr` is processed through the msh sanitization pipeline:

1. **ANSI Stripping** — All terminal escape sequences are removed:
   - CSI sequences (`\033[0;31m` → removed)
   - OSC sequences (terminal titles, hyperlinks → removed)
   - Cursor movement sequences → removed
   - Carriage return progress bars → removed

2. **Truncation** — If output exceeds `max_output_lines`:
   - First `N/2` lines are kept
   - Last `N/2` lines are kept
   - Middle section replaced with: `... [msh: truncated X lines to save tokens] ...`

3. **Prompt Detection** — Lines matching interactive prompt patterns are flagged:
   - `[y/N]`, `[yes/no]`, `(y/n)`
   - `password:`, `passphrase:`
   - `Are you sure`, `Press enter to continue`
   - `Ok to proceed?`, `Do you want to install`

## Session Semantics

Commands execute within a persistent session:

- **CWD Persistence** — `cd src/api` updates the session state. The next command runs in `src/api`.
- **Environment Layering** — Variables set via `env` in the request persist only for that command. Session-level variables persist across commands.
- **Shell Selection** — msh automatically uses `sh -c` on Unix and `cmd /C` on Windows.

## Example: Agent Integration

```python
# Python example — calling msh from an AI agent
import subprocess
import json

result = subprocess.run(
    ["msh", "exec", "npm run build", "--timeout", "60s"],
    capture_output=True, text=True
)

response = json.loads(result.stdout)

if response["status"] == "success":
    print(f"Build succeeded in {response['duration_ms']}ms")
    print(f"Files changed: {response.get('files_changed', [])}")
elif response["status"] == "timeout":
    print(f"Build timed out: {response['error']}")
elif response["status"] == "blocked":
    print(f"Build needs input: {response['prompt_detected']}")
else:
    print(f"Build failed (exit {response['exit_code']}): {response['stderr']}")
```
