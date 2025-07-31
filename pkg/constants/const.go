package constants

import "os"

var RestAPIUrl = "https://api.kubeasy.dev"

var KeyringServiceName = "kubeasy-cli"

var GithubRootURL = "https://github.com/kubeasy-dev"

var ExercisesRepoURL = GithubRootURL + "/challenges"

var ExercicesRepoBranch = "copilot/fix-6"

var CliSetupAppsURL = GithubRootURL + "/cli-setup"

var Version = "dev"

// LogFilePath defines the default path for the log file when debug is enabled
var LogFilePath = "kubeasy-cli.log"

// MockEnabled determines if the CLI should run in mock mode, bypassing API calls
// It can be controlled via the KUBEASY_MOCK environment variable or --mock flag
var MockEnabled = os.Getenv("KUBEASY_MOCK") == "true"
