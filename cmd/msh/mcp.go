package main

import (
	"fmt"
	"os"

	"time"

	"github.com/msh-protocol/msh/pkg/execution"
	"github.com/msh-protocol/msh/pkg/mcp"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start an MCP (Model Context Protocol) server over standard I/O",
	Long: `Starts msh as an MCP server using standard I/O. 
This allows native integration with AI agents like Claude Desktop and Cursor.`,
	Run: func(cmd *cobra.Command, args []string) {
		sessionManager := execution.NewSessionManager(30 * time.Minute)
		srv := mcp.NewMshServer(sessionManager)

		if err := srv.StartStdio(); err != nil {
			fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
			os.Exit(1)
		}
	},
}
