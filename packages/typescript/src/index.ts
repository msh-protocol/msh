declare const process: any;
declare function require(module: string): any;

const { spawn } = require('child_process');

export interface ExecOptions {
  cwd?: string;
  timeout?: string;
  maxOutputLines?: number;
  promptAnswers?: string[];
  usePty?: boolean;
  env?: Record<string, string>;
  sessionId?: string;
}

export interface HookResult {
  name: string;
  type: string;
  command: string;
  stdout?: string;
  stderr?: string;
  exit_code: number;
}

export interface ExecResponse {
  status: 'success' | 'error' | 'timeout' | 'blocked';
  exit_code: number;
  cwd: string;
  stdout: string;
  stderr: string;
  truncated: boolean;
  redacted: string[];
  files_changed: string[];
  hooks?: HookResult[];
  answers_used?: number;
  duration_ms: number;
  prompt_detected?: string;
  session_id?: string;
  error?: string;
}

export class MshClient {
  private baseUrl?: string;
  private token?: string;

  constructor(options?: { baseUrl?: string; token?: string }) {
    this.baseUrl = options?.baseUrl?.replace(/\/+$/, '');
    this.token = options?.token;
  }

  async exec(command: string, options?: ExecOptions): Promise<ExecResponse> {
    if (this.baseUrl) {
      return this.execHttp(command, options);
    }
    return this.execCli(command, options);
  }

  private async execCli(command: string, options?: ExecOptions): Promise<ExecResponse> {
    const args = ['exec', command];
    if (options?.cwd) args.push('--cwd', options.cwd);
    if (options?.timeout) args.push('--timeout', options.timeout);
    if (options?.maxOutputLines !== undefined) args.push('--max-lines', String(options.maxOutputLines));
    if (options?.usePty) args.push('--pty');
    if (options?.promptAnswers) {
      for (const ans of options.promptAnswers) {
        args.push('--answer', ans);
      }
    }

    const bin = process.platform === 'win32' ? 'msh.exe' : 'msh';
    return new Promise((resolve) => {
      const proc = spawn(bin, args, {
        shell: false,
        env: { ...process.env, ...options?.env },
      });

      let stdout = '';
      let stderr = '';

      proc.stdout?.on('data', (d: any) => {
        stdout += d.toString();
      });

      proc.stderr?.on('data', (d: any) => {
        stderr += d.toString();
      });

      proc.on('close', () => {
        const raw = stdout.trim() || stderr.trim();
        try {
          const parsed = JSON.parse(raw);
          resolve(parsed);
        } catch (err: any) {
          resolve({
            status: 'error',
            exit_code: -1,
            cwd: options?.cwd || process.cwd(),
            stdout,
            stderr: stderr || err.message,
            truncated: false,
            redacted: [],
            files_changed: [],
            duration_ms: 0,
            error: err.message,
          });
        }
      });

      proc.on('error', (err: any) => {
        resolve({
          status: 'error',
          exit_code: -1,
          cwd: options?.cwd || process.cwd(),
          stdout: '',
          stderr: err.message,
          truncated: false,
          redacted: [],
          files_changed: [],
          duration_ms: 0,
          error: err.message,
        });
      });
    });
  }

  private async execHttp(command: string, options?: ExecOptions): Promise<ExecResponse> {
    const url = `${this.baseUrl}/execute`;
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    };
    if (this.token) {
      headers['Authorization'] = `Bearer ${this.token}`;
    }

    const payload: Record<string, any> = {
      command,
      max_output_lines: options?.maxOutputLines ?? 500,
      use_pty: options?.usePty ?? false,
    };
    if (options?.cwd) payload.cwd = options.cwd;
    if (options?.timeout) payload.timeout = options.timeout;
    if (options?.promptAnswers) payload.prompt_answers = options.promptAnswers;
    if (options?.env) payload.env = options.env;
    if (options?.sessionId) payload.session_id = options.sessionId;

    try {
      const res = await fetch(url, {
        method: 'POST',
        headers,
        body: JSON.stringify(payload),
      });
      return (await res.json()) as ExecResponse;
    } catch (err: any) {
      return {
        status: 'error',
        exit_code: -1,
        cwd: options?.cwd || '',
        stdout: '',
        stderr: err.message,
        truncated: false,
        redacted: [],
        files_changed: [],
        duration_ms: 0,
        error: err.message,
      };
    }
  }
}

const defaultClient = new MshClient();

/**
 * Execute a command through the msh deterministic runtime with ANSI stripped,
 * secret redaction, and prompt handling.
 */
export async function exec(command: string, options?: ExecOptions): Promise<ExecResponse> {
  return defaultClient.exec(command, options);
}
