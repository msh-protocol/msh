# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **Interactive Prompt Answering** — Agents can now answer interactive prompts at runtime via `PromptAnswers` (`prompt_answers`). msh streams command output in real time, detects prompts (`[y/N]`, `password:`, etc.), and feeds the next answer to the process stdin instead of blocking. A run is only marked `blocked` if prompts remain after all answers are consumed. Exposed as the repeatable `--answer` flag (`msh exec`/`msh wrap`), the HTTP/JSON `prompt_answers` field, and the MCP `prompt_answer` option.
- **Secret Redaction** — `RedactSecrets` now masks API keys, tokens, and secrets from command output before it reaches the agent. Redaction is on by default (per command, MCP, and HTTP paths) and combines two layers: exact-value masking of environment secrets (env vars, `.env` values, session env) and shape-based masking of well-known formats (AWS access keys, GitHub tokens, Slack tokens, OpenAI/Anthropic/Stripe/Google keys, npm tokens, JWTs, private key blocks). Hook (`pre_exec`/`post_exec`) output is redacted too. Disable with `--no-redact` (CLI) or `"redact_secrets": false` (JSON/MCP).

### Changed
- **Protocol** — `ExecRequest` gains `prompt_answers` (answers fed to interactive prompts, in order) and `redact_secrets` (default true when unset); `ExecResponse` gains `answers_used` (how many answers were consumed) and `redacted` (which secret names/token formats were masked).
- **Engines** — `Engine.Run` now also reports how many prompt answers were consumed; the subprocess engine streams output in real time when answers are provided (both piped and PTY modes).

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
