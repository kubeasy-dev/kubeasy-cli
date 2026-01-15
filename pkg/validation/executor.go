package validation

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kubeasy-dev/kubeasy-cli/pkg/logger"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/validation/fieldpath"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

const (
	errNoMatchingPods          = "No matching pods found"
	errNoMatchingSourcePods    = "No matching source pods found"
	errNoRunningSourcePods     = "No running source pods found"
	errNoSourcePodSpecified    = "No source pod specified"
	errNoMatchingResources     = "No matching resources found"
	errNoTargetSpecified       = "No target name or labelSelector specified"
	errAllStatusChecksPassed   = "All status checks passed"
	errAllConnectivityPassed   = "All connectivity checks passed"
	errAllConditionsMet        = "All %d pod(s) meet the required conditions"
	errFoundAllExpectedStrings = "Found all expected strings in logs"
	errNoForbiddenEvents       = "No forbidden events found"
)

// Executor executes validations against a Kubernetes cluster
type Executor struct {
	clientset     kubernetes.Interface
	dynamicClient dynamic.Interface
	restConfig    *rest.Config
	namespace     string
}

// NewExecutor creates a new validation executor
func NewExecutor(clientset kubernetes.Interface, dynamicClient dynamic.Interface, restConfig *rest.Config, namespace string) *Executor {
	return &Executor{
		clientset:     clientset,
		dynamicClient: dynamicClient,
		restConfig:    restConfig,
		namespace:     namespace,
	}
}

// Execute runs a single validation and returns the result
func (e *Executor) Execute(ctx context.Context, v Validation) Result {
	result := Result{
		Key:     v.Key,
		Passed:  false,
		Message: "Unknown validation type",
	}

	var err error
	switch v.Type {
	case TypeStatus:
		spec := v.Spec.(StatusSpec)
		result.Passed, result.Message, err = e.executeStatus(ctx, spec)
	case TypeCondition:
		spec := v.Spec.(ConditionSpec)
		result.Passed, result.Message, err = e.executeCondition(ctx, spec)
	case TypeLog:
		spec := v.Spec.(LogSpec)
		result.Passed, result.Message, err = e.executeLog(ctx, spec)
	case TypeEvent:
		spec := v.Spec.(EventSpec)
		result.Passed, result.Message, err = e.executeEvent(ctx, spec)
	case TypeConnectivity:
		spec := v.Spec.(ConnectivitySpec)
		result.Passed, result.Message, err = e.executeConnectivity(ctx, spec)
	default:
		result.Message = fmt.Sprintf("Unknown validation type: %s", v.Type)
		return result
	}

	if err != nil {
		result.Passed = false
		result.Message = err.Error()
	}

	return result
}

// ExecuteAll runs all validations in parallel and returns results
// Results are returned in the same order as the input validations
func (e *Executor) ExecuteAll(ctx context.Context, validations []Validation) []Result {
	results := make([]Result, len(validations))
	var wg sync.WaitGroup

	for i, v := range validations {
		wg.Add(1)
		go func(idx int, val Validation) {
			defer wg.Done()
			results[idx] = e.Execute(ctx, val)
		}(i, v)
	}

	wg.Wait()
	return results
}

// executeStatus checks resource status fields using operators
// Field paths are relative to status (no "status." prefix needed)
func (e *Executor) executeStatus(ctx context.Context, spec StatusSpec) (bool, string, error) {
	logger.Debug("Executing status validation for %s", spec.Target.Kind)

	if len(spec.Checks) == 0 {
		return false, "No checks specified", nil
	}

	// Get the resource
	gvr, err := getGVRForKind(spec.Target.Kind)
	if err != nil {
		return false, "", err
	}

	var obj *unstructured.Unstructured

	switch {
	case spec.Target.Name != "":
		obj, err = e.dynamicClient.Resource(gvr).Namespace(e.namespace).Get(ctx, spec.Target.Name, metav1.GetOptions{})
	case len(spec.Target.LabelSelector) > 0:
		list, listErr := e.dynamicClient.Resource(gvr).Namespace(e.namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labels.SelectorFromSet(spec.Target.LabelSelector).String(),
		})
		if listErr != nil {
			return false, "", listErr
		}
		if len(list.Items) == 0 {
			return false, errNoMatchingResources, nil
		}
		obj = &list.Items[0]
		err = nil
	default:
		return false, errNoTargetSpecified, nil
	}

	if err != nil {
		return false, "", fmt.Errorf("failed to get resource: %w", err)
	}

	allPassed := true
	var messages []string

	for _, check := range spec.Checks {
		// Use fieldpath.Get() which supports array indexing and filtering
		// Field path is relative to status (fieldpath.Get auto-prefixes with "status.")
		value, found, err := fieldpath.Get(obj.Object, check.Field)
		if err != nil {
			allPassed = false
			messages = append(messages, fmt.Sprintf("Field %s: %v", check.Field, err))
			continue
		}
		if !found {
			allPassed = false
			messages = append(messages, fmt.Sprintf("Field %s not found", check.Field))
			continue
		}

		// Compare values
		passed, compErr := compareTypedValues(value, check.Operator, check.Value)
		if compErr != nil {
			allPassed = false
			messages = append(messages, fmt.Sprintf("Field %s: %v", check.Field, compErr))
			continue
		}

		if !passed {
			allPassed = false
			messages = append(messages, fmt.Sprintf("%s: got %v, expected %s %v", check.Field, value, check.Operator, check.Value))
		}
	}

	if allPassed {
		return true, errAllStatusChecksPassed, nil
	}
	return false, strings.Join(messages, "; "), nil
}

// executeCondition checks Kubernetes resource conditions (shorthand)
func (e *Executor) executeCondition(ctx context.Context, spec ConditionSpec) (bool, string, error) {
	logger.Debug("Executing condition validation for %s", spec.Target.Kind)

	if len(spec.Checks) == 0 {
		return false, "No checks specified", nil
	}

	pods, err := e.getTargetPods(ctx, spec.Target)
	if err != nil {
		return false, "", err
	}

	if len(pods) == 0 {
		return false, errNoMatchingPods, nil
	}

	allPassed := true
	var messages []string

	for _, pod := range pods {
		for _, check := range spec.Checks {
			passed := false
			conditionFound := false
			for _, podCond := range pod.Status.Conditions {
				if string(podCond.Type) == check.Type {
					conditionFound = true
					passed = podCond.Status == check.Status
					break
				}
			}
			if !conditionFound {
				logger.Debug("Pod %s: condition type %s not found (available: %v)", pod.Name, check.Type, getPodConditionTypes(&pod))
				allPassed = false
				messages = append(messages, fmt.Sprintf("Pod %s: condition %s not found", pod.Name, check.Type))
			} else if !passed {
				allPassed = false
				messages = append(messages, fmt.Sprintf("Pod %s: condition %s is not %s", pod.Name, check.Type, check.Status))
			}
		}
	}

	if allPassed {
		return true, fmt.Sprintf(errAllConditionsMet, len(pods)), nil
	}
	return false, strings.Join(messages, "; "), nil
}

// executeLog checks container logs for expected strings
func (e *Executor) executeLog(ctx context.Context, spec LogSpec) (bool, string, error) {
	logger.Debug("Executing log validation")

	pods, err := e.getTargetPods(ctx, spec.Target)
	if err != nil {
		return false, "", err
	}

	if len(pods) == 0 {
		return false, errNoMatchingPods, nil
	}

	sinceSeconds := int64(spec.SinceSeconds)
	var logErrors []string

	// Fetch logs once per pod for efficiency (instead of per expected string)
	podLogs := make(map[string]string)
	for _, pod := range pods {
		container := spec.Container
		if container == "" && len(pod.Spec.Containers) > 0 {
			container = pod.Spec.Containers[0].Name
		}

		opts := &corev1.PodLogOptions{
			Container:    container,
			SinceSeconds: &sinceSeconds,
		}

		req := e.clientset.CoreV1().Pods(e.namespace).GetLogs(pod.Name, opts)
		logs, err := req.Do(ctx).Raw()
		if err != nil {
			errMsg := fmt.Sprintf("pod %s: %v", pod.Name, err)
			logger.Debug("Failed to get logs for %s", errMsg)
			logErrors = append(logErrors, errMsg)
			continue
		}
		podLogs[pod.Name] = string(logs)
	}

	// Check all expected strings against the fetched logs
	var missingStrings []string
	for _, expected := range spec.ExpectedStrings {
		found := false
		for _, logs := range podLogs {
			if strings.Contains(logs, expected) {
				found = true
				break
			}
		}
		if !found {
			missingStrings = append(missingStrings, expected)
		}
	}

	if len(missingStrings) == 0 {
		return true, errFoundAllExpectedStrings, nil
	}

	// Include log errors in the failure message if present
	if len(logErrors) > 0 {
		return false, fmt.Sprintf("Missing strings in logs: %v (errors fetching logs: %s)", missingStrings, strings.Join(logErrors, "; ")), nil
	}
	return false, fmt.Sprintf("Missing strings in logs: %v", missingStrings), nil
}

// executeEvent checks for forbidden events
func (e *Executor) executeEvent(ctx context.Context, spec EventSpec) (bool, string, error) {
	logger.Debug("Executing event validation")

	pods, err := e.getTargetPods(ctx, spec.Target)
	if err != nil {
		return false, "", err
	}

	if len(pods) == 0 {
		return false, errNoMatchingPods, nil
	}

	// Get events for the namespace
	events, err := e.clientset.CoreV1().Events(e.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return false, "", fmt.Errorf("failed to list events: %w", err)
	}

	// Filter events by time
	sinceTime := time.Now().Add(-time.Duration(spec.SinceSeconds) * time.Second)

	var forbiddenFound []string
	podNames := make(map[string]bool)
	for _, pod := range pods {
		podNames[pod.Name] = true
	}

	for _, event := range events.Items {
		// Check if event is for one of our pods
		if event.InvolvedObject.Kind != "Pod" || !podNames[event.InvolvedObject.Name] {
			continue
		}

		// Check if event is recent enough
		if event.LastTimestamp.Time.Before(sinceTime) && event.EventTime.Time.Before(sinceTime) {
			continue
		}

		// Check if event reason is forbidden
		for _, forbidden := range spec.ForbiddenReasons {
			if event.Reason == forbidden {
				forbiddenFound = append(forbiddenFound, fmt.Sprintf("%s on %s", event.Reason, event.InvolvedObject.Name))
			}
		}
	}

	if len(forbiddenFound) == 0 {
		return true, errNoForbiddenEvents, nil
	}
	return false, fmt.Sprintf("Forbidden events detected: %v", forbiddenFound), nil
}

// executeConnectivity tests network connectivity
func (e *Executor) executeConnectivity(ctx context.Context, spec ConnectivitySpec) (bool, string, error) {
	logger.Debug("Executing connectivity validation")

	// Find source pod
	var sourcePod *corev1.Pod
	switch {
	case spec.SourcePod.Name != "":
		pod, err := e.clientset.CoreV1().Pods(e.namespace).Get(ctx, spec.SourcePod.Name, metav1.GetOptions{})
		if err != nil {
			return false, "", fmt.Errorf("failed to get source pod: %w", err)
		}
		sourcePod = pod
	case len(spec.SourcePod.LabelSelector) > 0:
		pods, err := e.clientset.CoreV1().Pods(e.namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labels.SelectorFromSet(spec.SourcePod.LabelSelector).String(),
		})
		if err != nil {
			return false, "", fmt.Errorf("failed to list source pods: %w", err)
		}
		if len(pods.Items) == 0 {
			return false, errNoMatchingSourcePods, nil
		}
		// Find a running pod
		for i := range pods.Items {
			if pods.Items[i].Status.Phase == corev1.PodRunning {
				sourcePod = &pods.Items[i]
				break
			}
		}
		if sourcePod == nil {
			return false, errNoRunningSourcePods, nil
		}
	default:
		return false, errNoSourcePodSpecified, nil
	}

	allPassed := true
	var messages []string

	for _, target := range spec.Targets {
		passed, msg := e.checkConnectivity(ctx, sourcePod, target)
		if !passed {
			allPassed = false
			messages = append(messages, msg)
		}
	}

	if allPassed {
		return true, errAllConnectivityPassed, nil
	}
	return false, strings.Join(messages, "; "), nil
}

// escapeShellArg escapes a string for safe use in shell single-quoted context
// This prevents command injection by properly handling embedded single quotes
func escapeShellArg(arg string) string {
	// In single quotes, the only special char is single quote itself
	// We close the single quote, add an escaped single quote, then reopen
	return strings.ReplaceAll(arg, "'", "'\"'\"'")
}

// checkConnectivity performs a curl request from a pod
func (e *Executor) checkConnectivity(ctx context.Context, pod *corev1.Pod, target ConnectivityCheck) (bool, string) {
	timeout := target.TimeoutSeconds
	if timeout == 0 {
		timeout = 5
	}

	// Escape URL to prevent command injection
	escapedURL := escapeShellArg(target.URL)

	// Build curl command
	cmd := []string{
		"sh", "-c",
		fmt.Sprintf("curl -s -o /dev/null -w '%%{http_code}' --connect-timeout %d '%s'", timeout, escapedURL),
	}

	req := e.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(e.namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Command: cmd,
			Stdout:  true,
			Stderr:  true,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(e.restConfig, "POST", req.URL())
	if err != nil {
		return false, fmt.Sprintf("Failed to create executor: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})

	if err != nil {
		// Try with wget as fallback
		logger.Debug("curl failed for %s: %v, trying wget", target.URL, err)
		cmd = []string{
			"sh", "-c",
			fmt.Sprintf("wget -q -O /dev/null -T %d '%s' && echo 200 || echo 000", timeout, escapedURL),
		}
		req = e.clientset.CoreV1().RESTClient().Post().
			Resource("pods").
			Name(pod.Name).
			Namespace(e.namespace).
			SubResource("exec").
			VersionedParams(&corev1.PodExecOptions{
				Command: cmd,
				Stdout:  true,
				Stderr:  true,
			}, scheme.ParameterCodec)

		exec, err = remotecommand.NewSPDYExecutor(e.restConfig, "POST", req.URL())
		if err != nil {
			return false, fmt.Sprintf("Connection to %s failed: %v", target.URL, err)
		}

		stdout.Reset()
		stderr.Reset()
		err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
			Stdout: &stdout,
			Stderr: &stderr,
		})
		if err != nil {
			return false, fmt.Sprintf("Connection to %s failed: %v", target.URL, err)
		}
	}

	statusCode := strings.TrimSpace(stdout.String())
	code, err := strconv.Atoi(statusCode)
	if err != nil {
		return false, fmt.Sprintf("Invalid response from %s: %s", target.URL, statusCode)
	}

	if code == target.ExpectedStatusCode {
		return true, ""
	}
	return false, fmt.Sprintf("Connection to %s: got status %d, expected %d", target.URL, code, target.ExpectedStatusCode)
}

// getTargetPods returns pods matching the target specification
func (e *Executor) getTargetPods(ctx context.Context, target Target) ([]corev1.Pod, error) {
	if target.Kind != "Pod" && target.Kind != "" {
		// For non-Pod resources, get pods owned by them
		return e.getPodsForResource(ctx, target)
	}

	// If a specific pod name is provided, get that pod
	if target.Name != "" {
		pod, err := e.clientset.CoreV1().Pods(e.namespace).Get(ctx, target.Name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get pod %s: %w", target.Name, err)
		}
		return []corev1.Pod{*pod}, nil
	}

	// Otherwise list pods by label selector
	opts := metav1.ListOptions{}
	if len(target.LabelSelector) > 0 {
		opts.LabelSelector = labels.SelectorFromSet(target.LabelSelector).String()
	}

	pods, err := e.clientset.CoreV1().Pods(e.namespace).List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	return pods.Items, nil
}

// getPodsForResource returns pods owned by a higher-level resource
func (e *Executor) getPodsForResource(ctx context.Context, target Target) ([]corev1.Pod, error) {
	gvr, err := getGVRForKind(target.Kind)
	if err != nil {
		return nil, err
	}

	var labelSelector string

	if target.Name != "" {
		// Get the resource and extract its selector
		obj, err := e.dynamicClient.Resource(gvr).Namespace(e.namespace).Get(ctx, target.Name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get %s %s: %w", target.Kind, target.Name, err)
		}
		selector, _, _ := unstructured.NestedStringMap(obj.Object, "spec", "selector", "matchLabels")
		if len(selector) > 0 {
			labelSelector = labels.SelectorFromSet(selector).String()
		}
	} else if len(target.LabelSelector) > 0 {
		labelSelector = labels.SelectorFromSet(target.LabelSelector).String()
	}

	pods, err := e.clientset.CoreV1().Pods(e.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	return pods.Items, nil
}

// getGVRForKind returns the GroupVersionResource for a given kind
func getGVRForKind(kind string) (schema.GroupVersionResource, error) {
	switch strings.ToLower(kind) {
	case "deployment":
		return schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}, nil
	case "statefulset":
		return schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"}, nil
	case "daemonset":
		return schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "daemonsets"}, nil
	case "replicaset":
		return schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"}, nil
	case "job":
		return schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}, nil
	case "pod":
		return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}, nil
	case "service":
		return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"}, nil
	default:
		return schema.GroupVersionResource{}, fmt.Errorf("unsupported resource kind: %s", kind)
	}
}

// getNestedInt64 extracts an int64 value from a nested map
func getNestedInt64(obj map[string]interface{}, fields ...string) (int64, bool, error) {
	val, found, err := unstructured.NestedFieldNoCopy(obj, fields...)
	if !found || err != nil {
		return 0, found, err
	}

	switch v := val.(type) {
	case int64:
		return v, true, nil
	case int32:
		return int64(v), true, nil
	case int:
		return int64(v), true, nil
	case float64:
		return int64(v), true, nil
	default:
		return 0, true, fmt.Errorf("unexpected type %T", val)
	}
}

// compareValues compares two values using the specified operator
func compareValues(actual int64, operator string, expected int64) bool {
	switch operator {
	case "==", "=":
		return actual == expected
	case "!=":
		return actual != expected
	case ">":
		return actual > expected
	case "<":
		return actual < expected
	case ">=":
		return actual >= expected
	case "<=":
		return actual <= expected
	default:
		return actual == expected
	}
}

// getPodConditionTypes returns a list of condition types present on a pod (for debugging)
func getPodConditionTypes(pod *corev1.Pod) []string {
	types := make([]string, len(pod.Status.Conditions))
	for i, cond := range pod.Status.Conditions {
		types[i] = string(cond.Type)
	}
	return types
}

// compareTypedValues compares two values using the specified operator
// Supports string, int64, float64, and bool types
func compareTypedValues(actual interface{}, operator string, expected interface{}) (bool, error) {
	// Handle nil values
	if actual == nil {
		return false, fmt.Errorf("actual value is nil")
	}

	// Type-specific comparison
	switch actualVal := actual.(type) {
	case string:
		expectedStr, ok := expected.(string)
		if !ok {
			return false, fmt.Errorf("type mismatch: actual is string, expected is %T", expected)
		}
		return compareStrings(actualVal, operator, expectedStr)

	case bool:
		expectedBool, ok := expected.(bool)
		if !ok {
			return false, fmt.Errorf("type mismatch: actual is bool, expected is %T", expected)
		}
		return compareBools(actualVal, operator, expectedBool)

	case int64:
		return compareNumeric(float64(actualVal), operator, expected)
	case int32:
		return compareNumeric(float64(actualVal), operator, expected)
	case int:
		return compareNumeric(float64(actualVal), operator, expected)
	case float64:
		return compareNumeric(actualVal, operator, expected)

	default:
		return false, fmt.Errorf("unsupported type: %T", actual)
	}
}

// compareStrings compares two strings using the specified operator
func compareStrings(actual, operator, expected string) (bool, error) {
	switch operator {
	case "==", "=":
		return actual == expected, nil
	case "!=":
		return actual != expected, nil
	default:
		return false, fmt.Errorf("operator %s not supported for strings (use == or !=)", operator)
	}
}

// compareBools compares two booleans using the specified operator
func compareBools(actual bool, operator string, expected bool) (bool, error) {
	switch operator {
	case "==", "=":
		return actual == expected, nil
	case "!=":
		return actual != expected, nil
	default:
		return false, fmt.Errorf("operator %s not supported for booleans (use == or !=)", operator)
	}
}

// compareNumeric compares numeric values using the specified operator.
// Handles int/float coercion by converting all numeric types to float64.
//
// Note: Converting large int64 values to float64 may lose precision for values
// greater than 2^53 (9,007,199,254,740,992). For typical Kubernetes use cases
// like replica counts, restart counts, and resource metrics, this is not an issue.
func compareNumeric(actual float64, operator string, expected interface{}) (bool, error) {
	var expectedFloat float64

	switch v := expected.(type) {
	case int:
		expectedFloat = float64(v)
	case int32:
		expectedFloat = float64(v)
	case int64:
		expectedFloat = float64(v)
	case float64:
		expectedFloat = v
	default:
		return false, fmt.Errorf("expected value must be numeric, got %T", expected)
	}

	switch operator {
	case "==", "=":
		return actual == expectedFloat, nil
	case "!=":
		return actual != expectedFloat, nil
	case ">":
		return actual > expectedFloat, nil
	case "<":
		return actual < expectedFloat, nil
	case ">=":
		return actual >= expectedFloat, nil
	case "<=":
		return actual <= expectedFloat, nil
	default:
		return false, fmt.Errorf("unknown operator: %s", operator)
	}
}
