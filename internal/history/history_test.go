package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fabianoflorentino/certificate-validate/internal/certificate"
)

func TestNewWithEmptyConfig(t *testing.T) {
	r := New(Config{})

	if r.path != "data/history.jsonl" {
		t.Errorf("expected path %q, got %q", "data/history.jsonl", r.path)
	}
	if r.maxEntries != 10000 {
		t.Errorf("expected maxEntries %d, got %d", 10000, r.maxEntries)
	}
	if r.maxDays != 90 {
		t.Errorf("expected maxDays %d, got %d", 90, r.maxDays)
	}
}

func TestNewWithCustomConfig(t *testing.T) {
	cfg := Config{
		FilePath:   "/custom/path/history.jsonl",
		MaxEntries: 500,
		MaxDays:    30,
	}
	r := New(cfg)

	if r.path != "/custom/path/history.jsonl" {
		t.Errorf("expected path %q, got %q", "/custom/path/history.jsonl", r.path)
	}
	if r.maxEntries != 500 {
		t.Errorf("expected maxEntries %d, got %d", 500, r.maxEntries)
	}
	if r.maxDays != 30 {
		t.Errorf("expected maxDays %d, got %d", 30, r.maxDays)
	}
}

func TestRecordCreatesFileAndAppends(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "history.jsonl")

	r := New(Config{FilePath: path, MaxEntries: 10, MaxDays: 90})

	certs := []*certificate.Certificate{
		{Hostname: "example.com", DaysLeft: 30},
	}
	r.Record(certs)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("expected history file to be created")
	}

	entries, err := r.readAll()
	if err != nil {
		t.Fatalf("readAll failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Host != "example.com" {
		t.Errorf("expected host %q, got %q", "example.com", entries[0].Host)
	}
	if entries[0].DaysLeft != 30 {
		t.Errorf("expected daysLeft %d, got %d", 30, entries[0].DaysLeft)
	}

	// Append a second entry
	certs2 := []*certificate.Certificate{
		{Hostname: "example.org", DaysLeft: 60},
	}
	r.Record(certs2)

	entries, err = r.readAll()
	if err != nil {
		t.Fatalf("readAll failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[1].Host != "example.org" {
		t.Errorf("expected second host %q, got %q", "example.org", entries[1].Host)
	}
}

func TestRecordSkipsNilCertificate(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "history.jsonl")

	r := New(Config{FilePath: path, MaxEntries: 10, MaxDays: 90})

	certs := []*certificate.Certificate{
		{Hostname: "example.com", DaysLeft: 30},
		nil,
		{Hostname: "example.org", DaysLeft: 60},
	}
	r.Record(certs)

	entries, err := r.readAll()
	if err != nil {
		t.Fatalf("readAll failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (nil skipped), got %d", len(entries))
	}
	if entries[0].Host != "example.com" {
		t.Errorf("expected first host %q, got %q", "example.com", entries[0].Host)
	}
	if entries[1].Host != "example.org" {
		t.Errorf("expected second host %q, got %q", "example.org", entries[1].Host)
	}
}

func TestRecordWithEmptyResults(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "history.jsonl")

	r := New(Config{FilePath: path, MaxEntries: 10, MaxDays: 90})

	// Record something first
	r.Record([]*certificate.Certificate{
		{Hostname: "example.com", DaysLeft: 30},
	})

	// Now record empty slice
	r.Record([]*certificate.Certificate{})

	entries, err := r.readAll()
	if err != nil {
		t.Fatalf("readAll failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
}

func TestGetHistoryFiltersByHostnameAndReturnsNewestFirst(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "history.jsonl")

	r := New(Config{FilePath: path, MaxEntries: 10, MaxDays: 90})

	now := time.Now().UTC()
	// Manually write entries with different timestamps
	entries := []Entry{
		{Host: "a.com", DaysLeft: 10, Timestamp: now.Add(-2 * time.Hour).Format(time.RFC3339)},
		{Host: "b.com", DaysLeft: 20, Timestamp: now.Add(-1 * time.Hour).Format(time.RFC3339)},
		{Host: "a.com", DaysLeft: 15, Timestamp: now.Format(time.RFC3339)},
	}
	if err := r.rewrite(entries); err != nil {
		t.Fatalf("rewrite failed: %v", err)
	}

	got, err := r.GetHistory("a.com")
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 entries for a.com, got %d", len(got))
	}
	// Newest first
	if got[0].DaysLeft != 15 {
		t.Errorf("expected first entry DaysLeft %d, got %d", 15, got[0].DaysLeft)
	}
	if got[1].DaysLeft != 10 {
		t.Errorf("expected second entry DaysLeft %d, got %d", 10, got[1].DaysLeft)
	}
}

func TestGetHistoryNonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "does-not-exist.jsonl")

	r := New(Config{FilePath: path, MaxEntries: 10, MaxDays: 90})

	got, err := r.GetHistory("example.com")
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %v", got)
	}
}

func TestGetHistoryWithMultipleHosts(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "history.jsonl")

	r := New(Config{FilePath: path, MaxEntries: 10, MaxDays: 90})

	now := time.Now().UTC()
	entries := []Entry{
		{Host: "host1.com", DaysLeft: 5, Timestamp: now.Add(-3 * time.Hour).Format(time.RFC3339)},
		{Host: "host2.com", DaysLeft: 10, Timestamp: now.Add(-2 * time.Hour).Format(time.RFC3339)},
		{Host: "host1.com", DaysLeft: 7, Timestamp: now.Add(-1 * time.Hour).Format(time.RFC3339)},
		{Host: "host3.com", DaysLeft: 20, Timestamp: now.Format(time.RFC3339)},
	}
	if err := r.rewrite(entries); err != nil {
		t.Fatalf("rewrite failed: %v", err)
	}

	got, err := r.GetHistory("host1.com")
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 entries for host1.com, got %d", len(got))
	}
	if got[0].DaysLeft != 7 {
		t.Errorf("expected newest DaysLeft %d, got %d", 7, got[0].DaysLeft)
	}
	if got[1].DaysLeft != 5 {
		t.Errorf("expected oldest DaysLeft %d, got %d", 5, got[1].DaysLeft)
	}

	got2, err := r.GetHistory("host2.com")
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}
	if len(got2) != 1 {
		t.Fatalf("expected 1 entry for host2.com, got %d", len(got2))
	}
	if got2[0].DaysLeft != 10 {
		t.Errorf("expected DaysLeft %d, got %d", 10, got2[0].DaysLeft)
	}

	got3, err := r.GetHistory("host3.com")
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}
	if len(got3) != 1 {
		t.Fatalf("expected 1 entry for host3.com, got %d", len(got3))
	}

	got4, err := r.GetHistory("unknown.com")
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}
	if len(got4) != 0 {
		t.Fatalf("expected 0 entries for unknown.com, got %d", len(got4))
	}
}

func TestRotateHonorsMaxEntries(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "history.jsonl")

	maxEntries := 3
	r := New(Config{FilePath: path, MaxEntries: maxEntries, MaxDays: 90})

	now := time.Now().UTC()
	entries := []Entry{
		{Host: "a.com", DaysLeft: 1, Timestamp: now.Add(-4 * time.Hour).Format(time.RFC3339)},
		{Host: "b.com", DaysLeft: 2, Timestamp: now.Add(-3 * time.Hour).Format(time.RFC3339)},
		{Host: "c.com", DaysLeft: 3, Timestamp: now.Add(-2 * time.Hour).Format(time.RFC3339)},
		{Host: "d.com", DaysLeft: 4, Timestamp: now.Add(-1 * time.Hour).Format(time.RFC3339)},
		{Host: "e.com", DaysLeft: 5, Timestamp: now.Format(time.RFC3339)},
	}
	if err := r.rewrite(entries); err != nil {
		t.Fatalf("rewrite failed: %v", err)
	}

	r.rotate()

	got, err := r.readAll()
	if err != nil {
		t.Fatalf("readAll failed: %v", err)
	}
	if len(got) != maxEntries {
		t.Fatalf("expected %d entries after rotate, got %d", maxEntries, len(got))
	}
	// Should keep the newest entries
	if got[0].Host != "c.com" {
		t.Errorf("expected first host %q, got %q", "c.com", got[0].Host)
	}
	if got[1].Host != "d.com" {
		t.Errorf("expected second host %q, got %q", "d.com", got[1].Host)
	}
	if got[2].Host != "e.com" {
		t.Errorf("expected third host %q, got %q", "e.com", got[2].Host)
	}
}

func TestRotateHonorsMaxDays(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "history.jsonl")

	maxDays := 7
	r := New(Config{FilePath: path, MaxEntries: 100, MaxDays: maxDays})

	now := time.Now().UTC()
	entries := []Entry{
		{Host: "old.com", DaysLeft: 1, Timestamp: now.AddDate(0, 0, -10).Format(time.RFC3339)},
		{Host: "recent.com", DaysLeft: 2, Timestamp: now.AddDate(0, 0, -3).Format(time.RFC3339)},
		{Host: "today.com", DaysLeft: 3, Timestamp: now.Format(time.RFC3339)},
	}
	if err := r.rewrite(entries); err != nil {
		t.Fatalf("rewrite failed: %v", err)
	}

	r.rotate()

	got, err := r.readAll()
	if err != nil {
		t.Fatalf("readAll failed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 entries after rotate (old removed), got %d", len(got))
	}
	for _, e := range got {
		if e.Host == "old.com" {
			t.Errorf("expected old.com to be removed by maxDays rotation")
		}
	}
}

func TestRotateDoesNothingWhenWithinLimits(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "history.jsonl")

	r := New(Config{FilePath: path, MaxEntries: 100, MaxDays: 90})

	now := time.Now().UTC()
	entries := []Entry{
		{Host: "a.com", DaysLeft: 10, Timestamp: now.Format(time.RFC3339)},
	}
	if err := r.rewrite(entries); err != nil {
		t.Fatalf("rewrite failed: %v", err)
	}

	r.rotate()

	got, err := r.readAll()
	if err != nil {
		t.Fatalf("readAll failed: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(got))
	}
}

func TestRewriteWritesCorrectEntries(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "history.jsonl")

	r := New(Config{FilePath: path, MaxEntries: 10, MaxDays: 90})

	entries := []Entry{
		{Host: "test.com", DaysLeft: 42, Timestamp: "2024-06-01T12:00:00Z"},
		{Host: "test2.com", DaysLeft: 21, Timestamp: "2024-06-02T12:00:00Z"},
	}
	if err := r.rewrite(entries); err != nil {
		t.Fatalf("rewrite failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	// Each entry should be a JSON line
	var gotEntries []Entry
	for _, line := range splitLines(data) {
		if len(line) == 0 {
			continue
		}
		var e Entry
		if err := json.Unmarshal(line, &e); err != nil {
			t.Fatalf("failed to unmarshal entry: %v", err)
		}
		gotEntries = append(gotEntries, e)
	}

	if len(gotEntries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(gotEntries))
	}
	if gotEntries[0].Host != "test.com" {
		t.Errorf("expected host %q, got %q", "test.com", gotEntries[0].Host)
	}
	if gotEntries[0].DaysLeft != 42 {
		t.Errorf("expected daysLeft %d, got %d", 42, gotEntries[0].DaysLeft)
	}
	if gotEntries[1].Host != "test2.com" {
		t.Errorf("expected host %q, got %q", "test2.com", gotEntries[1].Host)
	}
}

func splitLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' {
			lines = append(lines, data[start:i])
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}
