# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.3.0] - 2026-09-12

### Added
- **Official Python SDK (`packages/python`)** — Released `msh-protocol` Python client library (`from msh import exec`) with typed dataclasses, local CLI spawn, HTTP fallback, and drop-in tool integration for LangChain, CrewAI, and Claude tool calling.
- **Official TypeScript SDK (`packages/typescript`)** — Released `@msh-protocol/client` TypeScript package (`import { exec } from "@msh-protocol/client"`) for Node.js, providing typed, non-blocking execution wrappers for agent harnesses.
- **Automated CI/CD Workflows** — Added GitHub Actions multi-OS test matrix (`.github/workflows/ci.yml`) testing Linux, macOS, and Windows.
- **Automated Multi-Platform Release Pipeline** — Added `.github/workflows/release.yml` cross-compiling standalone signed binaries for 6 architectures (Linux amd64/arm64, macOS Intel/Apple Silicon, Windows amd64/arm64) with SHA256 checksums.
- **One-Line Install Scripts** — Added `scripts/install.sh` (curl-to-bash for Linux/macOS) and `scripts/install.ps1` (PowerShell for Windows).
- **Public CLI Evaluation Suite (`evals/`)** — Added automated benchmark running problematic real-world CLI patterns (ANSI color stripping, compiler dump context blowout truncation, interactive prompts, secret leakage).
- **Modern OpenAI Secret Masking** — Expanded regex pattern in `pkg/sanitize/redact.go` to capture modern project API keys (`sk-proj-...` and `sk-admin-...`).
- **Expanded Prompt Detection** — Added recognition for `npm init` confirmations, `npm install` proceed prompts, and SSH passphrase requests.

## [1.2.0] - 2026-09-12

### Added
- **Smart Command Passthrough Runner (`msh <command>`)** — Transformed `msh` into a universal command runner (analogous to `sudo`, `time`, or `uv run`). Running `msh <command> [args...]` (e.g., `msh git status`, `msh npm test`, `msh cargo check`) executes any external command directly through the deterministic runtime without requiring `wrap` or outer quotes. Features platform-accurate argument reconstruction and escaping (Windows `CommandLineToArgvW` and POSIX quoting), smart subcommand disambiguation, leading wrapper flags support (e.g., `msh --max-lines 500 git log`, `msh --timeout 1m pytest`), and exact process exit code preservation.
- **Streamed Execution History Persistence** — Integrated real-time database recording for remote `/stream/exec` runs on the fleet hub. Executions triggered from the dashboard terminal tiles are automatically persisted to `fleet.db`, making them immediately available in the History tab for inspection and replay.
- **Human-in-the-Loop (HITL) Interactive Prompting in Fleet UI** — Integrated real-time prompt interception into the multi-terminal fleet dashboard. When commands hit interactive confirmations (`[y/N]`, custom input), terminal tiles highlight with a glowing amber action banner featuring one-click `[ Yes (y) ]` and `[ No (n) ]` buttons or a custom input field. Submitting sends an `{"type":"answer","data":answer}` frame over the `/stream/exec` WebSocket tunnel directly into process stdin to unblock remote execution live.
- **One-Click Execution Replay from History** — Added an interactive daemon selector and `Re-run on Node ↻` action bar to the History tab. Operators inspecting historical `ExecRequest` records can dispatch the exact command with identical arguments to any active worker daemon with a single click, automatically navigating to the Multi-Terminal Grid and opening a live streaming session.
- **Cryptographically Secure Token Generation (`crypto/rand`)** — Introduced `protocol.GenerateToken` utilizing 128 bits of OS entropy to generate high-entropy tokens (e.g., `msh-7e2a4f...`, `node-a22009...`, `session-8b1c4e...`). Completely eliminated predictable timestamp-based tokens across `msh fleet start`, `msh serve`, and execution sessions.
- **Constant-Time Token Comparison (`crypto/subtle`)** — Introduced `protocol.SecureCompare` using `subtle.ConstantTimeCompare` across all HTTP and WebSocket authentication endpoints (`/api/*`, `/register`, `/stream/daemon`, `/stream/exec`, `/execute`), mitigating timing side-channel attacks.
- **Decoupled Daemon Node IDs & `--id` Flag** — Decoupled secret bearer tokens from public node identifiers in `msh serve` and `msh fleet`. Daemons generate unique random node IDs by default and support an explicit `--id` flag, preventing token leakage in web dashboards and avoiding registry collisions when multiple daemons share an auth token.
- **Live Fleet Metrics API & Dashboard** — Added a real-time `GET /api/metrics` endpoint on the fleet hub computing actual fleet metrics from active registrations and database execution records (total executions, active daemons, average latency, success/failure counts, reliability rate, and total compute time). Removed all mock/static placeholder data from the dashboard Metrics tab in favor of live polling.
- **Fleet Multi-Terminal Grid** — Upgraded the embedded fleet dashboard with a real-time multi-terminal grid for live observability of distributed agent swarms. Supports streaming multiple agent terminals simultaneously with independent WebSocket tunnels and buffers, flexible layouts (`Auto Grid`, `1 Column`, `2 Columns`, `3 Columns`), bulk controls (`Connect All`, `Disconnect All`, `Clear All`, `Close All`), per-tile controls (maximize/expand tile, pause, reconnect, clear buffer, close), live pulsating status badges, and log line counters. Passes authentication tokens dynamically on WebSocket connections to satisfy hub security.
- **Execution History Pagination & Replay** — Added pagination (`limit` and `offset`) to `/api/history` with `X-Total-Count` headers, allowing operators to browse through large execution logs. The fleet dashboard now includes previous/next page navigation, record counts, and expand/collapse execution inspection showing the raw `ExecRequest` and `ExecResponse` payloads for instant deterministic replay.
- **Docker & Kubernetes Engine Streaming & Policy Answering** — Upgraded Docker and Kubernetes execution engines to implement the `StreamingEngine` interface, enabling interactive prompt detection and answering within containers and pods. Added support for `.msh/prompts.yaml` reusable answer policy rules with regex matching.
- **Fleet Remote Streaming** — Added a `/stream/exec` WebSocket endpoint on the fleet hub. A remote client can now stream a command to any registered daemon: it connects to `ws://<hub>/stream/exec?token=<hub-token>&id=<daemon-id>`, sends a `start` frame carrying an `ExecRequest`, and receives live `output`/`prompt`/`result` events while relaying `answer`/`stop` frames. The hub tunnels the run to the target daemon over its existing registration connection and streams events, answers, and cancellation back — no inbound ports on the daemon required. The daemon executes with the same streaming engine as a local `/stream/exec`, so prompts can be answered mid-flight and results carry the standard `ExecResponse`. If `id` is omitted, the hub picks any registered daemon. `FleetMsg` gained `id` and `request` fields for the relay protocol; daemon tail-log writes are now serialized through a write pump shared with relayed exec events.
- **Live Streaming Exec** — New `/stream/exec` WebSocket endpoint on `msh serve`. Output streams to the client in real time (`{"type":"output"}` events) and interactive prompts can be answered mid-flight (`{"type":"prompt"}` → `{"type":"answer"}`), instead of pre-supplying all answers up front. Each run ends with a `{"type":"result"}` carrying the standard `ExecResponse`. The subprocess engine streams; docker/kubernetes engines fall back to a buffered run flushed on completion.
- **Interactive Prompt Answering** — Agents can now answer interactive prompts at runtime via `PromptAnswers` (`prompt_answers`). msh streams command output in real time, detects prompts (`[y/N]`, `password:`, etc.), and feeds the next answer to the process stdin instead of blocking. A run is only marked `blocked` if prompts remain after all answers are consumed. Exposed as the repeatable `--answer` flag (`msh exec`/`msh wrap`), the HTTP/JSON `prompt_answers` field, and the MCP `prompt_answer` option.
- **Secret Redaction** — `RedactSecrets` now masks API keys, tokens, and secrets from command output before it reaches the agent. Redaction is on by default (per command, MCP, and HTTP paths) and combines two layers: exact-value masking of environment secrets (env vars, `.env` values, session env) and shape-based masking of well-known formats (AWS access keys, GitHub tokens, Slack tokens, OpenAI/Anthropic/Stripe/Google keys, npm tokens, JWTs, private key blocks). Hook (`pre_exec`/`post_exec`) output is redacted too. Disable with `--no-redact` (CLI) or `"redact_secrets": false` (JSON/MCP).

### Fixed
- **Timing Side-Channel Vulnerabilities** — Fixed token comparison vulnerabilities across fleet hub and daemon servers by replacing variable-time string comparisons (`==`) with constant-time byte comparisons.
- **Predictable Token & Session ID Vulnerabilities** — Resolved predictability flaws (CWE-330) where administrative tokens and session IDs were generated from timestamps (`time.Now()`), allowing potential brute-force guessing.
- **Fleet WebSocket Write Race Condition** — Fixed concurrent writes to agent websocket connections during stream start/stop by routing lifecycle frames through the thread-safe `Node.SendMessage` pump.
- **Stream Goroutine & Resource Leaks** — Added context cancellation on UI client disconnects in `/stream/daemon`, preventing tailing loops from leaking memory and hanging in background.
- **Path Traversal Protection** — Hardened daemon log stream path resolution to reject directory traversal characters (`/`, `\`, `..`).

### Changed
- **Protocol** — `ExecRequest` gains `prompt_answers` (answers fed to interactive prompts, in order) and `redact_secrets` (default true when unset); `ExecResponse` gains `answers_used` (how many answers were consumed) and `redacted` (which secret names/token formats were masked).
- **`Executor`** — The shared request-prep and response-finalize stages were extracted so buffered and streaming runs produce identical results; `Engine.Run` now reports how many prompt answers were consumed.
- **Engines** — The subprocess engine streams output in real time and feeds answers when provided (both piped and PTY modes) via the new `StreamingEngine` interface; docker/kubernetes engines report consumed answers but remain buffered.

## [1.0.0] - 2026-08-07

### Added
- **`msh-fleet` (Enterprise Control Plane)** — Introduced the `msh fleet start` subcommand which spins up a centralized WebSocket registry hub (port 9000) for managing distributed `msh` daemons across your infrastructure.
- **Embedded React Dashboard** — A premium, dark-mode UI built with React and Vite is now embedded directly into the Go binary via `go:embed`. It auto-updates and displays all connected agents.
- **Reverse NAT Tunneling** — `msh serve` now connects outbound to the Fleet Hub via WebSockets, allowing the hub to tunnel `stream_start`/`stream_stop` commands and live log data through NATs and firewalls. This powers the "Live Terminal" feature in the dashboard.
- **Fleet Authentication** — Token-based security requiring `Authorization: Bearer <token>` for agent registration and UI API access.

## [0.6.0] - 2026-08-07

### Added
- **WebSocket Streaming** — Added the `/stream/daemon` WebSocket endpoint to the `msh serve` HTTP daemon. This enables real-time tailing of long-running daemon logs, paving the way for live observability dashboards. Connections authenticate via the `?token=` query parameter.

## [0.5.0] - 2026-08-07

### Added
- **Long-Running Daemons** — Introduced the `msh daemon` subcommand (`start`, `logs`, `kill`, `status`) to manage and stream logs from long-running background processes (like `npm run dev`) without blocking the execution loop.
- **Native `.env` Injection** — Added the `env_file` field to `ExecRequest` and the `--env-file` CLI flag to natively parse and inject environment variables from `.env` files before command execution.

## [0.4.0] - 2026-08-07

### Added
- **Plugin / Hook System** — Added support for `.msh/hooks.yaml` to define pre/post execution scripts (e.g. security scanners, telemetry loggers). Hooks are executed natively and their outputs are attached to the `ExecResponse` payload.
- **`msh wrap` Subcommand** — Introduced `msh wrap "command"` which executes a command through the deterministic runtime but prints the sanitized text directly to standard output/error instead of formatting it as JSON.
- **MCP Server** — Introduced `msh mcp` subcommand which starts a Model Context Protocol (MCP) server over standard I/O. This allows agents like Claude Desktop and Cursor to natively load the `execute_command` tool without custom integration code.

### Changed
- **Performance: Hybrid Filesystem Watcher** — Replaced the slow before/after full directory hashing with an `fsnotify` real-time watcher. Command execution in massive monorepos is now instantaneous while still capturing exact `files_changed` events.

## [0.3.0] - 2026-08-06

### Added
- **True Terminal Emulation (PTY)** — `msh` can now run commands inside a pseudo-terminal by passing `"use_pty": true` in the JSON payload (or `--pty` in the CLI). This allows AI agents to interact with complex CLI tools that refuse to run outside a TTY (like SSH or Docker). Note that in PTY mode, `stderr` is inherently merged into `stdout`.

## [0.2.0] - 2026-08-06

### Added
- **HTTP Daemon Mode** — `msh serve` starts an HTTP server (default port 8080) with a `POST /execute` endpoint.
- **Stateful Network Sessions** — Added `SessionID` to `ExecRequest` and `ExecResponse`. Passing a `SessionID` over HTTP maintains the current working directory and environment variables across multiple REST requests.
- **Thread-Safe Session Manager** — Added a concurrency-safe `SessionManager` that supports multi-tenant daemon execution and automatically reaps idle sessions after 30 minutes.

## [0.1.0] - 2026-08-06

### Added

- **Core Protocol** — `ExecRequest` and `ExecResponse` JSON contracts defining the machine-readable execution interface for AI agents.
- **ANSI Sanitization** — Regex-based stripping of all ANSI escape codes (CSI, OSC, 256-color, cursor movement) from command output.
- **Output Truncation** — Smart head/tail truncation that preserves the first and last N/2 lines with a `[msh: truncated X lines to save tokens]` marker. Prevents token blowouts from massive error dumps.
- **Interactive Prompt Detection** — Pattern-matching engine that detects `[y/N]`, `password:`, `Are you sure`, and other interactive prompts that would hang an agent's execution loop.
- **Timeout Enforcement** — Per-command timeout via `--timeout` flag. Commands exceeding the limit are killed and return `"status": "timeout"`.
- **Filesystem Change Detection** — Stat-based pre/post snapshots using mtime + size comparison to detect added, modified, and deleted files after each command.
- **Session State Management** — Persistent working directory tracking and environment variable layering across sequential commands within a session.
- **Cross-Platform Execution** — Automatic shell selection (`cmd /C` on Windows, `sh -c` on Unix).
- **CLI Interface** — Cobra-based CLI with `msh exec "command"` and `msh version` subcommands.
- **CLI Flags** — `--pretty`, `--cwd`, `--timeout`, `--max-lines`, `--no-files`.
- **Apache 2.0 License** — Permissive licensing with patent protection for maximum enterprise adoption.

### Technical Details

- Written in Go 1.26
- Zero external runtime dependencies (single static binary)
- 19 unit tests across protocol and sanitize packages
- Execution latency overhead: ~30ms per command


[Unreleased]: https://github.com/msh-protocol/msh/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/msh-protocol/msh/releases/tag/v0.1.0
