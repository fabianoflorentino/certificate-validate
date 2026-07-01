package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	// Version is the current version of certificate-validate.
	// Set via ldflags: -X github.com/fabianoflorentino/certificate-validate/internal/cmd.Version=v1.0.0
	Version = "dev"

	// Commit is the git commit hash at build time.
	// Set via ldflags: -X github.com/fabianoflorentino/certificate-validate/internal/cmd.Commit=abc123
	Commit = "none"

	// Date is the build timestamp.
	// Set via ldflags: -X github.com/fabianoflorentino/certificate-validate/internal/cmd.Date=2024-01-01T00:00:00Z
	Date = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("certificate-validate %s\n", Version)
		fmt.Printf("  commit:     %s\n", Commit)
		fmt.Printf("  built:      %s\n", Date)
		fmt.Printf("  go:         %s %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
