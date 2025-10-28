package constants

// WebsiteURL is the base URL for the Kubeasy website (used for CLI API routes)
var WebsiteURL = "http://localhost:3000"

var RestAPIUrl = WebsiteURL + "/api/cli"

var KeyringServiceName = "kubeasy-cli"

var GithubRootURL = "https://github.com/kubeasy-dev"

var ExercisesRepoURL = GithubRootURL + "/challenges"

var ExercicesRepoBranch = "main"

var CliSetupAppsURL = GithubRootURL + "/cli-setup"

var CliSetupAppsBranch = "main"

var Version = "dev"

var KubeasyClusterContext = "kind-kubeasy"

// LogFilePath defines the default path for the log file when debug is enabled
var LogFilePath = "kubeasy-cli.log"

var MaxLogLines = 1000 // Maximum number of lines to keep in the log file
