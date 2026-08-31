package k8smonitor

import (
	"context"
	"fmt"
	"log/slog"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Discover finds TLS Secrets and Ingresses across the given namespaces.
// If namespaces is empty, all namespaces are scanned.
func (c *Client) Discover(ctx context.Context, namespaces []string) ([]*SecretCert, []*IngressCert, error) {
	secretCerts, err := c.discoverSecrets(ctx, namespaces)
	if err != nil {
		return nil, nil, err
	}

	ingressCerts, err := c.discoverIngresses(ctx, namespaces)
	if err != nil {
		return nil, nil, err
	}

	return secretCerts, ingressCerts, nil
}

// SecretCert references a TLS Secret that carries kubernetes.io/tls data.
// It holds the raw certificate data along with Kubernetes metadata for
// analysis and metric labeling.
type SecretCert struct {
	Namespace   string
	Name        string
	Data        map[string][]byte
	Labels      map[string]string
	Annotations map[string]string
}

func (c *Client) discoverSecrets(ctx context.Context, namespaces []string) ([]*SecretCert, error) {
	var secrets []*SecretCert

	list, err := listSecrets(ctx, c.Clientset, namespaces)
	if err != nil {
		return nil, fmt.Errorf("list secrets: %w", err)
	}

	for _, s := range list {
		if s.Type != corev1.SecretTypeTLS {
			continue
		}
		secrets = append(secrets, &SecretCert{
			Namespace:   s.Namespace,
			Name:        s.Name,
			Data:        s.Data,
			Labels:      s.Labels,
			Annotations: s.Annotations,
		})
	}

	slog.Info("discovered TLS secrets", "count", len(secrets))
	return secrets, nil
}

// IngressCert references an Ingress that references a TLS secret.
// It holds the TLS configuration from the Ingress spec for certificate analysis.
type IngressCert struct {
	Namespace   string
	Name        string
	TLS         []networkingv1.IngressTLS
	Labels      map[string]string
	Annotations map[string]string
}

func (c *Client) discoverIngresses(ctx context.Context, namespaces []string) ([]*IngressCert, error) {
	var ingresses []*IngressCert

	list, err := listIngresses(ctx, c.Clientset, namespaces)
	if err != nil {
		return nil, fmt.Errorf("list ingresses: %w", err)
	}

	for _, ing := range list {
		if len(ing.Spec.TLS) == 0 {
			continue
		}
		ingresses = append(ingresses, &IngressCert{
			Namespace:   ing.Namespace,
			Name:        ing.Name,
			TLS:         ing.Spec.TLS,
			Labels:      ing.Labels,
			Annotations: ing.Annotations,
		})
	}

	slog.Info("discovered ingress TLS entries", "count", len(ingresses))
	return ingresses, nil
}

func listSecrets(ctx context.Context, cs kubernetes.Interface, namespaces []string) ([]corev1.Secret, error) {
	if len(namespaces) == 0 {
		list, err := cs.CoreV1().Secrets(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		return list.Items, nil
	}

	var out []corev1.Secret
	for _, ns := range namespaces {
		list, err := cs.CoreV1().Secrets(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("list secrets in namespace %s: %w", ns, err)
		}
		out = append(out, list.Items...)
	}
	return out, nil
}

func listIngresses(ctx context.Context, cs kubernetes.Interface, namespaces []string) ([]networkingv1.Ingress, error) {
	if len(namespaces) == 0 {
		list, err := cs.NetworkingV1().Ingresses(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		return list.Items, nil
	}

	var out []networkingv1.Ingress
	for _, ns := range namespaces {
		list, err := cs.NetworkingV1().Ingresses(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("list ingresses in namespace %s: %w", ns, err)
		}
		out = append(out, list.Items...)
	}
	return out, nil
}
