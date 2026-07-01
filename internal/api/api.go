package api

import (
	"context"
	"embed"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fabianoflorentino/certificate-validate/internal/config"
	"github.com/fabianoflorentino/certificate-validate/internal/metrics"
	"github.com/fabianoflorentino/certificate-validate/internal/service"
)

//go:embed static/*
var staticFiles embed.FS

// Handler holds dependencies for HTTP handlers.
type Handler struct {
	svc *service.CertService
	cfg *config.Config
}

// New creates a new Handler with the given dependencies.
func New(svc *service.CertService, cfg *config.Config) *Handler {
	return &Handler{
		svc: svc,
		cfg: cfg,
	}
}

// Router returns an http.Handler with all routes registered.
func (h *Handler) Router() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/cert/info/all", h.handleAll)
	mux.HandleFunc("GET /api/v1/cert/info/{hostname}", h.handleByHostname)
	mux.HandleFunc("GET /api/v1/cert/info/commonName", h.handleCommonName)
	mux.HandleFunc("GET /api/v1/cert/info/subjectAltName", h.handleSubjectAltName)
	mux.HandleFunc("GET /api/v1/cert/export/json", h.handleExportJSON)
	mux.HandleFunc("GET /api/v1/cert/export/csv", h.handleExportCSV)
	mux.HandleFunc("GET /api/v1/cert/history/{hostname}", h.handleHistory)
	mux.HandleFunc("GET /health", h.handleHealth)

	if h.cfg.Prometheus.Enabled {
		mux.Handle("GET /metrics", metrics.Handler())
	}

	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		slog.Error("failed to init static file server", "error", err)
	} else {
		mux.Handle("GET /", http.FileServer(http.FS(staticFS)))
	}

	return withMiddleware(mux)
}

func (h *Handler) handleAll(w http.ResponseWriter, r *http.Request) {
	result := h.svc.CheckAll(r.Context(), h.cfg.Hosts)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"certificates": result.Certificates,
		"errors":       result.Errors,
	})
}

func (h *Handler) handleByHostname(w http.ResponseWriter, r *http.Request) {
	hostname := r.PathValue("hostname")
	if hostname == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "hostname is required"})
		return
	}

	for _, host := range h.cfg.Hosts {
		if host.URL == hostname {
			cert, err := h.svc.CheckSingle(r.Context(), host.URL, host.PortInt())
			if err != nil {
				writeJSON(w, http.StatusBadGateway, map[string]string{
					"error": fmt.Sprintf("failed to fetch certificate: %v", err),
				})
				return
			}
			writeJSON(w, http.StatusOK, cert)
			return
		}
	}

	writeJSON(w, http.StatusNotFound, map[string]string{
		"error": fmt.Sprintf("host %s not found in configuration", hostname),
	})
}

func (h *Handler) handleCommonName(w http.ResponseWriter, r *http.Request) {
	result := h.svc.CheckAll(r.Context(), h.cfg.Hosts)
	names := make(map[string]string, len(result.Certificates))
	for _, c := range result.Certificates {
		if c != nil {
			names[c.Hostname] = c.CommonName
		}
	}
	writeJSON(w, http.StatusOK, names)
}

func (h *Handler) handleSubjectAltName(w http.ResponseWriter, r *http.Request) {
	result := h.svc.CheckAll(r.Context(), h.cfg.Hosts)
	sans := make(map[string][]string, len(result.Certificates))
	for _, c := range result.Certificates {
		if c != nil {
			sans[c.Hostname] = c.SubjectAltNames
		}
	}
	writeJSON(w, http.StatusOK, sans)
}

func (h *Handler) handleExportJSON(w http.ResponseWriter, r *http.Request) {
	result := h.svc.CheckAll(r.Context(), h.cfg.Hosts)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="certificates.json"`)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"certificates": result.Certificates,
		"errors":       result.Errors,
	})
}

func (h *Handler) handleExportCSV(w http.ResponseWriter, r *http.Request) {
	result := h.svc.CheckAll(r.Context(), h.cfg.Hosts)

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="certificates.csv"`)
	if _, err := w.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		http.Error(w, "failed to write csv header", http.StatusInternalServerError)
		return
	}

	csvWriter := csv.NewWriter(w)
	if err := csvWriter.Write([]string{
		"Hostname", "Port", "Common Name", "Issuer", "Type",
		"Days Left", "Not Before", "Not After", "Subject Alt Names",
		"TLS Version", "Cipher Suite",
	}); err != nil {
		http.Error(w, "failed to write csv header", http.StatusInternalServerError)
		return
	}

	for _, cert := range result.Certificates {
		if cert == nil {
			continue
		}
		if err := csvWriter.Write([]string{
			cert.Hostname,
			strconv.Itoa(cert.Port),
			cert.CommonName,
			cert.Issuer,
			cert.Type,
			strconv.Itoa(cert.DaysLeft),
			cert.NotBefore,
			cert.NotAfter,
			strings.Join(cert.SubjectAltNames, "; "),
			cert.TLSVersion,
			cert.CipherSuite,
		}); err != nil {
			http.Error(w, "failed to write csv row", http.StatusInternalServerError)
			return
		}
	}

	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		http.Error(w, "failed to finalize csv", http.StatusInternalServerError)
		return
	}
	csvWriter.Flush()
}

func (h *Handler) handleHistory(w http.ResponseWriter, r *http.Request) {
	hostname := r.PathValue("hostname")
	if hostname == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "hostname is required"})
		return
	}

	entries, err := h.svc.GetHistory(hostname)
	if entries == nil && err == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "history not enabled"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, entries)
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	hosts := h.cfg.Hosts
	if len(hosts) == 0 {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	type hostResult struct {
		name   string
		status string
	}

	results := make(chan hostResult, len(hosts))
	for _, host := range hosts {
		host := host
		go func() {
			port := host.Port
			if port == "" && len(host.Ports) > 0 {
				port = strconv.Itoa(host.Ports[0])
			}
			if port == "" {
				port = "443"
			}
			addr := net.JoinHostPort(host.URL, port)
			dialer := net.Dialer{Timeout: 3 * time.Second}
			conn, err := dialer.DialContext(ctx, "tcp", addr)
			if err != nil {
				results <- hostResult{name: host.Name, status: "unreachable"}
				return
			}
			_ = conn.Close()
			results <- hostResult{name: host.Name, status: "ok"}
		}()
	}

	hostStatuses := make(map[string]string, len(hosts))
	overall := "ok"
	for range hosts {
		r := <-results
		hostStatuses[r.name] = r.status
		if r.status != "ok" {
			overall = "degraded"
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": overall,
		"hosts":  hostStatuses,
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("json encode error", "error", err)
	}
}

func withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		slog.Info("request", "method", r.Method, "path", r.URL.Path, "remote", r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}
