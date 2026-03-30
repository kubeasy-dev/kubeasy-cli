package cmd

import (
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/kubeasy-dev/kubeasy-cli/internal/constants"
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

		if isPreRelease(current) {
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
		curNorm := normalizeSemver(current)
		latNorm := normalizeSemver(latest)

		switch compareSemver(curNorm, latNorm) {
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

// fetchLatestVersion returns the latest version tag from the download CDN.
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

// normalizeSemver trims a leading 'v' and strips pre-release/build metadata for comparison.
func normalizeSemver(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	// Drop pre-release/build metadata for simple compare
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	return v
}

// compareSemver compares two semantic versions (major.minor.patch). Returns -1 if a<b, 0 if equal, 1 if a>b.
func compareSemver(a, b string) int {
	as := splitToInt3(a)
	bs := splitToInt3(b)
	for i := 0; i < 3; i++ {
		if as[i] < bs[i] {
			return -1
		}
		if as[i] > bs[i] {
			return 1
		}
	}
	return 0
}

// isPreRelease returns true for non-semver version strings like "dev" or "nightly-abc1234".
// These start with a non-digit character after stripping a leading 'v'.
// Legitimate semver pre-releases like "2.7.0-rc.1" start with a digit and return false.
func isPreRelease(v string) bool {
	v = strings.TrimSpace(strings.TrimPrefix(v, "v"))
	return len(v) == 0 || v[0] < '0' || v[0] > '9'
}

func splitToInt3(v string) [3]int {
	var out [3]int
	parts := strings.Split(v, ".")
	for i := 0; i < len(parts) && i < 3; i++ {
		// lightweight atoi
		n := 0
		for _, ch := range parts[i] {
			if ch < '0' || ch > '9' {
				break
			}
			n = n*10 + int(ch-'0')
		}
		out[i] = n // #nosec G602 - i is bounded by loop condition i < 3
	}
	return out
}
