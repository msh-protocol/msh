package main

import (
	"fmt"
	"os"

	"github.com/msh-protocol/msh/pkg/execution"
	"github.com/msh-protocol/msh/pkg/protocol"
	"github.com/spf13/cobra"
)

var wrapCmd = &cobra.Command{
	Use:   `wrap "command"`,
	Short: "Execute a command and return sanitized raw text",
	Long: `Execute a shell command through the msh runtime, but instead of 
returning structured JSON, it prints the sanitized raw text directly to the terminal.
This allows human developers or non-JSON agents to use msh as a drop-in execution wrapper.`,
	Args: cobra.ExactArgs(1),
	Run:  runWrap,
}

func init() {
	wrapCmd.Flags().StringVar(&flagCwd, "cwd", "", "Override working directory")
	wrapCmd.Flags().StringVar(&flagTimeout, "timeout", "30s", "Execution timeout (e.g., 30s, 5m)")
	wrapCmd.Flags().IntVar(&flagMaxLines, "max-lines", 200, "Maximum output lines (0 for no limit)")
	wrapCmd.Flags().BoolVar(&flagNoFiles, "no-files", false, "Skip filesystem change detection")
	wrapCmd.Flags().BoolVar(&flagPty, "pty", false, "Run command in a pseudo-terminal (PTY)")
	wrapCmd.Flags().StringVar(&flagEnvFile, "env-file", "", "Path to a .env file to load before execution")
}

func runWrap(cmd *cobra.Command, args []string) {
	command := args[0]

	// Parse timeout duration
	timeout, err := parseTimeout(flagTimeout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[msh] invalid timeout: %v\n", err)
		os.Exit(1)
	}

	// Create session
	session, err := execution.NewSession(flagCwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[msh] failed to create session: %v\n", err)
		os.Exit(1)
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
	}

	// Execute
	executor := execution.NewExecutor(session)
	resp := executor.Execute(req)

	// Print stdout
	if resp.Stdout != "" {
		fmt.Println(resp.Stdout)
	}

	// Print stderr
	if resp.Stderr != "" {
		fmt.Fprintln(os.Stderr, resp.Stderr)
	}

	if resp.Status == protocol.StatusBlocked {
		fmt.Fprintf(os.Stderr, "\n[msh] process killed due to interactive prompt: %s\n", resp.PromptDetected)
	} else if resp.Status == protocol.StatusTimeout {
		fmt.Fprintf(os.Stderr, "\n[msh] process killed due to timeout (%s)\n", flagTimeout)
	}

	// Exit with the exact exit code of the underlying process
	// If exit code is -1 (e.g. killed by timeout or prompt), exit with 1
	if resp.ExitCode == -1 {
		os.Exit(1)
	}
	os.Exit(resp.ExitCode)
}
