package argocd

import (
	"embed"
	"fmt"
)

// EmbeddedManifests contains the ArgoCD application manifests embedded at compile time.
// This eliminates the need to fetch manifests from GitHub during installation.
//
//go:embed manifests/*.yaml
var EmbeddedManifests embed.FS

// GetArgoCDAppManifest returns the embedded ArgoCD application manifest
func GetArgoCDAppManifest() ([]byte, error) {
	data, err := EmbeddedManifests.ReadFile("manifests/argocd.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded ArgoCD manifest: %w", err)
	}
	return data, nil
}

// GetKyvernoAppManifest returns the embedded Kyverno application manifest
func GetKyvernoAppManifest() ([]byte, error) {
	data, err := EmbeddedManifests.ReadFile("manifests/kyverno.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded Kyverno manifest: %w", err)
	}
	return data, nil
}

// GetAllAppManifests returns all embedded application manifests.
// This function is useful for future extensibility when more apps are added.
func GetAllAppManifests() (map[string][]byte, error) {
	manifests := make(map[string][]byte)

	argocdManifest, err := GetArgoCDAppManifest()
	if err != nil {
		return nil, fmt.Errorf("failed to load ArgoCD manifest: %w", err)
	}
	manifests["argocd"] = argocdManifest

	kyvernoManifest, err := GetKyvernoAppManifest()
	if err != nil {
		return nil, fmt.Errorf("failed to load Kyverno manifest: %w", err)
	}
	manifests["kyverno"] = kyvernoManifest

	return manifests, nil
}
