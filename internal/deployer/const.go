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
