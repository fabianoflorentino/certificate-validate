package certificate

import "errors"

var (
	ErrHostUnreachable  = errors.New("host unreachable")
	ErrInvalidHostname  = errors.New("invalid hostname")
	ErrCertificateFetch = errors.New("failed to fetch certificate")
	ErrNoCertificate    = errors.New("no peer certificate presented")
)
