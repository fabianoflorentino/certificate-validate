package k8smonitor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// Webhook posts alerts for K8s certificates approaching expiration, with
// per-resource rate limiting to avoid alert spam.
type Webhook struct {
	url         string
	threshold   int
	minInterval time.Duration

	client      *http.Client
	mu          sync.Mutex
	lastAlerted map[string]time.Time
}

type k8sAlertPayload struct {
	Namespace  string `json:"namespace"`
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Hostname   string `json:"hostname"`
	CommonName string `json:"commonName"`
	Issuer     string `json:"issuer"`
	DaysLeft   int    `json:"daysLeft"`
	Threshold  int    `json:"threshold"`
	Message    string `json:"message"`
}

func newWebhook(url string, threshold int, minInterval time.Duration) *Webhook {
	return &Webhook{
		url:         url,
		threshold:   threshold,
		minInterval: minInterval,
		client:      &http.Client{Timeout: 15 * time.Second},
		lastAlerted: make(map[string]time.Time),
	}
}

// AlertIfNeeded posts alerts for any certificate whose days-left is at or
// below the configured threshold, respecting the rate limit interval.
func (w *Webhook) AlertIfNeeded(certs []*K8sCertificate) {
	for _, c := range certs {
		if c == nil || c.DaysLeft > w.threshold {
			continue
		}

		key := fmt.Sprintf("%s/%s/%s", c.K8sNamespace, c.K8sName, c.K8sKind)
		w.mu.Lock()
		lastAlert, alerted := w.lastAlerted[key]
		w.mu.Unlock()
		if alerted && time.Since(lastAlert) < w.minInterval {
			continue
		}

		if err := w.sendAlert(c); err != nil {
			slog.Error("kubernetes webhook alert failed", "resource", key, "error", err)
			continue
		}

		w.mu.Lock()
		w.lastAlerted[key] = time.Now()
		w.mu.Unlock()
		slog.Info("kubernetes webhook alert sent", "resource", key, "days_left", c.DaysLeft)
	}
}

func (w *Webhook) sendAlert(c *K8sCertificate) error {
	payload := k8sAlertPayload{
		Namespace:  c.K8sNamespace,
		Name:       c.K8sName,
		Kind:       string(c.K8sKind),
		Hostname:   c.Hostname,
		CommonName: c.CommonName,
		Issuer:     c.Issuer,
		DaysLeft:   c.DaysLeft,
		Threshold:  w.threshold,
		Message: fmt.Sprintf("Certificate %s/%s (%s) expires in %d days",
			c.K8sNamespace, c.K8sName, c.K8sKind, c.DaysLeft),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, w.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	return nil
}
