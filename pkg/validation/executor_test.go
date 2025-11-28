package validation

import (
	"context"
	"testing"

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
	clientset := fake.NewSimpleClientset()
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
		fake.NewSimpleClientset(),
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

	clientset := fake.NewSimpleClientset(pod)
	executor := NewExecutor(
		clientset,
		dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()),
		&rest.Config{},
		"test-ns",
	)

	validations := []Validation{
		{
			Key:  "pod-ready",
			Type: TypeStatus,
			Spec: StatusSpec{
				Target: Target{
					Kind: "Pod",
					Name: "test-pod",
				},
				Conditions: []StatusCondition{
					{Type: "Ready", Status: "True"},
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

func TestExecuteStatus_Success(t *testing.T) {
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

	clientset := fake.NewSimpleClientset(pod)
	executor := NewExecutor(clientset, nil, nil, "test-ns")

	spec := StatusSpec{
		Target: Target{
			Kind: "Pod",
			Name: "test-pod",
		},
		Conditions: []StatusCondition{
			{Type: "Ready", Status: "True"},
		},
	}

	passed, msg, err := executor.executeStatus(context.Background(), spec)

	require.NoError(t, err)
	assert.True(t, passed)
	assert.Contains(t, msg, "All 1 pod(s) meet the required conditions")
}

func TestExecuteStatus_ConditionNotMet(t *testing.T) {
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

	clientset := fake.NewSimpleClientset(pod)
	executor := NewExecutor(clientset, nil, nil, "test-ns")

	spec := StatusSpec{
		Target: Target{
			Kind: "Pod",
			Name: "test-pod",
		},
		Conditions: []StatusCondition{
			{Type: "Ready", Status: "True"},
		},
	}

	passed, msg, err := executor.executeStatus(context.Background(), spec)

	require.NoError(t, err)
	assert.False(t, passed)
	assert.Contains(t, msg, "condition Ready is not True")
}

func TestExecuteStatus_NoMatchingPods(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	executor := NewExecutor(clientset, nil, nil, "test-ns")

	spec := StatusSpec{
		Target: Target{
			Kind: "Pod",
			Name: "nonexistent-pod",
		},
		Conditions: []StatusCondition{
			{Type: "Ready", Status: "True"},
		},
	}

	passed, _, err := executor.executeStatus(context.Background(), spec)

	assert.Error(t, err)
	assert.False(t, passed)
}

func TestExecuteStatus_ByLabelSelector(t *testing.T) {
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

	clientset := fake.NewSimpleClientset(pod1, pod2)
	executor := NewExecutor(clientset, nil, nil, "test-ns")

	spec := StatusSpec{
		Target: Target{
			Kind:          "Pod",
			LabelSelector: map[string]string{"app": "test"},
		},
		Conditions: []StatusCondition{
			{Type: "Ready", Status: "True"},
		},
	}

	passed, msg, err := executor.executeStatus(context.Background(), spec)

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

	clientset := fake.NewSimpleClientset(pod)
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
	assert.Equal(t, errNoForbiddenEvents, msg)
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

	clientset := fake.NewSimpleClientset(pod, event)
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

func TestExecuteMetrics_Success(t *testing.T) {
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

	spec := MetricsSpec{
		Target: Target{
			Kind: "Deployment",
			Name: "test-deployment",
		},
		Checks: []MetricCheck{
			{Field: "status.readyReplicas", Operator: "==", Value: 3},
		},
	}

	passed, msg, err := executor.executeMetrics(context.Background(), spec)

	require.NoError(t, err)
	assert.True(t, passed)
	assert.Equal(t, errAllMetricsChecksPassed, msg)
}

func TestExecuteMetrics_CheckFailed(t *testing.T) {
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

	spec := MetricsSpec{
		Target: Target{
			Kind: "Deployment",
			Name: "test-deployment",
		},
		Checks: []MetricCheck{
			{Field: "status.readyReplicas", Operator: ">=", Value: 3},
		},
	}

	passed, msg, err := executor.executeMetrics(context.Background(), spec)

	require.NoError(t, err)
	assert.False(t, passed)
	assert.Contains(t, msg, "got 1, expected >= 3")
}

func TestExecuteMetrics_NoMatchingResources(t *testing.T) {
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

	spec := MetricsSpec{
		Target: Target{
			Kind:          "Pod",
			LabelSelector: map[string]string{"app": "nonexistent"},
		},
		Checks: []MetricCheck{
			{Field: "status.containerStatuses[0].restartCount", Operator: "==", Value: 0},
		},
	}

	passed, msg, err := executor.executeMetrics(context.Background(), spec)

	require.NoError(t, err)
	assert.False(t, passed)
	assert.Equal(t, errNoMatchingResources, msg)
}

func TestExecuteMetrics_NoTargetSpecified(t *testing.T) {
	executor := NewExecutor(nil, dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()), nil, "test-ns")

	spec := MetricsSpec{
		Target: Target{
			Kind: "Deployment",
			// No name or labelSelector
		},
		Checks: []MetricCheck{
			{Field: "status.readyReplicas", Operator: "==", Value: 3},
		},
	}

	passed, msg, err := executor.executeMetrics(context.Background(), spec)

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

	clientset := fake.NewSimpleClientset(pod)
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

	clientset := fake.NewSimpleClientset(pod1, pod2)
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
	clientset := fake.NewSimpleClientset(pod)

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
		fake.NewSimpleClientset(),
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
		fake.NewSimpleClientset(),
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
		name      string
		obj       map[string]interface{}
		fields    []string
		expected  int64
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
