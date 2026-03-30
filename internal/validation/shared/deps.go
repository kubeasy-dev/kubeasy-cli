// Package shared provides common types and helpers shared across validation executor sub-packages.
package shared

import (
	"sync"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Deps holds the Kubernetes clients and runtime context needed by all executors.
type Deps struct {
	Clientset     kubernetes.Interface
	DynamicClient dynamic.Interface
	RestConfig    *rest.Config
	Namespace     string
	ProbeMu       *sync.Mutex // serializes probe-mode connectivity checks
}
