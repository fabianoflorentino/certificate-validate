package k8smonitor

import (
	"time"

	"github.com/fabianoflorentino/certificate-validate/internal/certificate"
)

// Kind identifies the Kubernetes resource a certificate was discovered from.
type Kind string

const (
	KindSecret      Kind = "Secret"
	KindIngress     Kind = "Ingress"
	KindCertificate Kind = "CertificateCRD"
)

// RenewalState tracks the lifecycle of a certificate.
type RenewalState string

const (
	RenewalStateNone       RenewalState = "none"
	RenewalStatePending    RenewalState = "pending"
	RenewalStateInProgress RenewalState = "in_progress"
	RenewalStateSuccess    RenewalState = "success"
	RenewalStateFailed     RenewalState = "failed"
	RenewalStateStuck      RenewalState = "stuck"
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
