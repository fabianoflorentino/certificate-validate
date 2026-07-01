package revocation

import (
	"context"
	"crypto/x509"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fabianoflorentino/certificate-validate/internal/certificate"
)

func TestCheckOCSP_NoServers(t *testing.T) {
	status := CheckOCSP(nil, nil, nil)
	if status != certificate.RevocationNotReady {
		t.Errorf("got %q; want %q", status, certificate.RevocationNotReady)
	}
}

func TestCheckOCSP_EmptyServers(t *testing.T) {
	status := CheckOCSP(nil, nil, []string{})
	if status != certificate.RevocationNotReady {
		t.Errorf("got %q; want %q", status, certificate.RevocationNotReady)
	}
}

func TestCheckOCSP_NilLeaf(t *testing.T) {
	status := CheckOCSP(nil, nil, []string{"http://ocsp.example.com"})
	if status != certificate.RevocationNotReady {
		t.Errorf("got %q; want %q", status, certificate.RevocationNotReady)
	}
}

func TestCheckOCSP_RequestCreationFailsReturnsNotReady(t *testing.T) {
	status := CheckOCSP(&x509.Certificate{}, nil, []string{"http://ocsp.example.com"})
	if status != certificate.RevocationNotReady {
		t.Errorf("got %q; want %q", status, certificate.RevocationNotReady)
	}
}

func TestCheckOCSP_HTTPErrorReturnsUnknown(t *testing.T) {
	original := httpPost
	defer func() { httpPost = original }()

	httpPost = func(_ context.Context, _, _ string, _ []byte) (*http.Response, error) {
		return nil, errors.New("connection refused")
	}

	status := CheckOCSP(
		&x509.Certificate{},
		&x509.Certificate{},
		[]string{"http://ocsp.example.com"},
	)
	if status != certificate.RevocationUnknown {
		t.Errorf("got %q; want %q", status, certificate.RevocationUnknown)
	}
}

func TestCheckCRL_EmptyURLs(t *testing.T) {
	status := CheckCRL(nil, nil)
	if status != certificate.RevocationUnknown {
		t.Errorf("got %q; want %q", status, certificate.RevocationUnknown)
	}
}

func TestCheckCRL_HTTPError(t *testing.T) {
	status := CheckCRL(&x509.Certificate{}, []string{"http://crl.example.com/ca.crl"})
	if status != certificate.RevocationUnknown {
		t.Errorf("got %q; want %q", status, certificate.RevocationUnknown)
	}
}

func TestLogRevocation_Revoked(t *testing.T) {
	// Just ensure it doesn't panic.
	LogRevocation(&certificate.Certificate{
		Hostname:   "example.com",
		Port:       443,
		CommonName: "*.example.com",
	}, certificate.RevocationRevoked)
}

func TestLogRevocation_Good(t *testing.T) {
	LogRevocation(&certificate.Certificate{}, certificate.RevocationGood)
}

func TestCheck_FallsBackToCRLWhenOCSPNotReady(t *testing.T) {
	status := Check(
		&x509.Certificate{},
		nil,
		nil,
		[]string{"http://crl.example.com/ca.crl"},
	)
	if status != certificate.RevocationUnknown {
		t.Errorf("got %q; want %q", status, certificate.RevocationUnknown)
	}
}

func TestCheck_NotReadyWhenBothEmpty(t *testing.T) {
	status := Check(nil, nil, nil, nil)
	if status != certificate.RevocationNotReady {
		t.Errorf("got %q; want %q", status, certificate.RevocationNotReady)
	}
}

func TestCheckCRL_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	status := CheckCRL(&x509.Certificate{}, []string{srv.URL})
	if status != certificate.RevocationUnknown {
		t.Errorf("got %q; want %q", status, certificate.RevocationUnknown)
	}
}

func TestCheckCRL_InvalidCRLBytes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/pkix-crl")
		_, _ = w.Write([]byte("not a valid CRL"))
	}))
	defer srv.Close()

	status := CheckCRL(&x509.Certificate{}, []string{srv.URL})
	if status != certificate.RevocationUnknown {
		t.Errorf("got %q; want %q", status, certificate.RevocationUnknown)
	}
}

func TestTryCRL_HTTPRequestError(t *testing.T) {
	status := tryCRL(context.Background(), &x509.Certificate{}, "://invalid-url")
	if status != certificate.RevocationUnknown {
		t.Errorf("got %q; want %q", status, certificate.RevocationUnknown)
	}
}

func TestCheckOCSP_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	original := httpPost
	defer func() { httpPost = original }()

	httpPost = func(_ context.Context, url, contentType string, body []byte) (*http.Response, error) {
		return http.Post(url, contentType, nil) //nolint:noctx
	}

	status := CheckOCSP(&x509.Certificate{}, &x509.Certificate{}, []string{srv.URL + "/ocsp"})
	if status != certificate.RevocationUnknown {
		t.Errorf("got %q; want %q", status, certificate.RevocationUnknown)
	}
}

func TestCheckOCSP_InvalidResponseBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/ocsp-response")
		_, _ = w.Write([]byte("not valid ocsp data"))
	}))
	defer srv.Close()

	original := httpPost
	defer func() { httpPost = original }()

	httpPost = func(_ context.Context, url, contentType string, body []byte) (*http.Response, error) {
		return http.Post(url, contentType, nil) //nolint:noctx
	}

	status := CheckOCSP(&x509.Certificate{}, &x509.Certificate{}, []string{srv.URL + "/ocsp"})
	if status != certificate.RevocationUnknown {
		t.Errorf("got %q; want %q", status, certificate.RevocationUnknown)
	}
}
