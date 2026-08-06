package main

import (
	"fmt"
	"time"

	"github.com/msh-protocol/msh/pkg/server"
	"github.com/spf13/cobra"
)

var (
	flagPort int
	flagHost string
)

func init() {
	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the msh HTTP server daemon",
		Long: `Start msh in daemon mode. 
This opens an HTTP API that AI agents can use to execute commands 
over the network while maintaining persistent sessions.`,
		RunE: runServe,
	}

	serveCmd.Flags().IntVarP(&flagPort, "port", "p", 8080, "Port to listen on")
	serveCmd.Flags().StringVarP(&flagHost, "host", "H", "127.0.0.1", "Host IP to bind to")

	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	idleTimeout := 30 * time.Minute
	srv := server.NewServer(flagHost, flagPort, idleTimeout)

	fmt.Println("Starting msh daemon...")
	return srv.Start()
}
