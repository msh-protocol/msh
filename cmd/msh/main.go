// msh — The Deterministic Execution Protocol & Runtime for AI Agents.
//
// Usage:
//
//	msh exec "command"         Execute a command with structured JSON output
//	msh exec "command" --pretty  Pretty-print the JSON response
//	msh version                Print version information
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/msh-protocol/msh/pkg/execution"
	"github.com/msh-protocol/msh/pkg/protocol"
	"github.com/spf13/cobra"
)

var (
	// CLI flags
	flagCwd      string
	flagTimeout  string
	flagMaxLines int
	flagPretty   bool
	flagNoFiles  bool
	flagPty      bool
)

var rootCmd = &cobra.Command{
	Use:   "msh",
	Short: "The Deterministic Execution Protocol & Runtime for AI Agents",
	Long: `msh is an open-source execution layer built specifically for
autonomous coding agents, LLM tool-use systems, and agentic workflows.

Instead of forcing AI models to parse unstructured, ANSI-polluted terminal
streams, msh provides a machine-readable execution environment with strict
state preservation, automated log sanitization, and structured JSON output.`,
}

func main() {

	// --- exec command ---
	execCmd := &cobra.Command{
		Use:   `exec "command"`,
		Short: "Execute a command and return structured JSON",
		Long: `Execute a shell command through the msh runtime.
The command output is sanitized (ANSI codes stripped, output truncated)
and returned as a structured JSON payload.`,
		Args: cobra.ExactArgs(1),
		RunE: runExec,
	}

	execCmd.Flags().StringVar(&flagCwd, "cwd", "", "Override working directory")
	execCmd.Flags().StringVar(&flagTimeout, "timeout", "30s", "Execution timeout (e.g., 30s, 5m)")
	execCmd.Flags().IntVar(&flagMaxLines, "max-lines", 200, "Maximum output lines (0 for no limit)")
	execCmd.Flags().BoolVar(&flagPretty, "pretty", false, "Pretty-print JSON output")
	execCmd.Flags().BoolVar(&flagNoFiles, "no-files", false, "Skip filesystem change detection")
	execCmd.Flags().BoolVar(&flagPty, "pty", false, "Run command in a pseudo-terminal (PTY)")

	// --- version command ---
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print msh version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("msh version %s\n", protocol.Version)
			fmt.Printf("protocol: msh-protocol v%s\n", protocol.Version)
		},
	}

	rootCmd.AddCommand(execCmd)
	rootCmd.AddCommand(mcpCmd)
	rootCmd.AddCommand(versionCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runExec(cmd *cobra.Command, args []string) error {
	command := args[0]

	// Parse timeout duration
	timeout, err := parseTimeout(flagTimeout)
	if err != nil {
		return fmt.Errorf("invalid timeout: %w", err)
	}

	// Create session
	session, err := execution.NewSession(flagCwd)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	// Build execution request
	req := protocol.ExecRequest{
		Command:        command,
		Cwd:            flagCwd,
		Timeout:        timeout,
		MaxOutputLines: flagMaxLines,
		DetectFiles:    !flagNoFiles,
		UsePty:         flagPty,
	}

	// Execute
	executor := execution.NewExecutor(session)
	resp := executor.Execute(req)

	// Output JSON
	var output []byte
	if flagPretty {
		output, err = resp.ToPrettyJSON()
	} else {
		output, err = resp.ToJSON()
	}
	if err != nil {
		return fmt.Errorf("failed to serialize response: %w", err)
	}

	fmt.Println(string(output))
	return nil
}

func parseTimeout(s string) (time.Duration, error) {
	return time.ParseDuration(s)
}
