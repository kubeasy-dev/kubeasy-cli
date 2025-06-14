package kube

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/kubeasy-dev/kubeasy-cli/pkg/logger"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	yamlserializer "k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// FetchManifest downloads a manifest from the given URL
func FetchManifest(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error downloading manifest from %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	manifestBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading manifest from %s: %w", url, err)
	}

	return manifestBytes, nil
}

// ApplyManifest applies a Kubernetes manifest to the cluster
func ApplyManifest(ctx context.Context, manifestBytes []byte, namespace string, clientset *kubernetes.Clientset, dynamicClient dynamic.Interface) error {
	logger.Debug("ApplyManifest: Starting application of manifest in namespace '%s'", namespace)
	// Create decoder for YAML content
	decoder := yamlserializer.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)

	// Split manifest into separate documents
	documents := bytes.Split(manifestBytes, []byte("\n---\n"))
	logger.Debug("ApplyManifest: Manifest split into %d documents", len(documents))

	// Apply each document
	for i, doc := range documents {
		docNum := i + 1
		// Skip empty documents
		if len(bytes.TrimSpace(doc)) == 0 {
			logger.Debug("ApplyManifest: Skipping empty document #%d", docNum)
			continue
		}

		// Decode YAML to unstructured object
		obj := &unstructured.Unstructured{}
		_, gvk, err := decoder.Decode(doc, nil, obj)
		if err != nil {
			// Log error and continue with next document
			logger.Warning("ApplyManifest: Skipping document #%d, error decoding: %v", docNum, err)
			continue
		}

		// Log which object is being processed
		objName := obj.GetName()
		objKind := obj.GetKind()
		logger.Debug("ApplyManifest: Processing document #%d - Kind: %s, Name: %s", docNum, objKind, objName)

		// Get the GVR (Group Version Resource) for the object
		gvr := GetResourceGVR(gvk)
		if gvr.Resource == "" {
			logger.Warning("ApplyManifest: Could not determine GVR for Kind: %s, Group: %s, Version: %s in document #%d. Skipping.", objKind, gvk.Group, gvk.Version, docNum)
			continue
		}

		// Set namespace for namespaced resources
		isNamespaced := IsNamespaced(objKind)

		// Apply the resource
		var createdOrUpdated *unstructured.Unstructured
		var resourceClient dynamic.ResourceInterface

		if isNamespaced {
			// Set namespace if not already set
			if obj.GetNamespace() == "" {
				obj.SetNamespace(namespace)
				logger.Debug("ApplyManifest: Setting namespace '%s' for %s/%s", namespace, objKind, objName)
			}
			resourceClient = dynamicClient.Resource(gvr).Namespace(obj.GetNamespace())
			logger.Debug("ApplyManifest: Attempting to create namespaced resource %s/%s (GVR: %v) in namespace %s", objKind, objName, gvr, obj.GetNamespace())
		} else {
			resourceClient = dynamicClient.Resource(gvr)
			logger.Debug("ApplyManifest: Attempting to create cluster-scoped resource %s/%s (GVR: %v)", objKind, objName, gvr)
		}

		createdOrUpdated, err = resourceClient.Create(ctx, obj, metav1.CreateOptions{})

		if err != nil {
			// If the resource doesn't exist (API not available yet), continue
			if apierrors.IsNotFound(err) || strings.Contains(err.Error(), "the server could not find the requested resource") {
				logger.Warning("ApplyManifest: API for %s/%s not available, skipping document #%d. Error: %v", objKind, objName, docNum, err)
				continue
			}

			// If the resource already exists, try to update it
			if apierrors.IsAlreadyExists(err) {
				logger.Debug("ApplyManifest: Resource %s/%s already exists, attempting update...", objKind, objName)
				var updateErr error
				// Get the existing resource to retrieve the resourceVersion for update
				var existingObj *unstructured.Unstructured
				existingObj, updateErr = resourceClient.Get(ctx, objName, metav1.GetOptions{})

				if updateErr != nil {
					logger.Error("ApplyManifest: Failed to get existing resource %s/%s for update: %v. Skipping update for document #%d.", objKind, objName, updateErr, docNum)
					continue
				}

				// Set the resourceVersion from the existing object
				obj.SetResourceVersion(existingObj.GetResourceVersion())

				_, updateErr = resourceClient.Update(ctx, obj, metav1.UpdateOptions{})

				if updateErr != nil {
					// Log update error but continue processing other documents
					logger.Warning("ApplyManifest: Error updating resource %s/%s in document #%d: %v", objKind, objName, docNum, updateErr)
					continue // Continue with the next document
				}
				logger.Info("ApplyManifest: Resource %s/%s updated successfully (document #%d).", objKind, objName, docNum)
				continue // Continue with the next document after successful update
			}

			// Otherwise log the creation error and continue
			logger.Warning("ApplyManifest: Error creating resource %s/%s in document #%d: %v", objKind, objName, docNum, err)
			continue // Continue with the next document
		}

		// Log success if createdOrUpdated is not nil (which it should be on success)
		if createdOrUpdated != nil {
			logger.Info("ApplyManifest: Resource %s/%s created successfully (document #%d).", objKind, objName, docNum)
		}
	}

	logger.Debug("ApplyManifest: Finished applying manifest in namespace '%s'", namespace)
	return nil // Return nil even if some documents failed, as per previous logic
}

// IsNamespaced checks if a given Kubernetes kind is typically namespaced.
// This is a helper function and might not cover all edge cases or CRDs.
func IsNamespaced(kind string) bool {
	switch strings.ToLower(kind) {
	case "namespace", "node", "persistentvolume", "mutatingwebhookconfiguration", "validatingwebhookconfiguration", "customresourcedefinition", "clusterrole", "clusterrolebinding", "storageclass", "volumeattachment", "runtimeclass", "podsecuritypolicy", "priorityclass", "csidriver", "csinode", "apiservice", "certificatesigningrequest":
		return false
	default:
		return true
	}
}
