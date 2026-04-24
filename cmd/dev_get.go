package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/kubeasy-dev/kubeasy-cli/internal/ui"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation"
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

var devGetCmd = &cobra.Command{
	Use:   "get [challenge-slug]",
	Short: "Display challenge metadata from local files",
	Long: `Reads challenge metadata from local challenge.yaml and displays it.
No cluster or Kubeasy API required.

It searches for challenge.yaml in the current directory or ../challenges/<slug>/.`,
	Args:          cobra.ExactArgs(1),
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		challengeSlug := args[0]

		if err := validateChallengeSlug(challengeSlug); err != nil {
			ui.Error("Invalid challenge slug")
			return err
		}

		localPath := validation.FindLocalChallengeFile(challengeSlug)
		if localPath == "" {
			ui.Error(fmt.Sprintf("Failed to find local challenge file for slug %q", challengeSlug))
			ui.Info("Checked: ./" + challengeSlug + "/challenge.yaml and ../challenges/" + challengeSlug + "/challenge.yaml")
			return fmt.Errorf("challenge file not found")
		}

		data, err := os.ReadFile(localPath)
		if err != nil {
			ui.Error(fmt.Sprintf("Failed to read challenge file: %v", err))
			return err
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
}
