package cmd

import (
	"fmt"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/fabianoflorentino/certificate-validate/internal/k8smonitor"
	"github.com/spf13/cobra"
)

var (
	k8sNamespaces       []string
	k8sKubeconfig       string
	k8sCheckRevocation  bool
	k8sWatchInterval    int
	k8sWebhookURL       string
	k8sWebhookThreshold int
	k8sWebhookInterval  int
	k8sMetricsAddr      string
)

var k8sCmd = &cobra.Command{
	Use:   "k8s",
	Short: "Kubernetes certificate monitoring commands",
	Long:  `Commands for monitoring TLS certificates managed in a Kubernetes cluster.`,
}

var k8sMonitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Monitor TLS certificates in a Kubernetes cluster",
	Long: `Scan Kubernetes Secrets and Ingresses for TLS certificates and report
their validity, chain, and revocation status.

Discovery:
  - Lists all TLS Secrets (type kubernetes.io/tls)
  - Lists all Ingresses with TLS configuration

By default performs a single scan and prints each certificate as JSON.
Use --watch to run periodically, updating Prometheus metrics and firing
webhook alerts for certificates approaching expiration.

The command connects using in-cluster configuration when running inside a
cluster, or the default kubeconfig otherwise. Use --kubeconfig to override.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := k8smonitor.NewMonitor(k8smonitor.Config{
			Kubeconfig:       k8sKubeconfig,
			Namespaces:       k8sNamespaces,
			CheckRevocation:  k8sCheckRevocation,
			WatchInterval:    time.Duration(k8sWatchInterval) * time.Second,
			WebhookURL:       k8sWebhookURL,
			WebhookThreshold: k8sWebhookThreshold,
			WebhookInterval:  time.Duration(k8sWebhookInterval) * time.Second,
		})
		if err != nil {
			return fmt.Errorf("create kubernetes monitor: %w", err)
		}

		ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		if k8sMetricsAddr != "" {
			startMetricsServer(k8sMetricsAddr)
		}

		return m.Run(ctx, time.Duration(k8sWatchInterval)*time.Second)
	},
}

func init() {
	k8sMonitorCmd.Flags().StringSliceVarP(&k8sNamespaces, "namespace", "n", nil,
		"namespaces to scan (default: all)")
	k8sMonitorCmd.Flags().StringVar(&k8sKubeconfig, "kubeconfig", "",
		"path to kubeconfig file (default: in-cluster config or ~/.kube/config)")
	k8sMonitorCmd.Flags().BoolVar(&k8sCheckRevocation, "check-revocation", false,
		"perform OCSP/CRL revocation checks")
	k8sMonitorCmd.Flags().IntVar(&k8sWatchInterval, "watch-interval", 0,
		"repeat scan every N seconds (0 = single scan)")
	k8sMonitorCmd.Flags().StringVar(&k8sWebhookURL, "webhook-url", "",
		"URL to POST alerts for certificates at or below the threshold")
	k8sMonitorCmd.Flags().IntVar(&k8sWebhookThreshold, "webhook-threshold", 15,
		"alert when days-left is at or below this value")
	k8sMonitorCmd.Flags().IntVar(&k8sWebhookInterval, "webhook-interval", 0,
		"minimum seconds between alerts for the same certificate (default: 300)")
	k8sMonitorCmd.Flags().StringVar(&k8sMetricsAddr, "metrics-addr", "",
		"address to serve Prometheus metrics (e.g. :9102)")
	k8sCmd.AddCommand(k8sMonitorCmd)
	rootCmd.AddCommand(k8sCmd)
}

func startMetricsServer(addr string) {
	go func() {
		slog.Info("serving kubernetes monitor metrics", "addr", addr)
		if err := http.ListenAndServe(addr, k8smonitor.MetricsHandler()); err != nil {
			slog.Error("metrics server error", "error", err)
		}
	}()
}
