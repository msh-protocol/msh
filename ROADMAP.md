9# msh Protocol Roadmap

This roadmap outlines the planned development phases for `msh`, progressing from the initial CLI prototype to a fully-featured, distributed execution engine for AI agents.

## Phase 0: The Core Engine (v0.1) — ✅ Completed
*Build the foundational execution physics and structured output.*
- [x] JSON Protocol (`ExecRequest` / `ExecResponse`)
- [x] Subprocess execution with strict timeouts
- [x] ANSI stripping and output truncation
- [x] Interactive prompt detection (`[y/N]`, passwords)
- [x] Persistent session state (CWD, Env vars)
- [x] Filesystem diffing (Merkle snapshots)
- [x] CLI entrypoint (`msh exec`)

## Phase 1: The Network Layer (v0.2) — ✅ Completed
*Move from a local CLI to a network daemon, enabling remote agents to connect.*
- [x] **`msh serve`** — HTTP daemon mode.
- [x] REST API for submitting `ExecRequest` payloads and retrieving `ExecResponse`.
- [x] Server-side session management (multi-tenant execution).
- [x] WebSocket streaming for long-running output (human observability channel).

## Phase 2: Agent Adoption & Wrappers (v0.3) — ✅ Completed
*Make it effortless for existing tools to use `msh` without rewriting their codebases.*
- [x] **`msh wrap`** — Transparently wrap existing agent processes (e.g., `msh wrap claude-code`), intercepting their raw syscalls and forcing them through the sanitization pipeline.
- [x] **MCP Server Integration** — Native support for the Model Context Protocol (MCP), making `msh` instantly discoverable by Cursor, Windsurf, and other modern AI IDEs.

## Phase 3: True Terminal Emulation (v0.4) — ✅ Completed
*Support complex CLI tools that refuse to run outside a real TTY.*
- [x] Integrate `aymanbagabas/go-pty` for cross-platform pseudo-terminal support.
- [x] Handle raw terminal mode, window size changes (SIGWINCH), and complex TUI applications.

## Phase 3.5: The Monorepo Upgrade (v0.5.0) — ✅ Completed
*Quality of life improvements for massively complex codebases.*
- [x] **Long-Running Daemons** — Allow AIs to spawn, detach, and read logs from background processes like `npm run dev` or `docker-compose up`.
- [x] **Native `.env` Injection** — Parse and inject `.env` files dynamically to avoid flooding the JSON payload with hundreds of secrets.

## Phase 4: Enterprise Scale & Ecosystem (v1.0+) — ✅ Completed
*Build the moat through observability and fleet management.*
- [x] **Hybrid Filesystem Watcher** — Seamlessly switch to `fsnotify`/`inotify` for massive monorepos where snapshots are too slow.
- [x] **Plugin/Hook System** — Allow developers to write `.msh/hooks.yaml` to run security scans or token-cost trackers on every execution.
- [x] **msh-fleet** — The enterprise control plane for managing, auditing, and securing thousands of concurrent `msh` agents across a distributed infrastructure.
