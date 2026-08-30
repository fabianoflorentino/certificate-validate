package k8smonitor

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Client wraps the Kubernetes clientset used by the monitor.
type Client struct {
	Clientset kubernetes.Interface
}

// NewClient builds a Kubernetes clientset from in-cluster configuration
// when running inside a cluster, falling back to the local kubeconfig.
// If kubeconfig path is non-empty it takes precedence.
func NewClient(kubeconfig string) (*Client, error) {
	cfg, err := buildConfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("create kubernetes client: %w", err)
	}

	return &Client{Clientset: clientset}, nil
}

func buildConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("load kubeconfig %s: %w", kubeconfig, err)
		}
		return cfg, nil
	}

	// Prefer in-cluster config when running as a pod.
	cfg, err := rest.InClusterConfig()
	if err == nil {
		return cfg, nil
	}

	// Fall back to the default kubeconfig lookup rules.
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	cfg, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		&clientcmd.ConfigOverrides{},
	).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf(
			"build kubernetes config (in-cluster unavailable and no default kubeconfig found): %w",
			err)
	}
	return cfg, nil
}
