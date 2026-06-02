package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var cfgPath string

// rootCmd is the base command for certificate-validate.
var rootCmd = &cobra.Command{
	Use:   "certificate-validate",
	Short: "Validate SSL/TLS certificates",
	Long: `A modern tool to fetch and inspect SSL/TLS certificate information
from remote hosts. Supports CLI checks and an HTTP API.`,
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgPath, "config", "c", "config/settings.yml",
		"path to configuration file")
}
