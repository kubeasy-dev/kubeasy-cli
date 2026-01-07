package main

import (
	"fmt"

	"github.com/hypersequent/zen"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/validation"
)

func main() {
	c := zen.NewConverterWithOpts()

	// Génère les specs et types dépendants (ordre = dépendances d'abord)
	c.AddType(validation.Target{})
	c.AddType(validation.StatusCondition{})
	c.AddType(validation.StatusSpec{})
	c.AddType(validation.LogSpec{})
	c.AddType(validation.EventSpec{})
	c.AddType(validation.MetricCheck{})
	c.AddType(validation.MetricsSpec{})
	c.AddType(validation.SourcePod{})
	c.AddType(validation.ConnectivityCheck{})
	c.AddType(validation.ConnectivitySpec{})

	// Ne pas ajouter Validation car il a Spec/RawSpec avec interface{}

	output := "// ⚠️ AUTO-GENERATED - DO NOT EDIT\n"
	output += "// Source: github.com/kubeasy-dev/kubeasy-cli/pkg/validation\n"
	output += "// Run: go run hack/generate-schema/main.go > path/to/challengeObjectives.ts\n\n"
	output += "import { z } from \"zod\";\n\n"

	output += c.Export()

	// Ajoute l'enum et le schema principal manuellement
	output += `export const ObjectiveTypeSchema = z.enum([
  "status",
  "log",
  "event",
  "metrics",
  "connectivity",
]);
export type ObjectiveType = z.infer<typeof ObjectiveTypeSchema>;

export const ObjectiveSpecSchema = z.union([
  StatusSpecSchema,
  LogSpecSchema,
  EventSpecSchema,
  MetricsSpecSchema,
  ConnectivitySpecSchema,
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
`

	fmt.Print(output)
}