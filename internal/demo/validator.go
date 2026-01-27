package demo

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	DemoNamespace = "demo"
	DemoPodName   = "nginx"
)

// DemoObjective represents a hardcoded demo validation objective
type DemoObjective struct {
	Key         string
	Title       string
	Description string
	Category    string
}

// DemoObjectives are the hardcoded objectives for the demo challenge
// Must match the backend's DEMO_OBJECTIVES in /api/demo/submit/route.ts
var DemoObjectives = []DemoObjective{
	{
		Key:         "nginx-running",
		Title:       "Pod nginx is Running",
		Description: "The nginx pod must be running in the demo namespace",
		Category:    "status",
	},
}

// ObjectiveResult matches the API expected format
type ObjectiveResult struct {
	ObjectiveKey string `json:"objectiveKey"`
	Passed       bool   `json:"passed"`
	Message      string `json:"message,omitempty"`
}

// ValidateDemo runs the hardcoded demo validations
func ValidateDemo(ctx context.Context, clientset kubernetes.Interface) []ObjectiveResult {
	results := make([]ObjectiveResult, 0, len(DemoObjectives))

	for _, obj := range DemoObjectives {
		if obj.Key == "nginx-running" {
			result := validateNginxPod(ctx, clientset)
			results = append(results, result)
		}
	}

	return results
}

// validateNginxPod checks if the nginx pod is running in the demo namespace
func validateNginxPod(ctx context.Context, clientset kubernetes.Interface) ObjectiveResult {
	result := ObjectiveResult{
		ObjectiveKey: "nginx-running",
		Passed:       false,
	}

	// Get the pod
	pod, err := clientset.CoreV1().Pods(DemoNamespace).Get(ctx, DemoPodName, metav1.GetOptions{})
	if err != nil {
		result.Message = fmt.Sprintf("Pod not found: %v", err)
		return result
	}

	// Check if running
	if pod.Status.Phase == corev1.PodRunning {
		// Also verify it has at least one container ready
		for _, cond := range pod.Status.Conditions {
			if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
				result.Passed = true
				result.Message = "Pod nginx is running and ready"
				return result
			}
		}
		result.Message = fmt.Sprintf("Pod is %s but not ready yet", pod.Status.Phase)
		return result
	}

	result.Message = fmt.Sprintf("Pod status: %s", pod.Status.Phase)
	return result
}
