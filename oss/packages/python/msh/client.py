from __future__ import annotations

import json
import shutil
import subprocess
from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional
import requests


@dataclass
class ExecResponse:
    status: str
    exit_code: int
    cwd: str = ""
    stdout: str = ""
    stderr: str = ""
    truncated: bool = False
    redacted: List[str] = field(default_factory=list)
    files_changed: List[str] = field(default_factory=list)
    hooks: List[Dict[str, Any]] = field(default_factory=list)
    answers_used: int = 0
    duration_ms: int = 0
    prompt_detected: str = ""
    session_id: str = ""
    error: str = ""

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> ExecResponse:
        fields = {f.name for f in cls.__dataclass_fields__.values()}
        filtered = {k: v for k, v in data.items() if k in fields}
        return cls(**filtered)

    @property
    def ok(self) -> bool:
        return self.status == "success" and self.exit_code == 0

    def to_dict(self) -> Dict[str, Any]:
        return {
            "status": self.status,
            "exit_code": self.exit_code,
            "cwd": self.cwd,
            "stdout": self.stdout,
            "stderr": self.stderr,
            "truncated": self.truncated,
            "redacted": self.redacted,
            "files_changed": self.files_changed,
            "duration_ms": self.duration_ms,
            "prompt_detected": self.prompt_detected,
            "session_id": self.session_id,
            "error": self.error,
        }


class MshClient:
    """Client for executing commands through the msh deterministic runtime."""

    def __init__(self, base_url: Optional[str] = None, token: Optional[str] = None):
        self.base_url = base_url.rstrip("/") if base_url else None
        self.token = token

    def exec(
        self,
        command: str,
        cwd: Optional[str] = None,
        timeout: Optional[str] = "30s",
        max_output_lines: int = 500,
        prompt_answers: Optional[List[str]] = None,
        use_pty: bool = False,
        env: Optional[Dict[str, str]] = None,
        session_id: Optional[str] = None,
    ) -> ExecResponse:
        """Executes a command through msh and returns structured, model-safe results."""
        if self.base_url:
            return self._exec_http(
                command=command,
                cwd=cwd,
                timeout=timeout,
                max_output_lines=max_output_lines,
                prompt_answers=prompt_answers,
                use_pty=use_pty,
                env=env,
                session_id=session_id,
            )
        return self._exec_cli(
            command=command,
            cwd=cwd,
            timeout=timeout,
            max_output_lines=max_output_lines,
            prompt_answers=prompt_answers,
            use_pty=use_pty,
            env=env,
            session_id=session_id,
        )

    def _exec_cli(
        self,
        command: str,
        cwd: Optional[str] = None,
        timeout: Optional[str] = "30s",
        max_output_lines: int = 500,
        prompt_answers: Optional[List[str]] = None,
        use_pty: bool = False,
        env: Optional[Dict[str, str]] = None,
        session_id: Optional[str] = None,
    ) -> ExecResponse:
        msh_bin = shutil.which("msh") or "msh"
        args = [msh_bin, "exec", command]
        if cwd:
            args.extend(["--cwd", cwd])
        if timeout:
            args.extend(["--timeout", timeout])
        if max_output_lines is not None:
            args.extend(["--max-lines", str(max_output_lines)])
        if use_pty:
            args.append("--pty")
        if prompt_answers:
            for ans in prompt_answers:
                args.extend(["--answer", ans])

        try:
            proc = subprocess.run(
                args,
                capture_output=True,
                text=True,
                check=False,
            )
            raw = proc.stdout.strip()
            if not raw and proc.stderr:
                raw = proc.stderr.strip()
            data = json.loads(raw)
            return ExecResponse.from_dict(data)
        except Exception as e:
            return ExecResponse(
                status="error",
                exit_code=-1,
                cwd=cwd or "",
                stdout="",
                stderr=str(e),
                error=str(e),
            )

    def _exec_http(
        self,
        command: str,
        cwd: Optional[str] = None,
        timeout: Optional[str] = "30s",
        max_output_lines: int = 500,
        prompt_answers: Optional[List[str]] = None,
        use_pty: bool = False,
        env: Optional[Dict[str, str]] = None,
        session_id: Optional[str] = None,
    ) -> ExecResponse:
        url = f"{self.base_url}/execute"
        headers = {"Content-Type": "application/json"}
        if self.token:
            headers["Authorization"] = f"Bearer {self.token}"

        payload: Dict[str, Any] = {
            "command": command,
            "max_output_lines": max_output_lines,
            "use_pty": use_pty,
        }
        if cwd:
            payload["cwd"] = cwd
        if timeout:
            payload["timeout"] = timeout
        if prompt_answers:
            payload["prompt_answers"] = prompt_answers
        if env:
            payload["env"] = env
        if session_id:
            payload["session_id"] = session_id

        try:
            resp = requests.post(url, json=payload, headers=headers, timeout=120)
            data = resp.json()
            return ExecResponse.from_dict(data)
        except Exception as e:
            return ExecResponse(
                status="error",
                exit_code=-1,
                cwd=cwd or "",
                stdout="",
                stderr=str(e),
                error=str(e),
            )


# Top-level helper function for effortless imports: `from msh import exec`
_default_client = MshClient()


def exec(
    command: str,
    cwd: Optional[str] = None,
    timeout: Optional[str] = "30s",
    max_output_lines: int = 500,
    prompt_answers: Optional[List[str]] = None,
    use_pty: bool = False,
    session_id: Optional[str] = None,
) -> ExecResponse:
    """Execute a command through msh with ANSI stripped, secrets masked, and prompt handling."""
    return _default_client.exec(
        command=command,
        cwd=cwd,
        timeout=timeout,
        max_output_lines=max_output_lines,
        prompt_answers=prompt_answers,
        use_pty=use_pty,
        session_id=session_id,
    )
