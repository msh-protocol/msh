# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Released]

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
