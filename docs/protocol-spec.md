# msh Protocol Specification v0.1

> The machine-readable execution contract for AI agents.

This document defines the JSON protocol that AI agents use to communicate with the msh runtime. Any client implementing this specification can execute commands through msh and receive structured results.

## ExecRequest

An agent sends an `ExecRequest` to execute a command.

```json
{
  "session_id": "agent-123",
  "command": "npm run build",
  "cwd": "/workspace/app",
  "env": {
    "NODE_ENV": "production",
    "SUPER_API_KEY": "gkx_secretvalue123"
  },
  "env_file": "/workspace/app/.env",
  "timeout": 60000000000,
  "max_output_lines": 100,
  "detect_files": true,
  "use_pty": false,
  "redact_secrets": true,
  "prompt_answers": ["y"]
}
```

### Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `command` | string | ✅ | — | Shell command to execute |
| `session_id` | string | ❌ | generated | Session ID to persist CWD/env across requests |
| `cwd` | string | ❌ | Session CWD | Working directory override |
| `env` | object | ❌ | `{}` | Additional environment variables |
| `env_file` | string | ❌ | — | Path to a `.env` file to load before execution |
| `timeout` | integer | ❌ | `30s` | Max execution time (nanoseconds) |
| `max_output_lines` | integer | ❌ | `200` | Output truncation limit |
| `detect_files` | boolean | ❌ | `false` | Enable filesystem change detection |
| `use_pty` | boolean | ❌ | `false` | Run command inside a pseudo-terminal |
| `redact_secrets` | boolean | ❌ | `true` | Mask secrets in output (see Secret Redaction) |
| `prompt_answers` | string[] | ❌ | `[]` | Answers fed to interactive prompts at runtime, in order (e.g. `["y"]` to accept a `[y/N]` prompt) |
| `engine` | string | ❌ | `subprocess` | Execution engine: `subprocess`, `docker`, `kubernetes` |
| `docker_image` | string | ❌ | — | Image to use when `engine: "docker"` or `"kubernetes"` |
| `kubernetes_namespace` | string | ❌ | — | Namespace to use when `engine: "kubernetes"` |

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
  "redacted": ["SUPER_API_KEY"],
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
| `redacted` | string[] | ❌ | Secret names/token formats masked from output |
| `files_changed` | string[] | ❌ | Files added/modified/deleted (only when `detect_files: true`) |
| `duration_ms` | integer | ✅ | Wall-clock execution time in milliseconds |
| `prompt_detected` | string | ❌ | Interactive prompt text (only when `status: "blocked"`) |
| `answers_used` | integer | ❌ | Number of `prompt_answers` consumed before execution completed |
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

When `prompt_answers` are provided, the subprocess engine streams output in real time and feeds the next answer (plus a newline) to the process stdin the moment a prompt is detected — including prompts with no trailing newline. The run is only marked `blocked` if a prompt remains after all answers are consumed; each consumed answer is reported via `answers_used`. Only the subprocess engine answers prompts; docker/kubernetes engines report them but cannot feed input.

4. **Secret Redaction** — Secrets are masked from output before it reaches the agent:
   - **Exact-value masking** — the concrete values of environment secrets are replaced with `[REDACTED:<name>]`. Only variables whose names look sensitive (`TOKEN`, `KEY`, `SECRET`, `PASSWORD`, `PASS`, `CREDENTIAL`, `PRIVATE`, `AUTH`) are matched, and values shorter than 4 characters or containing whitespace are ignored to avoid corrupting ordinary text.
   - **Shape-based masking** — well-known token formats are masked even without a matching env var: AWS access keys, GitHub tokens (`ghp_*`, `github_pat_*`), Slack tokens (`xox*`), OpenAI/Anthropic/Stripe/Google/npm keys, JWTs, and PEM private key blocks.
   - Hook (`pre_exec`/`post_exec`) stdout/stderr is redacted as well.
   - The masked names/kinds are returned in `response.redacted`. Redaction is on by default and can be disabled per request with `"redact_secrets": false` (or `--no-redact` in the CLI).

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
    ["msh", "exec", "npm install --yes", "--answer", "y"],
    capture_output=True, text=True
)

response = json.loads(result.stdout)

if response["status"] == "success":
    print(f"Build succeeded in {response['duration_ms']}ms")
    print(f"Answers consumed: {response.get('answers_used', 0)}")
    print(f"Files changed: {response.get('files_changed', [])}")
elif response["status"] == "timeout":
    print(f"Build timed out: {response['error']}")
elif response["status"] == "blocked":
    print(f"Build needs input: {response['prompt_detected']}")
else:
    print(f"Build failed (exit {response['exit_code']}): {response['stderr']}")
```
