# @msh-protocol/client

[![npm version](https://img.shields.io/npm/v/@msh-protocol/client.svg)](https://www.npmjs.com/package/@msh-protocol/client)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

**`msh: sudo for AI Agents`** — TypeScript SDK for safe, structured, non-blocking command execution.

Protects AI agent harnesses and LLM tool chains from:
- ❌ **Hanging prompts** (`[y/N]`, password, SSH passphrases)
- ❌ **Context blowouts** (10,000-line build dumps)
- ❌ **ANSI escape code garbage**
- ❌ **Secret leaks** (AWS keys, GitHub PATs, OpenAI keys, RSA private keys)

---

## Installation

```bash
npm install @msh-protocol/client
```

*Prerequisite: Ensure `msh` CLI is installed on the host.*

---

## Quickstart

```typescript
import { exec } from '@msh-protocol/client';

const res = await exec('git status', { maxOutputLines: 200 });

if (res.status === 'success') {
  console.log('Output:', res.stdout);
} else if (res.status === 'blocked') {
  console.warn('Interactive prompt detected:', res.prompt_detected);
} else {
  console.error(`Process failed with exit code ${res.exit_code}:`, res.stderr);
}
```

---

## Response Structure (`ExecResponse`)

```typescript
interface ExecResponse {
  session_id: string;
  status: 'success' | 'error' | 'blocked';
  exit_code: number;
  cwd: string;
  stdout: string;
  stderr: string;
  truncated: boolean;
  prompt_detected?: string;
  answers_used?: number;
  redacted?: string[];
}
```

---

## License

Apache 2.0. Maintained by [msh-protocol](https://github.com/msh-protocol/msh).
