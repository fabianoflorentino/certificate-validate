// Package revocation provides OCSP and CRL checking for certificate revocation
// verification.
//
// The package implements certificate revocation checking using two mechanisms:
//
//   - OCSP (Online Certificate Status Protocol): queries OCSP responders to
//     check if a certificate has been revoked. Supports multiple OCSP servers
//     with fallback behavior.
//
//   - CRL (Certificate Revocation List): downloads and parses CRLs from
//     distribution points to check if a certificate's serial number is listed.
//
// # Revocation Checking
//
// The Check function performs both OCSP and CRL checks, returning the first
// definitive result. OCSP is preferred when available; CRL is used as fallback
// or when OCSP is unavailable.
//
// # Timeouts and Retries
//
// All network operations use short timeouts (10 seconds by default) to avoid
// blocking certificate validation on slow or unresponsive responders. The
// package is designed for high-throughput scanning where responder latency
// should not dominate the overall cycle time.
//
// # Usage
//
// Check revocation status of a certificate:
//
//	status := revocation.Check(leaf, issuer, leaf.OCSPServer, leaf.CRLDistributionPoints)
//	switch status {
//	case certificate.RevocationGood:
//		// certificate is valid
//	case certificate.RevocationRevoked:
//		// certificate has been revoked
//	case certificate.RevocationUnknown:
//		// status could not be determined
//	}
package revocation
