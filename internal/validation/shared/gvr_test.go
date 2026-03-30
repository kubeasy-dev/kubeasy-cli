package shared_test

import (
	"testing"

	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestGetGVRForKind(t *testing.T) {
	tests := []struct {
		name     string
		kind     string
		expected schema.GroupVersionResource
		wantErr  bool
	}{
		{
			name:     "Deployment",
			kind:     "Deployment",
			expected: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
		},
		{
			name:     "StatefulSet",
			kind:     "StatefulSet",
			expected: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"},
		},
		{
			name:     "DaemonSet",
			kind:     "DaemonSet",
			expected: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "daemonsets"},
		},
		{
			name:     "ReplicaSet",
			kind:     "ReplicaSet",
			expected: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"},
		},
		{
			name:     "Job",
			kind:     "Job",
			expected: schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"},
		},
		{
			name:     "Pod",
			kind:     "Pod",
			expected: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
		},
		{
			name:     "Service",
			kind:     "Service",
			expected: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"},
		},
		{
			name:     "ConfigMap",
			kind:     "ConfigMap",
			expected: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
		},
		{
			name:     "Secret",
			kind:     "Secret",
			expected: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"},
		},
		{
			name:     "PersistentVolumeClaim",
			kind:     "PersistentVolumeClaim",
			expected: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "persistentvolumeclaims"},
		},
		{
			name:     "ServiceAccount",
			kind:     "ServiceAccount",
			expected: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "serviceaccounts"},
		},
		{
			name:    "Unsupported",
			kind:    "UnsupportedKind",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gvr, err := shared.GetGVRForKind(tt.kind)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "unsupported resource kind")
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, gvr)
			}
		})
	}
}
