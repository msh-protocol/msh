package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/msh-protocol/msh/pkg/db"
	"github.com/msh-protocol/msh/pkg/execution"
	"github.com/msh-protocol/msh/pkg/protocol"
)

var verifyCmd = &cobra.Command{
	Use:   "verify [run-id]",
	Short: "Cryptographically verify the reproducibility of an execution",
	Long:  "Re-executes a command under identical request parameters to verify deterministic execution, exit code consistency, and drift.",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		database, err := db.InitDB()
		if err != nil {
			fmt.Printf("Error accessing database: %v\n", err)
			os.Exit(1)
		}

		records, err := database.GetExecutions(100, 0)
		if err != nil {
			fmt.Printf("Error reading execution history: %v\n", err)
			os.Exit(1)
		}

		if len(records) == 0 {
			fmt.Println("No execution records found to verify.")
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
			targetRecord = &records[0]
		}

		var req protocol.ExecRequest
		if err := json.Unmarshal([]byte(targetRecord.ReqJSON), &req); err != nil {
			fmt.Printf("Error parsing request JSON: %v\n", err)
			os.Exit(1)
		}

		var resp protocol.ExecResponse
		if err := json.Unmarshal([]byte(targetRecord.RespJSON), &resp); err != nil {
			fmt.Printf("Error parsing response JSON: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Verifying Execution #%d: '%s'...\n", targetRecord.ID, targetRecord.Command)

		result, err := execution.VerifyExecution(req, resp)
		if err != nil {
			fmt.Printf("Verification error: %v\n", err)
			os.Exit(1)
		}

		result.RunID = targetRecord.ID

		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		if result.IsDeterministic {
			fmt.Printf("  STATUS:        ✓ 100%% DETERMINISTIC & REPRODUCIBLE\n")
		} else {
			fmt.Printf("  STATUS:        ⚠️ DRIFT / NON-DETERMINISTIC DETECTED\n")
		}
		fmt.Printf("  Command:       %s\n", result.Command)
		fmt.Printf("  Exit Code:     Match: %t\n", result.ExitCodeMatch)
		fmt.Printf("  Output Match:  Match: %t (Similarity: %.1f%%)\n", result.OutputMatch, result.SimilarityScore*100)
		fmt.Printf("  Diff Match:    Match: %t\n", result.DiffMatch)
		fmt.Printf("  Original Hash: %s\n", result.OriginalHash)
		fmt.Printf("  Replay Hash:   %s\n", result.ReplayHash)
		fmt.Printf("  Summary:       %s\n", result.DriftSummary)
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

		if !result.IsDeterministic {
			os.Exit(2)
		}
	},
}

func init() {
	rootCmd.AddCommand(verifyCmd)
}
