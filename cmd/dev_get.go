package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kubeasy-dev/kubeasy-cli/internal/devutils"
	"github.com/kubeasy-dev/kubeasy-cli/internal/ui"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
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
	Objective        string `yaml:"objective"`
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
	Short: "Display local challenge metadata",
	Long: `Reads challenge.yaml from the local directory and displays its metadata,
description, objective, and objectives summary. No cluster or API required.

This is the dev equivalent of 'kubeasy challenge get'.`,
	Args:          cobra.ExactArgs(1),
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		challengeSlug := args[0]

		if err := validateChallengeSlug(challengeSlug); err != nil {
			ui.Error("Invalid challenge slug")
			return err
		}

		challengeDir, err := devutils.ResolveLocalChallengeDir(challengeSlug, devGetDir)
		if err != nil {
			ui.Error("Failed to find challenge directory")
			return err
		}

		challengeYAML := filepath.Join(challengeDir, "challenge.yaml")
		data, err := os.ReadFile(challengeYAML)
		if err != nil {
			ui.Error(fmt.Sprintf("Failed to read %s", challengeYAML))
			return err
		}

		var meta challengeMetadata
		if err := yaml.Unmarshal(data, &meta); err != nil {
			ui.Error("Failed to parse challenge.yaml")
			return fmt.Errorf("failed to parse challenge.yaml: %w", err)
		}

		// Header
		ui.Println()
		ui.Section(meta.Title)

		// Metadata
		ui.KeyValue("Slug", challengeSlug)
		ui.KeyValue("Type", meta.Type)
		ui.KeyValue("Theme", meta.Theme)
		ui.KeyValue("Difficulty", meta.Difficulty)
		ui.KeyValue("Estimated time", fmt.Sprintf("%d minutes", meta.EstimatedTime))
		ui.KeyValue("Objectives", fmt.Sprintf("%d", len(meta.Objectives)))
		ui.Println()

		// Description
		if desc := strings.TrimSpace(meta.Description); desc != "" {
			ui.Panel("Description", desc)
			ui.Println()
		}

		// Initial situation
		if sit := strings.TrimSpace(meta.InitialSituation); sit != "" {
			pterm.DefaultSection.Println("Initial Situation")
			pterm.Println(sit)
			ui.Println()
		}

		// Objective
		if obj := strings.TrimSpace(meta.Objective); obj != "" {
			pterm.DefaultSection.Println("Objective")
			pterm.Println(obj)
			ui.Println()
		}

		// Objectives list
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
	devGetCmd.Flags().StringVar(&devGetDir, "dir", "", "Path to challenge directory (default: auto-detect)")
}
