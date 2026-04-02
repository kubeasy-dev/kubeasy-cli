// Package validation provides the CLI-based validation system for Kubernetes challenges.
// Spec types are defined in the vtypes sub-package; this file re-exports them as
// type aliases so all existing callers continue to work without import changes.
package validation

import "github.com/kubeasy-dev/kubeasy-cli/internal/validation/vtypes"

// Type aliases — keep all external callers working without any import changes.
type (
	ValidationConfig  = vtypes.ValidationConfig
	Validation        = vtypes.Validation
	ValidationType    = vtypes.ValidationType
	Result            = vtypes.Result
	Target            = vtypes.Target
	StatusSpec        = vtypes.StatusSpec
	StatusCheck       = vtypes.StatusCheck
	ConditionSpec     = vtypes.ConditionSpec
	ConditionCheck    = vtypes.ConditionCheck
	LogSpec           = vtypes.LogSpec
	MatchMode         = vtypes.MatchMode
	EventSpec         = vtypes.EventSpec
	ConnectivitySpec  = vtypes.ConnectivitySpec
	SourcePod         = vtypes.SourcePod
	ConnectivityCheck = vtypes.ConnectivityCheck
	TLSConfig         = vtypes.TLSConfig
	RbacSpec          = vtypes.RbacSpec
	RbacCheck         = vtypes.RbacCheck
	SpecSpec          = vtypes.SpecSpec
	SpecCheck         = vtypes.SpecCheck
	TriggeredSpec     = vtypes.TriggeredSpec
	TriggerConfig     = vtypes.TriggerConfig
	TriggerType       = vtypes.TriggerType
	TypeRegistration  = vtypes.TypeRegistration
)

// Validation type constants.
const (
	TypeStatus       = vtypes.TypeStatus
	TypeCondition    = vtypes.TypeCondition
	TypeLog          = vtypes.TypeLog
	TypeEvent        = vtypes.TypeEvent
	TypeConnectivity = vtypes.TypeConnectivity
	TypeRbac         = vtypes.TypeRbac
	TypeSpec         = vtypes.TypeSpec
	TypeTriggered    = vtypes.TypeTriggered
)

// Connectivity mode constants.
const (
	ConnectivityModeExternal = vtypes.ConnectivityModeExternal
	ConnectivityModeInternal = vtypes.ConnectivityModeInternal
)

// MatchMode constants for log validation.
const (
	MatchModeAllOf = vtypes.MatchModeAllOf
	MatchModeAnyOf = vtypes.MatchModeAnyOf
)

// Trigger type constants.
const (
	TriggerTypeLoad    = vtypes.TriggerTypeLoad
	TriggerTypeWait    = vtypes.TriggerTypeWait
	TriggerTypeDelete  = vtypes.TriggerTypeDelete
	TriggerTypeRollout = vtypes.TriggerTypeRollout
	TriggerTypeScale   = vtypes.TriggerTypeScale
)

// RegisteredTypes lists all validation types for schema generation.
// Adding a new type: add it to vtypes.RegisteredTypes in vtypes/types.go.
var RegisteredTypes = vtypes.RegisteredTypes

// ChallengeYamlSpec is the single source of truth for the challenge.yaml file format.
type ChallengeYamlSpec = vtypes.ChallengeYamlSpec

// ChallengeDifficultyValues and ChallengeTypeValues drive lint validation and Zod schema generation.
var (
	ChallengeDifficultyValues = vtypes.ChallengeDifficultyValues
	ChallengeTypeValues       = vtypes.ChallengeTypeValues
)
