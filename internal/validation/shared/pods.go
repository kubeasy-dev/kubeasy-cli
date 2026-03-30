package shared

import (
	"context"
	"fmt"

	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/vtypes"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

// GetTargetPods returns pods matching the target specification.
func GetTargetPods(ctx context.Context, deps Deps, target vtypes.Target) ([]corev1.Pod, error) {
	if target.Kind != "Pod" && target.Kind != "" {
		return GetPodsForResource(ctx, deps, target)
	}

	if target.Name != "" {
		pod, err := deps.Clientset.CoreV1().Pods(deps.Namespace).Get(ctx, target.Name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get pod %s: %w", target.Name, err)
		}
		return []corev1.Pod{*pod}, nil
	}

	opts := metav1.ListOptions{}
	if len(target.LabelSelector) > 0 {
		opts.LabelSelector = labels.SelectorFromSet(target.LabelSelector).String()
	}

	pods, err := deps.Clientset.CoreV1().Pods(deps.Namespace).List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	return pods.Items, nil
}

// GetPodsForResource returns pods owned by a higher-level resource (Deployment, StatefulSet, etc.).
func GetPodsForResource(ctx context.Context, deps Deps, target vtypes.Target) ([]corev1.Pod, error) {
	gvr, err := GetGVRForKind(target.Kind)
	if err != nil {
		return nil, err
	}

	var labelSelector string

	switch {
	case target.Name != "":
		obj, err := deps.DynamicClient.Resource(gvr).Namespace(deps.Namespace).Get(ctx, target.Name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get %s %s: %w", target.Kind, target.Name, err)
		}
		selector, _, _ := unstructured.NestedStringMap(obj.Object, "spec", "selector", "matchLabels")
		if len(selector) > 0 {
			labelSelector = labels.SelectorFromSet(selector).String()
		}
	case len(target.LabelSelector) > 0:
		labelSelector = labels.SelectorFromSet(target.LabelSelector).String()
	default:
		return nil, fmt.Errorf("target %s: must specify name or labelSelector", target.Kind)
	}

	pods, err := deps.Clientset.CoreV1().Pods(deps.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	return pods.Items, nil
}
