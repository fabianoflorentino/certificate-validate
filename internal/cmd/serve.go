package cmd

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/fabianoflorentino/certificate-validate/internal/api"
	"github.com/fabianoflorentino/certificate-validate/internal/checker"
	"github.com/fabianoflorentino/certificate-validate/internal/config"
	"github.com/fabianoflorentino/certificate-validate/internal/fetcher"
	"github.com/fabianoflorentino/certificate-validate/internal/formatter"
	"github.com/fabianoflorentino/certificate-validate/internal/metrics"
	"github.com/fabianoflorentino/certificate-validate/internal/notifier"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the HTTP API server",
	Long: `Start the REST API server that serves certificate information
via HTTP endpoints.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		f := fetcher.New(10 * time.Second)
		fmtter := formatter.New()
		c := checker.New(f, fmtter)
		h := api.New(c, cfg)

		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel()

		if cfg.Prometheus.Enabled {
			hosts := toCheckerHosts(cfg.Hosts)
			metrics.StartUpdater(ctx, c, hosts, time.Duration(cfg.CheckTime)*time.Second)
			log.Println("Prometheus metrics enabled")
		}

		if cfg.Webhook.URL != "" {
			hosts := toCheckerHosts(cfg.Hosts)
			n := notifier.New(notifier.Config{
				URL:       cfg.Webhook.URL,
				Threshold: cfg.Webhook.Threshold,
				Interval:  time.Duration(cfg.Webhook.Interval) * time.Second,
			}, c, hosts)
			n.Start(ctx)
			log.Printf("Webhook notifier enabled (threshold: %d days, interval: %ds)",
				cfg.Webhook.Threshold, cfg.Webhook.Interval)
		}

		addr := fmt.Sprintf("%s:%s", getAPIHost(cfg), getAPIPort(cfg))
		log.Printf("Starting API server on %s", addr)

		server := &http.Server{
			Addr:         addr,
			Handler:      h.Router(),
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  60 * time.Second,
		}

		return server.ListenAndServe()
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

func getAPIHost(cfg *config.Config) string {
	for _, app := range cfg.AppConfigs {
		if app.Host != "" {
			return app.Host
		}
	}
	return "0.0.0.0"
}

func getAPIPort(cfg *config.Config) string {
	for _, app := range cfg.AppConfigs {
		if app.Port != "" {
			return app.Port
		}
	}
	return "5000"
}

func toCheckerHosts(cfgHosts []config.HostConfig) []checker.Host {
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
