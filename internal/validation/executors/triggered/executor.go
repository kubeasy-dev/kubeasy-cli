// Package triggered implements the "triggered" validation type.
// It runs a trigger action (load, delete, rollout, scale, wait), waits, then runs validators.
package triggered

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/kubeasy-dev/kubeasy-cli/internal/logger"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/shared"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/vtypes"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

const (
	defaultLoadRPS             = 10
	defaultLoadDurationSeconds = 10
)

// ExecuteFunc is a callback that executes a single validation and returns its result.
// The triggered executor uses this to run the "then" validators without importing
// the parent validation package (which would create a circular dependency).
type ExecuteFunc func(ctx context.Context, v vtypes.Validation) vtypes.Result

// Execute runs a trigger action, waits, then runs then validators.
func Execute(ctx context.Context, spec vtypes.TriggeredSpec, deps shared.Deps, execFn ExecuteFunc) (bool, string, error) {
	logger.Debug("Executing triggered validation: trigger type=%s", spec.Trigger.Type)

	if err := executeTrigger(ctx, spec.Trigger, deps); err != nil {
		return false, fmt.Sprintf("Trigger failed: %v", err), nil
	}

	if spec.WaitAfterSeconds > 0 {
		logger.Debug("Waiting %d seconds after trigger", spec.WaitAfterSeconds)
		select {
		case <-time.After(time.Duration(spec.WaitAfterSeconds) * time.Second):
		case <-ctx.Done():
			return false, "Context cancelled during post-trigger wait", ctx.Err()
		}
	}

	var failures []string
	for _, v := range spec.Then {
		result := execFn(ctx, v)
		if !result.Passed {
			failures = append(failures, fmt.Sprintf("[%s] %s", v.Key, result.Message))
		}
	}

	if len(failures) > 0 {
		return false, fmt.Sprintf("Then validators failed: %s", strings.Join(failures, "; ")), nil
	}

	return true, fmt.Sprintf("Trigger executed and all %d then validator(s) passed", len(spec.Then)), nil
}

func executeTrigger(ctx context.Context, trigger vtypes.TriggerConfig, deps shared.Deps) error {
	switch trigger.Type {
	case vtypes.TriggerTypeLoad:
		return executeTriggerLoad(ctx, trigger, deps)
	case vtypes.TriggerTypeWait:
		return executeTriggerWait(ctx, trigger)
	case vtypes.TriggerTypeDelete:
		return executeTriggerDelete(ctx, trigger, deps)
	case vtypes.TriggerTypeRollout:
		return executeTriggerRollout(ctx, trigger, deps)
	case vtypes.TriggerTypeScale:
		return executeTriggerScale(ctx, trigger, deps)
	default:
		return fmt.Errorf("unknown trigger type: %s", trigger.Type)
	}
}

func executeTriggerLoad(ctx context.Context, trigger vtypes.TriggerConfig, deps shared.Deps) error {
	rps := trigger.RequestsPerSecond
	if rps <= 0 {
		rps = defaultLoadRPS
	}
	durationSec := trigger.DurationSeconds
	if durationSec <= 0 {
		durationSec = defaultLoadDurationSeconds
	}
	duration := time.Duration(durationSec) * time.Second

	logger.Debug("Load trigger: %d rps for %v to %s", rps, duration, trigger.URL)

	if trigger.SourcePod != nil && (trigger.SourcePod.Name != "" || len(trigger.SourcePod.LabelSelector) > 0) {
		return executeTriggerLoadFromPod(ctx, trigger, deps, rps, durationSec)
	}

	client := &http.Client{
		Timeout:   5 * time.Second,
		Transport: &http.Transport{DisableKeepAlives: true},
	}

	deadline := time.Now().Add(duration)
	interval := time.Second / time.Duration(rps)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var wg sync.WaitGroup
	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return ctx.Err()
		case t := <-ticker.C:
			if t.After(deadline) {
				wg.Wait()
				return nil
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, trigger.URL, nil)
				if err != nil {
					return
				}
				resp, err := client.Do(req)
				if err == nil {
					_ = resp.Body.Close()
				}
			}()
		}
	}
}

func executeTriggerLoadFromPod(ctx context.Context, trigger vtypes.TriggerConfig, deps shared.Deps, rps, durationSec int) error {
	ns := deps.Namespace
	if trigger.SourcePod.Namespace != "" {
		ns = trigger.SourcePod.Namespace
	}

	var pod *corev1.Pod
	if trigger.SourcePod.Name != "" {
		p, err := deps.Clientset.CoreV1().Pods(ns).Get(ctx, trigger.SourcePod.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("load trigger: failed to get source pod %s: %w", trigger.SourcePod.Name, err)
		}
		pod = p
	} else {
		ls := labels.SelectorFromSet(trigger.SourcePod.LabelSelector).String()
		list, err := deps.Clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{LabelSelector: ls})
		if err != nil {
			return fmt.Errorf("load trigger: failed to list source pods: %w", err)
		}
		for i := range list.Items {
			if list.Items[i].Status.Phase == corev1.PodRunning {
				pod = &list.Items[i]
				break
			}
		}
		if pod == nil {
			return fmt.Errorf("load trigger: no running source pod found")
		}
	}

	if deps.RestConfig == nil || deps.RestConfig.Host == "" {
		return fmt.Errorf("load trigger: exec not available in test environment")
	}

	totalRequests := rps * durationSec
	quotedURL := "'" + strings.ReplaceAll(trigger.URL, "'", `'\''`) + "'"
	cmd := []string{
		"sh", "-c",
		fmt.Sprintf("for i in $(seq 1 %d); do curl -s -o /dev/null -- %s; done", totalRequests, quotedURL),
	}

	req := deps.Clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(pod.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Command: cmd,
			Stdout:  true,
			Stderr:  true,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(deps.RestConfig, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("load trigger: failed to create executor: %w", err)
	}

	if err := exec.StreamWithContext(ctx, remotecommand.StreamOptions{Stdout: nil, Stderr: nil}); err != nil {
		return fmt.Errorf("load trigger: exec failed: %w", err)
	}
	return nil
}

func executeTriggerWait(ctx context.Context, trigger vtypes.TriggerConfig) error {
	if trigger.WaitSeconds <= 0 {
		return nil
	}
	logger.Debug("Wait trigger: sleeping %d seconds", trigger.WaitSeconds)
	select {
	case <-time.After(time.Duration(trigger.WaitSeconds) * time.Second):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func executeTriggerDelete(ctx context.Context, trigger vtypes.TriggerConfig, deps shared.Deps) error {
	if trigger.Target == nil {
		return fmt.Errorf("delete trigger: target is required")
	}
	gvr, err := shared.GetGVRForKind(trigger.Target.Kind)
	if err != nil {
		return fmt.Errorf("delete trigger: %w", err)
	}

	if trigger.Target.Name != "" {
		logger.Debug("Delete trigger: %s %s/%s", trigger.Target.Kind, deps.Namespace, trigger.Target.Name)
	} else {
		logger.Debug("Delete trigger: %s %s selector=%v", trigger.Target.Kind, deps.Namespace, trigger.Target.LabelSelector)
	}

	if trigger.Target.Name != "" {
		return deps.DynamicClient.Resource(gvr).Namespace(deps.Namespace).Delete(
			ctx, trigger.Target.Name, metav1.DeleteOptions{},
		)
	}

	ls := labels.SelectorFromSet(trigger.Target.LabelSelector).String()
	list, err := deps.DynamicClient.Resource(gvr).Namespace(deps.Namespace).List(
		ctx, metav1.ListOptions{LabelSelector: ls},
	)
	if err != nil {
		return fmt.Errorf("delete trigger: failed to list resources: %w", err)
	}
	if len(list.Items) == 0 {
		return fmt.Errorf("delete trigger: no %s resources found matching selector %s", trigger.Target.Kind, ls)
	}
	return deps.DynamicClient.Resource(gvr).Namespace(deps.Namespace).DeleteCollection(
		ctx, metav1.DeleteOptions{}, metav1.ListOptions{LabelSelector: ls},
	)
}

func executeTriggerRollout(ctx context.Context, trigger vtypes.TriggerConfig, deps shared.Deps) error {
	if trigger.Target == nil {
		return fmt.Errorf("rollout trigger: target is required")
	}
	gvr, err := shared.GetGVRForKind(trigger.Target.Kind)
	if err != nil {
		return fmt.Errorf("rollout trigger: %w", err)
	}

	containerName := trigger.Container
	if containerName == "" {
		obj, err := deps.DynamicClient.Resource(gvr).Namespace(deps.Namespace).Get(ctx, trigger.Target.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("rollout trigger: failed to get %s %s: %w", trigger.Target.Kind, trigger.Target.Name, err)
		}
		containerName, err = firstContainerName(obj)
		if err != nil {
			return fmt.Errorf("rollout trigger: %w", err)
		}
	}

	logger.Debug("Rollout trigger: %s %s container=%s image=%s", trigger.Target.Kind, trigger.Target.Name, containerName, trigger.Image)

	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []map[string]interface{}{
						{"name": containerName, "image": trigger.Image},
					},
				},
			},
		},
	}
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("rollout trigger: failed to marshal patch: %w", err)
	}

	_, err = deps.DynamicClient.Resource(gvr).Namespace(deps.Namespace).Patch(
		ctx, trigger.Target.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("rollout trigger: patch failed: %w", err)
	}
	return nil
}

func executeTriggerScale(ctx context.Context, trigger vtypes.TriggerConfig, deps shared.Deps) error {
	if trigger.Target == nil {
		return fmt.Errorf("scale trigger: target is required")
	}
	if trigger.Replicas == nil {
		return fmt.Errorf("scale trigger: replicas is required")
	}
	gvr, err := shared.GetGVRForKind(trigger.Target.Kind)
	if err != nil {
		return fmt.Errorf("scale trigger: %w", err)
	}

	logger.Debug("Scale trigger: %s %s replicas=%d", trigger.Target.Kind, trigger.Target.Name, *trigger.Replicas)

	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"replicas": *trigger.Replicas,
		},
	}
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("scale trigger: failed to marshal patch: %w", err)
	}

	_, err = deps.DynamicClient.Resource(gvr).Namespace(deps.Namespace).Patch(
		ctx, trigger.Target.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("scale trigger: patch failed: %w", err)
	}
	return nil
}

// firstContainerName extracts the name of the first container from an unstructured resource.
func firstContainerName(obj *unstructured.Unstructured) (string, error) {
	containers, found, err := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", "containers")
	if err != nil || !found || len(containers) == 0 {
		return "", fmt.Errorf("no containers found in resource")
	}
	first, ok := containers[0].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid container entry format")
	}
	name, _, err := unstructured.NestedString(first, "name")
	if err != nil {
		return "", fmt.Errorf("failed to read container name: %w", err)
	}
	if name == "" {
		return "", fmt.Errorf("first container has no name")
	}
	return name, nil
}
