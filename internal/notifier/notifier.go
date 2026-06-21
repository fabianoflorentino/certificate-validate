package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/fabianoflorentino/certificate-validate/internal/certificate"
	"github.com/fabianoflorentino/certificate-validate/internal/checker"
)

// Config holds webhook notification settings.
type Config struct {
	URL       string
	Threshold int
	Interval  time.Duration
}

// Notifier sends webhook alerts when certificates approach expiration.
type Notifier struct {
	cfg     Config
	checker checker.CertChecker
	hosts   []checker.Host
	client  *http.Client

	mu          sync.Mutex
	lastAlerted map[string]time.Time
}

type alertPayload struct {
	Hostname   string `json:"hostname"`
	Port       int    `json:"port"`
	CommonName string `json:"commonName"`
	Issuer     string `json:"issuer"`
	DaysLeft   int    `json:"daysLeft"`
	Threshold  int    `json:"threshold"`
	Message    string `json:"message"`
}

// New creates a new Notifier.
func New(cfg Config, c checker.CertChecker, hosts []checker.Host) *Notifier {
	return &Notifier{
		cfg:         cfg,
		checker:     c,
		hosts:       hosts,
		client:      &http.Client{Timeout: 15 * time.Second},
		lastAlerted: make(map[string]time.Time),
	}
}

// Start begins the periodic alert check loop.
func (n *Notifier) Start(ctx context.Context) {
	ticker := time.NewTicker(n.cfg.Interval)
	go func() {
		n.checkAndAlert(ctx)
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				n.checkAndAlert(ctx)
			}
		}
	}()
}

func (n *Notifier) checkAndAlert(ctx context.Context) {
	for _, h := range n.hosts {
		select {
		case <-ctx.Done():
			return
		default:
		}

		cert, err := n.checker.Check(ctx, h.Hostname, h.Port)
		if err != nil {
			slog.Error("notifier check error", "host", h.Hostname, "port", h.Port, "error", err)
			continue
		}

		if cert.DaysLeft > n.cfg.Threshold {
			continue
		}

		key := fmt.Sprintf("%s:%d", cert.Hostname, cert.Port)
		n.mu.Lock()
		lastAlert, alerted := n.lastAlerted[key]
		n.mu.Unlock()

		// Re-alert only after threshold interval
		if alerted && time.Since(lastAlert) < n.cfg.Interval {
			continue
		}

		if err := n.sendAlert(cert); err != nil {
			slog.Error("notifier alert send failed", "host", key, "error", err)
			continue
		}

		n.mu.Lock()
		n.lastAlerted[key] = time.Now()
		n.mu.Unlock()
		slog.Info("notifier alert sent", "host", key, "days_left", cert.DaysLeft)
	}
}

func (n *Notifier) sendAlert(cert *certificate.Certificate) error {
	payload := alertPayload{
		Hostname:   cert.Hostname,
		Port:       cert.Port,
		CommonName: cert.CommonName,
		Issuer:     cert.Issuer,
		DaysLeft:   cert.DaysLeft,
		Threshold:  n.cfg.Threshold,
		Message:    fmt.Sprintf("Certificate for %s expires in %d days", cert.Hostname, cert.DaysLeft),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, n.cfg.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	return nil
}
