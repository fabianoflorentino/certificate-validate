package fetcher

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"net"
	"strconv"
	"testing"
	"time"
)

// Compile-time check that *tlsFetcher implements Fetcher.
var _ Fetcher = (*tlsFetcher)(nil)

func generateSelfSignedCert(t *testing.T) tls.Certificate {
	t.Helper()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "test.example.com",
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	return tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  priv,
	}
}

func startTLSServer(t *testing.T, cert tls.Certificate) (string, func()) {
	t.Helper()

	config := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	listener, err := tls.Listen("tcp", "127.0.0.1:0", config)
	if err != nil {
		t.Fatalf("failed to start TLS listener: %v", err)
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer func() { _ = c.Close() }()
				tlsConn := c.(*tls.Conn)
				_ = tlsConn.Handshake()
				time.Sleep(100 * time.Millisecond)
			}(conn)
		}
	}()

	return listener.Addr().String(), func() { _ = listener.Close() }
}

func TestNew(t *testing.T) {
	tests := []struct {
		name        string
		timeout     time.Duration
		wantTimeout time.Duration
	}{
		{
			name:        "positive timeout",
			timeout:     5 * time.Second,
			wantTimeout: 5 * time.Second,
		},
		{
			name:        "zero timeout defaults to 10s",
			timeout:     0,
			wantTimeout: 10 * time.Second,
		},
		{
			name:        "negative timeout defaults to 10s",
			timeout:     -1 * time.Second,
			wantTimeout: 10 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := New(tt.timeout)
			tf, ok := f.(*tlsFetcher)
			if !ok {
				t.Fatalf("expected *tlsFetcher, got %T", f)
			}
			if tf.timeout != tt.wantTimeout {
				t.Errorf("timeout = %v, want %v", tf.timeout, tt.wantTimeout)
			}
		})
	}
}

func TestFetch(t *testing.T) {
	cert := generateSelfSignedCert(t)
	addr, cleanup := startTLSServer(t, cert)
	defer cleanup()

	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("failed to split host port: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("failed to parse port: %v", err)
	}

	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatalf("failed to parse generated certificate: %v", err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(x509Cert)

	f := NewWithRootCAs(5*time.Second, pool)

	t.Run("successful fetch returns certificate with expected fields", func(t *testing.T) {
		got, err := f.Fetch(context.Background(), host, port)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == nil {
			t.Fatal("expected certificate, got nil")
		}
		if got.Hostname != host {
			t.Errorf("Hostname = %q, want %q", got.Hostname, host)
		}
		if got.Port != port {
			t.Errorf("Port = %d, want %d", got.Port, port)
		}
		if got.CommonName != "test.example.com" {
			t.Errorf("CommonName = %q, want %q", got.CommonName, "test.example.com")
		}
		if got.TLSVersion == "" {
			t.Error("TLSVersion is empty")
		}
		if got.CipherSuite == "" {
			t.Error("CipherSuite is empty")
		}
		if len(got.Chain) == 0 {
			t.Error("expected certificate chain to have entries")
		}
	})
}

func TestFetch_ContextCancelled(t *testing.T) {
	t.Run("cancelled context with unreachable host returns context error", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		// Grab a random port and immediately close it so the dial fails.
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("failed to create listener: %v", err)
		}
		addr := ln.Addr().String()
		if err := ln.Close(); err != nil {
			t.Fatalf("failed to close listener: %v", err)
		}

		host, portStr, err := net.SplitHostPort(addr)
		if err != nil {
			t.Fatalf("failed to split host port: %v", err)
		}
		port, err := strconv.Atoi(portStr)
		if err != nil {
			t.Fatalf("failed to parse port: %v", err)
		}

		f := New(1 * time.Second)
		_, err = f.Fetch(ctx, host, port)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	})
}

func TestFetcherInterfaceSatisfied(t *testing.T) {
	f := New(5 * time.Second)
	if f == nil {
		t.Fatal("expected non-nil Fetcher")
	}
}

func TestFetcherInterfaceCompliance(t *testing.T) {
	// Runtime verification that the returned value can be used as Fetcher.
	f := New(10 * time.Second)
	ctx := context.Background()

	// We only verify the method signature is callable via the interface;
	// a real successful call requires a running TLS server.
	cert, err := f.Fetch(ctx, "invalid.host.local.test", 1)
	if err == nil {
		t.Fatal("expected error for invalid host")
	}
	if cert != nil {
		t.Error("expected nil certificate for failed fetch")
	}
}
