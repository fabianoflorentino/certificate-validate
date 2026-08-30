package k8smonitor

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Monitor orchestrates scanning, analyzing, and alerting on Kubernetes TLS
// certificates.
type Monitor struct {
	client     *Client
	analyzer   *Analyzer
	namespaces []string
	webhook    *Webhook
}

// Config configures the Kubernetes monitor.
type Config struct {
	Kubeconfig       string
	Namespaces       []string
	CheckRevocation  bool
	WatchInterval    time.Duration
	WebhookURL       string
	WebhookThreshold int
	WebhookInterval  time.Duration
}

// NewMonitor builds a Monitor from the given configuration, connecting to the
// cluster via kubeconfig (or in-cluster/default fallback).
func NewMonitor(cfg Config) (*Monitor, error) {
	client, err := NewClient(cfg.Kubeconfig)
	if err != nil {
		return nil, err
	}

	m := &Monitor{
		client:     client,
		analyzer:   NewAnalyzer(cfg.CheckRevocation),
		namespaces: cfg.Namespaces,
	}

	if cfg.WebhookURL != "" {
		interval := cfg.WebhookInterval
		if interval <= 0 {
			interval = 5 * time.Minute
		}
		m.webhook = newWebhook(cfg.WebhookURL, cfg.WebhookThreshold, interval)
	}

	return m, nil
}

// Scan performs a single scan of the cluster and returns the collected
// certificates.
func (m *Monitor) Scan(ctx context.Context) ([]*K8sCertificate, error) {
	secrets, ingresses, err := m.client.Discover(ctx, m.namespaces)
	if err != nil {
		return nil, fmt.Errorf("discover cluster resources: %w", err)
	}

	var certs []*K8sCertificate
	for _, s := range secrets {
		cert := m.scanSecretCert(ctx, s)
		if cert != nil {
			certs = append(certs, cert)
		}
	}

	for _, ing := range ingresses {
		for _, t := range ing.TLS {
			if t.SecretName == "" {
				continue
			}
			secret, err := m.client.Clientset.CoreV1().
				Secrets(ing.Namespace).Get(ctx, t.SecretName, metav1.GetOptions{})
			if err != nil {
				slog.Error("get ingress referenced secret",
					"namespace", ing.Namespace, "secret", t.SecretName, "error", err)
				continue
			}
			cert := m.analyzeData(secret.Data, ing.Namespace, t.SecretName, KindIngress,
				secret.Labels, secret.Annotations)
			if cert != nil {
				certs = append(certs, cert)
			}
		}
	}

	return certs, nil
}

func (m *Monitor) scanSecretCert(ctx context.Context, s *SecretCert) *K8sCertificate {
	return m.analyzeData(s.Data, s.Namespace, s.Name, KindSecret, s.Labels, s.Annotations)
}

func (m *Monitor) analyzeData(data map[string][]byte, ns, name string, kind Kind, labels, annotations map[string]string) *K8sCertificate {
	leaf, chain, err := m.analyzer.ParseBundle(data["tls.crt"])
	if err != nil {
		slog.Error("parse certificate",
			"namespace", ns, "name", name, "kind", kind, "error", err)
		return nil
	}
	return m.analyzer.Analyze(leaf, chain, ns, name, kind, labels, annotations)
}

// Run executes the monitor. When a watch interval is set, it scans
// periodically and posts webhook alerts on expiring/expired certificates.
// Otherwise it performs a single scan and prints the results.
func (m *Monitor) Run(ctx context.Context, watchInterval time.Duration) error {
	if watchInterval <= 0 {
		certs, err := m.Scan(ctx)
		if err != nil {
			return err
		}
		UpdateMetrics(certs)
		printCerts(certs)
		slog.Info("kubernetes monitor scan complete", "certificates", len(certs))
		return nil
	}

	return m.watchLoop(ctx, watchInterval)
}

func (m *Monitor) watchLoop(ctx context.Context, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	runOnce := func() {
		certs, err := m.Scan(ctx)
		if err != nil {
			slog.Error("kubernetes monitor scan failed", "error", err)
			return
		}
		UpdateMetrics(certs)
		if m.webhook != nil {
			m.webhook.AlertIfNeeded(certs)
		}
		slog.Info("kubernetes monitor scan", "certificates", len(certs))
	}

	runOnce()
	for {
		select {
		case <-ctx.Done():
			slog.Info("kubernetes monitor stopped")
			return nil
		case <-ticker.C:
			runOnce()
		}
	}
}

func printCerts(certs []*K8sCertificate) {
	for _, c := range certs {
		if c == nil {
			continue
		}
		data, err := json.MarshalIndent(c, "", "  ")
		if err != nil {
			continue
		}
		fmt.Println(string(data))
	}
}
