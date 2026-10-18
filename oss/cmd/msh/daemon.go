package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/msh-protocol/msh/pkg/daemon"
	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage long-running background processes",
}

var daemonStartCmd = &cobra.Command{
	Use:   `start "command"`,
	Short: "Start a long-running command in the background",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		manager, err := daemon.NewManager()
		if err != nil {
			return err
		}
		
		cwd, _ := os.Getwd()
		if flagCwd != "" {
			cwd = flagCwd
		}

		state, err := manager.Start(args[0], cwd, nil)
		if err != nil {
			return err
		}

		out, _ := json.MarshalIndent(state, "", "  ")
		fmt.Println(string(out))
		return nil
	},
}

var daemonLogsCmd = &cobra.Command{
	Use:   "logs [id]",
	Short: "Read logs from a running daemon",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		manager, err := daemon.NewManager()
		if err != nil {
			return err
		}
		logs, err := manager.ReadLogs(args[0], flagMaxLines)
		if err != nil {
			return err
		}
		// Try to sanitize it similar to wrap? For now just print
		fmt.Println(strings.TrimSpace(logs))
		return nil
	},
}

var daemonKillCmd = &cobra.Command{
	Use:   "kill [id]",
	Short: "Kill a running daemon",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		manager, err := daemon.NewManager()
		if err != nil {
			return err
		}
		if err := manager.Kill(args[0]); err != nil {
			return err
		}
		fmt.Println(`{"status": "killed", "id": "` + args[0] + `"}`)
		return nil
	},
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status [id]",
	Short: "Get status of a running daemon",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		manager, err := daemon.NewManager()
		if err != nil {
			return err
		}
		state, err := manager.GetState(args[0])
		if err != nil {
			return err
		}
		out, _ := json.MarshalIndent(state, "", "  ")
		fmt.Println(string(out))
		return nil
	},
}

func init() {
	daemonStartCmd.Flags().StringVar(&flagCwd, "cwd", "", "Override working directory")
	daemonLogsCmd.Flags().IntVar(&flagMaxLines, "max-lines", 200, "Maximum output lines (0 for no limit)")

	daemonCmd.AddCommand(daemonStartCmd)
	daemonCmd.AddCommand(daemonLogsCmd)
	daemonCmd.AddCommand(daemonKillCmd)
	daemonCmd.AddCommand(daemonStatusCmd)
}
