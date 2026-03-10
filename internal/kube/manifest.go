package kube

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/kubeasy-dev/kubeasy-cli/internal/logger"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	yamlserializer "k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/client-go/dynamic"
)

// fetchManifestAllowedPrefixes lists the trusted domain prefixes for FetchManifest.
// Any URL not matching one of these prefixes is rejected before making an HTTP call.
var fetchManifestAllowedPrefixes = []string{
	"https://github.com/",
	"https://raw.githubusercontent.com/",
}

// FetchManifest downloads a manifest from the given URL
func FetchManifest(url string) ([]byte, error) {
	allowed := false
	for _, prefix := range fetchManifestAllowedPrefixes {
		if strings.HasPrefix(url, prefix) {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, fmt.Errorf("FetchManifest: URL %q is not from a trusted domain (allowed: %v)", url, fetchManifestAllowedPrefixes)
	}

	resp, err := http.Get(url) //nolint:gosec // URL validated against fetchManifestAllowedPrefixes
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
func ApplyManifest(ctx context.Context, manifestBytes []byte, namespace string, mapper meta.RESTMapper, dynamicClient dynamic.Interface) error {
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

		// Get the GVR and scope via the REST mapper
		mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			logger.Warning("ApplyManifest: Could not find mapping for Kind: %s, Group: %s, Version: %s in document #%d. Skipping.", objKind, gvk.Group, gvk.Version, docNum)
			continue
		}
		gvr := mapping.Resource

		// Set namespace for namespaced resources
		isNamespaced := mapping.Scope.Name() == meta.RESTScopeNameNamespace

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
					return fmt.Errorf("failed to get %s/%s for update: %w", objKind, objName, updateErr)
				}

				// Set the resourceVersion from the existing object
				obj.SetResourceVersion(existingObj.GetResourceVersion())

				_, updateErr = resourceClient.Update(ctx, obj, metav1.UpdateOptions{})

				if updateErr != nil {
					return fmt.Errorf("failed to update %s/%s: %w", objKind, objName, updateErr)
				}
				logger.Info("ApplyManifest: Resource %s/%s updated successfully (document #%d).", objKind, objName, docNum)
				continue // Continue with the next document after successful update
			}

			return fmt.Errorf("failed to create %s/%s: %w", objKind, objName, err)
		}

		// Log success if createdOrUpdated is not nil (which it should be on success)
		if createdOrUpdated != nil {
			logger.Info("ApplyManifest: Resource %s/%s created successfully (document #%d).", objKind, objName, docNum)
		}
	}

	logger.Debug("ApplyManifest: Finished applying manifest in namespace '%s'", namespace)
	return nil
}
