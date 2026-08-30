package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestK8sCmdRegistered(t *testing.T) {
	sub := findCommand(rootCmd, "k8s")
	if sub == nil {
		t.Fatal("expected root command to have a 'k8s' subcommand")
	}
}

func TestK8sMonitorCmdRegistered(t *testing.T) {
	k8s := findCommand(rootCmd, "k8s")
	if k8s == nil {
		t.Fatal("expected 'k8s' command")
	}
	monitor := findCommand(k8s, "monitor")
	if monitor == nil {
		t.Fatal("expected 'k8s' command to have a 'monitor' subcommand")
	}
}

func TestK8sMonitorFlags(t *testing.T) {
	k8s := findCommand(rootCmd, "k8s")
	monitor := findCommand(k8s, "monitor")
	if monitor == nil {
		t.Fatal("expected 'monitor' subcommand")
	}

	expectedFlags := []string{
		"namespace", "kubeconfig", "check-revocation", "watch-interval",
		"webhook-url", "webhook-threshold", "webhook-interval", "metrics-addr",
	}

	for _, name := range expectedFlags {
		if monitor.Flags().Lookup(name) == nil {
			t.Errorf("expected flag %q to be registered on k8s monitor", name)
		}
	}
}

func findCommand(cmd *cobra.Command, name string) *cobra.Command {
	for _, c := range cmd.Commands() {
		if c.Name() == name {
			return c
		}
	}
	return nil
}
