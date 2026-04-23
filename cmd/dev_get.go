package cmd

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/kubeasy-dev/kubeasy-cli/internal/devutils"
	"github.com/kubeasy-dev/kubeasy-cli/internal/ui"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"
)

// challengeMetadata holds the full challenge.yaml content for display.
type challengeMetadata struct {
	Title            string `yaml:"title"`
	Type             string `yaml:"type"`
	Theme            string `yaml:"theme"`
	Difficulty       string `yaml:"difficulty"`
	EstimatedTime    int    `yaml:"estimatedTime"`
	Description      string `yaml:"description"`
	InitialSituation string `yaml:"initialSituation"`
	Objectives       []struct {
		Key   string `yaml:"key"`
		Title string `yaml:"title"`
		Order int    `yaml:"order"`
		Type  string `yaml:"type"`
	} `yaml:"objectives"`
}

var devGetDir string

var devGetCmd = &cobra.Command{
	Use:   "get [challenge-slug]",
	Short: "Display challenge metadata from the local registry",
	Long: `Fetches challenge metadata from the local registry and displays it.
No cluster or Kubeasy API required.

Use --dir to read from a local directory instead of the registry.
This is the dev equivalent of 'kubeasy challenge get'.`,
	Args:          cobra.ExactArgs(1),
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		challengeSlug := args[0]

		if err := validateChallengeSlug(challengeSlug); err != nil {
			ui.Error("Invalid challenge slug")
			return err
		}

		var data []byte

		if devGetDir != "" {
			// Filesystem mode.
			challengeDir, err := devutils.ResolveLocalChallengeDir(challengeSlug, devGetDir)
			if err != nil {
				ui.Error("Failed to find challenge directory")
				return err
			}
			data, err = os.ReadFile(filepath.Join(challengeDir, "challenge.yaml"))
			if err != nil {
				ui.Error("Failed to read challenge.yaml")
				return err
			}
		} else {
			// Registry mode.
			url := fmt.Sprintf("%s/challenges/%s/yaml", devRegistryURL, challengeSlug)
			resp, err := http.Get(url) //nolint:noctx,gosec
			if err != nil {
				ui.Error(fmt.Sprintf("Failed to reach registry: %v", err))
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("registry returned HTTP %d for challenge %q", resp.StatusCode, challengeSlug)
			}
			data, err = io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("failed to read response: %w", err)
			}
		}

		var meta challengeMetadata
		if err := yaml.Unmarshal(data, &meta); err != nil {
			ui.Error("Failed to parse challenge.yaml")
			return fmt.Errorf("failed to parse challenge.yaml: %w", err)
		}

		ui.Println()
		ui.Section(meta.Title)

		ui.KeyValue("Slug", challengeSlug)
		ui.KeyValue("Type", meta.Type)
		ui.KeyValue("Theme", meta.Theme)
		ui.KeyValue("Difficulty", meta.Difficulty)
		ui.KeyValue("Estimated time", fmt.Sprintf("%d minutes", meta.EstimatedTime))
		ui.KeyValue("Objectives", fmt.Sprintf("%d", len(meta.Objectives)))
		ui.Println()

		if desc := strings.TrimSpace(meta.Description); desc != "" {
			ui.Panel("Description", desc)
			ui.Println()
		}

		if sit := strings.TrimSpace(meta.InitialSituation); sit != "" {
			pterm.DefaultSection.Println("Initial Situation")
			pterm.Println(sit)
			ui.Println()
		}

		if len(meta.Objectives) > 0 {
			pterm.DefaultSection.Println("Validation Objectives")
			rows := make([][]string, 0, len(meta.Objectives))
			for _, o := range meta.Objectives {
				rows = append(rows, []string{
					fmt.Sprintf("%d", o.Order),
					o.Key,
					o.Title,
					o.Type,
				})
			}
			if err := ui.Table([]string{"#", "KEY", "TITLE", "TYPE"}, rows); err != nil {
				return fmt.Errorf("failed to render table: %w", err)
			}
		}

		return nil
	},
}

func init() {
	devCmd.AddCommand(devGetCmd)
	devGetCmd.Flags().StringVar(&devGetDir, "dir", "", "Read from local directory instead of registry")
}
