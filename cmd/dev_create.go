package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/template"

	"github.com/kubeasy-dev/kubeasy-cli/internal/api"
	"github.com/kubeasy-dev/kubeasy-cli/internal/devutils"
	"github.com/kubeasy-dev/kubeasy-cli/internal/logger"
	"github.com/kubeasy-dev/kubeasy-cli/internal/ui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var defaultChallengeTypes = devutils.ValidTypes

var defaultChallengeThemes = []string{
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

var defaultChallengeDifficulties = devutils.ValidDifficulties

// fetchMetadata fetches challenge types, themes, and difficulties from the API.
// Falls back to hardcoded defaults when the API is unreachable.
func fetchMetadata() (types, themes, difficulties []string) {
	types = defaultChallengeTypes
	themes = defaultChallengeThemes
	difficulties = defaultChallengeDifficulties

	if t, err := api.GetTypes(); err == nil {
		types = t
	} else {
		logger.Debug("Failed to fetch types from API: %v", err)
		ui.Warning("Could not fetch challenge types from API — using offline defaults.")
	}

	if t, err := api.GetThemes(); err == nil {
		themes = t
	} else {
		logger.Debug("Failed to fetch themes from API: %v", err)
		ui.Warning("Could not fetch challenge themes from API — using offline defaults.")
	}

	if d, err := api.GetDifficulties(); err == nil {
		difficulties = d
	} else {
		logger.Debug("Failed to fetch difficulties from API: %v", err)
		ui.Warning("Could not fetch challenge difficulties from API — using offline defaults.")
	}

	return types, themes, difficulties
}

// challengeYAMLTemplate is a well-commented template for new challenge.yaml files.
var challengeYAMLTemplate = template.Must(template.New("challenge.yaml").Parse(`title: "{{.Title}}"
type: "{{.Type}}"
theme: "{{.Theme}}"
difficulty: "{{.Difficulty}}"
estimatedTime: {{.EstimatedTime}}

# TODO: Describe the symptoms the user will observe (NOT the root cause).
# Example: "A web application deployed in the cluster is not responding to requests.
#           The pod seems to start but crashes shortly after."
description: |
  TODO: Write a description of the observable symptoms here.

# TODO: Describe what the user will find when they start the challenge.
# Example: "A single pod is deployed in the namespace running a Node.js application.
#           A Service is configured to expose it within the cluster."
initialSituation: |
  TODO: Describe the initial state of the cluster here.

# TODO: State what the user needs to achieve (NOT how to do it).
# Example: "Make the application accessible and running stably."
objective: |
  TODO: Write the objective here.

# Define validation objectives that check the user's solution.
# Each objective runs against the cluster to verify the fix.
objectives: []
  # Example condition objective (uncomment and adapt):
  #
  # - key: pod-running
  #   title: "Application Running"
  #   description: "The application pod must be in Ready state"
  #   order: 1
  #   type: condition
  #   spec:
  #     target:
  #       kind: Pod
  #       labelSelector:
  #         app: my-app
  #     checks:
  #       - type: Ready
  #         status: "True"
  #
  # - key: no-crashes
  #   title: "Stable Operation"
  #   description: "No crash or eviction events"
  #   order: 2
  #   type: event
  #   spec:
  #     target:
  #       kind: Pod
  #       labelSelector:
  #         app: my-app
  #     forbiddenReasons:
  #       - "OOMKilled"
  #       - "CrashLoopBackOff"
  #     sinceSeconds: 300
`))

// deploymentManifestTemplate generates a starter deployment.yaml for a challenge.
var deploymentManifestTemplate = template.Must(template.New("deployment.yaml").Parse(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{.Slug}}
  labels:
    app: {{.Slug}}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: {{.Slug}}
  template:
    metadata:
      labels:
        app: {{.Slug}}
    spec:
      containers:
        - name: {{.Slug}}
          image: nginx:latest
          ports:
            - containerPort: 80
`))

// serviceManifestTemplate generates a starter service.yaml for a challenge.
var serviceManifestTemplate = template.Must(template.New("service.yaml").Parse(`apiVersion: v1
kind: Service
metadata:
  name: {{.Slug}}
  labels:
    app: {{.Slug}}
spec:
  selector:
    app: {{.Slug}}
  ports:
    - port: 80
      targetPort: 80
`))

// dev create flags
var (
	devCreateName          string
	devCreateSlug          string
	devCreateType          string
	devCreateTheme         string
	devCreateDifficulty    string
	devCreateEstimatedTime int
	devCreateWithManifests bool
)

var devCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Scaffold a new challenge directory",
	Long: `Creates a new challenge directory with a challenge.yaml template
and the required folder structure (manifests/, policies/).
The challenge is created in the current working directory.

In interactive mode (TTY), prompts guide you through the setup.
In non-interactive mode, use flags: --name, --type, --theme, --difficulty.`,
	Args:          cobra.NoArgs,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.Section("Create New Challenge")

		challengeTypes, challengeThemes, challengeDifficulties := fetchMetadata()

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

		slug = strings.TrimSpace(slug)
		if slug == "" {
			slug = devutils.GenerateSlug(name)
		}

		if err := validateChallengeSlug(slug); err != nil {
			return err
		}

		if !slices.Contains(challengeTypes, challengeType) {
			return fmt.Errorf("invalid type '%s' (valid: %s)", challengeType, strings.Join(challengeTypes, ", "))
		}
		if !slices.Contains(challengeThemes, theme) {
			return fmt.Errorf("invalid theme '%s' (valid: %s)", theme, strings.Join(challengeThemes, ", "))
		}
		if !slices.Contains(challengeDifficulties, difficulty) {
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
				// Clean up partially created directories
				_ = os.RemoveAll(slug)
				return fmt.Errorf("failed to create directory %s: %w", dir, err)
			}
		}

		// Clean up on any subsequent error
		success := false
		defer func() {
			if !success {
				_ = os.RemoveAll(slug)
			}
		}()

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

		// Generate challenge.yaml from template
		var buf bytes.Buffer
		err := challengeYAMLTemplate.Execute(&buf, struct {
			Title         string
			Type          string
			Theme         string
			Difficulty    string
			EstimatedTime int
		}{
			Title:         name,
			Type:          challengeType,
			Theme:         theme,
			Difficulty:    difficulty,
			EstimatedTime: estimatedTime,
		})
		if err != nil {
			return fmt.Errorf("failed to generate challenge.yaml: %w", err)
		}

		challengeYAMLPath := filepath.Join(slug, "challenge.yaml")
		if err := os.WriteFile(challengeYAMLPath, buf.Bytes(), 0o600); err != nil {
			return fmt.Errorf("failed to write challenge.yaml: %w", err)
		}

		// Generate manifest templates
		generateManifests := devCreateWithManifests
		if !generateManifests && interactive {
			generateManifests = ui.Confirmation("Generate starter manifests? (deployment + service)")
		}

		if generateManifests {
			tmplData := struct{ Slug string }{Slug: slug}

			var deployBuf bytes.Buffer
			if err := deploymentManifestTemplate.Execute(&deployBuf, tmplData); err != nil {
				return fmt.Errorf("failed to generate deployment.yaml: %w", err)
			}
			deployPath := filepath.Join(slug, "manifests", "deployment.yaml")
			if err := os.WriteFile(deployPath, deployBuf.Bytes(), 0o600); err != nil {
				return fmt.Errorf("failed to write deployment.yaml: %w", err)
			}

			var svcBuf bytes.Buffer
			if err := serviceManifestTemplate.Execute(&svcBuf, tmplData); err != nil {
				return fmt.Errorf("failed to generate service.yaml: %w", err)
			}
			svcPath := filepath.Join(slug, "manifests", "service.yaml")
			if err := os.WriteFile(svcPath, svcBuf.Bytes(), 0o600); err != nil {
				return fmt.Errorf("failed to write service.yaml: %w", err)
			}

			// Remove .gitkeep since we now have actual files
			_ = os.Remove(filepath.Join(slug, "manifests", ".gitkeep"))
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
		nextSteps := []string{}
		if !generateManifests {
			nextSteps = append(nextSteps, fmt.Sprintf("Add your Kubernetes manifests to %s/manifests/", slug))
		} else {
			nextSteps = append(nextSteps, fmt.Sprintf("Edit the generated manifests in %s/manifests/", slug))
		}
		nextSteps = append(nextSteps,
			fmt.Sprintf("Edit %s to fill in description, objectives, and validations", challengeYAMLPath),
			fmt.Sprintf("Run 'kubeasy dev apply %s' to deploy locally", slug),
			fmt.Sprintf("Run 'kubeasy dev validate %s' to test validations", slug),
		)
		_ = ui.BulletList(nextSteps)

		success = true
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
	devCreateCmd.Flags().BoolVar(&devCreateWithManifests, "with-manifests", false, "Generate starter deployment and service manifests")
}
