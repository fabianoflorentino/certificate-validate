package history

import (
	"bufio"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fabianoflorentino/certificate-validate/internal/certificate"
	"github.com/fabianoflorentino/certificate-validate/internal/checker"
)

// Entry is a single history record for one certificate check.
type Entry struct {
	Host      string `json:"host"`
	DaysLeft  int    `json:"daysLeft"`
	Timestamp string `json:"ts"`
}

// Config holds the recorder settings.
type Config struct {
	FilePath   string
	MaxEntries int
	MaxDays    int
}

// Store is the interface for recording and querying certificate history.
// Consumers (api) depend on this, not on the concrete Recorder.
type Store interface {
	Record(results []*certificate.Certificate)
	GetHistory(hostname string) ([]Entry, error)
}

// Compile-time check: *Recorder implements Store.
var _ Store = (*Recorder)(nil)

// Recorder manages append and rotation of the JSONL history file.
type Recorder struct {
	mu         sync.Mutex
	path       string
	maxEntries int
	maxDays    int
}

// New creates a Recorder. Defaults: data/history.jsonl, 10000 entries, 90 days.
func New(cfg Config) *Recorder {
	if cfg.FilePath == "" {
		cfg.FilePath = "data/history.jsonl"
	}
	if cfg.MaxEntries <= 0 {
		cfg.MaxEntries = 10000
	}
	if cfg.MaxDays <= 0 {
		cfg.MaxDays = 90
	}
	return &Recorder{
		path:       cfg.FilePath,
		maxEntries: cfg.MaxEntries,
		maxDays:    cfg.MaxDays,
	}
}

// Record appends certificate check results to the history file.
func (r *Recorder) Record(results []*certificate.Certificate) {
	if len(results) == 0 {
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)

	r.mu.Lock()
	defer r.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(r.path), 0755); err != nil {
		slog.Error("failed to create history dir", "error", err)
		return
	}

	f, err := os.OpenFile(r.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		slog.Error("failed to open history file", "error", err)
		return
	}
	defer func() {
		_ = f.Close()
	}()

	enc := json.NewEncoder(f)
	for _, c := range results {
		if c == nil {
			continue
		}
		if err := enc.Encode(Entry{
			Host:      c.Hostname,
			DaysLeft:  c.DaysLeft,
			Timestamp: now,
		}); err != nil {
			slog.Error("failed to encode history entry", "error", err)
		}
	}

	r.rotate()
}

// GetHistory returns entries for a given host, newest first.
func (r *Recorder) GetHistory(hostname string) ([]Entry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entries, err := r.readAll()
	if err != nil {
		return nil, err
	}

	filtered := make([]Entry, 0, len(entries))
	for _, e := range entries {
		if e.Host == hostname {
			filtered = append(filtered, e)
		}
	}

	// Reverse to newest first
	for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
		filtered[i], filtered[j] = filtered[j], filtered[i]
	}

	return filtered, nil
}

// StartRecorder periodically fetches all hosts and records history.
func StartRecorder(ctx context.Context, r *Recorder, c *checker.Checker, hosts []checker.Host, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		updateAndRecord(ctx, r, c, hosts)
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				updateAndRecord(ctx, r, c, hosts)
			}
		}
	}()
}

func updateAndRecord(ctx context.Context, r *Recorder, c *checker.Checker, hosts []checker.Host) {
	checkCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	certs, errs := c.CheckAll(checkCtx, hosts, 10)
	if len(errs) > 0 {
		for _, err := range errs {
			slog.Error("history fetch error", "error", err)
		}
	}
	r.Record(certs)
}

func (r *Recorder) readAll() ([]Entry, error) {
	f, err := os.Open(r.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer func() {
		_ = f.Close()
	}()

	var entries []Entry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e Entry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	return entries, scanner.Err()
}

func (r *Recorder) rotate() {
	entries, err := r.readAll()
	if err != nil || len(entries) == 0 {
		return
	}

	cutoff := time.Now().AddDate(0, 0, -r.maxDays)

	filtered := make([]Entry, 0, len(entries))
	for _, e := range entries {
		ts, err := time.Parse(time.RFC3339, e.Timestamp)
		if err != nil || ts.After(cutoff) {
			filtered = append(filtered, e)
		}
	}

	if len(filtered) > r.maxEntries {
		filtered = filtered[len(filtered)-r.maxEntries:]
	}

	if len(filtered) >= len(entries) {
		return
	}

	if err := r.rewrite(filtered); err != nil {
		slog.Error("history rotate rewrite failed", "error", err)
	}
}

func (r *Recorder) rewrite(entries []Entry) error {
	f, err := os.Create(r.path)
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
	}()

	enc := json.NewEncoder(f)
	for _, e := range entries {
		if err := enc.Encode(e); err != nil {
			return err
		}
	}
	return nil
}
