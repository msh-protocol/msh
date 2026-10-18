package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/msh-protocol/msh/pkg/branch"
	"github.com/spf13/cobra"
)

var (
	flagBranchFrom    string
	flagBranchDesc    string
	flagBranchJSON    bool
	flagBranchMsg     string
	flagBranchTimeout string
	flagBranchMax     int
)

var branchCmd = &cobra.Command{
	Use:   "branch [subcommand]",
	Short: "Speculative execution & shadow worktrees for AI agents (arXiv:2512.12806)",
	Long: `msh branch manages sub-second shadow git worktrees for autonomous agents.
Allows agents to trial speculative code refactors, parallel hypotheses, and 
exploratory tool calls without touching the main working tree.

Verified hypotheses can be merged cleanly with 'msh branch merge <name>', 
or discarded without residue using 'msh branch abort <name>'.`,
	Example: `  msh branch create hypo-1 --desc "Try alternative SQL migration"
  msh branch run hypo-1 "npm run build"
  msh branch diff hypo-1
  msh branch merge hypo-1
  msh branch abort hypo-1`,
}

var branchCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Spin up an isolated shadow worktree in milliseconds",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		mgr, err := branch.NewManager(flagCwd)
		if err != nil {
			return err
		}

		info, err := mgr.Create(name, flagBranchFrom, flagBranchDesc)
		if err != nil {
			return err
		}

		if flagBranchJSON {
			bytes, _ := json.MarshalIndent(info, "", "  ")
			fmt.Println(string(bytes))
			return nil
		}

		fmt.Printf("✓ Created shadow worktree '%s'\n", info.Name)
		fmt.Printf("  • Shadow Path: %s\n", info.Path)
		fmt.Printf("  • Git Branch:  %s\n", info.GitBranch)
		fmt.Printf("  • Base Ref:    %s (%s)\n", info.BaseRef, truncateHash(info.BaseCommit))
		if info.Description != "" {
			fmt.Printf("  • Purpose:     %s\n", info.Description)
		}
		fmt.Printf("\nExecute commands in isolation:\n")
		fmt.Printf("  msh branch run %s \"<command>\"\n", info.Name)
		return nil
	},
}

var branchListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all active shadow worktrees and hypotheses",
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := branch.NewManager(flagCwd)
		if err != nil {
			return err
		}

		branches, err := mgr.List()
		if err != nil {
			return err
		}

		if flagBranchJSON {
			bytes, _ := json.MarshalIndent(branches, "", "  ")
			fmt.Println(string(bytes))
			return nil
		}

		if len(branches) == 0 {
			fmt.Println("No active shadow branches found.")
			fmt.Println("Create one with: msh branch create <name>")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "NAME\tSTATUS\tRUNS\tMODIFIED\tCREATED\tBASE COMMIT\tDESCRIPTION")
		for _, b := range branches {
			mods := fmt.Sprintf("%d files", len(b.ModifiedFiles))
			if len(b.ModifiedFiles) == 0 {
				mods = "clean"
			}
			desc := b.Description
			if len(desc) > 30 {
				desc = desc[:27] + "..."
			}
			created := time.Since(b.CreatedAt).Round(time.Second).String() + " ago"
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%s\t%s\n",
				b.Name, b.Status, b.RunsCount, mods, created, truncateHash(b.BaseCommit), desc)
		}
		w.Flush()
		return nil
	},
}

var branchRunCmd = &cobra.Command{
	Use:   "run <name> <command>",
	Short: "Execute a command inside an isolated shadow worktree",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		command := args[1]

		mgr, err := branch.NewManager(flagCwd)
		if err != nil {
			return err
		}

		timeout := 2 * time.Minute
		if flagBranchTimeout != "" {
			if t, err := time.ParseDuration(flagBranchTimeout); err == nil {
				timeout = t
			}
		}

		resp, err := mgr.Run(name, command, timeout, flagBranchMax)
		if err != nil {
			return err
		}

		if flagBranchJSON {
			bytes, _ := json.MarshalIndent(resp, "", "  ")
			fmt.Println(string(bytes))
			return nil
		}

		// Print output
		if resp.Stdout != "" {
			fmt.Print(resp.Stdout)
		}
		if resp.Stderr != "" {
			fmt.Print(resp.Stderr)
		}

		if len(resp.FilesChanged) > 0 {
			fmt.Printf("\n[msh branch] %d file(s) modified in shadow worktree '%s':\n", len(resp.FilesChanged), name)
			for _, f := range resp.FilesChanged {
				fmt.Printf("  • %s\n", f)
			}
		}

		if resp.ExitCode != 0 {
			os.Exit(resp.ExitCode)
		}
		return nil
	},
}

var branchDiffCmd = &cobra.Command{
	Use:   "diff <name>",
	Short: "Display unified diff of modifications made in a shadow worktree",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		mgr, err := branch.NewManager(flagCwd)
		if err != nil {
			return err
		}

		diff, err := mgr.Diff(name)
		if err != nil {
			return err
		}

		if strings.TrimSpace(diff) == "" {
			fmt.Println("No modifications detected in shadow worktree.")
			return nil
		}

		fmt.Print(diff)
		return nil
	},
}

var branchMergeCmd = &cobra.Command{
	Use:   "merge <name>",
	Short: "Merge verified speculative changes into the main working tree",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		mgr, err := branch.NewManager(flagCwd)
		if err != nil {
			return err
		}

		if err := mgr.Merge(name, flagBranchMsg); err != nil {
			return err
		}

		fmt.Printf("✓ Speculative branch '%s' successfully merged into main.\n", name)
		fmt.Printf("  Shadow worktree and git branch cleaned up.\n")
		return nil
	},
}

var branchAbortCmd = &cobra.Command{
	Use:     "abort <name>",
	Aliases: []string{"discard", "rm"},
	Short:   "Discard speculative hypothesis cleanly and emit context prune payload",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		mgr, err := branch.NewManager(flagCwd)
		if err != nil {
			return err
		}

		payload, err := mgr.Abort(name)
		if err != nil {
			return err
		}

		if flagBranchJSON {
			bytes, _ := json.MarshalIndent(payload, "", "  ")
			fmt.Println(string(bytes))
			return nil
		}

		fmt.Printf("✓ Speculative hypothesis '%s' aborted.\n", name)
		fmt.Printf("  • Status:               Cleaned & Pruned\n")
		fmt.Printf("  • Context Prune Turns:  %d (suggested LLM history rewind, arXiv:2608.03836)\n", payload.ContextPruneTurns)
		if len(payload.FilesCleaned) > 0 {
			fmt.Printf("  • Reverted Files (%d):  %s\n", len(payload.FilesCleaned), strings.Join(payload.FilesCleaned, ", "))
		}
		fmt.Printf("  Main workspace remains 100%% clean.\n")
		return nil
	},
}

func init() {
	branchCreateCmd.Flags().StringVar(&flagBranchFrom, "from", "HEAD", "Base git reference or commit to branch from")
	branchCreateCmd.Flags().StringVar(&flagBranchDesc, "desc", "", "Description / hypothesis purpose")
	branchCreateCmd.Flags().BoolVar(&flagBranchJSON, "json", false, "Output structured JSON")

	branchListCmd.Flags().BoolVar(&flagBranchJSON, "json", false, "Output structured JSON")

	branchRunCmd.Flags().StringVar(&flagBranchTimeout, "timeout", "2m", "Command timeout")
	branchRunCmd.Flags().IntVar(&flagBranchMax, "max-lines", 500, "Maximum output lines")
	branchRunCmd.Flags().BoolVar(&flagBranchJSON, "json", false, "Output structured JSON response")

	branchMergeCmd.Flags().StringVarP(&flagBranchMsg, "message", "m", "", "Commit message for merge")

	branchAbortCmd.Flags().BoolVar(&flagBranchJSON, "json", false, "Output structured JSON prune payload")

	branchCmd.AddCommand(branchCreateCmd)
	branchCmd.AddCommand(branchListCmd)
	branchCmd.AddCommand(branchRunCmd)
	branchCmd.AddCommand(branchDiffCmd)
	branchCmd.AddCommand(branchMergeCmd)
	branchCmd.AddCommand(branchAbortCmd)

	rootCmd.AddCommand(branchCmd)
}

func truncateHash(hash string) string {
	if len(hash) > 8 {
		return hash[:8]
	}
	return hash
}
