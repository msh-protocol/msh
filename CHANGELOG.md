# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **Plugin / Hook System** — Added support for `.msh/hooks.yaml` to define pre/post execution scripts (e.g. security scanners, telemetry loggers). Hooks are executed natively and their outputs are attached to the `ExecResponse` payload.
- **`msh wrap` Subcommand** — Introduced `msh wrap "command"` which executes a command through the deterministic runtime but prints the sanitized text directly to standard output/error instead of formatting it as JSON.
- **MCP Server** — Introduced `msh mcp` subcommand which starts a Model Context Protocol (MCP) server over standard I/O. This allows agents like Claude Desktop and Cursor to natively load the `execute_command` tool without custom integration code.

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
