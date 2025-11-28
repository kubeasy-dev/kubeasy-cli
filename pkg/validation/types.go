package validation

// ValidationConfig represents the validations.yaml file structure
type ValidationConfig struct {
	Validations []Validation `yaml:"validations"`
}

// Validation represents a single validation definition
type Validation struct {
	Key         string         `yaml:"key"`
	Title       string         `yaml:"title"`
	Description string         `yaml:"description"`
	Order       int            `yaml:"order"`
	Type        ValidationType `yaml:"type"`
	Spec        interface{}    `yaml:"-"` // Parsed based on Type
	RawSpec     interface{}    `yaml:"spec"`
}

// ValidationType represents the type of validation
type ValidationType string

const (
	TypeStatus       ValidationType = "status"
	TypeLog          ValidationType = "log"
	TypeEvent        ValidationType = "event"
	TypeMetrics      ValidationType = "metrics"
	TypeConnectivity ValidationType = "connectivity"
)

// Target represents a Kubernetes resource target
type Target struct {
	Kind          string            `yaml:"kind"`
	Name          string            `yaml:"name,omitempty"`
	LabelSelector map[string]string `yaml:"labelSelector,omitempty"`
}

// StatusSpec represents the spec for status validation
type StatusSpec struct {
	Target     Target            `yaml:"target"`
	Conditions []StatusCondition `yaml:"conditions"`
}

// StatusCondition represents a condition to check
type StatusCondition struct {
	Type   string `yaml:"type"`
	Status string `yaml:"status"`
}

// LogSpec represents the spec for log validation
type LogSpec struct {
	Target          Target   `yaml:"target"`
	Container       string   `yaml:"container,omitempty"`
	ExpectedStrings []string `yaml:"expectedStrings"`
	SinceSeconds    int      `yaml:"sinceSeconds,omitempty"`
}

// EventSpec represents the spec for event validation
type EventSpec struct {
	Target           Target   `yaml:"target"`
	ForbiddenReasons []string `yaml:"forbiddenReasons"`
	SinceSeconds     int      `yaml:"sinceSeconds,omitempty"`
}

// MetricsSpec represents the spec for metrics validation
type MetricsSpec struct {
	Target Target        `yaml:"target"`
	Checks []MetricCheck `yaml:"checks"`
}

// MetricCheck represents a single metric check
type MetricCheck struct {
	Field    string `yaml:"field"`
	Operator string `yaml:"operator"` // "==", "!=", ">", "<", ">=", "<="
	Value    int64  `yaml:"value"`
}

// ConnectivitySpec represents the spec for connectivity validation
type ConnectivitySpec struct {
	SourcePod SourcePod           `yaml:"sourcePod"`
	Targets   []ConnectivityCheck `yaml:"targets"`
}

// SourcePod represents the pod to execute connectivity checks from
type SourcePod struct {
	Name          string            `yaml:"name,omitempty"`
	LabelSelector map[string]string `yaml:"labelSelector,omitempty"`
}

// ConnectivityCheck represents a single connectivity check
type ConnectivityCheck struct {
	URL                string `yaml:"url"`
	ExpectedStatusCode int    `yaml:"expectedStatusCode"`
	TimeoutSeconds     int    `yaml:"timeoutSeconds,omitempty"`
}

// Result represents the result of a validation execution
type Result struct {
	Key     string
	Passed  bool
	Message string
}
