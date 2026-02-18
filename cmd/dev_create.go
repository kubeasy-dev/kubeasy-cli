package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kubeasy-dev/kubeasy-cli/internal/devutils"
	"github.com/kubeasy-dev/kubeasy-cli/internal/ui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

var challengeTypes = []string{"fix", "build", "migrate"}

var challengeThemes = []string{
	"ingress-tls",
	"jobs-cronjobs",
	"monitoring-debugging",
	"networking",
	"pods-containers",
	"rbac-security",
	"resources-scaling",
	"scheduling-affinity",
	"volumes-secrets",
}

var challengeDifficulties = []string{"easy", "medium", "hard"}

// challengeConfig matches the official challenge.yaml schema (camelCase keys)
type challengeConfig struct {
	Title            string        `yaml:"title"`
	Description      string        `yaml:"description"`
	Theme            string        `yaml:"theme"`
	Type             string        `yaml:"type"`
	Difficulty       string        `yaml:"difficulty"`
	EstimatedTime    int           `yaml:"estimatedTime"`
	InitialSituation string        `yaml:"initialSituation"`
	Objective        string        `yaml:"objective"`
	OfTheWeek        bool          `yaml:"ofTheWeek"`
	StarterFriendly  bool          `yaml:"starterFriendly"`
	Objectives       []interface{} `yaml:"objectives"`
}

// dev create flags
var (
	devCreateName          string
	devCreateSlug          string
	devCreateType          string
	devCreateTheme         string
	devCreateDifficulty    string
	devCreateEstimatedTime int
)

func containsValue(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}

var devCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Scaffold a new challenge directory",
	Long: `Creates a new challenge directory with a challenge.yaml template
and the required folder structure (manifests/, policies/).
The challenge is created in the current working directory.

In interactive mode (TTY), prompts guide you through the setup.
In non-interactive mode, use flags: --name, --type, --theme, --difficulty.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.Section("Create New Challenge")

		interactive := term.IsTerminal(int(os.Stdin.Fd()))

		name := devCreateName
		slug := devCreateSlug
		challengeType := devCreateType
		theme := devCreateTheme
		difficulty := devCreateDifficulty
		estimatedTime := devCreateEstimatedTime

		if interactive {
			var err error

			// 1. Challenge name
			if name == "" {
				name, err = ui.TextInput("Challenge name")
				if err != nil {
					return fmt.Errorf("failed to read challenge name: %w", err)
				}
			}
			name = strings.TrimSpace(name)
			if name == "" {
				return fmt.Errorf("challenge name cannot be empty")
			}

			// 2. Slug
			if slug == "" {
				suggestedSlug := devutils.GenerateSlug(name)
				slug, err = ui.TextInputWithDefault("Challenge slug", suggestedSlug)
				if err != nil {
					return fmt.Errorf("failed to read challenge slug: %w", err)
				}
			}

			// 3. Type
			if challengeType == "" {
				challengeType, err = ui.Select("Challenge type", challengeTypes)
				if err != nil {
					return fmt.Errorf("failed to select challenge type: %w", err)
				}
			}

			// 4. Theme
			if theme == "" {
				theme, err = ui.Select("Challenge theme", challengeThemes)
				if err != nil {
					return fmt.Errorf("failed to select challenge theme: %w", err)
				}
			}

			// 5. Difficulty
			if difficulty == "" {
				difficulty, err = ui.Select("Difficulty", challengeDifficulties)
				if err != nil {
					return fmt.Errorf("failed to select difficulty: %w", err)
				}
			}

			// 6. Estimated time
			if estimatedTime <= 0 {
				timeStr, err := ui.TextInputWithDefault("Estimated time (minutes)", "30")
				if err != nil {
					return fmt.Errorf("failed to read estimated time: %w", err)
				}
				if _, scanErr := fmt.Sscanf(strings.TrimSpace(timeStr), "%d", &estimatedTime); scanErr != nil {
					return fmt.Errorf("estimated time must be a positive integer")
				}
			}
		} else {
			// Non-interactive: validate required flags
			if name == "" {
				return fmt.Errorf("--name is required in non-interactive mode")
			}
			if challengeType == "" {
				return fmt.Errorf("--type is required in non-interactive mode")
			}
			if theme == "" {
				return fmt.Errorf("--theme is required in non-interactive mode")
			}
			if difficulty == "" {
				return fmt.Errorf("--difficulty is required in non-interactive mode")
			}
		}

		// Defaults
		name = strings.TrimSpace(name)
		if name == "" {
			return fmt.Errorf("challenge name cannot be empty")
		}

		slug = strings.TrimSpace(slug)
		if slug == "" {
			slug = devutils.GenerateSlug(name)
		}

		if err := validateChallengeSlug(slug); err != nil {
			return err
		}

		if !containsValue(challengeTypes, challengeType) {
			return fmt.Errorf("invalid type '%s' (valid: %s)", challengeType, strings.Join(challengeTypes, ", "))
		}
		if !containsValue(challengeThemes, theme) {
			return fmt.Errorf("invalid theme '%s' (valid: %s)", theme, strings.Join(challengeThemes, ", "))
		}
		if !containsValue(challengeDifficulties, difficulty) {
			return fmt.Errorf("invalid difficulty '%s' (valid: %s)", difficulty, strings.Join(challengeDifficulties, ", "))
		}

		if estimatedTime <= 0 {
			estimatedTime = 30
		}

		// Check if directory already exists
		if _, err := os.Stat(slug); err == nil {
			return fmt.Errorf("directory '%s' already exists", slug)
		}

		// Create directory structure
		dirs := []string{
			slug,
			filepath.Join(slug, "manifests"),
			filepath.Join(slug, "policies"),
		}
		for _, dir := range dirs {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", dir, err)
			}
		}

		// Create .gitkeep files
		gitkeepPaths := []string{
			filepath.Join(slug, "manifests", ".gitkeep"),
			filepath.Join(slug, "policies", ".gitkeep"),
		}
		for _, gk := range gitkeepPaths {
			if err := os.WriteFile(gk, []byte{}, 0o600); err != nil {
				return fmt.Errorf("failed to create %s: %w", gk, err)
			}
		}

		// Generate challenge.yaml
		config := challengeConfig{
			Title:            name,
			Description:      "",
			Theme:            theme,
			Type:             challengeType,
			Difficulty:       difficulty,
			EstimatedTime:    estimatedTime,
			InitialSituation: "",
			Objective:        "",
			OfTheWeek:        false,
			StarterFriendly:  false,
			Objectives:       []interface{}{},
		}

		yamlData, err := yaml.Marshal(&config)
		if err != nil {
			return fmt.Errorf("failed to generate challenge.yaml: %w", err)
		}

		challengeYAMLPath := filepath.Join(slug, "challenge.yaml")
		if err := os.WriteFile(challengeYAMLPath, yamlData, 0o600); err != nil {
			return fmt.Errorf("failed to write challenge.yaml: %w", err)
		}

		// Display summary
		ui.Println()
		ui.Success(fmt.Sprintf("Challenge '%s' created!", slug))
		ui.Println()
		ui.KeyValue("Directory", slug+"/")
		ui.KeyValue("Type", challengeType)
		ui.KeyValue("Theme", theme)
		ui.KeyValue("Difficulty", difficulty)
		ui.KeyValue("Estimated time", fmt.Sprintf("%d minutes", estimatedTime))
		ui.Println()
		ui.Info("Next steps:")
		_ = ui.BulletList([]string{
			fmt.Sprintf("Add your Kubernetes manifests to %s/manifests/", slug),
			fmt.Sprintf("Edit %s to fill in description, objectives, and validations", challengeYAMLPath),
			fmt.Sprintf("Run 'kubeasy dev apply %s' to deploy locally", slug),
			fmt.Sprintf("Run 'kubeasy dev validate %s' to test validations", slug),
		})

		return nil
	},
}

func init() {
	devCmd.AddCommand(devCreateCmd)
	devCreateCmd.Flags().StringVar(&devCreateName, "name", "", "Challenge name")
	devCreateCmd.Flags().StringVar(&devCreateSlug, "slug", "", "Challenge slug (auto-generated from name if omitted)")
	devCreateCmd.Flags().StringVar(&devCreateType, "type", "", "Challenge type (fix, build, migrate)")
	devCreateCmd.Flags().StringVar(&devCreateTheme, "theme", "", "Challenge theme")
	devCreateCmd.Flags().StringVar(&devCreateDifficulty, "difficulty", "", "Challenge difficulty (easy, medium, hard)")
	devCreateCmd.Flags().IntVar(&devCreateEstimatedTime, "estimated-time", 0, "Estimated time in minutes (default: 30)")
}
