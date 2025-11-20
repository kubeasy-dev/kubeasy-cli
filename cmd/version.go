package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/kubeasy-dev/kubeasy-cli/pkg/constants"
	"github.com/spf13/cobra"
)

// versionCmd prints the current CLI version and checks NPM for updates.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version and check for updates",
	Run: func(cmd *cobra.Command, args []string) {
		current := constants.Version
		fmt.Printf("kubeasy-cli %s\n", current)
		fmt.Printf("Go %s - %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)

		latest, err := fetchNPMLatestVersion("@kubeasy-dev/kubeasy-cli")
		if err != nil {
			// Non-blocking: just inform user that update check failed
			fmt.Printf("Unable to check for updates on NPM: %v\n", err)
			return
		}

		// Compare semantic versions (normalize leading 'v')
		curNorm := normalizeSemver(current)
		latNorm := normalizeSemver(latest)

		switch compareSemver(curNorm, latNorm) {
		case -1:
			fmt.Printf("A new version is available: %s (you have %s)\n", latest, current)
			fmt.Println("Update with:")
			fmt.Println("  npm i -g @kubeasy-dev/kubeasy-cli@latest")
		case 0:
			fmt.Println("You're using the latest version.")
		case 1:
			// Local version is newer than NPM (e.g., dev build)
			fmt.Printf("Local version is newer than NPM (%s > %s).\n", current, latest)
		}
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

// fetchNPMLatestVersion returns the latest tag from NPM dist-tags for a package.
func fetchNPMLatestVersion(pkg string) (string, error) {
	// Scoped packages must be URL-encoded for this endpoint
	encoded := strings.ReplaceAll(pkg, "/", "%2F")
	url := fmt.Sprintf("https://registry.npmjs.org/-/package/%s/dist-tags", encoded)

	client := &http.Client{Timeout: 5 * time.Second}
	req, _ := http.NewRequest(http.MethodGet, url, nil)
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
	var tags map[string]string
	if err := json.Unmarshal(body, &tags); err != nil {
		return "", err
	}
	if latest, ok := tags["latest"]; ok && latest != "" {
		return latest, nil
	}
	return "", fmt.Errorf("tag 'latest' not found")
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
