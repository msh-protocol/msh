"""
Example: Drop-in execution tool for LangChain, CrewAI, or Anthropic Claude tool-calling.

Run:
  python packages/python/examples/agent_tool.py
"""

import os
import sys

sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "..")))
from msh import exec


def execute_shell_command(command: str) -> str:
    """Execute a terminal command with ANSI stripped, secrets masked, and prompt handling."""
    res = exec(command, max_output_lines=200)
    if res.status == "blocked":
        return f"Error: Command blocked on interactive prompt '{res.prompt_detected}'."
    if not res.ok:
        return f"Command failed (exit {res.exit_code}):\n{res.stderr or res.stdout}"
    return res.stdout


if __name__ == "__main__":
    output = execute_shell_command("git status")
    print("--- Model Safe Output ---")
    print(output)
