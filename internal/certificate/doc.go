// Package certificate provides types and utilities for parsing, analyzing,
// and representing SSL/TLS certificate information.
//
// The package defines the core Certificate type that holds extracted information
// from X.509 certificates, including subject, issuer, validity period, revocation
// status, and the certificate chain. It also provides utilities for building
// certificates from crypto/x509 types and computing fingerprints.
//
// # Certificate Model
//
// The Certificate struct is the central type, containing all relevant information
// extracted from a TLS connection:
//
//   - CommonName and SubjectAltNames for identity
//   - Issuer and chain information
//   - Validity period (NotBefore, NotAfter, DaysLeft)
//   - Revocation status (OCSP/CRL)
//   - TLS version and cipher suite
//
// # Revocation Status
//
// RevocationStatus represents the result of OCSP or CRL checks:
//
//   - RevocationGood: certificate is not revoked
//   - RevocationRevoked: certificate has been revoked
//   - RevocationUnknown: status could not be determined
//   - RevocationNotReady: no OCSP/CRL endpoints available
//
// # Usage
//
// Build a Certificate from an x509.Certificate:
//
//	cert := certificate.FromX509(x509Cert, "example.com", 443)
//	cert.Chain = certificate.BuildChain(peerCerts)
package certificate
