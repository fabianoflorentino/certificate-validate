package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
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

type serverDeps struct {
	checker  *checker.Checker
	registry *history.Recorder
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the HTTP API server",
	Long: `Start the REST API server that serves certificate information
via HTTP endpoints. Use --tls-cert and --tls-key to serve HTTPS.
Send SIGHUP to reload configuration without restarting.`,
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

		handler, deps, err := buildDeps(cfg)
		if err != nil {
			return err
		}

			var currentHandler atomic.Value
		currentHandler.Store(handler)

		addr := fmt.Sprintf("%s:%s", getAPIHost(cfg), getAPIPort(cfg))
		server := &http.Server{
			Addr: addr,
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				currentHandler.Load().(http.Handler).ServeHTTP(w, r)
			}),
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  60 * time.Second,
		}

		ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		reloadCh := make(chan os.Signal, 1)
		signal.Notify(reloadCh, syscall.SIGHUP)

		// restartBackground cancels the previous background context and starts fresh.
		var bgCancel context.CancelFunc
		restartBackground := func() {
			if bgCancel != nil {
				bgCancel()
			}
			bgCtx, cancel := context.WithCancel(ctx)
			bgCancel = cancel
			startBackground(bgCtx, cfg, deps)
		}
		restartBackground()

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

		for {
			select {
			case <-ctx.Done():
				bgCancel()
				slog.Info("shutting down HTTP server...")

				shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()

				if err := server.Shutdown(shutdownCtx); err != nil {
					slog.Error("HTTP server shutdown error", "error", err)
				}
				slog.Info("HTTP server stopped gracefully")
				return nil

			case <-reloadCh:
				slog.Info("SIGHUP received, reloading configuration...")

				newCfg, err := config.Load(cfgPath)
				if err != nil {
					slog.Error("config reload failed", "error", err)
					continue
				}

				warnings, err := newCfg.Validate()
				if err != nil {
					slog.Error("invalid config after reload", "error", err)
					continue
				}
				for _, w := range warnings {
					slog.Warn("config warning", "warning", w)
				}

				bgCancel()
				newHandler, newDeps, err := buildDeps(newCfg)
				if err != nil {
					slog.Error("rebuild dependencies failed", "error", err)
					bgCtx, cancel := context.WithCancel(ctx)
					bgCancel = cancel
					startBackground(bgCtx, cfg, deps)
					continue
				}

				currentHandler.Store(newHandler)
				cfg = newCfg
				deps = newDeps

				restartBackground()
				slog.Info("configuration reloaded successfully")
			}
		}
	},
}

func init() {
	serveCmd.Flags().StringVarP(&tlsCertFile, "tls-cert", "", "", "path to TLS certificate file")
	serveCmd.Flags().StringVarP(&tlsKeyFile, "tls-key", "", "", "path to TLS private key file")
	rootCmd.AddCommand(serveCmd)
}

func buildDeps(cfg *config.Config) (http.Handler, *serverDeps, error) {
	rootCAs, err := fetcher.LoadRootCAs(cfg.TrustedCAs)
	if err != nil {
		return nil, nil, fmt.Errorf("load trusted root CAs: %w", err)
	}
	perHostCAs, err := config.LoadPerHostCAs(cfg.Hosts)
	if err != nil {
		return nil, nil, fmt.Errorf("load per-host CAs: %w", err)
	}
	f := fetcher.NewWithPerHostCAs(10*time.Second, rootCAs, perHostCAs)
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

	return h.Router(), &serverDeps{checker: c, registry: rec}, nil
}

func startBackground(ctx context.Context, cfg *config.Config, deps *serverDeps) {
	hosts := config.ToCheckerHosts(cfg.Hosts)

	if cfg.Prometheus.Enabled {
		metrics.StartUpdater(ctx, deps.checker, hosts, time.Duration(cfg.CheckTime)*time.Second)
		slog.Info("Prometheus metrics enabled")
	}

	if deps.registry != nil {
		history.StartRecorder(ctx, deps.registry, deps.checker, hosts, time.Duration(cfg.CheckTime)*time.Second)
	}

	if cfg.Webhook.URL != "" {
		n := notifier.New(notifier.Config{
			URL:       cfg.Webhook.URL,
			Threshold: cfg.Webhook.Threshold,
			Interval:  time.Duration(cfg.Webhook.Interval) * time.Second,
		}, deps.checker, hosts)
		n.Start(ctx)
		slog.Info("webhook notifier enabled",
			"threshold", cfg.Webhook.Threshold,
			"interval", cfg.Webhook.Interval,
		)
	}
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
