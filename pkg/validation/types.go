// Package validation provides types and executors for CLI-based validation
// of Kubernetes resources. Supports 4 validation types: status, condition, log, event,
// and connectivity. See docs/VALIDATION_EXAMPLES.md for usage examples.
package validation

import (
	corev1 "k8s.io/api/core/v1"
)

// ValidationConfig represents the top-level structure of a challenge.yaml validation section
// It contains all validations that must pass for a challenge to be considered complete
type ValidationConfig struct {
	// Validations is the list of all validation checks for a challenge
	// Note: YAML key is "objectives" to match challenge.yaml format
	Validations []Validation `yaml:"objectives" json:"objectives"`
}

// Validation represents a single validation definition with its type and specification
// Each validation maps to one objective in the challenge
type Validation struct {
	// Key is the unique identifier for this validation, used to match backend objectives
	// Convention: use kebab-case, e.g., "deployment-ready", "service-accessible"
	Key string `yaml:"key" json:"key"`
	// Title is the human-readable name displayed to users
	Title string `yaml:"title" json:"title"`
	// Description explains what this validation checks and hints at the solution
	Description string `yaml:"description" json:"description"`
	// Order determines the display sequence (lower numbers appear first)
	Order int `yaml:"order" json:"order"`
	// Type specifies which validation executor to use (status, condition, log, event, connectivity)
	Type ValidationType `yaml:"type" json:"type"`
	// Spec contains the type-specific configuration (parsed based on Type)
	// Excluded from serialization - use RawSpec for marshaling
	Spec interface{} `yaml:"-" json:"-"`
	// RawSpec holds the unparsed spec before type-specific parsing
	// Used for YAML parsing and JSON serialization of the original spec
	RawSpec interface{} `yaml:"spec" json:"spec"`
}

// ValidationType represents the type of validation to execute
// Each type has a corresponding spec structure and executor
type ValidationType string

const (
	// TypeStatus validates arbitrary status fields with operators
	// Value: "status" - Use when checking numeric fields, string values, or any status field
	TypeStatus ValidationType = "status"
	// TypeCondition validates Kubernetes resource conditions (shorthand for common condition checks)
	// Value: "condition" - Use when checking Ready, Available, Progressing conditions
	TypeCondition ValidationType = "condition"
	// TypeLog searches container logs for expected strings
	// Value: "log" - Use when verifying application behavior, startup messages, or processed requests
	TypeLog ValidationType = "log"
	// TypeEvent validates that forbidden Kubernetes events are NOT present
	// Value: "event" - Use when ensuring pods aren't crash-looping, OOMKilled, or failing to schedule
	TypeEvent ValidationType = "event"
	// TypeConnectivity tests HTTP connectivity between pods
	// Value: "connectivity" - Use when verifying Services, NetworkPolicies, or inter-pod communication
	TypeConnectivity ValidationType = "connectivity"
)

// Target identifies a Kubernetes resource to validate
// Use either Name for exact match or LabelSelector for multiple resources
type Target struct {
	// Kind is the Kubernetes resource type to target
	// Common values: Deployment, Pod, Service, StatefulSet, Job, ConfigMap, Secret, DaemonSet
	Kind string `yaml:"kind" json:"kind"`
	// Name matches a specific resource by exact name
	// If both Name and LabelSelector are provided, Name takes precedence
	Name string `yaml:"name,omitempty" json:"name,omitempty"`
	// LabelSelector matches resources by labels, e.g., {"app": "nginx", "tier": "frontend"}
	// For single-resource validations, the first matching resource is used
	LabelSelector map[string]string `yaml:"labelSelector,omitempty" json:"labelSelector,omitempty"`
}

// StatusSpec validates arbitrary status fields using operators
// Use when: checking numeric fields, string values, or any status field
// Note: Field paths are relative to status (no "status." prefix needed)
type StatusSpec struct {
	// Target specifies which Kubernetes resource to check
	Target Target `yaml:"target" json:"target"`
	// Checks lists all field validations that must pass
	Checks []StatusCheck `yaml:"checks" json:"checks"`
}

// StatusCheck defines a field-based status validation
// Compares a resource status field value against an expected value using an operator
type StatusCheck struct {
	// Field is the path to the status field to check (relative to status)
	// Examples: "readyReplicas", "containerStatuses[0].restartCount", "conditions[type=Ready].status"
	// Array indexing: [0], [1], etc.
	// Array filtering: [key=value] to find element where field equals value
	Field string `yaml:"field" json:"field"`
	// Operator is the comparison operator to use
	// Supported: "==" (equal), "!=" (not equal), ">" (greater than),
	// "<" (less than), ">=" (greater or equal), "<=" (less or equal)
	Operator string `yaml:"operator" json:"operator"`
	// Value is the expected value to compare against
	// Supports: string, int64, bool, float64
	Value interface{} `yaml:"value" json:"value"`
}

// ConditionSpec validates Kubernetes resource conditions (shorthand)
// Use when: checking standard Kubernetes conditions like Ready, Available, Progressing
// This is a convenient shorthand for the most common validation pattern
type ConditionSpec struct {
	// Target specifies which Kubernetes resource to check
	Target Target `yaml:"target" json:"target"`
	// Checks lists the condition checks that must ALL pass
	Checks []ConditionCheck `yaml:"checks" json:"checks"`
}

// ConditionCheck defines a single condition check
// Maps to Kubernetes status.conditions[] entries
type ConditionCheck struct {
	// Type is the condition type to check
	// Deployment: "Available", "Progressing", "ReplicaFailure"
	// Pod: "Ready", "ContainersReady", "Initialized", "PodScheduled"
	// StatefulSet: "Ready"
	// Job: "Complete", "Failed"
	Type string `yaml:"type" json:"type"`
	// Status is the expected condition status
	// Values: "True", "False", "Unknown"
	Status corev1.ConditionStatus `yaml:"status" json:"status"`
}

// LogSpec searches container logs for expected strings
// Use when: verifying application behavior, startup completion, or processed requests
type LogSpec struct {
	// Target specifies which Pod(s) to check logs from
	Target Target `yaml:"target" json:"target"`
	// Container specifies which container's logs to check (optional if pod has single container)
	// Required for multi-container pods, e.g., "nginx", "sidecar", "init-container"
	Container string `yaml:"container,omitempty" json:"container,omitempty"`
	// ExpectedStrings lists strings that must ALL appear in the logs
	// Tips: use unique strings, avoid timestamps, consider log format
	// Examples: "Server started", "Connected to database", "HTTP/1.1 200"
	ExpectedStrings []string `yaml:"expectedStrings" json:"expectedStrings"`
	// SinceSeconds limits log search to recent entries (optional)
	// Useful for avoiding false positives from old logs, e.g., 300 for last 5 minutes
	SinceSeconds int `yaml:"sinceSeconds,omitempty" json:"sinceSeconds,omitempty"`
}

// EventSpec checks for absence of problematic Kubernetes events
// Use when: ensuring pods aren't crash-looping or failing to pull images
type EventSpec struct {
	// Target specifies which resource's events to check
	Target Target `yaml:"target" json:"target"`
	// ForbiddenReasons lists event reasons that should NOT be present
	// Common values: CrashLoopBackOff, ImagePullBackOff, OOMKilled, Error, BackOff,
	// FailedScheduling, FailedMount, Unhealthy, Evicted, NodeNotReady
	ForbiddenReasons []string `yaml:"forbiddenReasons" json:"forbiddenReasons"`
	// SinceSeconds limits the time window for event checking (optional)
	// Events older than this are ignored, e.g., 600 for last 10 minutes
	// When omitted (0), checks all events regardless of age
	SinceSeconds int `yaml:"sinceSeconds,omitempty" json:"sinceSeconds,omitempty"`
}

// ConnectivitySpec tests HTTP connectivity between pods in the cluster
// Use when: verifying Services work, NetworkPolicies allow traffic, or inter-pod communication
type ConnectivitySpec struct {
	// SourcePod specifies which pod to execute curl/wget commands from
	// The pod must have curl or wget installed
	// Security note: ensure source pods are trusted as they execute HTTP requests
	SourcePod SourcePod `yaml:"sourcePod" json:"sourcePod"`
	// Targets lists all connectivity checks to perform from the source pod
	Targets []ConnectivityCheck `yaml:"targets" json:"targets"`
}

// SourcePod identifies the pod from which connectivity checks are executed
// Use either Name or LabelSelector to identify the pod
type SourcePod struct {
	// Name matches a specific pod by exact name (mutually exclusive with LabelSelector)
	Name string `yaml:"name,omitempty" json:"name,omitempty"`
	// LabelSelector matches pods by labels, uses first matching pod
	// Example: {"app": "curl-client"} or {"role": "tester"}
	LabelSelector map[string]string `yaml:"labelSelector,omitempty" json:"labelSelector,omitempty"`
}

// ConnectivityCheck represents a single HTTP connectivity test
// Executes an HTTP request from source pod and validates the response code
type ConnectivityCheck struct {
	// URL is the HTTP endpoint to test from the source pod
	// Format: http://service-name:port/path or http://pod-ip:port/path
	// Examples: "http://nginx-service:80/health", "http://api:8080/ready"
	// For cross-namespace: "http://service-name.namespace.svc.cluster.local:port"
	URL string `yaml:"url" json:"url"`
	// ExpectedStatusCode is the HTTP status code that indicates success
	// Common values: 200 (OK), 201 (Created), 204 (No Content), 301/302 (Redirects)
	// Use 0 to verify connection failed (timeout or refused, useful for NetworkPolicy tests)
	ExpectedStatusCode int `yaml:"expectedStatusCode" json:"expectedStatusCode"`
	// TimeoutSeconds is the maximum time to wait for a response (optional)
	// Default is typically 10 seconds, increase for slow services
	TimeoutSeconds int `yaml:"timeoutSeconds,omitempty" json:"timeoutSeconds,omitempty"`
}

// Result represents the outcome of a single validation execution
// Returned by the executor and sent to the backend API
// Note: No YAML tags as this type is only used for executor output, never parsed from challenge.yaml
type Result struct {
	// Key matches the validation key for backend correlation
	Key string `json:"key"`
	// Passed indicates whether the validation succeeded
	Passed bool `json:"passed"`
	// Message provides details about the result (success info or failure reason)
	Message string `json:"message"`
}
