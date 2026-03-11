package deployer

// ChallengesOCIRegistry is the base OCI registry for challenge artifacts.
var ChallengesOCIRegistry = "ghcr.io/kubeasy-dev/challenges"

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
