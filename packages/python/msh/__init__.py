"""
msh: sudo for AI agents.
The machine-readable local execution layer for LLMs and autonomous coding agents.
"""

from msh.client import ExecResponse, MshClient, exec

__version__ = "1.4.0"
__all__ = ["exec", "MshClient", "ExecResponse"]
