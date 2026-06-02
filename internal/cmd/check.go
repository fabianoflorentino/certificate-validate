package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fabianoflorentino/certificate-validate/internal/checker"
	"github.com/fabianoflorentino/certificate-validate/internal/config"
	"github.com/fabianoflorentino/certificate-validate/internal/fetcher"
	"github.com/fabianoflorentino/certificate-validate/internal/formatter"
	"github.com/spf13/cobra"
)

var watch bool

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check certificates from configuration",
	Long: `Fetch and display certificate information for all hosts
defined in the configuration file. Use --watch to run periodically.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		app, err := buildApp()
		if err != nil {
			return err
		}

		hosts := toCheckerHostsFromConfig(cfg.Hosts)

		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		if watch {
			checkTime := time.Duration(cfg.CheckTime) * time.Second
			log.Printf("Starting watch loop (interval: %s)", checkTime)
			app.RunWatchLoop(ctx, hosts, checkTime)
			return nil
		}

		results, errs := app.CheckAll(ctx, hosts, 10)
		for _, data := range results {
			if data != nil {
				fmt.Println(string(data))
			}
		}
		for _, err := range errs {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		}

		return nil
	},
}

func init() {
	checkCmd.Flags().BoolVarP(&watch, "watch", "w", false,
		"continuously check certificates at the configured interval")
	rootCmd.AddCommand(checkCmd)
}

func buildApp() (*checker.Checker, error) {
	f := fetcher.New(10 * time.Second)
	fmtter := formatter.New()
	return checker.New(f, fmtter), nil
}

func toCheckerHostsFromConfig(cfgHosts []config.HostConfig) []checker.Host {
	hosts := make([]checker.Host, 0, len(cfgHosts))
	for _, h := range cfgHosts {
		hosts = append(hosts, checker.Host{
			Hostname: h.URL,
			Port:     h.PortInt(),
			Name:     h.Name,
		})
	}
	return hosts
}
