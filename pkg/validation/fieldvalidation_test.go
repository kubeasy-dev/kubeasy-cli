package validation

import (
	"strings"
	"testing"
)

func TestValidateFieldPath_ValidFields(t *testing.T) {
	tests := []struct {
		name      string
		kind      string
		fieldPath string
	}{
		// Deployment fields
		{
			name:      "Deployment readyReplicas",
			kind:      "Deployment",
			fieldPath: "readyReplicas",
		},
		{
			name:      "Deployment availableReplicas",
			kind:      "Deployment",
			fieldPath: "availableReplicas",
		},
		{
			name:      "Deployment replicas",
			kind:      "Deployment",
			fieldPath: "replicas",
		},
		{
			name:      "Deployment updatedReplicas",
			kind:      "Deployment",
			fieldPath: "updatedReplicas",
		},
		{
			name:      "Deployment conditions array",
			kind:      "Deployment",
			fieldPath: "conditions",
		},

		// Pod fields
		{
			name:      "Pod phase",
			kind:      "Pod",
			fieldPath: "phase",
		},
		{
			name:      "Pod containerStatuses array",
			kind:      "Pod",
			fieldPath: "containerStatuses",
		},
		{
			name:      "Pod containerStatuses with index",
			kind:      "Pod",
			fieldPath: "containerStatuses[0].restartCount",
		},
		{
			name:      "Pod containerStatuses with index and ready",
			kind:      "Pod",
			fieldPath: "containerStatuses[0].ready",
		},
		{
			name:      "Pod conditions with filter",
			kind:      "Pod",
			fieldPath: "conditions[type=Ready].status",
		},
		{
			name:      "Pod conditions with filter on reason",
			kind:      "Pod",
			fieldPath: "conditions[type=PodScheduled].reason",
		},
		{
			name:      "Pod hostIP",
			kind:      "Pod",
			fieldPath: "hostIP",
		},
		{
			name:      "Pod podIP",
			kind:      "Pod",
			fieldPath: "podIP",
		},

		// StatefulSet fields
		{
			name:      "StatefulSet readyReplicas",
			kind:      "StatefulSet",
			fieldPath: "readyReplicas",
		},
		{
			name:      "StatefulSet currentReplicas",
			kind:      "StatefulSet",
			fieldPath: "currentReplicas",
		},

		// DaemonSet fields
		{
			name:      "DaemonSet desiredNumberScheduled",
			kind:      "DaemonSet",
			fieldPath: "desiredNumberScheduled",
		},
		{
			name:      "DaemonSet numberReady",
			kind:      "DaemonSet",
			fieldPath: "numberReady",
		},

		// ReplicaSet fields
		{
			name:      "ReplicaSet readyReplicas",
			kind:      "ReplicaSet",
			fieldPath: "readyReplicas",
		},

		// Job fields
		{
			name:      "Job succeeded",
			kind:      "Job",
			fieldPath: "succeeded",
		},
		{
			name:      "Job failed",
			kind:      "Job",
			fieldPath: "failed",
		},
		{
			name:      "Job active",
			kind:      "Job",
			fieldPath: "active",
		},

		// CronJob fields
		{
			name:      "CronJob lastScheduleTime",
			kind:      "CronJob",
			fieldPath: "lastScheduleTime",
		},

		// Service fields
		{
			name:      "Service loadBalancer",
			kind:      "Service",
			fieldPath: "loadBalancer",
		},

		// Node fields
		{
			name:      "Node conditions",
			kind:      "Node",
			fieldPath: "conditions",
		},
		{
			name:      "Node conditions with filter",
			kind:      "Node",
			fieldPath: "conditions[type=Ready].status",
		},

		// PersistentVolumeClaim fields
		{
			name:      "PVC phase",
			kind:      "PersistentVolumeClaim",
			fieldPath: "phase",
		},

		// Namespace fields
		{
			name:      "Namespace phase",
			kind:      "Namespace",
			fieldPath: "phase",
		},

		// Ingress fields
		{
			name:      "Ingress loadBalancer",
			kind:      "Ingress",
			fieldPath: "loadBalancer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFieldPath(tt.kind, tt.fieldPath)
			if err != nil {
				t.Errorf("ValidateFieldPath(%q, %q) returned unexpected error: %v", tt.kind, tt.fieldPath, err)
			}
		})
	}
}

func TestValidateFieldPath_InvalidField(t *testing.T) {
	tests := []struct {
		name           string
		kind           string
		fieldPath      string
		wantErrContain string
	}{
		{
			name:           "Unknown field on Deployment",
			kind:           "Deployment",
			fieldPath:      "unknownField",
			wantErrContain: "not found",
		},
		{
			name:           "Unknown field on Pod",
			kind:           "Pod",
			fieldPath:      "nonExistentField",
			wantErrContain: "not found",
		},
		{
			name:           "Typo in field name",
			kind:           "Deployment",
			fieldPath:      "redyReplicas",
			wantErrContain: "not found",
		},
		{
			name:           "Wrong nested field",
			kind:           "Pod",
			fieldPath:      "containerStatuses[0].unknownField",
			wantErrContain: "not found",
		},
		{
			name:           "Invalid filter field",
			kind:           "Pod",
			fieldPath:      "conditions[unknownField=Ready].status",
			wantErrContain: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFieldPath(tt.kind, tt.fieldPath)
			if err == nil {
				t.Errorf("ValidateFieldPath(%q, %q) expected error, got nil", tt.kind, tt.fieldPath)
				return
			}
			if !strings.Contains(err.Error(), tt.wantErrContain) {
				t.Errorf("ValidateFieldPath(%q, %q) error = %v, want error containing %q",
					tt.kind, tt.fieldPath, err, tt.wantErrContain)
			}
		})
	}
}

func TestValidateFieldPath_ErrorShowsAvailableFields(t *testing.T) {
	err := ValidateFieldPath("Deployment", "unknownField")
	if err == nil {
		t.Fatal("expected error for unknown field")
	}

	errMsg := err.Error()

	// Error should mention "available fields"
	if !strings.Contains(errMsg, "available fields") {
		t.Errorf("error message should contain 'available fields', got: %s", errMsg)
	}

	// Error should list some known deployment status fields
	knownFields := []string{"readyReplicas", "availableReplicas", "replicas"}
	for _, field := range knownFields {
		if !strings.Contains(errMsg, field) {
			t.Errorf("error message should contain field %q, got: %s", field, errMsg)
		}
	}
}

func TestValidateFieldPath_ArraySyntax(t *testing.T) {
	tests := []struct {
		name      string
		kind      string
		fieldPath string
		wantErr   bool
	}{
		{
			name:      "Valid array index",
			kind:      "Pod",
			fieldPath: "containerStatuses[0]",
			wantErr:   false,
		},
		{
			name:      "Valid array index with nested field",
			kind:      "Pod",
			fieldPath: "containerStatuses[0].restartCount",
			wantErr:   false,
		},
		{
			name:      "Valid array filter",
			kind:      "Pod",
			fieldPath: "conditions[type=Ready]",
			wantErr:   false,
		},
		{
			name:      "Valid array filter with nested field",
			kind:      "Pod",
			fieldPath: "conditions[type=Ready].status",
			wantErr:   false,
		},
		{
			name:      "Array index on non-array field",
			kind:      "Deployment",
			fieldPath: "readyReplicas[0]",
			wantErr:   true,
		},
		{
			name:      "Array filter on non-array field",
			kind:      "Pod",
			fieldPath: "phase[type=Ready]",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFieldPath(tt.kind, tt.fieldPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFieldPath(%q, %q) error = %v, wantErr %v",
					tt.kind, tt.fieldPath, err, tt.wantErr)
			}
		})
	}
}

func TestValidateFieldPath_UnsupportedKind(t *testing.T) {
	tests := []struct {
		name           string
		kind           string
		fieldPath      string
		wantErrContain string
	}{
		{
			name:           "Unknown kind",
			kind:           "UnknownKind",
			fieldPath:      "someField",
			wantErrContain: "unsupported kind",
		},
		{
			name:           "Empty kind",
			kind:           "",
			fieldPath:      "someField",
			wantErrContain: "unsupported kind",
		},
		{
			name:           "ConfigMap (no status)",
			kind:           "ConfigMap",
			fieldPath:      "someField",
			wantErrContain: "unsupported kind",
		},
		{
			name:           "Secret (no status)",
			kind:           "Secret",
			fieldPath:      "someField",
			wantErrContain: "unsupported kind",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFieldPath(tt.kind, tt.fieldPath)
			if err == nil {
				t.Errorf("ValidateFieldPath(%q, %q) expected error, got nil", tt.kind, tt.fieldPath)
				return
			}
			if !strings.Contains(err.Error(), tt.wantErrContain) {
				t.Errorf("ValidateFieldPath(%q, %q) error = %v, want error containing %q",
					tt.kind, tt.fieldPath, err, tt.wantErrContain)
			}
		})
	}
}

func TestValidateFieldPath_InvalidPathSyntax(t *testing.T) {
	tests := []struct {
		name           string
		kind           string
		fieldPath      string
		wantErrContain string
	}{
		{
			name:           "Empty path",
			kind:           "Deployment",
			fieldPath:      "",
			wantErrContain: "empty",
		},
		{
			name:           "Mismatched brackets",
			kind:           "Pod",
			fieldPath:      "containerStatuses[0",
			wantErrContain: "bracket",
		},
		{
			name:           "Empty brackets",
			kind:           "Pod",
			fieldPath:      "containerStatuses[]",
			wantErrContain: "empty",
		},
		{
			name:           "Negative array index",
			kind:           "Pod",
			fieldPath:      "containerStatuses[-1]",
			wantErrContain: "non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFieldPath(tt.kind, tt.fieldPath)
			if err == nil {
				t.Errorf("ValidateFieldPath(%q, %q) expected error, got nil", tt.kind, tt.fieldPath)
				return
			}
			if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.wantErrContain)) {
				t.Errorf("ValidateFieldPath(%q, %q) error = %v, want error containing %q",
					tt.kind, tt.fieldPath, err, tt.wantErrContain)
			}
		})
	}
}

func TestGetSupportedKinds(t *testing.T) {
	kinds := GetSupportedKinds()

	// Should have all expected kinds
	expectedKinds := []string{
		"Pod", "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet",
		"Job", "CronJob", "Service", "Node", "Namespace",
		"PersistentVolume", "PersistentVolumeClaim", "Ingress",
		"ReplicationController",
	}

	for _, expected := range expectedKinds {
		found := false
		for _, kind := range kinds {
			if kind == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("GetSupportedKinds() missing expected kind %q", expected)
		}
	}

	// Should be sorted
	for i := 1; i < len(kinds); i++ {
		if kinds[i] < kinds[i-1] {
			t.Errorf("GetSupportedKinds() not sorted: %v comes before %v", kinds[i-1], kinds[i])
		}
	}
}

func TestIsKindSupported(t *testing.T) {
	tests := []struct {
		kind string
		want bool
	}{
		{"Pod", true},
		{"Deployment", true},
		{"StatefulSet", true},
		{"DaemonSet", true},
		{"Job", true},
		{"CronJob", true},
		{"Service", true},
		{"ConfigMap", false},
		{"Secret", false},
		{"UnknownKind", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			if got := IsKindSupported(tt.kind); got != tt.want {
				t.Errorf("IsKindSupported(%q) = %v, want %v", tt.kind, got, tt.want)
			}
		})
	}
}

func TestListAvailableFields(t *testing.T) {
	// Test with a known type
	fields := listAvailableFields(kindToStatusType["Deployment"])

	// Should contain known deployment status fields
	expectedFields := []string{"readyReplicas", "availableReplicas", "replicas", "conditions"}
	for _, expected := range expectedFields {
		if !strings.Contains(fields, expected) {
			t.Errorf("listAvailableFields(DeploymentStatus) missing field %q, got: %s", expected, fields)
		}
	}
}

func TestLowercaseFirst(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"ReadyReplicas", "readyReplicas"},
		{"Status", "status"},
		{"IP", "iP"},
		{"", ""},
		{"already", "already"},
		{"A", "a"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := lowercaseFirst(tt.input); got != tt.want {
				t.Errorf("lowercaseFirst(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestCapitalizeFirst(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"readyReplicas", "ReadyReplicas"},
		{"status", "Status"},
		{"ip", "Ip"},
		{"", ""},
		{"Already", "Already"},
		{"a", "A"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := capitalizeFirst(tt.input); got != tt.want {
				t.Errorf("capitalizeFirst(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateFieldPath_CaseInsensitive(t *testing.T) {
	// Field paths should work with the correct case from JSON tags
	tests := []struct {
		name      string
		kind      string
		fieldPath string
		wantErr   bool
	}{
		{
			name:      "lowercase json tag",
			kind:      "Deployment",
			fieldPath: "readyReplicas",
			wantErr:   false,
		},
		{
			name:      "uppercase first letter (Go style)",
			kind:      "Deployment",
			fieldPath: "ReadyReplicas",
			wantErr:   false,
		},
		{
			name:      "all uppercase",
			kind:      "Deployment",
			fieldPath: "READYREPLICAS",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFieldPath(tt.kind, tt.fieldPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFieldPath(%q, %q) error = %v, wantErr %v",
					tt.kind, tt.fieldPath, err, tt.wantErr)
			}
		})
	}
}

func TestValidateFieldPath_NestedStructs(t *testing.T) {
	tests := []struct {
		name      string
		kind      string
		fieldPath string
		wantErr   bool
	}{
		{
			name:      "Pod initContainerStatuses nested",
			kind:      "Pod",
			fieldPath: "initContainerStatuses[0].state",
			wantErr:   false,
		},
		{
			name:      "Service loadBalancer ingress",
			kind:      "Service",
			fieldPath: "loadBalancer.ingress",
			wantErr:   false,
		},
		{
			name:      "Ingress loadBalancer ingress with index",
			kind:      "Ingress",
			fieldPath: "loadBalancer.ingress[0].ip",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFieldPath(tt.kind, tt.fieldPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFieldPath(%q, %q) error = %v, wantErr %v",
					tt.kind, tt.fieldPath, err, tt.wantErr)
			}
		})
	}
}
