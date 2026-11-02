// Command webhook-receiver is a small local HTTP server used by the disposable
// Kubernetes alert demo. It appends every received request body to a JSONL file
// so the `k8s monitor` webhook payload can be inspected.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

const maxBodyBytes = 1 << 20

func main() {
	addr := flag.String("addr", "127.0.0.1:9099", "address to listen on")
	out := flag.String("out", "/tmp/webhook-alerts.jsonl", "JSONL file to append received payloads to")
	flag.Parse()

	file, err := os.OpenFile(*out, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		log.Fatalf("open output file: %v", err)
	}
	defer func() {
		_ = file.Close()
	}()

	mux := http.NewServeMux()
	mux.Handle("/", newHandler(file))

	log.Printf("webhook receiver listening on %s, writing to %s", *addr, *out)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

func newHandler(w io.Writer) http.Handler {
	var mu sync.Mutex

	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes))
		if err != nil {
			http.Error(rw, "read body", http.StatusBadRequest)
			return
		}

		payload := json.RawMessage(body)
		if !json.Valid(payload) {
			encoded, err := json.Marshal(string(body))
			if err != nil {
				http.Error(rw, "encode body", http.StatusInternalServerError)
				return
			}
			payload = encoded
		}

		record, err := json.Marshal(struct {
			ReceivedAt time.Time       `json:"receivedAt"`
			Path       string          `json:"path"`
			Payload    json.RawMessage `json:"payload"`
		}{
			ReceivedAt: time.Now().UTC(),
			Path:       r.URL.Path,
			Payload:    payload,
		})
		if err != nil {
			http.Error(rw, "encode record", http.StatusInternalServerError)
			return
		}

		mu.Lock()
		_, err = fmt.Fprintf(w, "%s\n", record)
		mu.Unlock()
		if err != nil {
			http.Error(rw, "write record", http.StatusInternalServerError)
			return
		}

		rw.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(rw, "ok\n")
	})
}
