package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewHandlerRecordsJSONPayload(t *testing.T) {
	var out bytes.Buffer
	handler := newHandler(&out)

	body := `{"namespace":"default","name":"example","daysLeft":89}`
	req := httptest.NewRequest(http.MethodPost, "/alert", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Body.String(); got != "ok\n" {
		t.Fatalf("body = %q, want %q", got, "ok\n")
	}

	line := strings.TrimSpace(out.String())
	var record struct {
		Path    string          `json:"path"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal([]byte(line), &record); err != nil {
		t.Fatalf("unmarshal record: %v (line %q)", err, line)
	}
	if record.Path != "/alert" {
		t.Fatalf("path = %q, want %q", record.Path, "/alert")
	}
	if string(record.Payload) != body {
		t.Fatalf("payload = %q, want %q", record.Payload, body)
	}
}

func TestNewHandlerHealthDoesNotRecord(t *testing.T) {
	var out bytes.Buffer
	handler := newHandler(&out)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if out.Len() != 0 {
		t.Fatalf("health check wrote %q, want no output", out.String())
	}
}

func TestNewHandlerRecordsInvalidPayloadAsString(t *testing.T) {
	var out bytes.Buffer
	handler := newHandler(&out)

	req := httptest.NewRequest(http.MethodPost, "/alert", strings.NewReader("not-json"))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var record struct {
		Payload any `json:"payload"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &record); err != nil {
		t.Fatalf("unmarshal record: %v", err)
	}
	if record.Payload != "not-json" {
		t.Fatalf("payload = %#v, want %q", record.Payload, "not-json")
	}
}
