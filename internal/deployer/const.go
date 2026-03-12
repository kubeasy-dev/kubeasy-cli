package deployer

import "fmt"

// ChallengesOCIRegistry is the base OCI registry for challenge artifacts.
var ChallengesOCIRegistry = "ghcr.io/kubeasy-dev/challenges"

// ProbePodName is the fixed name of the CLI-managed curl probe pod.
// Fixed (not random) so labels are stable and challenge authors can target it in NetworkPolicy.
const ProbePodName = "kubeasy-probe"

// ProbePodImageVersion is the curlimages/curl image tag for the probe pod.
// IMPORTANT: The comment format below is required for Renovate. Do not modify.
// renovate: datasource=docker depName=curlimages/curl
var ProbePodImageVersion = "8.13.0"

// probePodImage returns the full curlimages/curl image reference with version.
func probePodImage() string {
	return fmt.Sprintf("curlimages/curl:%s", ProbePodImageVersion)
}

// KyvernoVersion is the Kyverno release version used for infrastructure setup.
// IMPORTANT: The comment format below is required for Renovate. Do not modify.
// renovate: datasource=github-releases depName=kyverno/kyverno
var KyvernoVersion = "v1.17.1"

// LocalPathProvisionerVersion is the local-path-provisioner release version.
// IMPORTANT: The comment format below is required for Renovate. Do not modify.
// renovate: datasource=github-releases depName=rancher/local-path-provisioner
var LocalPathProvisionerVersion = "v0.0.34"

// NginxIngressVersion is the nginx-ingress-controller release version used for ingress setup.
// IMPORTANT: The comment format below is required for Renovate. Do not modify.
// renovate: datasource=github-releases depName=kubernetes/ingress-nginx
var NginxIngressVersion = "v1.15.0"

// GatewayAPICRDsVersion is the Gateway API CRDs release version.
// IMPORTANT: The comment format below is required for Renovate. Do not modify.
// renovate: datasource=github-releases depName=kubernetes-sigs/gateway-api
var GatewayAPICRDsVersion = "v1.2.1"

// CertManagerVersion is the cert-manager release version used for TLS certificate management.
// IMPORTANT: The comment format below is required for Renovate. Do not modify.
// renovate: datasource=github-releases depName=cert-manager/cert-manager
var CertManagerVersion = "v1.19.4"

// CloudProviderKindVersion is the cloud-provider-kind release version for LoadBalancer support.
// IMPORTANT: The comment format below is required for Renovate. Do not modify.
// renovate: datasource=github-releases depName=kubernetes-sigs/cloud-provider-kind
var CloudProviderKindVersion = "v0.10.0"
