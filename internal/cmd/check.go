package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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

var (
	watch  bool
	output string
)

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

		warnings, err := cfg.Validate()
		if err != nil {
			return fmt.Errorf("invalid config: %w", err)
		}
		for _, w := range warnings {
			slog.Warn("config warning", "warning", w)
		}

		app, err := buildApp(cfg)
		if err != nil {
			return err
		}

		hosts := toCheckerHostsFromConfig(cfg.Hosts)

		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		if watch {
			checkTime := time.Duration(cfg.CheckTime) * time.Second
			slog.Info("starting watch loop", "interval", checkTime)
			runWatchLoop(ctx, app, hosts, checkTime)
			return nil
		}

		certs, errs := app.CheckAll(ctx, hosts, 10)

		switch output {
		case "table":
			data, err := formatter.FormatTable(certs)
			if err != nil {
				return fmt.Errorf("format table: %w", err)
			}
			fmt.Print(string(data))
		default:
			for _, c := range certs {
				if c != nil {
					data, _ := json.MarshalIndent(c, "", "  ")
					fmt.Println(string(data))
				}
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
	checkCmd.Flags().StringVarP(&output, "output", "o", "json",
		"output format: json or table")
	rootCmd.AddCommand(checkCmd)
}

func buildApp(cfg *config.Config) (*checker.Checker, error) {
	rootCAs, err := fetcher.LoadRootCAs(cfg.TrustedCAs)
	if err != nil {
		return nil, fmt.Errorf("load trusted root CAs: %w", err)
	}
	f := fetcher.NewWithRootCAs(10*time.Second, rootCAs)
	fmtter := formatter.New()
	return checker.New(f, fmtter), nil
}

func toCheckerHostsFromConfig(cfgHosts []config.HostConfig) []checker.Host {
	return config.ToCheckerHosts(cfgHosts)
}

func runWatchLoop(ctx context.Context, c checker.CertChecker, hosts []checker.Host, checkTime time.Duration) {
	for {
		select {
		case <-ctx.Done():
			slog.Info("watch loop stopped")
			return
		default:
			certs, _ := c.CheckAll(ctx, hosts, 0)
			for _, cert := range certs {
				if cert != nil {
					if data, err := json.MarshalIndent(cert, "", "  "); err == nil {
						fmt.Println(string(data))
					}
				}
			}
			slog.Info("waiting before next check", "interval", checkTime)
			time.Sleep(checkTime)
		}
	}
}
