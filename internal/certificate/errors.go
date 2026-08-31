package certificate

import "errors"

// Sentinel errors returned by certificate fetching operations.
// Use errors.Is to check for specific error conditions.
var (
	// ErrHostUnreachable indicates the host could not be reached (DNS failure,
	// connection refused, or network timeout).
	ErrHostUnreachable = errors.New("host unreachable")

	// ErrInvalidHostname indicates the hostname is invalid or could not be resolved.
	ErrInvalidHostname = errors.New("invalid hostname")

	// ErrCertificateFetch indicates a general failure during certificate fetching.
	ErrCertificateFetch = errors.New("failed to fetch certificate")

	// ErrNoCertificate indicates the server did not present a certificate during
	// the TLS handshake.
	ErrNoCertificate = errors.New("no peer certificate presented")
)
