package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/msh-protocol/msh/pkg/execution"
	"github.com/msh-protocol/msh/pkg/protocol"
)

// MshServer wraps the MCP protocol handling around the core execution session manager.
type MshServer struct {
	sessionManager *execution.SessionManager
}

// NewMshServer creates a new MCP server integration instance.
func NewMshServer(sessionManager *execution.SessionManager) *MshServer {
	return &MshServer{sessionManager: sessionManager}
}

// StartStdio starts the MCP server using standard input/output for transport.
// This is the standard mechanism for local agents (like Claude Desktop or Cursor).
func (s *MshServer) StartStdio() error {
	// Create MCP server
	srv := server.NewMCPServer(
		"msh",
		"0.3.0",
		server.WithToolCapabilities(true),
	)

	// Add tool
	tool := mcp.NewTool("execute_command",
		mcp.WithDescription("Execute a shell command deterministically inside the msh runtime"),
		mcp.WithString("command",
			mcp.Required(),
			mcp.Description("The shell command to execute"),
		),
		mcp.WithString("cwd",
			mcp.Description("Override the working directory for this command"),
		),
		mcp.WithString("timeout",
			mcp.Description("Timeout string (e.g. '30s', '1m')"),
		),
		mcp.WithBoolean("use_pty",
			mcp.Description("Run command inside a pseudo-terminal (PTY)"),
		),
		mcp.WithString("session_id",
			mcp.Description("Session ID to persist CWD and env vars across multiple calls"),
		),
	)

	srv.AddTool(tool, s.handleExecuteCommand)

	// Start standard I/O server
	return server.ServeStdio(srv)
}

// handleExecuteCommand is the callback invoked when an LLM decides to use the execute_command tool.
func (s *MshServer) handleExecuteCommand(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	argsMap, ok := request.Params.Arguments.(map[string]interface{})
	if !ok {
		return mcp.NewToolResultError("invalid arguments format"), nil
	}

	command, ok := argsMap["command"].(string)
	if !ok || command == "" {
		return mcp.NewToolResultError("command argument is required"), nil
	}

	req := protocol.ExecRequest{
		Command: command,
	}

	if cwd, ok := argsMap["cwd"].(string); ok {
		req.Cwd = cwd
	}
	if timeoutStr, ok := argsMap["timeout"].(string); ok {
		if t, err := time.ParseDuration(timeoutStr); err == nil {
			req.Timeout = t
		}
	}
	if usePty, ok := argsMap["use_pty"].(bool); ok {
		req.UsePty = usePty
	}
	
	sessionID, _ := argsMap["session_id"].(string)

	// Get or create session
	session, err := s.sessionManager.GetOrCreateSession(sessionID, req.Cwd)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to initialize session: %v", err)), nil
	}

	// Execute command safely via the Executor
	executor := execution.NewExecutor(session)
	resp := executor.Execute(req)
	resp.SessionID = session.ID

	// Format response as a readable JSON string for the LLM
	respBytes, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to encode response: %v", err)), nil
	}

	return mcp.NewToolResultText(string(respBytes)), nil
}
