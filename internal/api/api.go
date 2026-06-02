package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/fabianoflorentino/certificate-validate/internal/checker"
	"github.com/fabianoflorentino/certificate-validate/internal/config"
)

// Handler holds dependencies for HTTP handlers.
type Handler struct {
	checker *checker.Checker
	cfg     *config.Config
}

// New creates a new Handler with the given dependencies.
func New(c *checker.Checker, cfg *config.Config) *Handler {
	return &Handler{
		checker: c,
		cfg:     cfg,
	}
}

// Router returns an http.Handler with all routes registered.
func (h *Handler) Router() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/cert/info/all", h.handleAll)
	mux.HandleFunc("GET /api/v1/cert/info/{hostname}", h.handleByHostname)
	mux.HandleFunc("GET /api/v1/cert/info/commonName", h.handleCommonName)
	mux.HandleFunc("GET /api/v1/cert/info/subjectAltName", h.handleSubjectAltName)
	return withMiddleware(mux)
}

func (h *Handler) handleAll(w http.ResponseWriter, r *http.Request) {
	hosts := toCheckerHosts(h.cfg.Hosts)
	results, errs := h.checker.CheckAll(r.Context(), hosts, 10)

	certs := make([]json.RawMessage, 0, len(results))
	for _, data := range results {
		if data != nil {
			certs = append(certs, data)
		}
	}

	errMessages := make([]string, 0, len(errs))
	for _, err := range errs {
		errMessages = append(errMessages, err.Error())
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"certificates": certs,
		"errors":       errMessages,
	})
}

func (h *Handler) handleByHostname(w http.ResponseWriter, r *http.Request) {
	hostname := r.PathValue("hostname")
	if hostname == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "hostname is required"})
		return
	}

	hosts := toCheckerHosts(h.cfg.Hosts)
	for _, host := range hosts {
		if host.Hostname == hostname {
			cert, err := h.checker.Check(r.Context(), host.Hostname, host.Port)
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
	hosts := toCheckerHosts(h.cfg.Hosts)
	names := make(map[string]string, len(hosts))

	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, host := range hosts {
		wg.Add(1)
		host := host
		go func() {
			defer wg.Done()
			cert, err := h.checker.Check(r.Context(), host.Hostname, host.Port)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				names[host.Name] = fmt.Sprintf("error: %v", err)
				return
			}
			names[host.Name] = cert.CommonName
		}()
	}
	wg.Wait()

	writeJSON(w, http.StatusOK, names)
}

func (h *Handler) handleSubjectAltName(w http.ResponseWriter, r *http.Request) {
	hosts := toCheckerHosts(h.cfg.Hosts)
	sans := make(map[string][]string, len(hosts))

	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, host := range hosts {
		wg.Add(1)
		host := host
		go func() {
			defer wg.Done()
			cert, err := h.checker.Check(r.Context(), host.Hostname, host.Port)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				return
			}
			sans[host.Name] = cert.SubjectAltNames
		}()
	}
	wg.Wait()

	writeJSON(w, http.StatusOK, sans)
}

func toCheckerHosts(cfgHosts []config.HostConfig) []checker.Host {
	hosts := make([]checker.Host, 0, len(cfgHosts))
	for _, h := range cfgHosts {
		hosts = append(hosts, checker.Host{
			Hostname: h.URL,
			Port:     h.PortInt(),
			Name:     h.Name,
		})
	}
	return hosts
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("json encode error: %v", err)
	}
}

func withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		log.Printf("%s %s %s", r.Method, r.URL.Path, r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}
