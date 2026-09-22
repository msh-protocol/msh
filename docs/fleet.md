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

<p align="center">
  <img src="assets/fleet-dashboard.png" alt="msh fleet Autonomous Orchestration & Telemetry Dashboard" width="100%" />
</p>

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

### Execution History & Real-Time Telemetry
The **History** tab provides a real-time, audit-grade log of every command executed across the fleet (the hub persists the request/response JSON of each exec).
- **KPI Summary Strip**: High-level telemetry cards tracking Total Executions, Success Rate percentage, Average Execution Latency, and Workspaces with Modified Diffs.
- **Search & Multi-Filter Toolbar**: Real-time client-side search across commands, session IDs, run hashes, and modified filenames, along with status filter pills (`All`, `Succeeded`, `Failed`, `With Diffs`, `Root Cause`).
- **Interactive Table**: Smooth rotating disclosure chevrons, relative time indicators (`Just now`, `2m ago`), origin badges (`Local` vs `Remote`), syntax-highlighted code panels, and quick command copy buttons.
- **Telemetry Export & Download Chooser**: Click **Export History** to open the export chooser modal. Supports:
  - Formats: JSON (full structured payloads and diffs), CSV (tabular metrics for Sheets/Excel), or LOG (timestamped console stream).
  - Scope: Current filtered view or full cluster execution history.
  - Destination Folder Chooser: Uses the browser's native File System Access API (`showSaveFilePicker`) to let operators pick the exact destination folder on disk and custom filename, alongside one-click clipboard copying and quick download fallback.
- **Execution Deletion & Clear History**: Remove test runs or sensitive executions using the per-row trash button (`DELETE /api/history?id=<id>`), or wipe all history using the **Clear History** header button (`DELETE /api/history?all=true`), both protected by two-step click confirmation.
- **One-Click Replay on Node**: Any historical `ExecRequest` can be immediately re-dispatched to any active daemon. Select a target daemon from the dropdown in the replay bar and click **Re-run on Node**. The dashboard seamlessly switches to the Multi-Terminal Grid, opens or focuses the live terminal tile for that daemon, and streams execution live.

`GET /api/history` accepts `limit` (default 50, max 200) and `offset` query
parameters, always newest-first, and reports the total record count in the
`X-Total-Count` response header so the UI can render page controls.
`DELETE /api/history` accepts `id=<id>` to delete a single execution record or `all=true` to clear all execution records.

### Visual Git-Style File Diff Drawer & Atomic Rollback ("Undo Changes")
When commands modify, create, or delete files in a workspace, `msh` captures the unified git-style diffs and stores them in the execution response. The Fleet UI provides a rich slide-over **Diff Drawer** for code inspection:
- **Interactive File Chips**: Every historical run that modified files displays interactive clickable chips (e.g., `src/auth.ts`, `package.json`) and an `Open Diff Drawer` action button.
- **Split (Side-by-Side) vs Unified Diff View Toggle**: Seamlessly switch between GitHub-style side-by-side split view (old file on left, new file on right with aligned lines) and unified inline diff view. The view preference is automatically remembered.
- **Surgical Single-File Rollback (`Revert File`)**: Revert changes to a specific file from the active execution without reverting other modified files. Available directly in the Diff Drawer or via the CLI:
  ```bash
  msh undo <run-id> --file <relative-file-path>
  ```
- **Atomic Workspace Rollback (`Undo Run Changes`)**: Full workspace rollback that inverts all stored unified patches in the execution, restoring modified/deleted files and removing newly created files.
- **Multi-File Tab Navigation**: Seamlessly toggle between all changed files in the execution with status badges (`A` for Added, `M` for Modified, `D` for Deleted).
- **Diff Metrics & Copy**: Displays exact addition/deletion counts per file and a one-click `Copy Diff` button for clipboard sharing.

### Cryptographic Run Verification ("Verify")
Every command executed through `msh` generates a tamper-evident SHA-256 `run_hash` fingerprint computed over command arguments, working directory, stdout/stderr, and filesystem changes.
- **One-Click Replay Verification**: Click **Verify** in the History tab (or run `msh verify <run-id>` from CLI) to re-dispatch the exact command and compare execution bit-for-bit.
- **Fidelity Scoring**: Produces an exact reproducibility score, detecting whether output or exit codes drifted due to flaky network dependencies or non-deterministic state.

### Semantic Error Root Cause Extraction
When a build or test command fails, `msh` analyzes stdout/stderr and isolates the exact root cause:
- **Language-Aware Parsers**: TypeScript (`TSxxxx`), Python (`Traceback` / `pytest`), Go compiler/tests, Rust (`error[Exxxx]`), C/C++, and shell command errors.
- **Root Cause Card**: Displays a diagnostic card in the History expanded view highlighting the exact file, line, error type, and failure message so developers and LLMs don't have to sift through hundreds of lines of log noise.

### Human-in-the-Loop (HITL) Interactive Prompting
When commands running on remote worker daemons hit interactive prompts (e.g. confirmations like `[y/N]`, password entries, or tool questions), `msh` detects the prompt and emits an `awaiting` prompt frame over the `/stream/exec` tunnel:

```json
{"type": "prompt", "prompt": "Do you want to continue? [y/N]", "awaiting": true}
```

The Fleet UI intercepts this frame mid-stream and highlights the terminal tile with a glowing amber **Human-in-the-Loop Required** banner:
- **Quick Action Buttons**: Instant one-click `[ Yes (y) ]` and `[ No (n) ]` buttons for rapid confirmation.
- **Custom Input**: Text field for arbitrary prompt responses, passwords, or custom inputs.
- **Bi-directional Pipe**: Submitting an answer transmits `{"type": "answer", "data": "..."}` over the WebSocket directly into the remote process's standard input (`stdin`), unblocking execution in real-time.

### Centralized Swarm Broadcast Command Bar
Located prominently atop the Terminal Grid, the Swarm Broadcast Command Bar enables operators to dispatch commands across all or selected daemons simultaneously:
- **Parallel Dispatch**: Select "All Daemons" or a specific worker node from the target selector; `msh` tunnels and initiates execution across all targets in parallel.
- **Quick Command Presets**: One-click quick actions for common swarm operations (`git status`, `uptime`, `whoami`, `df -h`, `docker ps`).
- **Live Visual Feedback**: Dynamic dispatching state indicators and instant streaming into all active terminal tiles.

### Shell-Grade Terminal Command History (Up / Down Arrow Recall)
Both the centralized Swarm Broadcast command bar and each per-node execution bar (`$ ...`) provide interactive shell-grade command history recall:
- **Arrow Navigation**: Press <kbd>↑</kbd> to recall previous commands in reverse chronological order; press <kbd>↓</kbd> to step forward through history.
- **Draft Preservation**: Typing a partial command and pressing <kbd>↑</kbd> temporarily preserves your typed text in a draft buffer; scrolling back down to the present restores your uncommitted draft intact.
- **Persistent Storage**: Command history is persisted across browser refreshes via `localStorage` (up to 50 recent unique commands per bar), with automatic deduplication.

### Dedicated Node Fleet Status Pill & Slide-Over Drawer
The top navigation bar features a tactile, live status indicator displaying connected worker daemons:
- **Status Pill (`🟢 X Nodes Online`)**: Displays total active worker daemons connected to the hub. Clicking the pill (or pressing <kbd>n</kbd>) opens the slide-over **Node Fleet Drawer**.
- **Worker Daemon Details**: Inspect connected worker nodes with hostname, operating system (`windows`, `linux`, `darwin`), CPU architecture (`amd64`, `arm64`), unique daemon ID, and relative connection uptime ticker (`Connected 12m ago`).
- **One-Click Roundtrip Latency Ping**: Each node card features an interactive **Test Ping** button. Calling `/api/nodes/ping?id=<daemon-id>` dispatches a low-level WebSocket ping/pong control frame to measure exact roundtrip network latency (e.g. `0ms · Operational`), verifying daemon responsiveness without disturbing running processes.

### Global Keyboard Navigation & Cheatsheet Modal (`?`)
The Fleet UI is fully navigable via cluster-wide hotkeys:
- <kbd>1</kbd> — Switch to **Live Fleet** terminal grid
- <kbd>2</kbd> — Switch to **Execution History** audit log
- <kbd>3</kbd> — Switch to **Cluster Metrics** analytics
- <kbd>/</kbd> — Instantly focus the **Swarm Broadcast** command input (or search bar in History)
- <kbd>r</kbd> — Refresh telemetry, node list, and history records
- <kbd>n</kbd> — Toggle the **Connected Nodes Fleet Drawer**
- <kbd>?</kbd> — Open the **Keyboard Shortcuts Cheatsheet Modal**
- <kbd>Esc</kbd> — Dismiss open drawers, modals, or active focus

*Note: Navigation hotkeys are automatically bypassed when typing in text inputs or textareas.*

### In-Terminal Search & Log Inspection Controls
Each terminal tile in the multi-node grid provides precision controls for high-density debugging:
- **In-Terminal Log Keyword Filtering**: Click the **Filter** button to reveal an inline search bar; filters terminal output lines in real-time with match counting and zero-result diagnostics.
- **Follow Logs / Scroll Lock ("Tail" vs "Hold")**: Toggle auto-scroll behavior on and off per tile. When held, operators can freely scroll up to inspect previous logs without the viewport snapping to the bottom when new stream chunks arrive.
- **Log Export & Clipboard Sharing**: One-click **Copy** exports the clean terminal log buffer directly to the system clipboard; the **Download** button saves the full session output as a `<daemon-id>-logs.txt` file.

### Advanced Telemetry & Metrics Analytics
The **Metrics** tab delivers real-time cluster telemetry and diagnostic intelligence computed across all fleet activity via `GET /api/metrics`:
- **Interactive SVG Latency Sparkline**: Continuous real-time latency plot across the last 30 executions with area fill gradient, interactive data points (Sage Moss for success, Terracotta Red for failure), and hover tooltips showing command, latency in milliseconds, timestamp, and status.
- **Top Commands Leaderboard**: Ranked table of the top 5 most frequently executed operations with invocation counts, average latency, and visual success-rate progress meters.
- **Reliability & Error Distribution**: Categorized failure breakdown tracking exit codes, timeouts, and execution errors, or a verified zero-failure clean state badge when running 100% reliably.
- **Cluster Footprint & Capacity Matrix**: Telemetry tracking worker node allocation, aggregate job counts, system availability, and pipeline status.

### Vintage Literary Print & Muted Editorial Design System
Designed after classical letterpress publishing and fine editorial print:
- **Zero Harsh Neons**: Replaced generic blues, neon greens, and bright orange with Aged Book Paper (`#f6f3eb`, `#fdfcf9`), Walnut Ink (`#5c3a2e`), Soft Lampblack (`#231f1d`), Book Margin Rules (`#ded7c7`), and Aged Library Sage (`#385a49`).
- **Warm Obsidian Terminal Surfaces**: Terminal tiles rendered in deep espresso obsidian (`#181614`) with warm window controls and terminal logo (`>_`).
- **Refined Typography**: Editorial serif headings (`Fraunces`) paired with geometric sans (`Plus Jakarta Sans`) and code monospace (`JetBrains Mono`).
- **Pill Navigation Switcher**: Floating pill navbar tab switcher with dark espresso active indicator and tactile micro-animations.

### Visual Diff Drawer & Binary Asset Safeguards
Operators inspecting command history can launch the slide-over **File Revisions & Diffs** drawer:
- **Unified & Side-by-Side Split Diff Views**: Toggle between inline unified diffs and GitHub-style dual-column side-by-side views with aligned line numbers and persistent preferences.
- **Surgical Single-File Rollback**: Revert individual modified files directly from the diff view without affecting other files in the execution.
- **Binary Asset Protection**: Compiled payloads (e.g. `msh.exe`, `.dll`, `.so`, `.bin`) are presented on dedicated **Binary Specimen Cards** with informational callouts explaining that non-textual machine payloads cannot be reverse-patched. Rollback buttons are safely disabled for binary artifacts, and multi-file rollbacks automatically skip binary files so that source code changes are restored cleanly.
- **In-Drawer Alert Banners**: Error and success feedback renders directly inside the drawer as animated notification banners, eliminating blocking browser `alert()` popups.

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
