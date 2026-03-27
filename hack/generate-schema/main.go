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
	output += "// Run: go run hack/generate-schema/main.go > path/to/challengeObjectives.ts\n\n"
	output += "import { z } from \"zod\";\n\n"

	output += c.Export()

	// Build enum values and union members from the registry
	enumValues := make([]string, 0, len(validation.RegisteredTypes))
	unionMembers := make([]string, 0, len(validation.RegisteredTypes))
	for _, reg := range validation.RegisteredTypes {
		enumValues = append(enumValues, fmt.Sprintf("  %q", string(reg.Type)))
		unionMembers = append(unionMembers, fmt.Sprintf("  %sSchema", reg.SpecName))
	}

	output += fmt.Sprintf(`export const ObjectiveTypeSchema = z.enum([
%s,
]);
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

	fmt.Print(output)
}
