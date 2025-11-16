package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
)

var cmd = &cobra.Command{
	Use:   "app [cmd]",
	Short: "Service for assigning reviewers for Pull Requests",
}

func init() {
	cmd.AddCommand(serveCmd)
	cmd.AddCommand(generateApiKeyCmd)
}

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
