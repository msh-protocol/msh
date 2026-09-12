# msh-protocol: Python SDK

[![PyPI version](https://img.shields.io/pypi/v/msh-protocol.svg)](https://pypi.org/project/msh-protocol/)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

**`msh: sudo for AI Agents`** — Python client library for safe, structured, non-blocking command execution.

Protects LLMs and AI agent harnesses from:
- ❌ **Hanging prompts** (`[y/N]`, password, SSH passphrases)
- ❌ **Context blowouts** (10,000-line compiler dumps)
- ❌ **ANSI escape code corruption**
- ❌ **Secret leaks** (AWS keys, GitHub PATs, OpenAI keys, RSA private keys)

---

## Installation

```bash
pip install msh-protocol
```

*Prerequisite: Ensure `msh` CLI is installed on the host (`curl -fsSL https://raw.githubusercontent.com/msh-protocol/msh/main/scripts/install.sh | bash`).*

---

## Quickstart

```python
from msh import exec

# Run any command with deterministic output truncation & secret redaction
res = exec("git status", max_output_lines=200)

if res.ok:
    print(res.stdout)
elif res.status == "blocked":
    print(f"Command paused for user confirmation: {res.prompt_detected}")
else:
    print(f"Command failed (exit code {res.exit_code}): {res.stderr}")
```

---

## LangChain / Claude Tool Integration

Drop into any agent harness in 15 lines:

```python
from msh import exec

def safe_shell(command: str) -> str:
    """Execute shell commands safely without hanging or blowing token context."""
    res = exec(command, max_output_lines=300)
    if not res.ok:
        return f"Error (exit code {res.exit_code}):\n{res.stderr or res.stdout}"
    return res.stdout
```

---

## Response Structure (`ExecResponse`)

| Field | Type | Description |
| :--- | :--- | :--- |
| `status` | `str` | `"success"`, `"error"`, or `"blocked"` (interactive prompt detected) |
| `exit_code` | `int` | Exact process exit code (preserves exit fidelity) |
| `stdout` | `str` | Clean, ANSI-stripped stdout |
| `stderr` | `str` | Clean, ANSI-stripped stderr |
| `truncated` | `bool` | `True` if output exceeded `max_output_lines` limit |
| `prompt_detected` | `str?` | Intercepted prompt text if process required input |
| `ok` | `bool` | Convenience helper: `True` if `exit_code == 0` and `status == "success"` |

---

## License

Apache 2.0. Maintained by [msh-protocol](https://github.com/msh-protocol/msh).
