package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/fabianoflorentino/certificate-validate/internal/api"
	"github.com/fabianoflorentino/certificate-validate/internal/checker"
	"github.com/fabianoflorentino/certificate-validate/internal/config"
	"github.com/fabianoflorentino/certificate-validate/internal/fetcher"
	"github.com/fabianoflorentino/certificate-validate/internal/formatter"
	"github.com/fabianoflorentino/certificate-validate/internal/history"
	"github.com/fabianoflorentino/certificate-validate/internal/metrics"
	"github.com/fabianoflorentino/certificate-validate/internal/notifier"
	"github.com/fabianoflorentino/certificate-validate/internal/service"
	"github.com/spf13/cobra"
)

var (
	tlsCertFile string
	tlsKeyFile  string
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the HTTP API server",
	Long: `Start the REST API server that serves certificate information
via HTTP endpoints. Use --tls-cert and --tls-key to serve HTTPS.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		rootCAs, err := fetcher.LoadRootCAs(cfg.TrustedCAs)
		if err != nil {
			return fmt.Errorf("load trusted root CAs: %w", err)
		}
		f := fetcher.NewWithRootCAs(10*time.Second, rootCAs)
		fmtter := formatter.New()
		c := checker.New(f, fmtter)

		var rec *history.Recorder
		if cfg.History.Enabled {
			rec = history.New(history.Config{
				FilePath:   cfg.History.FilePath,
				MaxEntries: cfg.History.MaxEntries,
				MaxDays:    cfg.History.MaxDays,
			})
			slog.Info("history recording enabled",
				"file", cfg.History.FilePath,
				"max_entries", cfg.History.MaxEntries,
				"max_days", cfg.History.MaxDays,
			)
		}

		var mUpdater service.MetricsUpdater
		if cfg.Prometheus.Enabled {
			mUpdater = metrics.Update
		}
		svc := service.NewCertService(c, rec, mUpdater)
		h := api.New(svc, cfg)

		ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		if cfg.Prometheus.Enabled {
			hosts := config.ToCheckerHosts(cfg.Hosts)
			metrics.StartUpdater(ctx, c, hosts, time.Duration(cfg.CheckTime)*time.Second)
			slog.Info("Prometheus metrics enabled")
		}

		if rec != nil {
			hosts := config.ToCheckerHosts(cfg.Hosts)
			history.StartRecorder(ctx, rec, c, hosts, time.Duration(cfg.CheckTime)*time.Second)
		}

		if cfg.Webhook.URL != "" {
			hosts := config.ToCheckerHosts(cfg.Hosts)
			n := notifier.New(notifier.Config{
				URL:       cfg.Webhook.URL,
				Threshold: cfg.Webhook.Threshold,
				Interval:  time.Duration(cfg.Webhook.Interval) * time.Second,
			}, c, hosts)
			n.Start(ctx)
			slog.Info("webhook notifier enabled",
				"threshold", cfg.Webhook.Threshold,
				"interval", cfg.Webhook.Interval,
			)
		}

		addr := fmt.Sprintf("%s:%s", getAPIHost(cfg), getAPIPort(cfg))
		server := &http.Server{
			Addr:         addr,
			Handler:      h.Router(),
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  60 * time.Second,
		}

		go func() {
			if tlsCertFile != "" && tlsKeyFile != "" {
				slog.Info("starting HTTPS server", "addr", addr, "cert", tlsCertFile, "key", tlsKeyFile)
				if err := server.ListenAndServeTLS(tlsCertFile, tlsKeyFile); err != nil && err != http.ErrServerClosed {
					slog.Error("HTTPS server error", "error", err)
				}
			} else {
				slog.Info("starting HTTP server", "addr", addr)
				if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					slog.Error("HTTP server error", "error", err)
				}
			}
		}()

		<-ctx.Done()
		slog.Info("shutting down HTTP server...")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("HTTP server shutdown error", "error", err)
		}
		slog.Info("HTTP server stopped gracefully")
		return nil
	},
}

func init() {
	serveCmd.Flags().StringVarP(&tlsCertFile, "tls-cert", "", "", "path to TLS certificate file")
	serveCmd.Flags().StringVarP(&tlsKeyFile, "tls-key", "", "", "path to TLS private key file")
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
