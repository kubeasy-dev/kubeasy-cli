package validation

// ValidationConfig represents the top-level structure of a challenge.yaml validation section
// It contains all validations that must pass for a challenge to be considered complete
type ValidationConfig struct {
	// Validations is the list of all validation checks for a challenge
	Validations []Validation `yaml:"validations"`
}

// Validation represents a single validation definition with its type and specification
// Each validation maps to one objective in the challenge
type Validation struct {
	// Key is the unique identifier for this validation, used to match backend objectives
	// Convention: use kebab-case, e.g., "deployment-ready", "service-accessible"
	Key string `yaml:"key"`
	// Title is the human-readable name displayed to users
	Title string `yaml:"title"`
	// Description explains what this validation checks and hints at the solution
	Description string `yaml:"description"`
	// Order determines the display sequence (lower numbers appear first)
	Order int `yaml:"order"`
	// Type specifies which validation executor to use (status, log, event, metrics, connectivity)
	Type ValidationType `yaml:"type"`
	// Spec contains the type-specific configuration (parsed based on Type)
	Spec interface{} `yaml:"-"` // Parsed based on Type
	// RawSpec holds the unparsed YAML spec before type-specific parsing
	RawSpec interface{} `yaml:"spec"`
}

// ValidationType represents the type of validation to execute
// Each type has a corresponding spec structure and executor
type ValidationType string

const (
	// TypeStatus validates Kubernetes resource conditions (Ready, Available, Progressing)
	// Use when: checking if Deployments, Pods, StatefulSets are in expected state
	TypeStatus ValidationType = "status"
	// TypeLog searches container logs for expected strings
	// Use when: verifying application behavior, startup messages, or processed requests
	TypeLog ValidationType = "log"
	// TypeEvent checks for absence of problematic Kubernetes events
	// Use when: ensuring pods aren't crash-looping, OOMKilled, or failing to schedule
	TypeEvent ValidationType = "event"
	// TypeMetrics validates numeric fields from resource status
	// Use when: checking replica counts, restart counts, or other numeric conditions
	TypeMetrics ValidationType = "metrics"
	// TypeConnectivity tests HTTP connectivity between pods
	// Use when: verifying Services, NetworkPolicies, or inter-pod communication
	TypeConnectivity ValidationType = "connectivity"
)

// Target identifies a Kubernetes resource to validate
// Use either Name for exact match or LabelSelector for multiple resources
type Target struct {
	// Kind is the Kubernetes resource type to target
	// Common values: Deployment, Pod, Service, StatefulSet, Job, ConfigMap, Secret, DaemonSet
	Kind string `yaml:"kind"`
	// Name matches a specific resource by exact name (mutually exclusive with LabelSelector)
	Name string `yaml:"name,omitempty"`
	// LabelSelector matches resources by labels, e.g., {"app": "nginx", "tier": "frontend"}
	// Returns first matching resource for single-resource validations
	LabelSelector map[string]string `yaml:"labelSelector,omitempty"`
}

// StatusSpec validates Kubernetes resource conditions (Ready, Available, etc.)
// Use when: checking if a Deployment/Pod/StatefulSet is in the expected state
type StatusSpec struct {
	// Target specifies which Kubernetes resource to check
	Target Target `yaml:"target"`
	// Conditions lists the status conditions that must ALL be met
	// Deployment conditions: Available, Progressing, ReplicaFailure
	// Pod conditions: Ready, ContainersReady, Initialized, PodScheduled
	Conditions []StatusCondition `yaml:"conditions"`
}

// StatusCondition represents a single condition to verify on a resource
// Maps to Kubernetes status.conditions[] entries
type StatusCondition struct {
	// Type is the condition type to check
	// Deployment: "Available", "Progressing", "ReplicaFailure"
	// Pod: "Ready", "ContainersReady", "Initialized", "PodScheduled"
	Type string `yaml:"type"`
	// Status is the expected condition status
	// Values: "True", "False", "Unknown"
	Status string `yaml:"status"`
}

// LogSpec searches container logs for expected strings
// Use when: verifying application behavior, startup completion, or processed requests
type LogSpec struct {
	// Target specifies which Pod(s) to check logs from
	Target Target `yaml:"target"`
	// Container specifies which container's logs to check (optional if pod has single container)
	// Required for multi-container pods, e.g., "nginx", "sidecar", "init-container"
	Container string `yaml:"container,omitempty"`
	// ExpectedStrings lists strings that must ALL appear in the logs
	// Tips: use unique strings, avoid timestamps, consider log format
	// Examples: "Server started", "Connected to database", "HTTP/1.1 200"
	ExpectedStrings []string `yaml:"expectedStrings"`
	// SinceSeconds limits log search to recent entries (optional)
	// Useful for avoiding false positives from old logs, e.g., 300 for last 5 minutes
	SinceSeconds int `yaml:"sinceSeconds,omitempty"`
}

// EventSpec checks for absence of problematic Kubernetes events
// Use when: ensuring pods aren't crash-looping or failing to pull images
type EventSpec struct {
	// Target specifies which resource's events to check
	Target Target `yaml:"target"`
	// ForbiddenReasons lists event reasons that should NOT be present
	// Common values: CrashLoopBackOff, ImagePullBackOff, OOMKilled, Error, BackOff,
	// FailedScheduling, FailedMount, Unhealthy, Evicted, NodeNotReady
	ForbiddenReasons []string `yaml:"forbiddenReasons"`
	// SinceSeconds limits the time window for event checking (optional)
	// Events older than this are ignored, e.g., 600 for last 10 minutes
	SinceSeconds int `yaml:"sinceSeconds,omitempty"`
}

// MetricsSpec validates numeric fields from resource status
// Use when: checking replica counts, restart counts, or resource quantities
type MetricsSpec struct {
	// Target specifies which Kubernetes resource to check
	Target Target `yaml:"target"`
	// Checks lists all numeric field validations that must pass
	Checks []MetricCheck `yaml:"checks"`
}

// MetricCheck represents a single numeric field validation
// Compares a resource field value against an expected value using an operator
type MetricCheck struct {
	// Field is the dot-notation path to the numeric field in the resource
	// Common paths: status.readyReplicas, status.availableReplicas, status.replicas,
	// spec.replicas, status.containerStatuses[0].restartCount
	Field string `yaml:"field"`
	// Operator is the comparison operator to use
	// Supported: "==" (equal), "!=" (not equal), ">" (greater than),
	// "<" (less than), ">=" (greater or equal), "<=" (less or equal)
	Operator string `yaml:"operator"`
	// Value is the expected numeric value to compare against
	Value int64 `yaml:"value"`
}

// ConnectivitySpec tests HTTP connectivity between pods in the cluster
// Use when: verifying Services work, NetworkPolicies allow traffic, or inter-pod communication
type ConnectivitySpec struct {
	// SourcePod specifies which pod to execute curl/wget commands from
	// The pod must have curl or wget installed
	SourcePod SourcePod `yaml:"sourcePod"`
	// Targets lists all connectivity checks to perform from the source pod
	Targets []ConnectivityCheck `yaml:"targets"`
}

// SourcePod identifies the pod from which connectivity checks are executed
// Use either Name or LabelSelector to identify the pod
type SourcePod struct {
	// Name matches a specific pod by exact name (mutually exclusive with LabelSelector)
	Name string `yaml:"name,omitempty"`
	// LabelSelector matches pods by labels, uses first matching pod
	// Example: {"app": "curl-client"} or {"role": "tester"}
	LabelSelector map[string]string `yaml:"labelSelector,omitempty"`
}

// ConnectivityCheck represents a single HTTP connectivity test
// Executes an HTTP request from source pod and validates the response code
type ConnectivityCheck struct {
	// URL is the HTTP endpoint to test from the source pod
	// Format: http://service-name:port/path or http://pod-ip:port/path
	// Examples: "http://nginx-service:80/health", "http://api:8080/ready"
	// For cross-namespace: "http://service-name.namespace.svc.cluster.local:port"
	URL string `yaml:"url"`
	// ExpectedStatusCode is the HTTP status code that indicates success
	// Common values: 200 (OK), 201 (Created), 204 (No Content), 301/302 (Redirects)
	// Use 000 to check for connection refused (NetworkPolicy blocking)
	ExpectedStatusCode int `yaml:"expectedStatusCode"`
	// TimeoutSeconds is the maximum time to wait for a response (optional)
	// Default is typically 10 seconds, increase for slow services
	TimeoutSeconds int `yaml:"timeoutSeconds,omitempty"`
}

// Result represents the outcome of a single validation execution
// Returned by the executor and sent to the backend API
type Result struct {
	// Key matches the validation key for backend correlation
	Key string
	// Passed indicates whether the validation succeeded
	Passed bool
	// Message provides details about the result (success info or failure reason)
	Message string
}
