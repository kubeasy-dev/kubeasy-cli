package shared

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GetGVRForKind returns the GroupVersionResource for a given kind.
func GetGVRForKind(kind string) (schema.GroupVersionResource, error) {
	switch strings.ToLower(kind) {
	// apps/v1
	case "deployment":
		return schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}, nil
	case "statefulset":
		return schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"}, nil
	case "daemonset":
		return schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "daemonsets"}, nil
	case "replicaset":
		return schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"}, nil
	// batch/v1
	case "job":
		return schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}, nil
	case "cronjob":
		return schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "cronjobs"}, nil
	// core/v1 (namespaced)
	case "pod":
		return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}, nil
	case "service":
		return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"}, nil
	case "configmap":
		return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}, nil
	case "secret":
		return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}, nil
	case "persistentvolumeclaim":
		return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "persistentvolumeclaims"}, nil
	case "serviceaccount":
		return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "serviceaccounts"}, nil
	case "endpoints":
		return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "endpoints"}, nil
	case "resourcequota":
		return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "resourcequotas"}, nil
	case "limitrange":
		return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "limitranges"}, nil
	case "replicationcontroller":
		return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "replicationcontrollers"}, nil
	// core/v1 (cluster-scoped)
	case "namespace":
		return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}, nil
	case "node":
		return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "nodes"}, nil
	case "persistentvolume":
		return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "persistentvolumes"}, nil
	// networking.k8s.io/v1
	case "ingress":
		return schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"}, nil
	case "networkpolicy":
		return schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"}, nil
	case "ingressclass":
		return schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "ingressclasses"}, nil
	// rbac.authorization.k8s.io/v1
	case "role":
		return schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles"}, nil
	case "rolebinding":
		return schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings"}, nil
	case "clusterrole":
		return schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"}, nil
	case "clusterrolebinding":
		return schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterrolebindings"}, nil
	// autoscaling/v2
	case "horizontalpodautoscaler":
		return schema.GroupVersionResource{Group: "autoscaling", Version: "v2", Resource: "horizontalpodautoscalers"}, nil
	// policy/v1
	case "poddisruptionbudget":
		return schema.GroupVersionResource{Group: "policy", Version: "v1", Resource: "poddisruptionbudgets"}, nil
	// storage.k8s.io/v1
	case "storageclass":
		return schema.GroupVersionResource{Group: "storage.k8s.io", Version: "v1", Resource: "storageclasses"}, nil
	case "volumeattachment":
		return schema.GroupVersionResource{Group: "storage.k8s.io", Version: "v1", Resource: "volumeattachments"}, nil
	// scheduling.k8s.io/v1
	case "priorityclass":
		return schema.GroupVersionResource{Group: "scheduling.k8s.io", Version: "v1", Resource: "priorityclasses"}, nil
	// cert-manager.io/v1
	case "certificate":
		return schema.GroupVersionResource{Group: "cert-manager.io", Version: "v1", Resource: "certificates"}, nil
	case "certificaterequest":
		return schema.GroupVersionResource{Group: "cert-manager.io", Version: "v1", Resource: "certificaterequests"}, nil
	case "issuer":
		return schema.GroupVersionResource{Group: "cert-manager.io", Version: "v1", Resource: "issuers"}, nil
	case "clusterissuer":
		return schema.GroupVersionResource{Group: "cert-manager.io", Version: "v1", Resource: "clusterissuers"}, nil
	// acme.cert-manager.io/v1
	case "order":
		return schema.GroupVersionResource{Group: "acme.cert-manager.io", Version: "v1", Resource: "orders"}, nil
	case "challenge":
		return schema.GroupVersionResource{Group: "acme.cert-manager.io", Version: "v1", Resource: "challenges"}, nil
	default:
		return schema.GroupVersionResource{}, fmt.Errorf("unsupported resource kind: %s", kind)
	}
}
