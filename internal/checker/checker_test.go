package checker

import (
	"context"
	"errors"
	"testing"

	"github.com/fabianoflorentino/certificate-validate/internal/certificate"
)

// mockFetcher implements Fetcher for testing.
type mockFetcher struct {
	fetchFunc func(ctx context.Context, hostname string, port int) (*certificate.Certificate, error)
}

func (m *mockFetcher) Fetch(ctx context.Context, hostname string, port int) (*certificate.Certificate, error) {
	return m.fetchFunc(ctx, hostname, port)
}

// mockFormatter implements Formatter for testing.
type mockFormatter struct {
	formatFunc func(cert *certificate.Certificate) ([]byte, error)
}

func (m *mockFormatter) Format(cert *certificate.Certificate) ([]byte, error) {
	return m.formatFunc(cert)
}

func TestNew(t *testing.T) {
	c := New(&mockFetcher{}, &mockFormatter{})
	if c == nil {
		t.Fatal("New returned nil")
	}
}

func TestCheck_Success(t *testing.T) {
	want := &certificate.Certificate{Hostname: "example.com", DaysLeft: 100}
	c := New(
		&mockFetcher{
			fetchFunc: func(_ context.Context, hostname string, port int) (*certificate.Certificate, error) {
				return want, nil
			},
		},
		&mockFormatter{},
	)

	got, err := c.Check(context.Background(), "example.com", 443)
	if err != nil {
		t.Fatalf("Check() unexpected error: %v", err)
	}
	if got.Hostname != "example.com" {
		t.Errorf("got hostname %q; want %q", got.Hostname, "example.com")
	}
}

func TestCheck_FetchError(t *testing.T) {
	c := New(
		&mockFetcher{
			fetchFunc: func(_ context.Context, hostname string, port int) (*certificate.Certificate, error) {
				return nil, errors.New("connection refused")
			},
		},
		&mockFormatter{},
	)

	_, err := c.Check(context.Background(), "example.com", 443)
	if err == nil {
		t.Fatal("Check() expected error, got nil")
	}
}

func TestFormat(t *testing.T) {
	want := []byte(`{"hostname":"example.com"}`)
	c := New(
		&mockFetcher{},
		&mockFormatter{
			formatFunc: func(cert *certificate.Certificate) ([]byte, error) {
				return want, nil
			},
		},
	)

	got, err := c.Format(&certificate.Certificate{Hostname: "example.com"})
	if err != nil {
		t.Fatalf("Format() unexpected error: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("got %s; want %s", string(got), string(want))
	}
}

func TestCheckAll_Success(t *testing.T) {
	c := New(
		&mockFetcher{
			fetchFunc: func(_ context.Context, hostname string, port int) (*certificate.Certificate, error) {
				return &certificate.Certificate{Hostname: hostname, Port: port}, nil
			},
		},
		&mockFormatter{},
	)

	hosts := []Host{
		{Hostname: "a.com", Port: 443, Name: "A"},
		{Hostname: "b.com", Port: 80, Name: "B"},
	}

	certs, errs := c.CheckAll(context.Background(), hosts, 5)
	if len(errs) > 0 {
		t.Fatalf("CheckAll() unexpected errors: %v", errs)
	}
	if len(certs) != 2 {
		t.Fatalf("got %d certificates; want 2", len(certs))
	}
	if certs[0].Hostname != "a.com" {
		t.Errorf("certs[0].Hostname = %q; want %q", certs[0].Hostname, "a.com")
	}
	if certs[1].Hostname != "b.com" {
		t.Errorf("certs[1].Hostname = %q; want %q", certs[1].Hostname, "b.com")
	}
}

func TestCheckAll_PartialErrors(t *testing.T) {
	c := New(
		&mockFetcher{
			fetchFunc: func(_ context.Context, hostname string, port int) (*certificate.Certificate, error) {
				if hostname == "bad.com" {
					return nil, errors.New("timeout")
				}
				return &certificate.Certificate{Hostname: hostname, Port: port}, nil
			},
		},
		&mockFormatter{},
	)

	hosts := []Host{
		{Hostname: "good.com", Port: 443, Name: "good"},
		{Hostname: "bad.com", Port: 443, Name: "bad"},
	}

	certs, errs := c.CheckAll(context.Background(), hosts, 5)
	if len(errs) != 1 {
		t.Errorf("got %d errors; want 1", len(errs))
	}
	if len(certs) != 2 {
		t.Fatalf("got %d certificates; want 2", len(certs))
	}
	if certs[0] == nil {
		t.Error("certs[0] is nil; expected good cert")
	}
	if certs[1] != nil {
		t.Error("certs[1] is not nil; expected nil for failed host")
	}
}

func TestCheckAll_EmptyHosts(t *testing.T) {
	c := New(&mockFetcher{}, &mockFormatter{})

	certs, errs := c.CheckAll(context.Background(), nil, 5)
	if len(errs) > 0 {
		t.Errorf("expected no errors; got %v", errs)
	}
	if len(certs) != 0 {
		t.Errorf("got %d certificates; want 0", len(certs))
	}
}

func TestCheckAll_DefaultMaxParallel(t *testing.T) {
	c := New(
		&mockFetcher{
			fetchFunc: func(_ context.Context, hostname string, port int) (*certificate.Certificate, error) {
				return &certificate.Certificate{Hostname: hostname}, nil
			},
		},
		&mockFormatter{},
	)

	hosts := []Host{{Hostname: "a.com", Port: 443}}
	certs, errs := c.CheckAll(context.Background(), hosts, 0)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(certs) != 1 {
		t.Errorf("got %d certificates; want 1", len(certs))
	}
}

func TestCertCheckerInterface(t *testing.T) {
	var _ CertChecker = (*Checker)(nil)
}
