package validation

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

func TestNewExecutor(t *testing.T) {
	clientset := fake.NewClientset()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	restConfig := &rest.Config{}
	namespace := "test-namespace"

	executor := NewExecutor(clientset, dynamicClient, restConfig, namespace)

	require.NotNil(t, executor)
	assert.Equal(t, namespace, executor.namespace)
	assert.NotNil(t, executor.clientset)
	assert.NotNil(t, executor.dynamicClient)
	assert.NotNil(t, executor.restConfig)
}

func TestExecute_UnknownType(t *testing.T) {
	executor := NewExecutor(
		fake.NewClientset(),
		dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()),
		&rest.Config{},
		"test-ns",
	)

	validation := Validation{
		Key:  "test-key",
		Type: "invalid-type",
		Spec: StatusSpec{},
	}

	result := executor.Execute(context.Background(), validation)

	assert.False(t, result.Passed)
	assert.Contains(t, result.Message, "Unknown validation type")
	assert.Equal(t, "test-key", result.Key)
}

func TestExecuteAll(t *testing.T) {
	// Create a running pod
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-ns",
			Labels:    map[string]string{"app": "test"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}

	clientset := fake.NewClientset(pod)
	executor := NewExecutor(
		clientset,
		dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()),
		&rest.Config{},
		"test-ns",
	)

	validations := []Validation{
		{
			Key:  "pod-ready",
			Type: TypeCondition,
			Spec: ConditionSpec{
				Target: Target{
					Kind: "Pod",
					Name: "test-pod",
				},
				Checks: []ConditionCheck{
					{Type: "Ready", Status: corev1.ConditionTrue},
				},
			},
		},
		{
			Key:  "unknown-type",
			Type: "invalid",
			Spec: StatusSpec{},
		},
	}

	results := executor.ExecuteAll(context.Background(), validations)

	require.Len(t, results, 2)
	assert.True(t, results[0].Passed)
	assert.Equal(t, "pod-ready", results[0].Key)
	assert.False(t, results[1].Passed)
	assert.Equal(t, "unknown-type", results[1].Key)
}

func TestExecuteCondition_Success(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-ns",
		},
		Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}

	clientset := fake.NewClientset(pod)
	executor := NewExecutor(clientset, nil, nil, "test-ns")

	spec := ConditionSpec{
		Target: Target{
			Kind: "Pod",
			Name: "test-pod",
		},
		Checks: []ConditionCheck{
			{Type: "Ready", Status: corev1.ConditionTrue},
		},
	}

	passed, msg, err := executor.executeCondition(context.Background(), spec)

	require.NoError(t, err)
	assert.True(t, passed)
	assert.Contains(t, msg, "All 1 pod(s) meet the required conditions")
}

func TestExecuteCondition_ConditionNotMet(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-ns",
		},
		Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionFalse,
				},
			},
		},
	}

	clientset := fake.NewClientset(pod)
	executor := NewExecutor(clientset, nil, nil, "test-ns")

	spec := ConditionSpec{
		Target: Target{
			Kind: "Pod",
			Name: "test-pod",
		},
		Checks: []ConditionCheck{
			{Type: "Ready", Status: corev1.ConditionTrue},
		},
	}

	passed, msg, err := executor.executeCondition(context.Background(), spec)

	require.NoError(t, err)
	assert.False(t, passed)
	assert.Contains(t, msg, "condition Ready is not True")
}

func TestExecuteCondition_NoMatchingPods(t *testing.T) {
	clientset := fake.NewClientset()
	executor := NewExecutor(clientset, nil, nil, "test-ns")

	spec := ConditionSpec{
		Target: Target{
			Kind: "Pod",
			Name: "nonexistent-pod",
		},
		Checks: []ConditionCheck{
			{Type: "Ready", Status: corev1.ConditionTrue},
		},
	}

	passed, _, err := executor.executeCondition(context.Background(), spec)

	assert.Error(t, err)
	assert.False(t, passed)
}

func TestExecuteCondition_ByLabelSelector(t *testing.T) {
	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-1",
			Namespace: "test-ns",
			Labels:    map[string]string{"app": "test"},
		},
		Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionTrue},
			},
		},
	}

	pod2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-2",
			Namespace: "test-ns",
			Labels:    map[string]string{"app": "test"},
		},
		Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionTrue},
			},
		},
	}

	clientset := fake.NewClientset(pod1, pod2)
	executor := NewExecutor(clientset, nil, nil, "test-ns")

	spec := ConditionSpec{
		Target: Target{
			Kind:          "Pod",
			LabelSelector: map[string]string{"app": "test"},
		},
		Checks: []ConditionCheck{
			{Type: "Ready", Status: corev1.ConditionTrue},
		},
	}

	passed, msg, err := executor.executeCondition(context.Background(), spec)

	require.NoError(t, err)
	assert.True(t, passed)
	assert.Contains(t, msg, "All 2 pod(s) meet the required conditions")
}

func TestExecuteEvent_NoForbiddenEvents(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-ns",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}

	clientset := fake.NewClientset(pod)
	executor := NewExecutor(clientset, nil, nil, "test-ns")

	spec := EventSpec{
		Target: Target{
			Kind: "Pod",
			Name: "test-pod",
		},
		ForbiddenReasons: []string{"OOMKilled", "Evicted"},
		SinceSeconds:     300,
	}

	passed, msg, err := executor.executeEvent(context.Background(), spec)

	require.NoError(t, err)
	assert.True(t, passed)
	assert.Equal(t, msgNoForbiddenEvents, msg)
}

func TestExecuteEvent_ForbiddenEventDetected(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-ns",
			UID:       "test-uid",
		},
	}

	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "oom-event",
			Namespace: "test-ns",
		},
		InvolvedObject: corev1.ObjectReference{
			Kind: "Pod",
			Name: "test-pod",
			UID:  "test-uid",
		},
		Reason:        "OOMKilled",
		LastTimestamp: metav1.Now(),
		EventTime:     metav1.NowMicro(),
	}

	clientset := fake.NewClientset(pod, event)
	executor := NewExecutor(clientset, nil, nil, "test-ns")

	spec := EventSpec{
		Target: Target{
			Kind: "Pod",
			Name: "test-pod",
		},
		ForbiddenReasons: []string{"OOMKilled", "Evicted"},
		SinceSeconds:     300,
	}

	passed, msg, err := executor.executeEvent(context.Background(), spec)

	require.NoError(t, err)
	assert.False(t, passed)
	assert.Contains(t, msg, "Forbidden events detected")
	assert.Contains(t, msg, "OOMKilled")
}

func TestExecuteStatus_Success(t *testing.T) {
	deployment := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      "test-deployment",
				"namespace": "test-ns",
			},
			"spec": map[string]interface{}{
				"replicas": int64(3),
			},
			"status": map[string]interface{}{
				"readyReplicas": int64(3),
			},
		},
	}

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme, deployment)

	executor := NewExecutor(nil, dynamicClient, nil, "test-ns")

	spec := StatusSpec{
		Target: Target{
			Kind: "Deployment",
			Name: "test-deployment",
		},
		Checks: []StatusCheck{
			{Field: "readyReplicas", Operator: "==", Value: int64(3)},
		},
	}

	passed, msg, err := executor.executeStatus(context.Background(), spec)

	require.NoError(t, err)
	assert.True(t, passed)
	assert.Equal(t, msgAllStatusChecksPassed, msg)
}

func TestExecuteStatus_CheckFailed(t *testing.T) {
	deployment := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      "test-deployment",
				"namespace": "test-ns",
			},
			"status": map[string]interface{}{
				"readyReplicas": int64(1),
			},
		},
	}

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme, deployment)

	executor := NewExecutor(nil, dynamicClient, nil, "test-ns")

	spec := StatusSpec{
		Target: Target{
			Kind: "Deployment",
			Name: "test-deployment",
		},
		Checks: []StatusCheck{
			{Field: "readyReplicas", Operator: ">=", Value: int64(3)},
		},
	}

	passed, msg, err := executor.executeStatus(context.Background(), spec)

	require.NoError(t, err)
	assert.False(t, passed)
	assert.Contains(t, msg, "got 1, expected >= 3")
}

func TestExecuteStatus_NoMatchingResources(t *testing.T) {
	// Create a pod with different labels
	podUnstructured := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      "test-pod",
				"namespace": "test-ns",
				"labels": map[string]interface{}{
					"app": "other",
				},
			},
			"status": map[string]interface{}{
				"containerStatuses": []interface{}{
					map[string]interface{}{
						"restartCount": int64(0),
					},
				},
			},
		},
	}

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme, podUnstructured)
	executor := NewExecutor(nil, dynamicClient, nil, "test-ns")

	spec := StatusSpec{
		Target: Target{
			Kind:          "Pod",
			LabelSelector: map[string]string{"app": "nonexistent"},
		},
		Checks: []StatusCheck{
			{Field: "containerStatuses.0.restartCount", Operator: "==", Value: int64(0)},
		},
	}

	passed, msg, err := executor.executeStatus(context.Background(), spec)

	require.NoError(t, err)
	assert.False(t, passed)
	assert.Equal(t, errNoMatchingResources, msg)
}

func TestExecuteStatus_NoTargetSpecified(t *testing.T) {
	executor := NewExecutor(nil, dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()), nil, "test-ns")

	spec := StatusSpec{
		Target: Target{
			Kind: "Deployment",
			// No name or labelSelector
		},
		Checks: []StatusCheck{
			{Field: "readyReplicas", Operator: "==", Value: int64(3)},
		},
	}

	passed, msg, err := executor.executeStatus(context.Background(), spec)

	require.NoError(t, err)
	assert.False(t, passed)
	assert.Equal(t, errNoTargetSpecified, msg)
}

func TestGetGVRForKind(t *testing.T) {
	tests := []struct {
		name     string
		kind     string
		expected schema.GroupVersionResource
		wantErr  bool
	}{
		{
			name: "Deployment",
			kind: "Deployment",
			expected: schema.GroupVersionResource{
				Group:    "apps",
				Version:  "v1",
				Resource: "deployments",
			},
		},
		{
			name: "StatefulSet",
			kind: "StatefulSet",
			expected: schema.GroupVersionResource{
				Group:    "apps",
				Version:  "v1",
				Resource: "statefulsets",
			},
		},
		{
			name: "DaemonSet",
			kind: "DaemonSet",
			expected: schema.GroupVersionResource{
				Group:    "apps",
				Version:  "v1",
				Resource: "daemonsets",
			},
		},
		{
			name: "Pod",
			kind: "Pod",
			expected: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
		},
		{
			name:    "Unsupported",
			kind:    "UnsupportedKind",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gvr, err := getGVRForKind(tt.kind)

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

func TestGetNestedInt64(t *testing.T) {
	tests := []struct {
		name     string
		obj      map[string]interface{}
		fields   []string
		expected int64
		found    bool
		wantErr  bool
	}{
		{
			name: "int64 value",
			obj: map[string]interface{}{
				"status": map[string]interface{}{
					"readyReplicas": int64(5),
				},
			},
			fields:   []string{"status", "readyReplicas"},
			expected: 5,
			found:    true,
		},
		{
			name: "int32 value",
			obj: map[string]interface{}{
				"status": map[string]interface{}{
					"readyReplicas": int32(3),
				},
			},
			fields:   []string{"status", "readyReplicas"},
			expected: 3,
			found:    true,
		},
		{
			name: "int value",
			obj: map[string]interface{}{
				"count": 10,
			},
			fields:   []string{"count"},
			expected: 10,
			found:    true,
		},
		{
			name: "float64 value",
			obj: map[string]interface{}{
				"value": float64(7.5),
			},
			fields:   []string{"value"},
			expected: 7,
			found:    true,
		},
		{
			name: "field not found",
			obj: map[string]interface{}{
				"other": "value",
			},
			fields: []string{"missing"},
			found:  false,
		},
		{
			name: "invalid type",
			obj: map[string]interface{}{
				"value": "string",
			},
			fields:  []string{"value"},
			found:   true,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, found, err := getNestedInt64(tt.obj, tt.fields...)

			if tt.wantErr {
				assert.Error(t, err)
			} else if tt.found {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, value)
			}
			assert.Equal(t, tt.found, found)
		})
	}
}

func TestCompareValues(t *testing.T) {
	tests := []struct {
		name     string
		actual   int64
		operator string
		expected int64
		want     bool
	}{
		{"equal ==", 5, "==", 5, true},
		{"equal =", 5, "=", 5, true},
		{"not equal !=", 5, "!=", 3, true},
		{"greater than >", 10, ">", 5, true},
		{"less than <", 3, "<", 5, true},
		{"greater or equal >=", 5, ">=", 5, true},
		{"less or equal <=", 5, "<=", 5, true},
		{"equal fails", 5, "==", 3, false},
		{"greater fails", 3, ">", 5, false},
		{"unknown operator defaults to ==", 5, "unknown", 5, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compareValues(tt.actual, tt.operator, tt.expected)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestGetTargetPods_ByName(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-ns",
		},
	}

	clientset := fake.NewClientset(pod)
	executor := NewExecutor(clientset, nil, nil, "test-ns")

	pods, err := executor.getTargetPods(context.Background(), Target{
		Kind: "Pod",
		Name: "test-pod",
	})

	require.NoError(t, err)
	require.Len(t, pods, 1)
	assert.Equal(t, "test-pod", pods[0].Name)
}

func TestGetTargetPods_ByLabelSelector(t *testing.T) {
	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-1",
			Namespace: "test-ns",
			Labels:    map[string]string{"app": "test"},
		},
	}

	pod2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-2",
			Namespace: "test-ns",
			Labels:    map[string]string{"app": "test"},
		},
	}

	clientset := fake.NewClientset(pod1, pod2)
	executor := NewExecutor(clientset, nil, nil, "test-ns")

	pods, err := executor.getTargetPods(context.Background(), Target{
		Kind:          "Pod",
		LabelSelector: map[string]string{"app": "test"},
	})

	require.NoError(t, err)
	assert.Len(t, pods, 2)
}

func TestGetPodsForResource_Deployment(t *testing.T) {
	deployment := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      "test-deployment",
				"namespace": "test-ns",
			},
			"spec": map[string]interface{}{
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"app": "test",
					},
				},
			},
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-ns",
			Labels:    map[string]string{"app": "test"},
		},
	}

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme, deployment)
	clientset := fake.NewClientset(pod)

	executor := NewExecutor(clientset, dynamicClient, nil, "test-ns")

	pods, err := executor.getPodsForResource(context.Background(), Target{
		Kind: "Deployment",
		Name: "test-deployment",
	})

	require.NoError(t, err)
	require.Len(t, pods, 1)
	assert.Equal(t, "test-pod", pods[0].Name)
}
func TestExecute_LogType(t *testing.T) {
	executor := NewExecutor(
		fake.NewClientset(),
		dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()),
		&rest.Config{},
		"test-ns",
	)

	validation := Validation{
		Key:  "test-log",
		Type: "log",
		Spec: LogSpec{
			Target: Target{
				Kind: "Pod",
				Name: "nonexistent",
			},
			ExpectedStrings: []string{"test"},
		},
	}

	result := executor.Execute(context.Background(), validation)

	assert.False(t, result.Passed)
	assert.Equal(t, "test-log", result.Key)
	// Log validation returns an error when pod not found
	assert.NotEmpty(t, result.Message)
}

func TestExecute_ConnectivityType(t *testing.T) {
	executor := NewExecutor(
		fake.NewClientset(),
		nil,
		&rest.Config{},
		"test-ns",
	)

	validation := Validation{
		Key:  "test-connectivity",
		Type: "connectivity",
		Spec: ConnectivitySpec{
			SourcePod: SourcePod{
				Name: "nonexistent",
			},
			Targets: []ConnectivityCheck{
				{URL: "http://test"},
			},
		},
	}

	result := executor.Execute(context.Background(), validation)

	assert.False(t, result.Passed)
	assert.Equal(t, "test-connectivity", result.Key)
}

func TestGetGVRForKind_AllKinds(t *testing.T) {
	tests := []struct {
		kind        string
		expectedGVR schema.GroupVersionResource
		expectError bool
	}{
		{
			kind: "Pod",
			expectedGVR: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
		},
		{
			kind: "Service",
			expectedGVR: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "services",
			},
		},
		{
			kind:        "UnknownKind",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			gvr, err := getGVRForKind(tt.kind)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedGVR, gvr)
			}
		})
	}
}

func TestGetNestedInt64_AllTypes(t *testing.T) {
	tests := []struct {
		name       string
		obj        map[string]interface{}
		fields     []string
		expected   int64
		shouldFind bool
	}{
		{
			name:       "int value",
			obj:        map[string]interface{}{"value": 42},
			fields:     []string{"value"},
			expected:   42,
			shouldFind: true,
		},
		{
			name:       "float64 value",
			obj:        map[string]interface{}{"value": 42.0},
			fields:     []string{"value"},
			expected:   42,
			shouldFind: true,
		},
		{
			name:       "nested value",
			obj:        map[string]interface{}{"status": map[string]interface{}{"count": int32(10)}},
			fields:     []string{"status", "count"},
			expected:   10,
			shouldFind: true,
		},
		{
			name:       "missing field",
			obj:        map[string]interface{}{"value": 42},
			fields:     []string{"missing"},
			shouldFind: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, found, err := getNestedInt64(tt.obj, tt.fields...)
			require.NoError(t, err)
			assert.Equal(t, tt.shouldFind, found)
			if found {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestCompareValues_AllOperators(t *testing.T) {
	tests := []struct {
		name     string
		actual   int64
		operator string
		expected int64
		result   bool
	}{
		{"equal - true", 5, "==", 5, true},
		{"equal - false", 5, "==", 3, false},
		{"not equal - true", 5, "!=", 3, true},
		{"not equal - false", 5, "!=", 5, false},
		{"greater - true", 5, ">", 3, true},
		{"greater - false", 3, ">", 5, false},
		{"less - true", 3, "<", 5, true},
		{"less - false", 5, "<", 3, false},
		{"greater or equal - true (greater)", 5, ">=", 3, true},
		{"greater or equal - true (equal)", 5, ">=", 5, true},
		{"greater or equal - false", 3, ">=", 5, false},
		{"less or equal - true (less)", 3, "<=", 5, true},
		{"less or equal - true (equal)", 5, "<=", 5, true},
		{"less or equal - false", 5, "<=", 3, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compareValues(tt.actual, tt.operator, tt.expected)
			assert.Equal(t, tt.result, result)
		})
	}
}

func TestNewExecutor_NilValues(t *testing.T) {
	executor := NewExecutor(nil, nil, nil, "")

	assert.NotNil(t, executor)
	assert.Equal(t, "", executor.namespace)
}

// =============================================================================
// Log Validator Tests
// =============================================================================

func TestExecuteLog_NoMatchingPods(t *testing.T) {
	clientset := fake.NewClientset()
	executor := NewExecutor(clientset, nil, nil, "test-ns")

	spec := LogSpec{
		Target: Target{
			Kind: "Pod",
			Name: "nonexistent-pod",
		},
		ExpectedStrings: []string{"test string"},
		SinceSeconds:    300,
	}

	passed, _, err := executor.executeLog(context.Background(), spec)

	// When a specific pod name is provided and it doesn't exist, an error is returned
	assert.Error(t, err)
	assert.False(t, passed)
}

func TestExecuteLog_NoMatchingPodsWithLabelSelector(t *testing.T) {
	// Create a pod with different labels
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-ns",
			Labels:    map[string]string{"app": "other"},
		},
	}

	clientset := fake.NewClientset(pod)
	executor := NewExecutor(clientset, nil, nil, "test-ns")

	spec := LogSpec{
		Target: Target{
			Kind:          "Pod",
			LabelSelector: map[string]string{"app": "test"},
		},
		ExpectedStrings: []string{"test string"},
		SinceSeconds:    300,
	}

	passed, msg, err := executor.executeLog(context.Background(), spec)

	require.NoError(t, err)
	assert.False(t, passed)
	assert.Equal(t, errNoMatchingPods, msg)
}

func TestExecuteLog_ByLabelSelector(t *testing.T) {
	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-1",
			Namespace: "test-ns",
			Labels:    map[string]string{"app": "test"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "main"},
			},
		},
	}

	pod2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-2",
			Namespace: "test-ns",
			Labels:    map[string]string{"app": "test"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "main"},
			},
		},
	}

	clientset := fake.NewClientset(pod1, pod2)
	executor := NewExecutor(clientset, nil, nil, "test-ns")

	spec := LogSpec{
		Target: Target{
			Kind:          "Pod",
			LabelSelector: map[string]string{"app": "test"},
		},
		ExpectedStrings: []string{"test string"},
		SinceSeconds:    300,
	}

	// This will fail because fake client doesn't support GetLogs
	// but it proves the label selector logic works
	passed, msg, err := executor.executeLog(context.Background(), spec)

	require.NoError(t, err)
	assert.False(t, passed)
	// Should contain "Missing strings" since logs can't be fetched
	assert.Contains(t, msg, "Missing strings")
}

func TestExecuteLog_WithSpecificContainer(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-ns",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "sidecar"},
				{Name: "main"},
			},
		},
	}

	clientset := fake.NewClientset(pod)
	executor := NewExecutor(clientset, nil, nil, "test-ns")

	spec := LogSpec{
		Target: Target{
			Kind: "Pod",
			Name: "test-pod",
		},
		Container:       "main",
		ExpectedStrings: []string{"test string"},
		SinceSeconds:    300,
	}

	// Fake client doesn't support GetLogs, but this tests the container selection path
	passed, msg, err := executor.executeLog(context.Background(), spec)

	require.NoError(t, err)
	assert.False(t, passed)
	assert.Contains(t, msg, "Missing strings")
}

func TestExecuteLog_MultipleExpectedStrings(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-ns",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "main"},
			},
		},
	}

	clientset := fake.NewClientset(pod)
	executor := NewExecutor(clientset, nil, nil, "test-ns")

	spec := LogSpec{
		Target: Target{
			Kind: "Pod",
			Name: "test-pod",
		},
		ExpectedStrings: []string{"string1", "string2", "string3"},
		SinceSeconds:    300,
	}

	passed, msg, err := executor.executeLog(context.Background(), spec)

	require.NoError(t, err)
	assert.False(t, passed)
	// All strings should be missing
	assert.Contains(t, msg, "string1")
	assert.Contains(t, msg, "string2")
	assert.Contains(t, msg, "string3")
}

func TestExecuteLog_DefaultContainer(t *testing.T) {
	// Test that the first container is used when none specified
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-ns",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "first-container"},
				{Name: "second-container"},
			},
		},
	}

	clientset := fake.NewClientset(pod)
	executor := NewExecutor(clientset, nil, nil, "test-ns")

	spec := LogSpec{
		Target: Target{
			Kind: "Pod",
			Name: "test-pod",
		},
		// No container specified - should use first
		ExpectedStrings: []string{"test"},
		SinceSeconds:    300,
	}

	// This exercises the default container selection code path
	passed, msg, err := executor.executeLog(context.Background(), spec)

	require.NoError(t, err)
	assert.False(t, passed)
	assert.Contains(t, msg, "Missing strings")
}

// =============================================================================
// Event Validator Tests
// =============================================================================

func TestExecuteEvent_ByLabelSelector(t *testing.T) {
	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-1",
			Namespace: "test-ns",
			Labels:    map[string]string{"app": "test"},
			UID:       "uid-1",
		},
	}

	pod2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-2",
			Namespace: "test-ns",
			Labels:    map[string]string{"app": "test"},
			UID:       "uid-2",
		},
	}

	clientset := fake.NewClientset(pod1, pod2)
	executor := NewExecutor(clientset, nil, nil, "test-ns")

	spec := EventSpec{
		Target: Target{
			Kind:          "Pod",
			LabelSelector: map[string]string{"app": "test"},
		},
		ForbiddenReasons: []string{"OOMKilled", "Evicted"},
		SinceSeconds:     300,
	}

	passed, msg, err := executor.executeEvent(context.Background(), spec)

	require.NoError(t, err)
	assert.True(t, passed)
	assert.Equal(t, msgNoForbiddenEvents, msg)
}

func TestExecuteEvent_MultipleForbiddenReasons(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-ns",
			UID:       "test-uid",
		},
	}

	// Create events with different forbidden reasons
	event1 := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "event-1",
			Namespace: "test-ns",
		},
		InvolvedObject: corev1.ObjectReference{
			Kind: "Pod",
			Name: "test-pod",
			UID:  "test-uid",
		},
		Reason:        "OOMKilled",
		LastTimestamp: metav1.Now(),
		EventTime:     metav1.NowMicro(),
	}

	event2 := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "event-2",
			Namespace: "test-ns",
		},
		InvolvedObject: corev1.ObjectReference{
			Kind: "Pod",
			Name: "test-pod",
			UID:  "test-uid",
		},
		Reason:        "Evicted",
		LastTimestamp: metav1.Now(),
		EventTime:     metav1.NowMicro(),
	}

	clientset := fake.NewClientset(pod, event1, event2)
	executor := NewExecutor(clientset, nil, nil, "test-ns")

	spec := EventSpec{
		Target: Target{
			Kind: "Pod",
			Name: "test-pod",
		},
		ForbiddenReasons: []string{"OOMKilled", "Evicted", "BackOff"},
		SinceSeconds:     300,
	}

	passed, msg, err := executor.executeEvent(context.Background(), spec)

	require.NoError(t, err)
	assert.False(t, passed)
	assert.Contains(t, msg, "Forbidden events detected")
	assert.Contains(t, msg, "OOMKilled")
	assert.Contains(t, msg, "Evicted")
}

func TestExecuteEvent_EventsOnMultiplePods(t *testing.T) {
	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-1",
			Namespace: "test-ns",
			Labels:    map[string]string{"app": "test"},
			UID:       "uid-1",
		},
	}

	pod2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-2",
			Namespace: "test-ns",
			Labels:    map[string]string{"app": "test"},
			UID:       "uid-2",
		},
	}

	// Event on first pod
	event1 := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "event-1",
			Namespace: "test-ns",
		},
		InvolvedObject: corev1.ObjectReference{
			Kind: "Pod",
			Name: "test-pod-1",
			UID:  "uid-1",
		},
		Reason:        "BackOff",
		LastTimestamp: metav1.Now(),
		EventTime:     metav1.NowMicro(),
	}

	clientset := fake.NewClientset(pod1, pod2, event1)
	executor := NewExecutor(clientset, nil, nil, "test-ns")

	spec := EventSpec{
		Target: Target{
			Kind:          "Pod",
			LabelSelector: map[string]string{"app": "test"},
		},
		ForbiddenReasons: []string{"BackOff"},
		SinceSeconds:     300,
	}

	passed, msg, err := executor.executeEvent(context.Background(), spec)

	require.NoError(t, err)
	assert.False(t, passed)
	assert.Contains(t, msg, "BackOff on test-pod-1")
}

func TestExecuteEvent_OldEventsIgnored(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-ns",
			UID:       "test-uid",
		},
	}

	// Create an old event (older than sinceSeconds)
	oldTime := metav1.NewTime(time.Now().Add(-1 * time.Hour))
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "old-event",
			Namespace: "test-ns",
		},
		InvolvedObject: corev1.ObjectReference{
			Kind: "Pod",
			Name: "test-pod",
			UID:  "test-uid",
		},
		Reason:        "OOMKilled",
		LastTimestamp: oldTime,
		EventTime:     metav1.NewMicroTime(oldTime.Time),
	}

	clientset := fake.NewClientset(pod, event)
	executor := NewExecutor(clientset, nil, nil, "test-ns")

	spec := EventSpec{
		Target: Target{
			Kind: "Pod",
			Name: "test-pod",
		},
		ForbiddenReasons: []string{"OOMKilled"},
		SinceSeconds:     300, // Only check last 5 minutes
	}

	passed, msg, err := executor.executeEvent(context.Background(), spec)

	require.NoError(t, err)
	assert.True(t, passed)
	assert.Equal(t, msgNoForbiddenEvents, msg)
}

func TestExecuteEvent_NoMatchingPods(t *testing.T) {
	clientset := fake.NewClientset()
	executor := NewExecutor(clientset, nil, nil, "test-ns")

	spec := EventSpec{
		Target: Target{
			Kind: "Pod",
			Name: "nonexistent-pod",
		},
		ForbiddenReasons: []string{"OOMKilled"},
		SinceSeconds:     300,
	}

	passed, _, err := executor.executeEvent(context.Background(), spec)

	assert.Error(t, err)
	assert.False(t, passed)
}

func TestExecuteEvent_NonPodEventsIgnored(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-ns",
			UID:       "test-uid",
		},
	}

	// Event on a non-Pod resource
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deployment-event",
			Namespace: "test-ns",
		},
		InvolvedObject: corev1.ObjectReference{
			Kind: "Deployment",
			Name: "test-deployment",
		},
		Reason:        "OOMKilled",
		LastTimestamp: metav1.Now(),
		EventTime:     metav1.NowMicro(),
	}

	clientset := fake.NewClientset(pod, event)
	executor := NewExecutor(clientset, nil, nil, "test-ns")

	spec := EventSpec{
		Target: Target{
			Kind: "Pod",
			Name: "test-pod",
		},
		ForbiddenReasons: []string{"OOMKilled"},
		SinceSeconds:     300,
	}

	passed, msg, err := executor.executeEvent(context.Background(), spec)

	require.NoError(t, err)
	assert.True(t, passed)
	assert.Equal(t, msgNoForbiddenEvents, msg)
}

// =============================================================================
// Condition Validator Tests
// =============================================================================

func TestExecuteCondition_MultipleConditions(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-ns",
		},
		Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionTrue},
				{Type: corev1.PodScheduled, Status: corev1.ConditionTrue},
				{Type: corev1.ContainersReady, Status: corev1.ConditionTrue},
			},
		},
	}

	clientset := fake.NewClientset(pod)
	executor := NewExecutor(clientset, nil, nil, "test-ns")

	spec := ConditionSpec{
		Target: Target{
			Kind: "Pod",
			Name: "test-pod",
		},
		Checks: []ConditionCheck{
			{Type: "Ready", Status: corev1.ConditionTrue},
			{Type: "PodScheduled", Status: corev1.ConditionTrue},
			{Type: "ContainersReady", Status: corev1.ConditionTrue},
		},
	}

	passed, msg, err := executor.executeCondition(context.Background(), spec)

	require.NoError(t, err)
	assert.True(t, passed)
	assert.Contains(t, msg, "All 1 pod(s) meet the required conditions")
}

func TestExecuteCondition_PartialConditionsMet(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-ns",
		},
		Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionTrue},
				{Type: corev1.PodScheduled, Status: corev1.ConditionFalse},
			},
		},
	}

	clientset := fake.NewClientset(pod)
	executor := NewExecutor(clientset, nil, nil, "test-ns")

	spec := ConditionSpec{
		Target: Target{
			Kind: "Pod",
			Name: "test-pod",
		},
		Checks: []ConditionCheck{
			{Type: "Ready", Status: corev1.ConditionTrue},
			{Type: "PodScheduled", Status: corev1.ConditionTrue},
		},
	}

	passed, msg, err := executor.executeCondition(context.Background(), spec)

	require.NoError(t, err)
	assert.False(t, passed)
	assert.Contains(t, msg, "PodScheduled is not True")
}

func TestExecuteCondition_ConditionNotFound(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-ns",
		},
		Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionTrue},
			},
		},
	}

	clientset := fake.NewClientset(pod)
	executor := NewExecutor(clientset, nil, nil, "test-ns")

	spec := ConditionSpec{
		Target: Target{
			Kind: "Pod",
			Name: "test-pod",
		},
		Checks: []ConditionCheck{
			{Type: "NonExistentCondition", Status: corev1.ConditionTrue},
		},
	}

	passed, msg, err := executor.executeCondition(context.Background(), spec)

	require.NoError(t, err)
	assert.False(t, passed)
	assert.Contains(t, msg, "condition NonExistentCondition not found")
}

func TestExecuteCondition_MultiplePodsMixedResults(t *testing.T) {
	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-1",
			Namespace: "test-ns",
			Labels:    map[string]string{"app": "test"},
		},
		Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionTrue},
			},
		},
	}

	pod2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-2",
			Namespace: "test-ns",
			Labels:    map[string]string{"app": "test"},
		},
		Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionFalse},
			},
		},
	}

	clientset := fake.NewClientset(pod1, pod2)
	executor := NewExecutor(clientset, nil, nil, "test-ns")

	spec := ConditionSpec{
		Target: Target{
			Kind:          "Pod",
			LabelSelector: map[string]string{"app": "test"},
		},
		Checks: []ConditionCheck{
			{Type: "Ready", Status: corev1.ConditionTrue},
		},
	}

	passed, msg, err := executor.executeCondition(context.Background(), spec)

	require.NoError(t, err)
	assert.False(t, passed)
	assert.Contains(t, msg, "test-pod-2")
	assert.Contains(t, msg, "Ready is not True")
}

func TestExecuteCondition_ForDeployment(t *testing.T) {
	deployment := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      "test-deployment",
				"namespace": "test-ns",
			},
			"spec": map[string]interface{}{
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"app": "test",
					},
				},
			},
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-ns",
			Labels:    map[string]string{"app": "test"},
		},
		Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionTrue},
			},
		},
	}

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme, deployment)
	clientset := fake.NewClientset(pod)
	executor := NewExecutor(clientset, dynamicClient, nil, "test-ns")

	spec := ConditionSpec{
		Target: Target{
			Kind: "Deployment",
			Name: "test-deployment",
		},
		Checks: []ConditionCheck{
			{Type: "Ready", Status: corev1.ConditionTrue},
		},
	}

	passed, msg, err := executor.executeCondition(context.Background(), spec)

	require.NoError(t, err)
	assert.True(t, passed)
	assert.Contains(t, msg, "All 1 pod(s) meet the required conditions")
}

// =============================================================================
// Connectivity Validator Tests
// =============================================================================

func TestExecuteConnectivity_NoMatchingSourcePods(t *testing.T) {
	clientset := fake.NewClientset()
	executor := NewExecutor(clientset, nil, &rest.Config{}, "test-ns")

	spec := ConnectivitySpec{
		SourcePod: SourcePod{
			LabelSelector: map[string]string{"app": "nonexistent"},
		},
		Targets: []ConnectivityCheck{
			{URL: "http://test-service", ExpectedStatusCode: 200},
		},
	}

	passed, msg, err := executor.executeConnectivity(context.Background(), spec)

	require.NoError(t, err)
	assert.False(t, passed)
	assert.Equal(t, errNoMatchingSourcePods, msg)
}

func TestExecuteConnectivity_NoRunningSourcePods(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-ns",
			Labels:    map[string]string{"app": "test"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending, // Not running
		},
	}

	clientset := fake.NewClientset(pod)
	executor := NewExecutor(clientset, nil, &rest.Config{}, "test-ns")

	spec := ConnectivitySpec{
		SourcePod: SourcePod{
			LabelSelector: map[string]string{"app": "test"},
		},
		Targets: []ConnectivityCheck{
			{URL: "http://test-service", ExpectedStatusCode: 200},
		},
	}

	passed, msg, err := executor.executeConnectivity(context.Background(), spec)

	require.NoError(t, err)
	assert.False(t, passed)
	assert.Equal(t, errNoRunningSourcePods, msg)
}

func TestExecuteConnectivity_NoSourcePodSpecified(t *testing.T) {
	clientset := fake.NewClientset()
	executor := NewExecutor(clientset, nil, &rest.Config{}, "test-ns")

	spec := ConnectivitySpec{
		SourcePod: SourcePod{
			// Neither name nor labelSelector specified
		},
		Targets: []ConnectivityCheck{
			{URL: "http://test-service", ExpectedStatusCode: 200},
		},
	}

	passed, msg, err := executor.executeConnectivity(context.Background(), spec)

	require.NoError(t, err)
	assert.False(t, passed)
	assert.Equal(t, errNoSourcePodSpecified, msg)
}

// Note: TestExecuteConnectivity_SourcePodByName and TestExecuteConnectivity_SourcePodByLabelSelectorRunning
// cannot be fully tested with fake clientset because fake.NewClientset() does not provide
// a real RESTClient for pod exec operations. These scenarios are tested through integration tests.
// The core logic of source pod lookup by name is implicitly tested in TestExecuteConnectivity_SourcePodNotFound
// and the label selector logic is tested in TestExecuteConnectivity_NoRunningSourcePods.

func TestExecuteConnectivity_SourcePodNotFound(t *testing.T) {
	clientset := fake.NewClientset()
	executor := NewExecutor(clientset, nil, &rest.Config{}, "test-ns")

	spec := ConnectivitySpec{
		SourcePod: SourcePod{
			Name: "nonexistent-pod",
		},
		Targets: []ConnectivityCheck{
			{URL: "http://test-service", ExpectedStatusCode: 200},
		},
	}

	passed, _, err := executor.executeConnectivity(context.Background(), spec)

	assert.False(t, passed)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get source pod")
}

// =============================================================================
// Status (Field-Based) Validator Tests
// =============================================================================

func TestExecuteStatus_MultipleChecks(t *testing.T) {
	deployment := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      "test-deployment",
				"namespace": "test-ns",
			},
			"spec": map[string]interface{}{
				"replicas": int64(3),
			},
			"status": map[string]interface{}{
				"readyReplicas":     int64(3),
				"availableReplicas": int64(3),
				"replicas":          int64(3),
			},
		},
	}

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme, deployment)
	executor := NewExecutor(nil, dynamicClient, nil, "test-ns")

	spec := StatusSpec{
		Target: Target{
			Kind: "Deployment",
			Name: "test-deployment",
		},
		Checks: []StatusCheck{
			{Field: "readyReplicas", Operator: "==", Value: int64(3)},
			{Field: "availableReplicas", Operator: ">=", Value: int64(3)},
			{Field: "replicas", Operator: "<=", Value: int64(5)},
		},
	}

	passed, msg, err := executor.executeStatus(context.Background(), spec)

	require.NoError(t, err)
	assert.True(t, passed)
	assert.Equal(t, msgAllStatusChecksPassed, msg)
}

func TestExecuteStatus_MultipleChecksMixedResults(t *testing.T) {
	deployment := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      "test-deployment",
				"namespace": "test-ns",
			},
			"status": map[string]interface{}{
				"readyReplicas":     int64(2),
				"availableReplicas": int64(1),
			},
		},
	}

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme, deployment)
	executor := NewExecutor(nil, dynamicClient, nil, "test-ns")

	spec := StatusSpec{
		Target: Target{
			Kind: "Deployment",
			Name: "test-deployment",
		},
		Checks: []StatusCheck{
			{Field: "readyReplicas", Operator: "==", Value: int64(3)},
			{Field: "availableReplicas", Operator: ">=", Value: int64(3)},
		},
	}

	passed, msg, err := executor.executeStatus(context.Background(), spec)

	require.NoError(t, err)
	assert.False(t, passed)
	assert.Contains(t, msg, "readyReplicas")
	assert.Contains(t, msg, "availableReplicas")
}

func TestExecuteStatus_ByLabelSelector(t *testing.T) {
	deployment := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      "test-deployment",
				"namespace": "test-ns",
				"labels": map[string]interface{}{
					"app": "test",
				},
			},
			"status": map[string]interface{}{
				"readyReplicas": int64(3),
			},
		},
	}

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme, deployment)
	executor := NewExecutor(nil, dynamicClient, nil, "test-ns")

	spec := StatusSpec{
		Target: Target{
			Kind:          "Deployment",
			LabelSelector: map[string]string{"app": "test"},
		},
		Checks: []StatusCheck{
			{Field: "readyReplicas", Operator: "==", Value: int64(3)},
		},
	}

	passed, msg, err := executor.executeStatus(context.Background(), spec)

	require.NoError(t, err)
	assert.True(t, passed)
	assert.Equal(t, msgAllStatusChecksPassed, msg)
}

func TestExecuteStatus_FieldNotFound(t *testing.T) {
	deployment := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      "test-deployment",
				"namespace": "test-ns",
			},
			"status": map[string]interface{}{},
		},
	}

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme, deployment)
	executor := NewExecutor(nil, dynamicClient, nil, "test-ns")

	spec := StatusSpec{
		Target: Target{
			Kind: "Deployment",
			Name: "test-deployment",
		},
		Checks: []StatusCheck{
			{Field: "nonexistent", Operator: "==", Value: int64(3)},
		},
	}

	passed, msg, err := executor.executeStatus(context.Background(), spec)

	require.NoError(t, err)
	assert.False(t, passed)
	assert.Contains(t, msg, "Field nonexistent not found")
}

func TestExecuteStatus_StatefulSet(t *testing.T) {
	statefulset := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "StatefulSet",
			"metadata": map[string]interface{}{
				"name":      "test-statefulset",
				"namespace": "test-ns",
			},
			"status": map[string]interface{}{
				"readyReplicas":   int64(3),
				"currentReplicas": int64(3),
			},
		},
	}

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme, statefulset)
	executor := NewExecutor(nil, dynamicClient, nil, "test-ns")

	spec := StatusSpec{
		Target: Target{
			Kind: "StatefulSet",
			Name: "test-statefulset",
		},
		Checks: []StatusCheck{
			{Field: "readyReplicas", Operator: "==", Value: int64(3)},
		},
	}

	passed, msg, err := executor.executeStatus(context.Background(), spec)

	require.NoError(t, err)
	assert.True(t, passed)
	assert.Equal(t, msgAllStatusChecksPassed, msg)
}

func TestExecuteStatus_DaemonSet(t *testing.T) {
	daemonset := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "DaemonSet",
			"metadata": map[string]interface{}{
				"name":      "test-daemonset",
				"namespace": "test-ns",
			},
			"status": map[string]interface{}{
				"numberReady":            int64(3),
				"desiredNumberScheduled": int64(3),
			},
		},
	}

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme, daemonset)
	executor := NewExecutor(nil, dynamicClient, nil, "test-ns")

	spec := StatusSpec{
		Target: Target{
			Kind: "DaemonSet",
			Name: "test-daemonset",
		},
		Checks: []StatusCheck{
			{Field: "numberReady", Operator: "==", Value: int64(3)},
		},
	}

	passed, msg, err := executor.executeStatus(context.Background(), spec)

	require.NoError(t, err)
	assert.True(t, passed)
	assert.Equal(t, msgAllStatusChecksPassed, msg)
}

func TestExecuteStatus_ResourceNotFound(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	executor := NewExecutor(nil, dynamicClient, nil, "test-ns")

	spec := StatusSpec{
		Target: Target{
			Kind: "Deployment",
			Name: "nonexistent",
		},
		Checks: []StatusCheck{
			{Field: "readyReplicas", Operator: "==", Value: int64(3)},
		},
	}

	passed, _, err := executor.executeStatus(context.Background(), spec)

	assert.False(t, passed)
	assert.Error(t, err)
}

func TestExecuteStatus_AllOperators(t *testing.T) {
	// In these tests:
	// - "actual" is the value in the resource (status.readyReplicas)
	// - "value" is the expected value to compare against in the check
	// The comparison is: actual <operator> value
	tests := []struct {
		name     string
		operator string
		value    int64 // expected value in the StatusCheck
		actual   int64 // actual value from the resource
		expected bool  // expected test result
	}{
		{"equals ==", "==", 5, 5, true},
		{"equals = ", "=", 5, 5, true},
		{"not equals", "!=", 5, 3, true},                 // 3 != 5 = true
		{"greater than - pass", ">", 3, 5, true},         // 5 > 3 = true
		{"greater than - fail", ">", 10, 5, false},       // 5 > 10 = false
		{"less than - pass", "<", 10, 5, true},           // 5 < 10 = true
		{"less than - fail", "<", 3, 5, false},           // 5 < 3 = false
		{"greater or equal - equal", ">=", 5, 5, true},   // 5 >= 5 = true
		{"greater or equal - greater", ">=", 3, 5, true}, // 5 >= 3 = true
		{"less or equal - equal", "<=", 5, 5, true},      // 5 <= 5 = true
		{"less or equal - less", "<=", 10, 5, true},      // 5 <= 10 = true
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deployment := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "test-ns",
					},
					"status": map[string]interface{}{
						"readyReplicas": tt.actual,
					},
				},
			}

			scheme := runtime.NewScheme()
			dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme, deployment)
			executor := NewExecutor(nil, dynamicClient, nil, "test-ns")

			spec := StatusSpec{
				Target: Target{
					Kind: "Deployment",
					Name: "test-deployment",
				},
				Checks: []StatusCheck{
					{Field: "readyReplicas", Operator: tt.operator, Value: tt.value},
				},
			}

			passed, _, err := executor.executeStatus(context.Background(), spec)

			require.NoError(t, err)
			assert.Equal(t, tt.expected, passed)
		})
	}
}

// =============================================================================
// Shell Escape Tests
// =============================================================================

func TestEscapeShellArg(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple string", "http://example.com", "http://example.com"},
		{"string with single quote", "it's a test", "it'\"'\"'s a test"},
		{"multiple single quotes", "don't won't can't", "don'\"'\"'t won'\"'\"'t can'\"'\"'t"},
		{"empty string", "", ""},
		{"string with special chars", "http://test?a=1&b=2", "http://test?a=1&b=2"},
		{"string with spaces", "hello world", "hello world"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapeShellArg(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// =============================================================================
// getPodConditionTypes Tests
// =============================================================================

func TestGetPodConditionTypes(t *testing.T) {
	tests := []struct {
		name       string
		conditions []corev1.PodCondition
		expected   []string
	}{
		{
			name: "multiple conditions",
			conditions: []corev1.PodCondition{
				{Type: corev1.PodReady},
				{Type: corev1.PodScheduled},
				{Type: corev1.ContainersReady},
			},
			expected: []string{"Ready", "PodScheduled", "ContainersReady"},
		},
		{
			name:       "empty conditions",
			conditions: []corev1.PodCondition{},
			expected:   []string{},
		},
		{
			name: "single condition",
			conditions: []corev1.PodCondition{
				{Type: corev1.PodReady},
			},
			expected: []string{"Ready"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := &corev1.Pod{
				Status: corev1.PodStatus{
					Conditions: tt.conditions,
				},
			}
			result := getPodConditionTypes(pod)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// =============================================================================
// GVR Tests for all resource types
// =============================================================================

func TestGetGVRForKind_AllSupportedKinds(t *testing.T) {
	tests := []struct {
		kind     string
		expected schema.GroupVersionResource
	}{
		{
			kind: "Deployment",
			expected: schema.GroupVersionResource{
				Group: "apps", Version: "v1", Resource: "deployments",
			},
		},
		{
			kind: "StatefulSet",
			expected: schema.GroupVersionResource{
				Group: "apps", Version: "v1", Resource: "statefulsets",
			},
		},
		{
			kind: "DaemonSet",
			expected: schema.GroupVersionResource{
				Group: "apps", Version: "v1", Resource: "daemonsets",
			},
		},
		{
			kind: "ReplicaSet",
			expected: schema.GroupVersionResource{
				Group: "apps", Version: "v1", Resource: "replicasets",
			},
		},
		{
			kind: "Job",
			expected: schema.GroupVersionResource{
				Group: "batch", Version: "v1", Resource: "jobs",
			},
		},
		{
			kind: "Pod",
			expected: schema.GroupVersionResource{
				Group: "", Version: "v1", Resource: "pods",
			},
		},
		{
			kind: "Service",
			expected: schema.GroupVersionResource{
				Group: "", Version: "v1", Resource: "services",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			gvr, err := getGVRForKind(tt.kind)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, gvr)
		})
	}
}

func TestGetGVRForKind_CaseInsensitive(t *testing.T) {
	tests := []string{"deployment", "DEPLOYMENT", "Deployment", "dEpLoYmEnT"}

	for _, kind := range tests {
		t.Run(kind, func(t *testing.T) {
			gvr, err := getGVRForKind(kind)
			require.NoError(t, err)
			assert.Equal(t, "deployments", gvr.Resource)
		})
	}
}

// Comprehensive status validation tests

func TestExecuteStatus_StringComparison(t *testing.T) {
	pod := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      "test-pod",
				"namespace": "test-ns",
			},
			"status": map[string]interface{}{
				"phase": "Running",
			},
		},
	}

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme, pod)
	executor := NewExecutor(nil, dynamicClient, nil, "test-ns")

	tests := []struct {
		name     string
		field    string
		operator string
		value    string
		want     bool
	}{
		{"equal match", "phase", "==", "Running", true},
		{"equal no match", "phase", "==", "Pending", false},
		{"not equal match", "phase", "!=", "Pending", true},
		{"not equal no match", "phase", "!=", "Running", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := StatusSpec{
				Target: Target{Kind: "Pod", Name: "test-pod"},
				Checks: []StatusCheck{
					{Field: tt.field, Operator: tt.operator, Value: tt.value},
				},
			}

			passed, _, err := executor.executeStatus(context.Background(), spec)
			require.NoError(t, err)
			assert.Equal(t, tt.want, passed)
		})
	}
}

func TestExecuteStatus_BooleanComparison(t *testing.T) {
	pod := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      "test-pod",
				"namespace": "test-ns",
			},
			"status": map[string]interface{}{
				"containerStatuses": []interface{}{
					map[string]interface{}{
						"ready":   true,
						"started": false,
					},
				},
			},
		},
	}

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme, pod)
	executor := NewExecutor(nil, dynamicClient, nil, "test-ns")

	tests := []struct {
		name     string
		field    string
		operator string
		value    bool
		want     bool
	}{
		{"true equals true", "containerStatuses[0].ready", "==", true, true},
		{"true equals false", "containerStatuses[0].ready", "==", false, false},
		{"false equals false", "containerStatuses[0].started", "==", false, true},
		{"not equal", "containerStatuses[0].ready", "!=", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := StatusSpec{
				Target: Target{Kind: "Pod", Name: "test-pod"},
				Checks: []StatusCheck{
					{Field: tt.field, Operator: tt.operator, Value: tt.value},
				},
			}

			passed, _, err := executor.executeStatus(context.Background(), spec)
			require.NoError(t, err)
			assert.Equal(t, tt.want, passed)
		})
	}
}

func TestExecuteStatus_NumericOperators(t *testing.T) {
	deployment := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      "test-deployment",
				"namespace": "test-ns",
			},
			"status": map[string]interface{}{
				"readyReplicas":     int64(3),
				"availableReplicas": int64(5),
				"replicas":          int64(5),
			},
		},
	}

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme, deployment)
	executor := NewExecutor(nil, dynamicClient, nil, "test-ns")

	tests := []struct {
		name     string
		field    string
		operator string
		value    int64
		want     bool
	}{
		{"equal match", "readyReplicas", "==", 3, true},
		{"equal no match", "readyReplicas", "==", 5, false},
		{"not equal match", "readyReplicas", "!=", 5, true},
		{"not equal no match", "readyReplicas", "!=", 3, false},
		{"greater than match", "availableReplicas", ">", 3, true},
		{"greater than no match", "readyReplicas", ">", 5, false},
		{"less than match", "readyReplicas", "<", 5, true},
		{"less than no match", "availableReplicas", "<", 3, false},
		{"greater or equal match exact", "readyReplicas", ">=", 3, true},
		{"greater or equal match greater", "availableReplicas", ">=", 3, true},
		{"greater or equal no match", "readyReplicas", ">=", 5, false},
		{"less or equal match exact", "readyReplicas", "<=", 3, true},
		{"less or equal match less", "readyReplicas", "<=", 5, true},
		{"less or equal no match", "availableReplicas", "<=", 3, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := StatusSpec{
				Target: Target{Kind: "Deployment", Name: "test-deployment"},
				Checks: []StatusCheck{
					{Field: tt.field, Operator: tt.operator, Value: tt.value},
				},
			}

			passed, _, err := executor.executeStatus(context.Background(), spec)
			require.NoError(t, err)
			assert.Equal(t, tt.want, passed)
		})
	}
}

func TestExecuteStatus_ArrayIndexAccess(t *testing.T) {
	pod := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      "test-pod",
				"namespace": "test-ns",
			},
			"status": map[string]interface{}{
				"containerStatuses": []interface{}{
					map[string]interface{}{
						"name":         "container-1",
						"restartCount": int64(0),
					},
					map[string]interface{}{
						"name":         "container-2",
						"restartCount": int64(5),
					},
				},
			},
		},
	}

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme, pod)
	executor := NewExecutor(nil, dynamicClient, nil, "test-ns")

	tests := []struct {
		name     string
		field    string
		operator string
		value    interface{}
		want     bool
	}{
		{"first container restart count", "containerStatuses[0].restartCount", "==", int64(0), true},
		{"second container restart count", "containerStatuses[1].restartCount", "==", int64(5), true},
		{"first container name", "containerStatuses[0].name", "==", "container-1", true},
		{"second container name", "containerStatuses[1].name", "==", "container-2", true},
		{"restart count comparison", "containerStatuses[1].restartCount", "<", int64(10), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := StatusSpec{
				Target: Target{Kind: "Pod", Name: "test-pod"},
				Checks: []StatusCheck{
					{Field: tt.field, Operator: tt.operator, Value: tt.value},
				},
			}

			passed, _, err := executor.executeStatus(context.Background(), spec)
			require.NoError(t, err)
			assert.Equal(t, tt.want, passed)
		})
	}
}

func TestExecuteStatus_ArrayFilterAccess(t *testing.T) {
	deployment := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      "test-deployment",
				"namespace": "test-ns",
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Available",
						"status": "True",
					},
					map[string]interface{}{
						"type":   "Progressing",
						"status": "True",
					},
					map[string]interface{}{
						"type":   "ReplicaFailure",
						"status": "False",
					},
				},
			},
		},
	}

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme, deployment)
	executor := NewExecutor(nil, dynamicClient, nil, "test-ns")

	tests := []struct {
		name     string
		field    string
		operator string
		value    string
		want     bool
	}{
		{"available condition", "conditions[type=Available].status", "==", "True", true},
		{"progressing condition", "conditions[type=Progressing].status", "==", "True", true},
		{"replica failure condition", "conditions[type=ReplicaFailure].status", "==", "False", true},
		{"available not false", "conditions[type=Available].status", "!=", "False", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := StatusSpec{
				Target: Target{Kind: "Deployment", Name: "test-deployment"},
				Checks: []StatusCheck{
					{Field: tt.field, Operator: tt.operator, Value: tt.value},
				},
			}

			passed, _, err := executor.executeStatus(context.Background(), spec)
			require.NoError(t, err)
			assert.Equal(t, tt.want, passed)
		})
	}
}

func TestExecuteStatus_TypeMismatchError(t *testing.T) {
	deployment := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      "test-deployment",
				"namespace": "test-ns",
			},
			"status": map[string]interface{}{
				"readyReplicas": int64(3),
			},
		},
	}

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme, deployment)
	executor := NewExecutor(nil, dynamicClient, nil, "test-ns")

	// Try to compare int with string - should produce an error about numeric type
	spec := StatusSpec{
		Target: Target{Kind: "Deployment", Name: "test-deployment"},
		Checks: []StatusCheck{
			{Field: "readyReplicas", Operator: "==", Value: "three"},
		},
	}

	passed, msg, err := executor.executeStatus(context.Background(), spec)
	require.NoError(t, err)
	assert.False(t, passed)
	assert.Contains(t, msg, "expected value must be numeric")
}

func TestExecuteStatus_FieldNotFoundComprehensive(t *testing.T) {
	deployment := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      "test-deployment",
				"namespace": "test-ns",
			},
			"status": map[string]interface{}{
				"readyReplicas": int64(3),
			},
		},
	}

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme, deployment)
	executor := NewExecutor(nil, dynamicClient, nil, "test-ns")

	spec := StatusSpec{
		Target: Target{Kind: "Deployment", Name: "test-deployment"},
		Checks: []StatusCheck{
			{Field: "nonExistentField", Operator: "==", Value: int64(3)},
		},
	}

	passed, msg, err := executor.executeStatus(context.Background(), spec)
	require.NoError(t, err)
	assert.False(t, passed)
	assert.Contains(t, msg, "not found")
}

func TestExecuteStatus_MultipleChecksComprehensive(t *testing.T) {
	deployment := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      "test-deployment",
				"namespace": "test-ns",
			},
			"status": map[string]interface{}{
				"readyReplicas":     int64(3),
				"availableReplicas": int64(3),
				"replicas":          int64(3),
			},
		},
	}

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme, deployment)
	executor := NewExecutor(nil, dynamicClient, nil, "test-ns")

	t.Run("all checks pass", func(t *testing.T) {
		spec := StatusSpec{
			Target: Target{Kind: "Deployment", Name: "test-deployment"},
			Checks: []StatusCheck{
				{Field: "readyReplicas", Operator: "==", Value: int64(3)},
				{Field: "availableReplicas", Operator: ">=", Value: int64(3)},
				{Field: "replicas", Operator: "==", Value: int64(3)},
			},
		}

		passed, msg, err := executor.executeStatus(context.Background(), spec)
		require.NoError(t, err)
		assert.True(t, passed)
		assert.Equal(t, msgAllStatusChecksPassed, msg)
	})

	t.Run("one check fails", func(t *testing.T) {
		spec := StatusSpec{
			Target: Target{Kind: "Deployment", Name: "test-deployment"},
			Checks: []StatusCheck{
				{Field: "readyReplicas", Operator: "==", Value: int64(3)},
				{Field: "availableReplicas", Operator: ">=", Value: int64(5)}, // This fails
				{Field: "replicas", Operator: "==", Value: int64(3)},
			},
		}

		passed, msg, err := executor.executeStatus(context.Background(), spec)
		require.NoError(t, err)
		assert.False(t, passed)
		assert.Contains(t, msg, "availableReplicas")
	})
}
