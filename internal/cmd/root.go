package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion scripts",
	Long: fmt.Sprintf(`Generate shell completion script for %s.

To load completions:

  Bash:
    $ source <(%[1]s completion bash)
    # To load permanently:
    $ %[1]s completion bash > /etc/bash_completion.d/%[1]s

  Zsh:
    $ source <(%[1]s completion zsh)
    # To load permanently:
    $ %[1]s completion zsh > "${fpath[1]}/_%[1]s"

  Fish:
    $ %[1]s completion fish | source
    # To load permanently:
    $ %[1]s completion fish > ~/.config/fish/completions/%[1]s.fish

  PowerShell:
    PS> %[1]s completion powershell | Out-String | Invoke-Expression
`, "certificate-validate"),
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return cmd.Root().GenBashCompletion(os.Stdout)
		case "zsh":
			return cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			return cmd.Root().GenFishCompletion(os.Stdout, true)
		case "powershell":
			return cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
		default:
			return fmt.Errorf("unsupported shell: %s", args[0])
		}
	},
}

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
	rootCmd.AddCommand(completionCmd)
	rootCmd.PersistentFlags().StringVarP(&cfgPath, "config", "c", "config/settings.yml",
		"path to configuration file")
}
