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

	"github.com/fabianoflorentino/certificate-validate/internal/certificate"
	"github.com/fabianoflorentino/certificate-validate/internal/checker"
	"github.com/fabianoflorentino/certificate-validate/internal/config"
	"github.com/fabianoflorentino/certificate-validate/internal/fetcher"
	"github.com/fabianoflorentino/certificate-validate/internal/formatter"
	"github.com/fabianoflorentino/certificate-validate/internal/notifier"
	"github.com/spf13/cobra"
)

var (
	watch     bool
	output    string
	checkHost string
	checkPort int
	minDays   int
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check certificates from a remote host",
	Long: `Fetch and display SSL/TLS certificate information.

Without --host, reads all hosts from the configuration file.
With --host, checks a single host directly without a config file.

Use --watch to run periodically from the configuration.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if checkHost != "" {
			return runCheckHost(cmd, checkHost, checkPort)
		}

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
			if cfg.Webhook.URL != "" {
				interval := time.Duration(cfg.Webhook.Interval) * time.Second
				if interval <= 0 {
					interval = time.Duration(cfg.CheckTime) * time.Second
				}
				n := notifier.New(notifier.Config{
					URL:       cfg.Webhook.URL,
					Threshold: cfg.Webhook.Threshold,
					Interval:  interval,
				}, app, hosts)
				n.Start(ctx)
				slog.Info("webhook notifier enabled",
					"threshold", cfg.Webhook.Threshold,
					"interval", interval,
				)
			}

			checkTime := time.Duration(cfg.CheckTime) * time.Second
			slog.Info("starting watch loop", "interval", checkTime)
			runWatchLoop(ctx, app, hosts, checkTime)
			return nil
		}

		certs, errs := app.CheckAll(ctx, hosts, 10)

		printCerts(filterByMinDays(certs), errs)
		return nil
	},
}

func init() {
	checkCmd.Flags().BoolVarP(&watch, "watch", "w", false,
		"continuously check certificates at the configured interval")
	checkCmd.Flags().StringVarP(&output, "output", "o", "json",
		"output format: json or table")
	checkCmd.Flags().StringVar(&checkHost, "host", "",
		"check a single host directly (no config file needed)")
	checkCmd.Flags().IntVar(&checkPort, "port", 443,
		"port to use with --host (default: 443)")
	checkCmd.Flags().IntVar(&minDays, "min-days", 0,
		"only show certificates with this many or fewer days remaining (0 = show all)")
	rootCmd.AddCommand(checkCmd)
}

func buildApp(cfg *config.Config) (*checker.Checker, error) {
	rootCAs, err := fetcher.LoadRootCAs(cfg.TrustedCAs)
	if err != nil {
		return nil, fmt.Errorf("load trusted root CAs: %w", err)
	}
	perHostCAs, err := config.LoadPerHostCAs(cfg.Hosts)
	if err != nil {
		return nil, fmt.Errorf("load per-host CAs: %w", err)
	}
	f := fetcher.NewWithPerHostCAs(10*time.Second, rootCAs, perHostCAs)
	fmtter := formatter.New()
	return checker.New(f, fmtter), nil
}

func toCheckerHostsFromConfig(cfgHosts []config.HostConfig) []checker.Host {
	return config.ToCheckerHosts(cfgHosts)
}

// filterByMinDays removes certificates with DaysLeft > minDays.
// When minDays is 0, returns the input unchanged.
func filterByMinDays(certs []*certificate.Certificate) []*certificate.Certificate {
	if minDays <= 0 {
		return certs
	}
	filtered := make([]*certificate.Certificate, 0, len(certs))
	for _, c := range certs {
		if c != nil && c.DaysLeft <= minDays {
			filtered = append(filtered, c)
		}
	}
	return filtered
}

func runCheckHost(cmd *cobra.Command, hostname string, port int) error {
	f := fetcher.New(10 * time.Second)
	fmtter := formatter.New()
	c := checker.New(f, fmtter)

	ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cert, err := c.Check(ctx, hostname, port)
	if err != nil {
		return fmt.Errorf("check %s:%d: %w", hostname, port, err)
	}

	if minDays > 0 && cert != nil && cert.DaysLeft > minDays {
		return nil
	}

	switch output {
	case "table":
		data, err := formatter.FormatTable([]*certificate.Certificate{cert})
		if err != nil {
			return fmt.Errorf("format table: %w", err)
		}
		fmt.Print(string(data))
	default:
		data, _ := json.MarshalIndent(cert, "", "  ")
		fmt.Println(string(data))
	}
	return nil
}

func runWatchLoop(ctx context.Context, c checker.CertChecker, hosts []checker.Host, checkTime time.Duration) {
	for {
		select {
		case <-ctx.Done():
			slog.Info("watch loop stopped")
			return
		default:
			certs, _ := c.CheckAll(ctx, hosts, 0)
			for _, cert := range filterByMinDays(certs) {
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

func printCerts(certs []*certificate.Certificate, errs []error) {
	switch output {
	case "table":
		data, err := formatter.FormatTable(certs)
		if err != nil {
			slog.Error("format table", "error", err)
			return
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
}
