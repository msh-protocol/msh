# Getting Started with msh

> Set up and run msh in under 2 minutes.

## Prerequisites

- [Go 1.21+](https://go.dev/dl/) installed

## Installation

### From Source

```bash
git clone https://github.com/msh-protocol/msh.git
cd msh
go build ./cmd/msh/
```

### Via `go install`

```bash
go install github.com/msh-protocol/msh/cmd/msh@latest
```

## Quick Start

### Execute a command

```bash
msh exec "echo hello world"
```

Output:
```json
{"status":"success","exit_code":0,"cwd":"/your/current/dir","stdout":"hello world","stderr":"","truncated":false,"duration_ms":30}
```

### Pretty-print the JSON

```bash
msh exec "ls -la" --pretty
```

Output:
```json
{
  "status": "success",
  "exit_code": 0,
  "cwd": "/your/current/dir",
  "stdout": "total 48\ndrwxr-xr-x  6 user staff  192 Aug  6 09:00 .\n...",
  "stderr": "",
  "truncated": false,
  "duration_ms": 25
}
```

### Set a timeout

```bash
msh exec "npm run build" --timeout 60s
```

If the command takes longer than 60 seconds, msh kills it and returns:
```json
{
  "status": "timeout",
  "exit_code": -1,
  "error": "command timed out after 1m0s"
}
```

### Limit output lines (save tokens)

```bash
msh exec "cat huge_log.txt" --max-lines 50
```

If the output exceeds 50 lines, msh keeps the first 25 and last 25 lines:
```
First 25 lines...
... [msh: truncated 9950 lines to save tokens] ...
Last 25 lines...
```

### Skip filesystem detection

```bash
msh exec "echo fast" --no-files
```

Skips the pre/post filesystem snapshot, making execution faster for commands that don't modify files.

## Running as an HTTP Daemon

You can run `msh` as a background server to accept execution requests over the network. This is ideal for distributed agent architectures.

```bash
msh serve --port 8080
```

### POST `/execute`

Send a JSON `ExecRequest` to execute commands. If you provide a `session_id`, `msh` will persist state (like your working directory) across multiple requests.

```bash
curl -X POST http://127.0.0.1:8080/execute \
  -H "Content-Type: application/json" \
  -d '{
    "command": "npm run build",
    "session_id": "my-agent-session",
    "timeout": "60s"
  }'
```

### GET `/health`

Check if the daemon is running.

```bash
curl http://127.0.0.1:8080/health
```

## CLI Reference

```
msh exec "command" [flags]

Flags:
  --cwd string        Override working directory
  --timeout string    Execution timeout (default "30s")
  --max-lines int     Maximum output lines (default 200)
  --pretty            Pretty-print JSON output
  --no-files          Skip filesystem change detection

msh serve [flags]

Flags:
  --port int          Port to listen on (default 8080)
  --host string       Host IP to bind to (default 127.0.0.1)

msh version           Print version information
```

## What's Next

- Read the [Protocol Specification](protocol-spec.md) for the full JSON contract
- Read the [Architecture Guide](architecture.md) for internals
