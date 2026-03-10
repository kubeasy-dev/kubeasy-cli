package kube

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/client-go/rest"

	"github.com/kubeasy-dev/kubeasy-cli/internal/constants"
	"github.com/kubeasy-dev/kubeasy-cli/internal/logger"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// LoggingRoundTripper wraps HTTP transport to log requests/responses
type LoggingRoundTripper struct {
	rt http.RoundTripper
}

// RoundTrip implements http.RoundTripper
func (l *LoggingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Log the request
	logger.Debug("K8s HTTP: %s %s", req.Method, req.URL.String())

	// Execute the request
	resp, err := l.rt.RoundTrip(req)

	// Log the response
	if err != nil {
		logger.Debug("K8s HTTP: Response error: %v", err)
	} else {
		logger.Debug("K8s HTTP: Response status: %s", resp.Status)
	}

	return resp, err
}

// GetKubernetesClient returns the Kubernetes clientset using the Kubeasy context
func GetKubernetesClient() (*kubernetes.Clientset, error) {
	logger.Debug("Attempting to get Kubernetes clientset...")

	// Use the default kubeconfig location
	config, err := getRestConfig()
	if err != nil {
		logger.Error("Error building kubeconfig with context %s: %v", constants.KubeasyClusterContext, err)
		return nil, fmt.Errorf("error building kubeconfig with context %s: %w", constants.KubeasyClusterContext, err)
	}

	// Create the clientset
	logger.Debug("Creating Kubernetes clientset from config...")
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Error("Error creating Kubernetes client: %v", err)
		return nil, fmt.Errorf("error creating Kubernetes client: %w", err)
	}

	logger.Info("Kubernetes clientset obtained successfully for context %s.", constants.KubeasyClusterContext)
	return clientset, nil
}

// getRestConfig loads kubeconfig and returns a rest.Config with Kubeasy context
func getRestConfig() (*rest.Config, error) {
	kubeConfigPath := filepath.Join(homedir.HomeDir(), ".kube", "config")
	logger.Debug("Using kubeconfig path: %s", kubeConfigPath)

	configLoadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeConfigPath}
	configOverrides := &clientcmd.ConfigOverrides{
		CurrentContext: constants.KubeasyClusterContext,
	}
	logger.Debug("Forcing Kubernetes context: %s", constants.KubeasyClusterContext)

	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		configLoadingRules,
		configOverrides,
	).ClientConfig()
	if err != nil {
		return nil, err
	}

	// Enable HTTP request/response logging in debug mode
	currentLogger := logger.GetLogger()
	if currentLogger != nil {
		// Wrap transport to log HTTP requests/responses
		config.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
			return &LoggingRoundTripper{rt: rt}
		}
	}

	return config, nil
}

// GetServerVersion returns the Kubernetes server version of the connected cluster.
// Returns the version without the "v" prefix (e.g., "1.35.0").
func GetServerVersion() (string, error) {
	clientset, err := GetKubernetesClient()
	if err != nil {
		return "", fmt.Errorf("failed to get Kubernetes client: %w", err)
	}

	serverVersion, err := clientset.Discovery().ServerVersion()
	if err != nil {
		return "", fmt.Errorf("failed to get server version: %w", err)
	}

	return strings.TrimPrefix(serverVersion.GitVersion, "v"), nil
}

// GetDynamicClient returns the Kubernetes dynamic client using the Kubeasy context
func GetDynamicClient() (dynamic.Interface, error) {
	logger.Debug("Attempting to get Kubernetes dynamic client...")

	// Use the default kubeconfig location
	config, err := getRestConfig()
	if err != nil {
		logger.Error("Error building kubeconfig with context %s: %v", constants.KubeasyClusterContext, err)
		return nil, fmt.Errorf("error building kubeconfig with context %s: %w", constants.KubeasyClusterContext, err)
	}

	// Create the dynamic client
	logger.Debug("Creating Kubernetes dynamic client from config...")
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		logger.Error("Error creating dynamic client: %v", err)
		return nil, fmt.Errorf("error creating dynamic client: %w", err)
	}

	logger.Info("Kubernetes dynamic client obtained successfully for context %s.", constants.KubeasyClusterContext)
	return dynamicClient, nil
}

// CreateNamespace creates a namespace if it doesn't exist
func CreateNamespace(ctx context.Context, clientset kubernetes.Interface, namespace string) error {
	logger.Debug("Checking if namespace '%s' exists...", namespace)
	// Check if namespace already exists
	_, err := clientset.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err == nil {
		// Namespace already exists, but wait for it to be Active
		logger.Info("Namespace '%s' already exists.", namespace)
		return WaitForNamespaceActive(ctx, clientset, namespace)
	}

	if !apierrors.IsNotFound(err) {
		logger.Error("Error checking namespace %s: %v", namespace, err)
		return fmt.Errorf("error checking namespace %s: %w", namespace, err)
	}

	// Create the namespace
	logger.Info("Namespace '%s' not found, attempting to create...", namespace)
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}

	_, err = clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			// Race condition: namespace was created between Get and Create
			logger.Info("Namespace '%s' created concurrently.", namespace)
			return WaitForNamespaceActive(ctx, clientset, namespace)
		}
		logger.Error("Error creating namespace %s: %v", namespace, err)
		return fmt.Errorf("error creating namespace %s: %w", namespace, err)
	}

	logger.Info("Namespace '%s' created successfully.", namespace)

	// Wait for namespace to become Active before returning
	return WaitForNamespaceActive(ctx, clientset, namespace)
}

// WaitForNamespaceActive waits for a namespace to reach the Active phase.
// This is important to avoid race conditions when ArgoCD tries to sync resources
// to a namespace that isn't fully ready yet.
func WaitForNamespaceActive(ctx context.Context, clientset kubernetes.Interface, namespace string) error {
	logger.Debug("Waiting for namespace '%s' to become Active...", namespace)

	// Use a default timeout if context has no deadline
	waitCtx := ctx
	var cancel context.CancelFunc
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		waitCtx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-waitCtx.Done():
			logger.Error("Timeout waiting for namespace '%s' to become Active", namespace)
			return fmt.Errorf("timeout waiting for namespace '%s' to become Active: %w", namespace, waitCtx.Err())
		case <-ticker.C:
			ns, err := clientset.CoreV1().Namespaces().Get(waitCtx, namespace, metav1.GetOptions{})
			if err != nil {
				logger.Warning("Error checking namespace '%s' status: %v (retrying...)", namespace, err)
				continue
			}

			logger.Debug("Namespace '%s' phase: %s", namespace, ns.Status.Phase)

			if ns.Status.Phase == corev1.NamespaceActive {
				logger.Info("Namespace '%s' is now Active", namespace)
				return nil
			}

			// If namespace is terminating, something is wrong
			if ns.Status.Phase == corev1.NamespaceTerminating {
				logger.Error("Namespace '%s' is Terminating unexpectedly", namespace)
				return fmt.Errorf("namespace '%s' is Terminating", namespace)
			}
		}
	}
}

// DeleteNamespace deletes a namespace if it exists
func DeleteNamespace(ctx context.Context, clientset kubernetes.Interface, namespace string) error {
	logger.Debug("Checking if namespace '%s' exists for deletion...", namespace)

	// Check if namespace exists
	_, err := clientset.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Namespace '%s' does not exist, nothing to delete.", namespace)
			return nil
		}
		logger.Error("Error checking namespace %s: %v", namespace, err)
		return fmt.Errorf("error checking namespace %s: %w", namespace, err)
	}

	// Delete the namespace
	logger.Info("Deleting namespace '%s'...", namespace)
	err = clientset.CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Race condition: namespace was deleted between Get and Delete
			logger.Info("Namespace '%s' was already deleted.", namespace)
			return nil
		}
		logger.Error("Error deleting namespace %s: %v", namespace, err)
		return fmt.Errorf("error deleting namespace %s: %w", namespace, err)
	}

	logger.Info("Namespace '%s' deletion initiated successfully.", namespace)
	return nil
}

// WaitForDeploymentsReady waits for deployments to become ready in a namespace
func WaitForDeploymentsReady(ctx context.Context, clientset *kubernetes.Clientset, namespace string, deploymentNames []string) error {
	logger.Info("Waiting for Deployments in namespace '%s' to be ready: %s", namespace, strings.Join(deploymentNames, ", "))
	for _, deploymentName := range deploymentNames {
		logger.Debug("Waiting for Deployment %s/%s to become ready...", namespace, deploymentName)
		err := wait.PollUntilContextTimeout(ctx, 2*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
			deployment, err := clientset.AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					logger.Warning("Deployment %s/%s not found, retrying...", namespace, deploymentName)
					return false, nil
				}
				return false, fmt.Errorf("error getting Deployment %s/%s: %w", namespace, deploymentName, err)
			}
			if deployment.Spec.Replicas == nil {
				logger.Debug("Deployment %s/%s spec.replicas not set yet, retrying...", namespace, deploymentName)
				return false, nil
			}
			desired := *deployment.Spec.Replicas
			logger.Debug("Deployment %s/%s: Ready=%d/%d, Updated=%d, Available=%d",
				namespace, deploymentName,
				deployment.Status.ReadyReplicas, desired,
				deployment.Status.UpdatedReplicas, deployment.Status.AvailableReplicas)
			if deployment.Generation <= deployment.Status.ObservedGeneration &&
				deployment.Status.UpdatedReplicas >= desired &&
				deployment.Status.AvailableReplicas >= desired &&
				deployment.Status.ReadyReplicas >= desired {
				logger.Info("Deployment %s/%s is ready.", namespace, deploymentName)
				return true, nil
			}
			return false, nil
		})
		if err != nil {
			return fmt.Errorf("timeout waiting for Deployment %s/%s to be ready: %w", namespace, deploymentName, err)
		}
	}
	logger.Info("All specified Deployments in namespace %s are ready.", namespace)
	return nil
}

// WaitForStatefulSetsReady waits for statefulsets to become ready in a namespace
func WaitForStatefulSetsReady(ctx context.Context, clientset *kubernetes.Clientset, namespace string, stsNames []string) error {
	logger.Info("Waiting for StatefulSets in namespace '%s' to be ready: %s", namespace, strings.Join(stsNames, ", "))
	for _, stsName := range stsNames {
		logger.Debug("Waiting for StatefulSet %s/%s to become ready...", namespace, stsName)
		err := wait.PollUntilContextTimeout(ctx, 2*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
			sts, err := clientset.AppsV1().StatefulSets(namespace).Get(ctx, stsName, metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					logger.Warning("StatefulSet %s/%s not found, retrying...", namespace, stsName)
					return false, nil
				}
				return false, fmt.Errorf("error getting StatefulSet %s/%s: %w", namespace, stsName, err)
			}
			if sts.Spec.Replicas == nil {
				logger.Debug("StatefulSet %s/%s spec.replicas not set yet, retrying...", namespace, stsName)
				return false, nil
			}
			desired := *sts.Spec.Replicas
			logger.Debug("StatefulSet %s/%s: Ready=%d/%d, Updated=%d, CurrentRevision=%s, UpdateRevision=%s",
				namespace, stsName,
				sts.Status.ReadyReplicas, desired,
				sts.Status.UpdatedReplicas,
				sts.Status.CurrentRevision, sts.Status.UpdateRevision)
			if sts.Generation <= sts.Status.ObservedGeneration &&
				sts.Status.ReadyReplicas >= desired &&
				sts.Status.UpdatedReplicas >= desired &&
				sts.Status.CurrentRevision == sts.Status.UpdateRevision {
				logger.Info("StatefulSet %s/%s is ready.", namespace, stsName)
				return true, nil
			}
			return false, nil
		})
		if err != nil {
			return fmt.Errorf("timeout waiting for StatefulSet %s/%s to be ready: %w", namespace, stsName, err)
		}
	}
	logger.Info("All specified StatefulSets in namespace %s are ready.", namespace)
	return nil
}
