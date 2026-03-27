package validation

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/kubeasy-dev/kubeasy-cli/internal/logger"
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

// executeTriggered runs a trigger action, waits, then runs then validators.
// It is a pure orchestrator — all validation logic is delegated to existing executors.
func (e *Executor) executeTriggered(ctx context.Context, spec TriggeredSpec) (bool, string, error) {
	logger.Debug("Executing triggered validation: trigger type=%s", spec.Trigger.Type)

	// Step 1: execute the trigger
	if err := e.executeTrigger(ctx, spec.Trigger); err != nil {
		return false, fmt.Sprintf("Trigger failed: %v", err), nil
	}

	// Step 2: wait after trigger
	if spec.WaitAfterSeconds > 0 {
		logger.Debug("Waiting %d seconds after trigger", spec.WaitAfterSeconds)
		select {
		case <-time.After(time.Duration(spec.WaitAfterSeconds) * time.Second):
		case <-ctx.Done():
			return false, "Context cancelled during post-trigger wait", ctx.Err()
		}
	}

	// Step 3: run then validators
	var failures []string
	for _, v := range spec.Then {
		result := e.Execute(ctx, v)
		if !result.Passed {
			failures = append(failures, fmt.Sprintf("[%s] %s", v.Key, result.Message))
		}
	}

	if len(failures) > 0 {
		return false, fmt.Sprintf("Then validators failed: %s", strings.Join(failures, "; ")), nil
	}

	return true, fmt.Sprintf("Trigger executed and all %d then validator(s) passed", len(spec.Then)), nil
}

// executeTrigger dispatches to the appropriate trigger implementation
func (e *Executor) executeTrigger(ctx context.Context, trigger TriggerConfig) error {
	switch trigger.Type {
	case TriggerTypeLoad:
		return e.executeTriggerLoad(ctx, trigger)
	case TriggerTypeWait:
		return e.executeTriggerWait(ctx, trigger)
	case TriggerTypeDelete:
		return e.executeTriggerDelete(ctx, trigger)
	case TriggerTypeRollout:
		return e.executeTriggerRollout(ctx, trigger)
	case TriggerTypeScale:
		return e.executeTriggerScale(ctx, trigger)
	default:
		return fmt.Errorf("unknown trigger type: %s", trigger.Type)
	}
}

// executeTriggerLoad sends HTTP traffic to a URL for a specified duration.
// When SourcePod is set, curl is exec'd inside the pod; otherwise net/http is used from the CLI host.
func (e *Executor) executeTriggerLoad(ctx context.Context, trigger TriggerConfig) error {
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

	// Pod exec-based load generation
	if trigger.SourcePod != nil && (trigger.SourcePod.Name != "" || len(trigger.SourcePod.LabelSelector) > 0) {
		return e.executeTriggerLoadFromPod(ctx, trigger, rps, durationSec)
	}

	// CLI host-based load generation via net/http
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

// executeTriggerLoadFromPod exec's curl in a source pod to generate load.
// Runs totalRequests = rps * durationSeconds sequential curl invocations.
// Note: because curl calls are sequential, the actual rate is bounded by
// min(rps, 1/response_time). Do not rely on precise RPS for rate-sensitive
// autoscaling triggers via the pod-exec path; use the host-based path instead.
func (e *Executor) executeTriggerLoadFromPod(ctx context.Context, trigger TriggerConfig, rps, durationSec int) error {
	ns := e.namespace
	if trigger.SourcePod.Namespace != "" {
		ns = trigger.SourcePod.Namespace
	}

	// Find the source pod
	var pod *corev1.Pod
	if trigger.SourcePod.Name != "" {
		p, err := e.clientset.CoreV1().Pods(ns).Get(ctx, trigger.SourcePod.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("load trigger: failed to get source pod %s: %w", trigger.SourcePod.Name, err)
		}
		pod = p
	} else {
		ls := labels.SelectorFromSet(trigger.SourcePod.LabelSelector).String()
		list, err := e.clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{LabelSelector: ls})
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

	// Guard: exec not available without a real REST config
	if e.restConfig == nil || e.restConfig.Host == "" {
		return fmt.Errorf("load trigger: exec not available in test environment")
	}

	totalRequests := rps * durationSec
	// Run a shell loop inside the pod. The URL is single-quoted and internal single
	// quotes are escaped to prevent shell injection (POSIX quoting: ' → '\'' ).
	quotedURL := "'" + strings.ReplaceAll(trigger.URL, "'", `'\''`) + "'"
	cmd := []string{
		"sh", "-c",
		fmt.Sprintf("for i in $(seq 1 %d); do curl -s -o /dev/null -- %s; done", totalRequests, quotedURL),
	}

	req := e.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(pod.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Command: cmd,
			Stdout:  true,
			Stderr:  true,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(e.restConfig, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("load trigger: failed to create executor: %w", err)
	}

	if err := exec.StreamWithContext(ctx, remotecommand.StreamOptions{Stdout: nil, Stderr: nil}); err != nil {
		return fmt.Errorf("load trigger: exec failed: %w", err)
	}
	return nil
}

// executeTriggerWait sleeps for waitSeconds.
func (e *Executor) executeTriggerWait(ctx context.Context, trigger TriggerConfig) error {
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

// executeTriggerDelete deletes a Kubernetes resource identified by target.
func (e *Executor) executeTriggerDelete(ctx context.Context, trigger TriggerConfig) error {
	if trigger.Target == nil {
		return fmt.Errorf("delete trigger: target is required")
	}
	gvr, err := getGVRForKind(trigger.Target.Kind)
	if err != nil {
		return fmt.Errorf("delete trigger: %w", err)
	}

	logger.Debug("Delete trigger: %s %s/%s", trigger.Target.Kind, e.namespace, trigger.Target.Name)

	if trigger.Target.Name != "" {
		return e.dynamicClient.Resource(gvr).Namespace(e.namespace).Delete(
			ctx, trigger.Target.Name, metav1.DeleteOptions{},
		)
	}

	// Delete by label selector — verify at least one resource exists to catch label typos
	ls := labels.SelectorFromSet(trigger.Target.LabelSelector).String()
	list, err := e.dynamicClient.Resource(gvr).Namespace(e.namespace).List(
		ctx, metav1.ListOptions{LabelSelector: ls},
	)
	if err != nil {
		return fmt.Errorf("delete trigger: failed to list resources: %w", err)
	}
	if len(list.Items) == 0 {
		return fmt.Errorf("delete trigger: no %s resources found matching selector %s", trigger.Target.Kind, ls)
	}
	return e.dynamicClient.Resource(gvr).Namespace(e.namespace).DeleteCollection(
		ctx, metav1.DeleteOptions{}, metav1.ListOptions{LabelSelector: ls},
	)
}

// executeTriggerRollout patches a Deployment container image to trigger a rolling update.
func (e *Executor) executeTriggerRollout(ctx context.Context, trigger TriggerConfig) error {
	if trigger.Target == nil {
		return fmt.Errorf("rollout trigger: target is required")
	}
	gvr, err := getGVRForKind(trigger.Target.Kind)
	if err != nil {
		return fmt.Errorf("rollout trigger: %w", err)
	}

	containerName := trigger.Container
	if containerName == "" {
		// Resolve container name from the live resource
		obj, err := e.dynamicClient.Resource(gvr).Namespace(e.namespace).Get(ctx, trigger.Target.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("rollout trigger: failed to get %s %s: %w", trigger.Target.Kind, trigger.Target.Name, err)
		}
		containerName, err = firstContainerName(obj)
		if err != nil {
			return fmt.Errorf("rollout trigger: %w", err)
		}
	}

	logger.Debug("Rollout trigger: %s %s container=%s image=%s", trigger.Target.Kind, trigger.Target.Name, containerName, trigger.Image)

	// Strategic merge patch preserves other containers unchanged
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

	_, err = e.dynamicClient.Resource(gvr).Namespace(e.namespace).Patch(
		ctx, trigger.Target.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("rollout trigger: patch failed: %w", err)
	}
	return nil
}

// executeTriggerScale patches the replica count of a resource.
func (e *Executor) executeTriggerScale(ctx context.Context, trigger TriggerConfig) error {
	if trigger.Target == nil {
		return fmt.Errorf("scale trigger: target is required")
	}
	if trigger.Replicas == nil {
		return fmt.Errorf("scale trigger: replicas is required")
	}
	gvr, err := getGVRForKind(trigger.Target.Kind)
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

	_, err = e.dynamicClient.Resource(gvr).Namespace(e.namespace).Patch(
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
	name, _, _ := unstructured.NestedString(first, "name")
	if name == "" {
		return "", fmt.Errorf("first container has no name")
	}
	return name, nil
}
