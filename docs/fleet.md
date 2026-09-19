# msh-fleet: Enterprise Control Plane

The `msh-fleet` sub-system is the centralized command-and-control registry for managing distributed `msh` agent daemons. It enables secure, real-time observability and NAT-bypassing terminal streaming for thousands of concurrent AI agents.

## Architecture

The system consists of two primary components:
1. **The Fleet Hub (`msh fleet start`)**: A lightweight, centralized WebSocket server and UI dashboard. It acts as the registry for all daemons.
2. **The Daemons (`msh serve --fleet <url>`)**: The worker nodes where AI agents execute commands. They connect *outbound* to the Fleet Hub, bypassing inbound firewalls and NATs.

## Quick Start

### 1. Start the Fleet Hub
Run the following command on your central server (or locally):
```bash
msh fleet start
```
*Outputs:*
```
[msh-fleet] Generated Admin Token: msh-7e2a4f0b9c1d8e3a5f7b2c0e1d4a6b8c
msh fleet hub listening on http://127.0.0.1:9000
```
*Keep this token safe! It is a cryptographically secure 128-bit token required to connect daemons and access the dashboard.*

### 2. Connect a Daemon
On your worker machines, start the `msh` daemon and point it to your Fleet Hub using the generated token (and an optional unique `--id`):
```bash
msh serve --port 8080 --fleet ws://127.0.0.1:9000 --token msh-7e2a4f0b9c1d8e3a5f7b2c0e1d4a6b8c --id worker-node-1
```
The daemon will automatically register itself with the hub and establish a persistent, bidirectional WebSocket tunnel. Node IDs are decoupled from auth tokens, ensuring credentials are never exposed on dashboard screens or public endpoints.

### 3. Access the Dashboard
Open `http://127.0.0.1:9000` in your browser. 
You will be prompted for your Admin Token. Once authenticated, you will see a real-time list of all connected daemons.

## Features

### Reverse NAT Tunneling
Because daemons connect *outbound* to the Fleet Hub via WebSockets, you do not need to open inbound ports on your worker machines. The Fleet Hub can route commands and stream logs back to the dashboard through this established tunnel.

### Live Terminal Streaming & Multi-Terminal Grid
Click the **Live Terminal** button on any connected node in the dashboard. The Fleet Hub instructs the daemon to tail its execution logs (`.msh/daemons/<id>.log`) and stream them directly to your browser with zero latency.

The dashboard features a **Multi-Terminal Grid** allowing operators to supervise swarms of daemons simultaneously:
- **Side-by-Side Streaming**: Stream multiple agent terminals side-by-side in real-time, each with its own independent WebSocket connection and buffer.
- **Flexible Layouts**: Toggle between `Auto Grid`, `1 Column` (stacked wide views), `2 Columns`, or `3 Columns` to match your display configuration.
- **Bulk Fleet Controls**: Use **Connect All** to instantly attach live streams for every registered daemon, or **Disconnect All** / **Clear All** / **Close All** to manage open streams at scale.
- **Tile Controls**:
  - **Expand / Restore**: Maximize any terminal tile to fill the entire viewport for deep inspection, then restore back to the multi-node grid.
  - **Pause & Reconnect**: Pause streaming without losing existing logs, or reconnect to a live session with a single click.
  - **Buffer Clear & Close**: Clear output buffers individually or dismiss tiles.
  - **Live Indicators**: Real-time pulsing status badges and log line counters for each agent.

### Execution History & One-Click Replay
The **History** tab shows a real-time log of every command executed across the
fleet (the hub persists the request/response JSON of each exec). The list is
paginated (`/api/history?limit=&offset=`), and clicking a row expands a **replay**
view showing the full stored request and response payloads.

- **One-Click Replay on Node**: Any historical `ExecRequest` can be immediately re-dispatched to any active daemon. Select a target daemon from the dropdown in the replay bar and click **Re-run on Node ↻**. The dashboard seamlessly switches to the Multi-Terminal Grid, opens or focuses the live terminal tile for that daemon, and streams execution live.

`GET /api/history` accepts `limit` (default 50, max 200) and `offset` query
parameters, always newest-first, and reports the total record count in the
`X-Total-Count` response header so the UI can render page controls.

### Visual Git-Style File Diff Drawer & Atomic Rollback ("Undo Changes ↺")
When commands modify, create, or delete files in a workspace, `msh` captures the unified git-style diffs and stores them in the execution response. The Fleet UI provides a rich slide-over **Diff Drawer** for code inspection:
- **Interactive File Chips**: Every historical run that modified files displays interactive clickable chips (e.g., `📄 src/auth.ts`, `📄 package.json`) and an `Open Diff Drawer ⎘` action button.
- **Unified Git Diff Viewer**: Full dual-gutter line numbering (old vs new), syntax-highlighted additions (`+` green), deletions (`-` red), and hunk headers (`@@` indigo).
- **Atomic Workspace Rollback (`Undo Changes ↺`)**: One-click surgical rollback in the dashboard or via `msh undo <run-id>` CLI. Inverts the stored unified patch, restores modified/deleted files, and removes created files without affecting other changes.
- **Multi-File Tab Navigation**: Seamlessly toggle between all changed files in the execution with status badges (`A` for Added, `M` for Modified, `D` for Deleted).
- **Diff Metrics & Copy**: Displays exact addition/deletion counts per file and a one-click `Copy Diff` button for clipboard sharing.

### Cryptographic Run Verification ("Verify ⛨")
Every command executed through `msh` generates a tamper-evident SHA-256 `run_hash` fingerprint computed over command arguments, working directory, stdout/stderr, and filesystem changes.
- **One-Click Replay Verification**: Click **Verify ⛨** in the History tab (or run `msh verify <run-id>` from CLI) to re-dispatch the exact command and compare execution bit-for-bit.
- **Fidelity Scoring**: Produces an exact reproducibility score, detecting whether output or exit codes drifted due to flaky network dependencies or non-deterministic state.

### Semantic Error Root Cause Extraction
When a build or test command fails, `msh` analyzes stdout/stderr and isolates the exact root cause:
- **Language-Aware Parsers**: TypeScript (`TSxxxx`), Python (`Traceback` / `pytest`), Go compiler/tests, Rust (`error[Exxxx]`), C/C++, and shell command errors.
- **Root Cause Card**: Displays an amber/red diagnostic card in the History expanded view highlighting the exact file, line, error type, and failure message so developers and LLMs don't have to sift through hundreds of lines of log noise.

### Human-in-the-Loop (HITL) Interactive Prompting
When commands running on remote worker daemons hit interactive prompts (e.g. confirmations like `[y/N]`, password entries, or tool questions), `msh` detects the prompt and emits an `awaiting` prompt frame over the `/stream/exec` tunnel:

```json
{"type": "prompt", "prompt": "Do you want to continue? [y/N]", "awaiting": true}
```

The Fleet UI intercepts this frame mid-stream and highlights the terminal tile with a glowing amber **Human-in-the-Loop Required** banner:
- **Quick Action Buttons**: Instant one-click `[ Yes (y) ]` and `[ No (n) ]` buttons for rapid confirmation.
- **Custom Input**: Text field for arbitrary prompt responses, passwords, or custom inputs.
- **Bi-directional Pipe**: Submitting an answer transmits `{"type": "answer", "data": "..."}` over the WebSocket directly into the remote process's standard input (`stdin`), unblocking execution in real-time.

### Live Metrics & Analytics
The **Metrics** tab displays live performance and fleet health metrics computed directly from active registrations and the execution database via `GET /api/metrics`:
- **Total Executions**: Real execution count with success vs error tallies.
- **Average Latency**: Computed average execution duration in milliseconds.
- **Active Daemons**: Dynamic count of currently connected worker daemons.
- **Success Rate**: Live reliability percentage and failure counters.
- **Fleet Compute Overview**: Total compute time elapsed across all runs.

### Remote Streaming Exec
The hub exposes `/stream/exec`, so a client can run a command live on any
registered daemon without inbound ports:

```
ws://127.0.0.1:9000/stream/exec?token=<hub-token>&id=<daemon-id>
```

Send the usual streaming frames: `start` (with an `ExecRequest`), receive
`output`/`prompt` events, reply with `answer`, and cancel with `stop`. The hub
tunnels the run to the daemon and relays events back. Omit `id` to use any
registered daemon. See `docs/protocol-spec.md` for the full frame format.

### Secure by Default
- **Cryptographically Secure Entropy**: Admin and worker tokens are generated with 128-bit OS entropy from `crypto/rand`, preventing token predictability and brute-force attacks.
- **Timing Attack Mitigation**: All token and credential comparisons use `crypto/subtle.ConstantTimeCompare`, eliminating timing side-channel leaks.
- **Decoupled Identity & Credentials**: Node identifiers (`ID`) are fully decoupled from secret authorization tokens, preventing authentication credentials from leaking via dashboard UIs or public metrics.
- **Path Traversal Protection**: Daemon log streaming endpoints reject directory traversal characters (`/`, `\`, `..`).
- **Resource & Goroutine Leak Protection**: Bidirectional streaming channels monitor client disconnection with context cancellation, terminating background log tailers immediately upon client exit.
- **Strict Origin & Cache Policies**: Embedded UI assets are served directly from the single Go binary with strict Cache-Control headers and managed CORS headers.
