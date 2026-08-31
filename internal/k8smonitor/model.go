package k8smonitor

import (
	"time"

	"github.com/fabianoflorentino/certificate-validate/internal/certificate"
)

// Kind identifies the Kubernetes resource a certificate was discovered from.
type Kind string

// Kind constants for Kubernetes resource types.
const (
	// KindSecret indicates the certificate was discovered from a TLS Secret.
	KindSecret Kind = "Secret"

	// KindIngress indicates the certificate was discovered from an Ingress.
	KindIngress Kind = "Ingress"

	// KindCertificate indicates the certificate was discovered from a cert-manager Certificate CRD.
	KindCertificate Kind = "CertificateCRD"
)

// RenewalState tracks the lifecycle of a certificate during auto-renewal.
type RenewalState string

// RenewalState constants for tracking certificate renewal progress.
const (
	// RenewalStateNone indicates no renewal has been attempted.
	RenewalStateNone RenewalState = "none"

	// RenewalStatePending indicates a renewal is pending (not yet started).
	RenewalStatePending RenewalState = "pending"

	// RenewalStateInProgress indicates a renewal is in progress.
	RenewalStateInProgress RenewalState = "in_progress"

	// RenewalStateSuccess indicates the renewal completed successfully.
	RenewalStateSuccess RenewalState = "success"

	// RenewalStateFailed indicates the renewal failed.
	RenewalStateFailed RenewalState = "failed"

	// RenewalStateStuck indicates the renewal is stuck (serial unchanged after timeout).
	RenewalStateStuck RenewalState = "stuck"
)

// K8sCertificate extends the core Certificate model with Kubernetes metadata
// and renewal state as defined in the integration roadmap.
type K8sCertificate struct {
	certificate.Certificate

	K8sNamespace   string            `json:"k8s_namespace"`
	K8sName        string            `json:"k8s_name"`
	K8sKind        Kind              `json:"k8s_kind"`
	K8sAPIVersion  string            `json:"k8s_api_version,omitempty"`
	K8sAnnotations map[string]string `json:"k8s_annotations,omitempty"`
	K8sLabels      map[string]string `json:"k8s_labels,omitempty"`

	RenewalState     RenewalState `json:"renewal_state"`
	LastRenewal      *time.Time   `json:"last_renewal,omitempty"`
	LastRenewalError string       `json:"last_renewal_error,omitempty"`
	RenewalAttempts  int          `json:"renewal_attempts"`

	SerialNumber string `json:"serial_number"`
	RenewalCount int    `json:"renewal_count"`
}
