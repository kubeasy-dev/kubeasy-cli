// Package vtypes defines validation types for the Kubeasy CLI.
// Shared types (specs, targets, triggers) are re-exported from the registry package.
// CLI-specific types (Validation, Result, ValidationConfig) are defined here.
package vtypes

import (
	"time"

	"github.com/kubeasy-dev/registry/pkg/challenges"
)

// --- Shared types re-exported from the registry ---

type (
	ValidationType    = challenges.ObjectiveType
	Target            = challenges.Target
	StatusSpec        = challenges.StatusSpec
	StatusCheck       = challenges.StatusCheck
	ConditionSpec     = challenges.ConditionSpec
	ConditionCheck    = challenges.ConditionCheck
	LogSpec           = challenges.LogSpec
	MatchMode         = challenges.MatchMode
	EventSpec         = challenges.EventSpec
	ConnectivitySpec  = challenges.ConnectivitySpec
	SourcePod         = challenges.SourcePod
	ConnectivityCheck = challenges.ConnectivityCheck
	TLSConfig         = challenges.TLSConfig
	RbacSpec          = challenges.RbacSpec
	RbacCheck         = challenges.RbacCheck
	SpecSpec          = challenges.SpecSpec
	SpecCheck         = challenges.SpecCheck
	TriggerConfig     = challenges.TriggerConfig
	TriggerType       = challenges.TriggerType
)

// Validation type constants.
const (
	TypeStatus       = challenges.TypeStatus
	TypeCondition    = challenges.TypeCondition
	TypeLog          = challenges.TypeLog
	TypeEvent        = challenges.TypeEvent
	TypeConnectivity = challenges.TypeConnectivity
	TypeRbac         = challenges.TypeRbac
	TypeSpec         = challenges.TypeSpec
	TypeTriggered    = challenges.TypeTriggered
)

// Connectivity mode constants.
const (
	ConnectivityModeExternal = "external"
	ConnectivityModeInternal = "internal"
)

// MatchMode constants for log validation.
const (
	MatchModeAllOf = challenges.MatchModeAllOf
	MatchModeAnyOf = challenges.MatchModeAnyOf
)

// Trigger type constants.
const (
	TriggerTypeLoad    = challenges.TriggerTypeLoad
	TriggerTypeWait    = challenges.TriggerTypeWait
	TriggerTypeDelete  = challenges.TriggerTypeDelete
	TriggerTypeRollout = challenges.TriggerTypeRollout
	TriggerTypeScale   = challenges.TriggerTypeScale
)

// DifficultyValues and ChallengeTypeValues drive lint validation.
var (
	ChallengeDifficultyValues = challenges.DifficultyValues
	ChallengeTypeValues       = challenges.ChallengeTypeValues
)

// --- CLI-specific types ---

// ValidationConfig is the top-level structure holding all validations for a challenge.
type ValidationConfig struct {
	Validations []Validation `yaml:"objectives" json:"objectives"`
}

// Validation is a single validation check ready for execution.
type Validation struct {
	Key         string         `yaml:"key" json:"key"`
	Title       string         `yaml:"title" json:"title"`
	Description string         `yaml:"description" json:"description"`
	Order       int            `yaml:"order" json:"order"`
	Type        ValidationType `yaml:"type" json:"type"`
	// Spec is the typed spec (e.g. StatusSpec, LogSpec). Populated by fromObjective().
	Spec interface{} `yaml:"-" json:"-"`
}

// TriggeredSpec orchestrates a trigger action followed by CLI Validation validators.
// Uses []Validation for Then (not []Objective) to carry typed Spec values.
type TriggeredSpec struct {
	Trigger          TriggerConfig `yaml:"trigger" json:"trigger"`
	WaitAfterSeconds int           `yaml:"waitAfterSeconds" json:"waitAfterSeconds"`
	Then             []Validation  `yaml:"then" json:"then"`
}

// Result is the outcome of a single validation execution.
type Result struct {
	Key      string        `json:"key"`
	Passed   bool          `json:"passed"`
	Message  string        `json:"message"`
	Duration time.Duration `json:"-"`
}

// ChallengeYamlSpec represents the full structure of a challenge.yaml file.
// Used for lint and dev commands. Objectives use []Validation for two-step YAML parsing.
type ChallengeYamlSpec struct {
	Title              string       `yaml:"title"`
	Description        string       `yaml:"description"`
	Theme              string       `yaml:"theme"`
	Difficulty         string       `yaml:"difficulty"`
	Type               string       `yaml:"type"`
	EstimatedTime      int          `yaml:"estimatedTime"`
	InitialSituation   string       `yaml:"initialSituation"`
	MinRequiredVersion string       `yaml:"minRequiredVersion,omitempty"`
	Objectives         []Validation `yaml:"objectives"`
}

// TypeRegistration associates a ValidationType with its spec struct for schema generation.
type TypeRegistration struct {
	Type     ValidationType
	Spec     interface{}
	SpecName string
}

// RegisteredTypes lists all validation types in display order for schema generation.
// This is used by external tools (like hack/generate-schema) to produce the
// JSON Schema/Zod definitions for the challenge.yaml format.
var RegisteredTypes = []TypeRegistration{
	{TypeStatus, StatusSpec{}, "StatusSpec"},
	{TypeCondition, ConditionSpec{}, "ConditionSpec"},
	{TypeLog, LogSpec{}, "LogSpec"},
	{TypeEvent, EventSpec{}, "EventSpec"},
	{TypeConnectivity, ConnectivitySpec{}, "ConnectivitySpec"},
	{TypeRbac, RbacSpec{}, "RbacSpec"},
	{TypeSpec, SpecSpec{}, "SpecSpec"},
	{TypeTriggered, TriggeredSpec{}, "TriggeredSpec"},
}
