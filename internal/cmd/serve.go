package cmd

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/fabianoflorentino/certificate-validate/internal/api"
	"github.com/fabianoflorentino/certificate-validate/internal/checker"
	"github.com/fabianoflorentino/certificate-validate/internal/config"
	"github.com/fabianoflorentino/certificate-validate/internal/fetcher"
	"github.com/fabianoflorentino/certificate-validate/internal/formatter"
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
