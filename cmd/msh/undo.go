package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/msh-protocol/msh/pkg/db"
	"github.com/msh-protocol/msh/pkg/fs"
	"github.com/msh-protocol/msh/pkg/protocol"
)

var undoCmd = &cobra.Command{
	Use:   "undo [run-id]",
	Short: "Rollback file changes made by an execution",
	Long:  "Surgically rolls back file changes made by the specified execution (or the latest execution) using captured diff patches.",
	Run: func(cmd *cobra.Command, args []string) {
		database, err := db.InitDB()
		if err != nil {
			fmt.Printf("Error accessing database: %v\n", err)
			os.Exit(1)
		}

		records, err := database.GetExecutions(50, 0)
		if err != nil || len(records) == 0 {
			fmt.Println("No execution records found to rollback.")
			return
		}

		var targetRecord *db.ExecutionRecord
		if len(args) > 0 {
			targetID, err := strconv.Atoi(args[0])
			if err != nil {
				fmt.Printf("Invalid execution ID: %s\n", args[0])
				os.Exit(1)
			}
			for i := range records {
				if records[i].ID == targetID {
					targetRecord = &records[i]
					break
				}
			}
			if targetRecord == nil {
				fmt.Printf("Execution #%d not found in recent history.\n", targetID)
				os.Exit(1)
			}
		} else {
			// Find the most recent record that modified files
			for i := range records {
				var resp protocol.ExecResponse
				if err := json.Unmarshal([]byte(records[i].RespJSON), &resp); err == nil {
					if len(resp.FilesChanged) > 0 || len(resp.FileDiffs) > 0 {
						targetRecord = &records[i]
						break
					}
				}
			}
			if targetRecord == nil {
				fmt.Println("No recent execution found that modified files.")
				return
			}
		}

		var resp protocol.ExecResponse
		if err := json.Unmarshal([]byte(targetRecord.RespJSON), &resp); err != nil {
			fmt.Printf("Error parsing execution response for #%d: %v\n", targetRecord.ID, err)
			os.Exit(1)
		}

		cwd := resp.Cwd
		if cwd == "" {
			cwd, _ = os.Getwd()
		}

		reverted, err := fs.RollbackExecution(cwd, resp.FilesChanged, resp.FileDiffs)
		if err != nil {
			fmt.Printf("Rollback failed for execution #%d: %v\n", targetRecord.ID, err)
			os.Exit(1)
		}

		fmt.Printf("✓ Successfully rolled back execution #%d (%s)\n", targetRecord.ID, targetRecord.Command)
		fmt.Printf("  Reverted %d file(s):\n", len(reverted))
		for _, f := range reverted {
			fmt.Printf("   • %s\n", f)
		}
	},
}

func init() {
	rootCmd.AddCommand(undoCmd)
}
