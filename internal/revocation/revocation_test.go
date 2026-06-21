package revocation

import (
	"context"
	"crypto/x509"
	"errors"
	"net/http"
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
