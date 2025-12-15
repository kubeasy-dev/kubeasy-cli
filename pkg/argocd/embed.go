package argocd

import "embed"

// EmbeddedManifests contains the ArgoCD application manifests embedded at compile time.
// This eliminates the need to fetch manifests from GitHub during installation.
//
//go:embed manifests/*.yaml
var EmbeddedManifests embed.FS

// GetArgoCDAppManifest returns the embedded ArgoCD application manifest
func GetArgoCDAppManifest() ([]byte, error) {
	return EmbeddedManifests.ReadFile("manifests/argocd.yaml")
}

// GetKyvernoAppManifest returns the embedded Kyverno application manifest
func GetKyvernoAppManifest() ([]byte, error) {
	return EmbeddedManifests.ReadFile("manifests/kyverno.yaml")
}

// GetAllAppManifests returns all embedded application manifests
func GetAllAppManifests() (map[string][]byte, error) {
	manifests := make(map[string][]byte)

	argocdManifest, err := GetArgoCDAppManifest()
	if err != nil {
		return nil, err
	}
	manifests["argocd"] = argocdManifest

	kyvernoManifest, err := GetKyvernoAppManifest()
	if err != nil {
		return nil, err
	}
	manifests["kyverno"] = kyvernoManifest

	return manifests, nil
}
