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
		// core/v1 additions
		{
			name:     "Namespace",
			kind:     "Namespace",
			expected: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"},
		},
		{
			name:     "Node",
			kind:     "Node",
			expected: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "nodes"},
		},
		{
			name:     "PersistentVolume",
			kind:     "PersistentVolume",
			expected: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "persistentvolumes"},
		},
		{
			name:     "Endpoints",
			kind:     "Endpoints",
			expected: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "endpoints"},
		},
		{
			name:     "ResourceQuota",
			kind:     "ResourceQuota",
			expected: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "resourcequotas"},
		},
		{
			name:     "LimitRange",
			kind:     "LimitRange",
			expected: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "limitranges"},
		},
		{
			name:     "ReplicationController",
			kind:     "ReplicationController",
			expected: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "replicationcontrollers"},
		},
		// networking.k8s.io/v1
		{
			name:     "Ingress",
			kind:     "Ingress",
			expected: schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
		},
		{
			name:     "NetworkPolicy",
			kind:     "NetworkPolicy",
			expected: schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"},
		},
		{
			name:     "IngressClass",
			kind:     "IngressClass",
			expected: schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "ingressclasses"},
		},
		// rbac.authorization.k8s.io/v1
		{
			name:     "Role",
			kind:     "Role",
			expected: schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles"},
		},
		{
			name:     "RoleBinding",
			kind:     "RoleBinding",
			expected: schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings"},
		},
		{
			name:     "ClusterRole",
			kind:     "ClusterRole",
			expected: schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"},
		},
		{
			name:     "ClusterRoleBinding",
			kind:     "ClusterRoleBinding",
			expected: schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterrolebindings"},
		},
		// autoscaling/v2
		{
			name:     "HorizontalPodAutoscaler",
			kind:     "HorizontalPodAutoscaler",
			expected: schema.GroupVersionResource{Group: "autoscaling", Version: "v2", Resource: "horizontalpodautoscalers"},
		},
		// policy/v1
		{
			name:     "PodDisruptionBudget",
			kind:     "PodDisruptionBudget",
			expected: schema.GroupVersionResource{Group: "policy", Version: "v1", Resource: "poddisruptionbudgets"},
		},
		// storage.k8s.io/v1
		{
			name:     "StorageClass",
			kind:     "StorageClass",
			expected: schema.GroupVersionResource{Group: "storage.k8s.io", Version: "v1", Resource: "storageclasses"},
		},
		{
			name:     "VolumeAttachment",
			kind:     "VolumeAttachment",
			expected: schema.GroupVersionResource{Group: "storage.k8s.io", Version: "v1", Resource: "volumeattachments"},
		},
		// scheduling.k8s.io/v1
		{
			name:     "PriorityClass",
			kind:     "PriorityClass",
			expected: schema.GroupVersionResource{Group: "scheduling.k8s.io", Version: "v1", Resource: "priorityclasses"},
		},
		// cert-manager.io/v1
		{
			name:     "Certificate",
			kind:     "Certificate",
			expected: schema.GroupVersionResource{Group: "cert-manager.io", Version: "v1", Resource: "certificates"},
		},
		{
			name:     "CertificateRequest",
			kind:     "CertificateRequest",
			expected: schema.GroupVersionResource{Group: "cert-manager.io", Version: "v1", Resource: "certificaterequests"},
		},
		{
			name:     "Issuer",
			kind:     "Issuer",
			expected: schema.GroupVersionResource{Group: "cert-manager.io", Version: "v1", Resource: "issuers"},
		},
		{
			name:     "ClusterIssuer",
			kind:     "ClusterIssuer",
			expected: schema.GroupVersionResource{Group: "cert-manager.io", Version: "v1", Resource: "clusterissuers"},
		},
		// acme.cert-manager.io/v1
		{
			name:     "Order",
			kind:     "Order",
			expected: schema.GroupVersionResource{Group: "acme.cert-manager.io", Version: "v1", Resource: "orders"},
		},
		{
			name:     "AcmeChallenge",
			kind:     "Challenge",
			expected: schema.GroupVersionResource{Group: "acme.cert-manager.io", Version: "v1", Resource: "challenges"},
		},
		// case-insensitive normalization
		{
			name:     "lowercase kind",
			kind:     "deployment",
			expected: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
		},
		{
			name:     "uppercase kind",
			kind:     "DEPLOYMENT",
			expected: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
		},
		// pre-existing gap
		{
			name:     "CronJob",
			kind:     "CronJob",
			expected: schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "cronjobs"},
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
