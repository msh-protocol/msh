# msh Protocol Use Cases

> Real-world applications and architectural patterns enabled by `msh`.

`msh` transforms the brittle, unstructured terminal into a robust API for autonomous agents. Here are the core architectures it unlocks.

---

## 1. The Headless Remote Agent (Fleet Management)
*Use Case: Running 1,000 parallel agents across a Kubernetes cluster without managing 1,000 SSH connections or PTYs.*

**The Problem:** Existing tools like Cursor and Aider assume they are running locally on the developer's laptop. Scaling an agent to the cloud usually means spinning up expensive, heavy virtual machines or dealing with SSH tunnel complexity.

**The msh Solution:** Deploy `msh serve` alongside the agent's workspace in a lightweight container. The "brain" (the LLM orchestration) can run anywhere, submitting HTTP `POST /execute` commands to the remote workspace. 
- The agent maintains its place in the directory structure using the `session_id`.
- Multiple agents can safely interact with the same workspace via distinct sessions.

## 2. Token-Optimized Build Debugging
*Use Case: An agent runs a complex build script that fails on step 40 out of 50.*

**The Problem:** `npm install` or `go build` can spit out 5,000 lines of output. If an agent tries to read this, it burns $0.50 in context window costs and gets distracted by irrelevant middle logs. If it hits an interactive prompt (`Do you want to share telemetry? [y/N]`), the agent hangs indefinitely.

**The msh Solution:** `msh` acts as a token-saving firewall.
- **Truncation:** If the output is 5,000 lines, `msh` only sends back the first 100 lines (to see what command ran) and the last 100 lines (where the actual error is), saving 4,800 lines of tokens.
- **Prompt Detection:** `msh` automatically detects the telemetry prompt, kills the process, and returns `"status": "blocked"`, prompting the agent to retry the command with the correct flags (e.g., `CI=1`).
- **ANSI Stripping:** Clears out color codes that confuse the LLM parser.

## 3. High-Fidelity Workspace Rollbacks
*Use Case: An agent modifies files to solve a bug, but realizes the approach is completely wrong and needs to revert.*

**The Problem:** Agents struggle to track exactly which files they touched across a sprawling monorepo during a 20-step execution plan.

**The msh Solution:** Because `msh` uses Merkle-style snapshots before and after every execution, the `ExecResponse` contains an exact `files_changed` array.
- The agent has a deterministic list of every file modified during its task.
- It can use this list to easily `git checkout -- <file>` or dynamically revert its changes without performing a slow, global `git status` diff parsing step.

## 4. Multi-Tenant Agent Backends
*Use Case: You are building an AI SaaS where multiple users have autonomous agents running tasks for them.*

**The Problem:** You need to isolate agent terminal executions, but spawning and managing long-lived subprocesses or WebSockets for every user is architecturally heavy.

**The msh Solution:** Run a centralized `msh serve` cluster. Agents submit commands with their unique `session_id`. `msh` manages the state routing internally, executing the commands safely, tracking the CWDs, and automatically cleaning up inactive sessions after a timeout.

## 5. Wrapping Legacy CLIs
*Use Case: An agent needs to interface with a proprietary, highly interactive internal tool.*

**The Problem:** Internal CLIs are notorious for requiring interactive inputs, throwing raw ANSI control sequences, and failing weirdly without a PTY.

**The msh Solution (Planned for v0.3):** Using `msh wrap legacy-tool`, the agent forces the legacy tool through the `msh` sanitization and structured JSON pipeline, taming it into a clean, REST-like interface that the LLM can easily consume.
