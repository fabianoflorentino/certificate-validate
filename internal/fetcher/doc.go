// Package fetcher provides TLS certificate fetching from remote hosts.
//
// The Fetcher interface defines the contract for retrieving certificate
// information from a host. The default implementation (tlsFetcher) performs
// TLS handshakes to extract certificate details including the certificate chain,
// TLS version, and cipher suite.
//
// # Features
//
//   - Configurable connection timeouts
//   - Custom root CA certificate pools
//   - Per-host CA certificates for internal/mutual TLS
//   - Automatic revocation checking (OCSP/CRL)
//
// # Usage
//
// Create a fetcher and retrieve a certificate:
//
//	f := fetcher.New(10 * time.Second)
//	cert, err := f.Fetch(ctx, "example.com", 443)
//
// With custom root CAs:
//
//	rootCAs := fetcher.LoadRootCAs("/path/to/ca.pem")
//	f := fetcher.NewWithRootCAs(10*time.Second, rootCAs)
//
// With per-host CAs:
//
//	perHostCAs := map[string]*x509.CertPool{
//		"internal.example.com": internalCA,
//	}
//	f := fetcher.NewWithPerHostCAs(10*time.Second, nil, perHostCAs)
package fetcher
