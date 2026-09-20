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

	"github.com/msh-protocol/msh/pkg/db"
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
	flagEnvFile  string
	flagNoRedact bool
	flagAnswers  []string
)

var rootCmd = &cobra.Command{
	Use:   "msh [command | flags] [args...]",
	Short: "The Deterministic Execution Protocol & Runtime for AI Agents",
	Long: `msh is an open-source execution layer built specifically for
autonomous coding agents, LLM tool-use systems, and agentic workflows.

Run any command through the msh deterministic runtime simply by prefixing it:
  msh git status
  msh npm run build
  msh pytest -v

All command outputs are automatically sanitized, truncated, secret-redacted,
and interactive prompts are detected and answered.`,
	Example: `  # Smart Passthrough execution (run any command directly)
  msh git status
  msh npm test
  msh --max-lines 500 git log
  msh --timeout 1m python script.py

  # Management subcommands
  msh exec "npm run build"     # Structured JSON response
  msh fleet start             # Start enterprise control plane hub
  msh serve --port 8080       # Start HTTP execution daemon`,
}

func main() {
	// Check for Smart Command Passthrough mode (e.g., `msh git status`, `msh npm run build`)
	if len(os.Args) > 1 {
		if flags, cmdStr, isPassthrough := ParsePassthroughArgs(os.Args[1:]); isPassthrough {
			RunPassthrough(flags, cmdStr)
			return
		}
	}

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
	execCmd.Flags().StringVar(&flagEnvFile, "env-file", "", "Path to a .env file to load before execution")
	execCmd.Flags().BoolVar(&flagNoRedact, "no-redact", false, "Disable secret redaction of command output")
	execCmd.Flags().StringSliceVar(&flagAnswers, "answer", nil, "Answer to feed an interactive prompt (repeatable, used in order when prompts are detected)")

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
	rootCmd.AddCommand(wrapCmd)
	rootCmd.AddCommand(daemonCmd)
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
		EnvFile:        flagEnvFile,
		RedactSecrets:  redactFlag(),
		PromptAnswers:  flagAnswers,
	}

	// Execute
	executor := execution.NewExecutor(session)
	resp := executor.Execute(req)

	if database, err := db.InitDB(); err == nil {
		_ = database.SaveExecution(req, resp)
	}

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

// redactFlag returns the RedactSecrets pointer for an ExecRequest.
// Redaction is on by default (nil); it is only disabled when --no-redact
// is explicitly passed.
func redactFlag() *bool {
	if flagNoRedact {
		f := false
		return &f
	}
	return nil
}
