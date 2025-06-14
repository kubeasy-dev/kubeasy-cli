package kube

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/client-go/rest"

	"github.com/kubeasy-dev/kubeasy-cli/pkg/logger"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

const (
	// KubeasyClusterContext is the Kubernetes context name to use
	KubeasyClusterContext = "kind-kubeasy"
)

// GetKubernetesClient returns the Kubernetes clientset using the Kubeasy context
func GetKubernetesClient() (*kubernetes.Clientset, error) {
	logger.Debug("Attempting to get Kubernetes clientset...")
	// Use the default kubeconfig location
	config, err := getRestConfig()
	if err != nil {
		logger.Error("Error building kubeconfig with context %s: %v", KubeasyClusterContext, err)
		return nil, fmt.Errorf("error building kubeconfig with context %s: %w", KubeasyClusterContext, err)
	}

	// Create the clientset
	logger.Debug("Creating Kubernetes clientset from config...")
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Error("Error creating Kubernetes client: %v", err)
		return nil, fmt.Errorf("error creating Kubernetes client: %w", err)
	}

	logger.Info("Kubernetes clientset obtained successfully for context %s.", KubeasyClusterContext)
	return clientset, nil
}

// getRestConfig loads kubeconfig and returns a rest.Config with Kubeasy context
func getRestConfig() (*rest.Config, error) {
	kubeConfigPath := filepath.Join(homedir.HomeDir(), ".kube", "config")
	logger.Debug("Using kubeconfig path: %s", kubeConfigPath)

	configLoadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeConfigPath}
	configOverrides := &clientcmd.ConfigOverrides{
		CurrentContext: KubeasyClusterContext, // Force using kind-kubeasy context
	}
	logger.Debug("Forcing Kubernetes context: %s", KubeasyClusterContext)

	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		configLoadingRules,
		configOverrides,
	).ClientConfig()
	if err != nil {
		return nil, err
	}
	return config, nil
}

// GetDynamicClient returns the Kubernetes dynamic client using the Kubeasy context
func GetDynamicClient() (dynamic.Interface, error) {
	logger.Debug("Attempting to get Kubernetes dynamic client...")
	// Use the default kubeconfig location
	config, err := getRestConfig()
	if err != nil {
		logger.Error("Error building kubeconfig with context %s: %v", KubeasyClusterContext, err)
		return nil, fmt.Errorf("error building kubeconfig with context %s: %w", KubeasyClusterContext, err)
	}

	// Create the dynamic client
	logger.Debug("Creating Kubernetes dynamic client from config...")
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		logger.Error("Error creating dynamic client: %v", err)
		return nil, fmt.Errorf("error creating dynamic client: %w", err)
	}

	logger.Info("Kubernetes dynamic client obtained successfully for context %s.", KubeasyClusterContext)
	return dynamicClient, nil
}

// CreateNamespace creates a namespace if it doesn't exist
func CreateNamespace(ctx context.Context, clientset *kubernetes.Clientset, namespace string) error {
	logger.Debug("Checking if namespace '%s' exists...", namespace)
	// Check if namespace already exists
	_, err := clientset.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err == nil {
		// Namespace already exists
		logger.Info("Namespace '%s' already exists.", namespace)
		return nil
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
			return nil
		}
		logger.Error("Error creating namespace %s: %v", namespace, err)
		return fmt.Errorf("error creating namespace %s: %w", namespace, err)
	}

	logger.Info("Namespace '%s' created successfully.", namespace)
	return nil
}

// GetResourceGVR returns the Group-Version-Resource for a Kubernetes resource
func GetResourceGVR(gvk *schema.GroupVersionKind) schema.GroupVersionResource {
	gvr := schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: strings.ToLower(gvk.Kind) + "s", // Simple pluralization
	}

	// Handle special cases and exceptions
	switch strings.ToLower(gvk.Kind) {
	case "deployment", "daemonset", "statefulset", "replicaset":
		if gvk.Group == "apps" {
			gvr.Resource = strings.ToLower(gvk.Kind) + "s"
		}
	case "endpoints", "configmap", "secret", "service", "serviceaccount", "namespace":
		if gvk.Group == "" {
			gvr.Resource = strings.ToLower(gvk.Kind) + "s"
		}
	case "ingress":
		if gvk.Group == "networking.k8s.io" || gvk.Group == "extensions" {
			gvr.Resource = "ingresses"
		}
	case "networkpolicy":
		if gvk.Group == "networking.k8s.io" {
			gvr.Resource = "networkpolicies"
		}
	case "customresourcedefinition":
		if gvk.Group == "apiextensions.k8s.io" {
			gvr.Resource = "customresourcedefinitions"
		}
	case "clusterrole", "clusterrolebinding", "role", "rolebinding":
		if gvk.Group == "rbac.authorization.k8s.io" {
			gvr.Resource = strings.ToLower(gvk.Kind) + "s"
		}
	case "endpoint":
		if gvk.Group == "" {
			gvr.Resource = "endpoints"
		}
	case "podsecuritypolicy":
		if gvk.Group == "policy" {
			gvr.Resource = "podsecuritypolicies"
		}
	}

	return gvr
}

// WaitForDeploymentsReady waits for deployments to become ready in a namespace
func WaitForDeploymentsReady(ctx context.Context, clientset *kubernetes.Clientset, namespace string, deploymentNames []string) error {
	logger.Info("Waiting for Deployments in namespace '%s' to be ready: %s", namespace, strings.Join(deploymentNames, ", "))
	for _, deploymentName := range deploymentNames {
		logger.Debug("Waiting for Deployment %s/%s to become ready...", namespace, deploymentName)
		// Wait for the deployment to have the desired number of ready replicas
		for {
			deployment, err := clientset.AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					// This deployment might be optional or not created yet, log and continue checking
					logger.Warning("Deployment %s/%s not found, continuing wait...", namespace, deploymentName)
					// Add a small delay before retrying Get
					select {
					case <-ctx.Done():
						errMsg := fmt.Sprintf("timeout waiting for Deployment %s/%s to appear", namespace, deploymentName)
						logger.Error("%s", errMsg)
						return fmt.Errorf("%s", errMsg)
					case <-time.After(2 * time.Second):
						continue // Retry Get
					}
				}
				errMsg := fmt.Sprintf("error getting Deployment %s/%s: %v", namespace, deploymentName, err)
				logger.Error("%s", errMsg)
				return fmt.Errorf("%s", errMsg)
			}

			// Check if desired replicas is set (it might not be immediately)
			if deployment.Spec.Replicas == nil {
				logger.Debug("Deployment %s/%s spec.replicas not set yet, continuing wait...", namespace, deploymentName)
			} else {
				desiredReplicas := *deployment.Spec.Replicas
				readyReplicas := deployment.Status.ReadyReplicas
				updatedReplicas := deployment.Status.UpdatedReplicas
				availableReplicas := deployment.Status.AvailableReplicas
				logger.Debug("Deployment %s/%s status: Ready=%d/%d, Updated=%d, Available=%d",
					namespace, deploymentName, readyReplicas, desiredReplicas, updatedReplicas, availableReplicas)

				// Check if the deployment is stable and ready
				if deployment.Generation <= deployment.Status.ObservedGeneration &&
					updatedReplicas >= desiredReplicas &&
					availableReplicas >= desiredReplicas &&
					readyReplicas >= desiredReplicas {
					logger.Info("Deployment %s/%s is ready.", namespace, deploymentName)
					break // This specific deployment is ready, move to the next one
				}
			}

			// Wait before checking again
			select {
			case <-ctx.Done():
				// Get final status on timeout
				finalDep, getErr := clientset.AppsV1().Deployments(namespace).Get(context.Background(), deploymentName, metav1.GetOptions{})
				finalStatus := "Unknown"
				if getErr != nil {
					finalStatus = fmt.Sprintf("Error getting status (%v)", getErr)
				} else if finalDep.Spec.Replicas != nil {
					finalStatus = fmt.Sprintf("Ready=%d/%d, Updated=%d, Available=%d, Generation=%d, ObservedGeneration=%d",
						finalDep.Status.ReadyReplicas, *finalDep.Spec.Replicas, finalDep.Status.UpdatedReplicas,
						finalDep.Status.AvailableReplicas, finalDep.Generation, finalDep.Status.ObservedGeneration)
				}
				errMsg := fmt.Sprintf("timeout waiting for Deployment %s/%s to be ready. Final status: %s", namespace, deploymentName, finalStatus)
				logger.Error("%s", errMsg)
				return fmt.Errorf("%s", errMsg)
			case <-time.After(2 * time.Second):
				logger.Debug("Retrying status check for Deployment %s/%s...", namespace, deploymentName)
				// Continue loop
			}
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
		// Wait for the StatefulSet to have the correct number of ready replicas
		for {
			sts, err := clientset.AppsV1().StatefulSets(namespace).Get(ctx, stsName, metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					// This STS might be optional or not created yet, log and continue checking
					logger.Warning("StatefulSet %s/%s not found, continuing wait...", namespace, stsName)
					// Add a small delay before retrying Get
					select {
					case <-ctx.Done():
						errMsg := fmt.Sprintf("timeout waiting for StatefulSet %s/%s to appear", namespace, stsName)
						logger.Error("%s", errMsg)
						return fmt.Errorf("%s", errMsg)
					case <-time.After(2 * time.Second):
						continue // Retry Get
					}
				}
				errMsg := fmt.Sprintf("error getting StatefulSet %s/%s: %v", namespace, stsName, err)
				logger.Error("%s", errMsg)
				return fmt.Errorf("%s", errMsg)
			}

			// Check if desired replicas is set (it might not be immediately)
			if sts.Spec.Replicas == nil {
				logger.Debug("StatefulSet %s/%s spec.replicas not set yet, continuing wait...", namespace, stsName)
			} else {
				desiredReplicas := *sts.Spec.Replicas
				readyReplicas := sts.Status.ReadyReplicas
				updatedReplicas := sts.Status.UpdatedReplicas // Use UpdatedReplicas for StatefulSets as well
				currentRevision := sts.Status.CurrentRevision
				updateRevision := sts.Status.UpdateRevision
				logger.Debug("StatefulSet %s/%s status: ReadyReplicas=%d, DesiredReplicas=%d, UpdatedReplicas=%d, CurrentRevision=%s, UpdateRevision=%s",
					namespace, stsName, readyReplicas, desiredReplicas, updatedReplicas, currentRevision, updateRevision)

				// Check if the StatefulSet is stable and ready
				if sts.Generation <= sts.Status.ObservedGeneration &&
					readyReplicas >= desiredReplicas &&
					updatedReplicas >= desiredReplicas && // Ensure pods are updated
					currentRevision == updateRevision { // Ensure update rollout is complete
					logger.Info("StatefulSet %s/%s is ready.", namespace, stsName)
					break // This specific StatefulSet is ready, move to the next one
				}
			}

			// Wait before checking again
			select {
			case <-ctx.Done():
				// Get final status on timeout
				finalSts, getErr := clientset.AppsV1().StatefulSets(namespace).Get(context.Background(), stsName, metav1.GetOptions{})
				finalStatus := "Unknown"
				if getErr != nil {
					finalStatus = fmt.Sprintf("Error getting status (%v)", getErr)
				} else if finalSts.Spec.Replicas != nil {
					finalStatus = fmt.Sprintf("Ready=%d/%d, Updated=%d, Generation=%d, ObservedGeneration=%d, CurrentRevision=%s, UpdateRevision=%s",
						finalSts.Status.ReadyReplicas, *finalSts.Spec.Replicas, finalSts.Status.UpdatedReplicas,
						finalSts.Generation, finalSts.Status.ObservedGeneration, finalSts.Status.CurrentRevision, finalSts.Status.UpdateRevision)
				}
				errMsg := fmt.Sprintf("timeout waiting for StatefulSet %s/%s to be ready. Final status: %s", namespace, stsName, finalStatus)
				logger.Error("%s", errMsg)
				return fmt.Errorf("%s", errMsg)
			case <-time.After(2 * time.Second):
				logger.Debug("Retrying status check for StatefulSet %s/%s...", namespace, stsName)
				// Continue loop
			}
		}
	}

	logger.Info("All specified StatefulSets in namespace %s are ready.", namespace)
	return nil
}
