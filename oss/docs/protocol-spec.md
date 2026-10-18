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
| `file_diffs` | map[string]string | ❌ | Map of relative file paths to unified git-style diffs |
| `error_root_cause` | object | ❌ | Isolated compiler/runtime root cause (type, message, file, line) |
| `run_hash` | string | ✅ | Cryptographic SHA-256 fingerprint for deterministic verification |
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

When `prompt_answers` are provided, the subprocess engine streams output in real time and feeds the next answer (plus a newline) to the process stdin the moment a prompt is detected — including prompts with no trailing newline. The run is only marked `blocked` if a prompt remains after all answers are consumed; each consumed answer is reported via `answers_used`. The docker engine does the same: `docker run -i` attaches the container stdin, so prompts inside the container are answered through the same streaming path and `answers_used` is reported. Kubernetes answers prompts via its PTY path.

A workspace can also commit **reusable answers** in `.msh/prompts.yaml`:

```yaml
prompts:
  - match: "(?i)install.*\\[y/N\\]"
    answer: "n"
```

Each `match` is a regex tested against the detected prompt text; the first match feeds the answer whenever no explicit `prompt_answers`/`--answer` was supplied for it. Explicit per-request answers take precedence, then the policy file, then live answers (streaming clients), then the run is reported waiting.

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

## Live Streaming Exec (`/stream/exec`)

For long-running or interactive commands, msh offers a WebSocket endpoint that
streams output in real time and lets an agent answer prompts mid-flight.

Endpoint: `ws://<host>/stream/exec` (authenticate with `?token=...` or a
`Authorization: Bearer <token>` header).

Message flow, one per JSON frame:

1. **start** (client → server) — begins a run. Contains the same ExecRequest as
   `/execute`:

   ```json
   {"type":"start","request":{"command":"npm i --yes","max_output_lines":100}}
   ```

2. **output** (server → client) — raw chunk as it is read from the process:

   ```json
   {"type":"output","stream":"stdout","data":"building...\n"}
   ```

   `stream` is `"stdout"`, `"stderr"`, or `"pty"` (PTY merges both).

3. **prompt** (server → client, interactive runs only) — a prompt was detected.
   `awaiting: true` means no answer is available yet and the run is waiting for
   one; `awaiting: false` reports an answer was just fed.

   ```json
   {"type":"prompt","prompt":"Do you want to continue? [y/N]","awaiting":true}
   ```

4. **answer** (client → server) — reply to a pending prompt. Fed to the process
   stdin with a trailing newline:

   ```json
   {"type":"answer","data":"y"}
   ```

   Pre-supplied `prompt_answers` in the start request are consumed first; only
   when they run out does the server wait on `answer` messages.

5. **result** (server → client) — final structured ExecResponse:

   ```json
   {"type":"result","response":{"status":"success","exit_code":0,...}}
   ```

6. **stop** (client → server) — cancels the run (kills the process).

Client disconnects also cancel the run. If the client falls too far behind,
output chunks are dropped (sliding window) so a slow consumer cannot stall
command execution; the final `result` always carries the complete sanitized
output.

### Remote (`/stream/exec` on the fleet hub)

The fleet hub exposes the same protocol at `ws://<hub>/stream/exec` so a client
can run a command on a registered daemon through its outbound tunnel:

- Authenticate with `?token=<hub-token>` (or `Authorization: Bearer <hub-token>`).
- Select a target with `?id=<daemon-id>` (per `/api/nodes`); if omitted the hub
  picks any registered daemon.
- The message flow is identical to a local `/stream/exec`: send `start`, receive
  `output`/`prompt`/`result`, send `answer`/`stop`. The hub relays each frame
  over the daemon's registration tunnel, so no inbound ports are needed on the
  worker. `answer` and `stop` are forwarded to the live run; a client disconnect
  cancels it.

Internally, relayed runs use `FleetMsg` frames on the hub↔daemon tunnel: the hub
sends `exec_start` (with the `request`), `exec_answer`, and `exec_stop`; the
daemon answers with `exec_output`/`exec_prompt`/`exec_result`/`exec_error`,
each carrying the client-facing event JSON in `data`.
