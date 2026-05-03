package cmd

import (
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/kubeasy-dev/kubeasy-cli/internal/constants"
	"github.com/kubeasy-dev/kubeasy-cli/internal/semver"
	"github.com/spf13/cobra"
)

// versionCmd prints the current CLI version and checks the R2 CDN for updates.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version and check for updates",
	Run: func(cmd *cobra.Command, args []string) {
		current := constants.Version
		fmt.Printf("kubeasy-cli %s\n", current)
		fmt.Printf("Go %s - %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)

		if semver.IsPreRelease(current) {
			fmt.Printf("Pre-release build (%s), skipping update check.\n", current)
			return
		}

		latest, err := fetchLatestVersion()
		if err != nil {
			// Non-blocking: just inform user that update check failed
			fmt.Printf("Unable to check for updates: %v\n", err)
			return
		}

		// Compare semantic versions (normalize leading 'v')
		curNorm := semver.Normalize(current)
		latNorm := semver.Normalize(latest)

		switch semver.Compare(curNorm, latNorm) {
		case -1:
			fmt.Printf("A new version is available: %s (you have %s)\n", latest, current)
			fmt.Println("Download it from:")
			fmt.Printf("  %s/kubeasy-cli/releases/latest\n", constants.GithubRootURL)
		case 0:
			fmt.Println("You're using the latest version.")
		case 1:
			// Local version is newer than latest release (e.g., dev build)
			fmt.Printf("Local version is newer than latest release (%s > %s).\n", current, latest)
		}
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

// fetchLatestVersion returns the latest version from the R2 CDN.
func fetchLatestVersion() (string, error) {
	url := constants.DownloadBaseURL + "/latest"

	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "kubeasy-cli-version-check")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	version := strings.TrimSpace(string(body))
	if version == "" {
		return "", fmt.Errorf("empty version response")
	}
	return version, nil
}
