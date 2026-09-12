package main

import (
	"fmt"
	"os"

	"github.com/msh-protocol/msh/pkg/fleet"
	"github.com/msh-protocol/msh/pkg/protocol"
	"github.com/spf13/cobra"
)

var fleetCmd = &cobra.Command{
	Use:   "fleet",
	Short: "Manage the msh fleet enterprise control plane",
}

var fleetStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the centralized msh-fleet hub and dashboard",
	Run: func(cmd *cobra.Command, args []string) {
		host, _ := cmd.Flags().GetString("host")
		port, _ := cmd.Flags().GetInt("port")
		token, _ := cmd.Flags().GetString("token")

		if token == "" {
			token = protocol.GenerateToken("msh-")
			fmt.Printf("\n[msh-fleet] Generated Admin Token: %s\n\n", token)
		}

		server := fleet.NewServer(host, port, token)
		if err := server.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Error starting fleet server: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	fleetStartCmd.Flags().String("host", "127.0.0.1", "Host to listen on")
	fleetStartCmd.Flags().Int("port", 9000, "Port to listen on")
	fleetStartCmd.Flags().String("token", "", "Admin authentication token (generated if empty)")

	fleetCmd.AddCommand(fleetStartCmd)
	rootCmd.AddCommand(fleetCmd)
}
