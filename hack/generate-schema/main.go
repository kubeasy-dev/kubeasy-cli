package main

import (
	"fmt"
	"strings"

	"github.com/hypersequent/zen"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation"
)

func main() {
	c := zen.NewConverterWithOpts()

	// Register shared types that specs depend on (must come before specs)
	c.AddType(validation.Target{})
	c.AddType(validation.StatusCheck{})
	c.AddType(validation.ConditionCheck{})
	c.AddType(validation.SourcePod{})
	c.AddType(validation.ConnectivityCheck{})
	c.AddType(validation.RbacCheck{})

	// Register all spec types from the canonical registry — no manual updates needed
	// when adding a new validation type: just update validation.RegisteredTypes.
	for _, reg := range validation.RegisteredTypes {
		c.AddType(reg.Spec)
	}

	// Note: Validation itself is not added — it has Spec/RawSpec as interface{}

	output := "// ⚠️ AUTO-GENERATED - DO NOT EDIT\n"
	output += "// Source: github.com/kubeasy-dev/kubeasy-cli/internal/validation\n"
	output += "// Run: go run hack/generate-schema/main.go > path/to/challengeObjectives.ts\n"
	output += "// biome-ignore-all lint: auto-generated file\n\n"
	output += "import { z } from \"zod\";\n\n"

	output += c.Export()

	// Build enum values and union members from the registry
	enumValues := make([]string, 0, len(validation.RegisteredTypes))
	unionMembers := make([]string, 0, len(validation.RegisteredTypes))
	for _, reg := range validation.RegisteredTypes {
		enumValues = append(enumValues, fmt.Sprintf("  %q", string(reg.Type)))
		unionMembers = append(unionMembers, fmt.Sprintf("  %sSchema", reg.SpecName))
	}

	output += fmt.Sprintf(`export const objectiveTypeValues = [
%s,
] as const;
export const ObjectiveTypeSchema = z.enum(objectiveTypeValues);
export type ObjectiveType = z.infer<typeof ObjectiveTypeSchema>;

export const ObjectiveSpecSchema = z.union([
%s,
]);
export type ObjectiveSpec = z.infer<typeof ObjectiveSpecSchema>;

export const ObjectiveSchema = z.object({
  key: z.string(),
  title: z.string(),
  description: z.string(),
  order: z.number().int(),
  type: ObjectiveTypeSchema,
  spec: ObjectiveSpecSchema,
});
export type Objective = z.infer<typeof ObjectiveSchema>;
`,
		strings.Join(enumValues, ",\n"),
		strings.Join(unionMembers, ",\n"),
	)

	// Build ChallengeYamlSchema from ChallengeDifficultyValues and ChallengeTypeValues
	diffValues := make([]string, len(validation.ChallengeDifficultyValues))
	for i, v := range validation.ChallengeDifficultyValues {
		diffValues[i] = fmt.Sprintf("%q", v)
	}
	typeValues := make([]string, len(validation.ChallengeTypeValues))
	for i, v := range validation.ChallengeTypeValues {
		typeValues[i] = fmt.Sprintf("%q", v)
	}

	output += fmt.Sprintf(`
export const challengeYamlDifficultyValues = [%s] as const;
export const ChallengeYamlDifficultySchema = z.enum(challengeYamlDifficultyValues);
export type ChallengeYamlDifficulty = z.infer<typeof ChallengeYamlDifficultySchema>;

export const challengeYamlTypeValues = [%s] as const;
export const ChallengeYamlTypeSchema = z.enum(challengeYamlTypeValues);
export type ChallengeYamlType = z.infer<typeof ChallengeYamlTypeSchema>;

// ChallengeYamlSchema is the single source of truth for the challenge.yaml file format.
// Generated from ChallengeYamlSpec in github.com/kubeasy-dev/kubeasy-cli/internal/validation/vtypes.
// Required fields map to non-omitempty struct fields; optional fields map to omitempty fields.
export const ChallengeYamlSchema = z.object({
  title: z.string().min(1),
  description: z.string().min(1),
  theme: z.string().min(1),
  difficulty: ChallengeYamlDifficultySchema,
  type: ChallengeYamlTypeSchema.default("fix"),
  estimatedTime: z.number().int().positive(),
  initialSituation: z.string().min(1),
  minRequiredVersion: z.string().optional(),
  objectives: z.array(ObjectiveSchema).default([]),
});
export type ChallengeYaml = z.infer<typeof ChallengeYamlSchema>;
`,
		strings.Join(diffValues, ", "),
		strings.Join(typeValues, ", "),
	)

	fmt.Print(output)
}
