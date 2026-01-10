package constants

// WebsiteURL is the base URL for the Kubeasy website (used for CLI API routes)
var WebsiteURL = "http://localhost:3000"

var RestAPIUrl = WebsiteURL + "/api/cli"

var KeyringServiceName = "kubeasy-cli"

var GithubRootURL = "https://github.com/kubeasy-dev"

var ExercisesRepoURL = GithubRootURL + "/challenges"

var ExercicesRepoBranch = "main"

var Version = "dev"

var KubeasyClusterContext = "kind-kubeasy"

// KubernetesVersion is the target Kubernetes version for the Kind cluster
// This should match the k8s.io/* library versions in go.mod (v0.X.Y -> 1.X.Y)
// renovate: datasource=docker depName=kindest/node
var KubernetesVersion = "1.35.0"

// KindNodeImage is the container image used for Kind cluster nodes
// renovate: datasource=docker depName=kindest/node
var KindNodeImage = "kindest/node:v1.35.0"

// LogFilePath defines the default path for the log file when debug is enabled
var LogFilePath = "kubeasy-cli.log"

var MaxLogLines = 1000 // Maximum number of lines to keep in the log file
