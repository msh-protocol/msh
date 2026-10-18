package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var pluginCmd = &cobra.Command{
	Use:   "plugin [command]",
	Short: "Manage msh plugins and hooks",
}

var installCmd = &cobra.Command{
	Use:   "install [name]",
	Short: "Install a plugin from the marketplace",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		pluginName := args[0]
		fmt.Printf("Installing plugin: %s\n", pluginName)

		// For prototype, download from a hypothetical github repo
		url := fmt.Sprintf("https://raw.githubusercontent.com/msh-protocol/plugins/main/%s.yaml", pluginName)
		resp, err := http.Get(url)
		if err != nil || resp.StatusCode != 200 {
			if resp != nil {
				fmt.Printf("Error: Plugin '%s' not found (Status %d)\n", pluginName, resp.StatusCode)
			} else {
				fmt.Printf("Error: %v\n", err)
			}
			os.Exit(1)
		}
		defer resp.Body.Close()

		mshDir := ".msh"
		if err := os.MkdirAll(mshDir, 0755); err != nil {
			fmt.Printf("Error creating .msh directory: %v\n", err)
			os.Exit(1)
		}

		// Save the downloaded YAML into hooks.yaml (appending it for now, or just creating it if it doesn't exist)
		// A full implementation would merge the YAML files. For this prototype, we'll just write it.
		hookPath := filepath.Join(mshDir, "hooks.yaml")
		
		out, err := os.OpenFile(hookPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			fmt.Printf("Error opening %s: %v\n", hookPath, err)
			os.Exit(1)
		}
		defer out.Close()

		// Write a newline and a comment to separate plugins
		out.WriteString(fmt.Sprintf("\n# Plugin: %s\n", pluginName))

		_, err = io.Copy(out, resp.Body)
		if err != nil {
			fmt.Printf("Error saving plugin: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Successfully installed plugin '%s' into %s\n", pluginName, hookPath)
	},
}

func init() {
	pluginCmd.AddCommand(installCmd)
	rootCmd.AddCommand(pluginCmd)
}
