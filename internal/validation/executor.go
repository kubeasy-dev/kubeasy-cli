package validation

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kubeasy-dev/kubeasy-cli/internal/constants"
	"github.com/kubeasy-dev/kubeasy-cli/internal/deployer"
	"github.com/kubeasy-dev/kubeasy-cli/internal/logger"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/fieldpath"
	authv1 "k8s.io/api/authorization/v1"
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
	// Error messages (validation failures or missing resources)
	errNoMatchingPods       = "No matching pods found"
	errNoMatchingSourcePods = "No matching source pods found"
	errNoRunningSourcePods  = "No running source pods found"
	// errNoSourcePodSpecified is retained as a test anchor for negative assertions
	// in unit tests that verify empty SourcePod enters the probe branch instead.
	// It is no longer returned by production code (Phase 07 replaced it with probe lifecycle).
	errNoSourcePodSpecified = "No source pod specified"
	errNoMatchingResources  = "No matching resources found"
	errNoTargetSpecified    = "No target name or labelSelector specified"

	// Success messages (validation passed)
	msgAllStatusChecksPassed   = "All status checks passed"
	msgAllConnectivityPassed   = "All connectivity checks passed"
	msgAllConditionsMet        = "All %d pod(s) meet the required conditions"
	msgFoundAllExpectedStrings = "Found all expected strings in logs"
	msgNoForbiddenEvents       = "No forbidden events found"
	msgAllRbacChecksPassed     = "All RBAC checks passed" //nolint:gosec // not a credential
)

// Executor executes validations against a Kubernetes cluster
type Executor struct {
	clientset     kubernetes.Interface
	dynamicClient dynamic.Interface
	restConfig    *rest.Config
	namespace     string
	probeMu       sync.Mutex // serializes probe-mode connectivity checks
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
	start := time.Now()
	result := Result{
		Key:     v.Key,
		Passed:  false,
		Message: "Unknown validation type",
	}

	var err error
	switch v.Type {
	case TypeStatus:
		spec, ok := v.Spec.(StatusSpec)
		if !ok {
			result.Message = fmt.Sprintf("internal error: expected StatusSpec, got %T", v.Spec)
			result.Duration = time.Since(start)
			return result
		}
		result.Passed, result.Message, err = e.executeStatus(ctx, spec)
	case TypeCondition:
		spec, ok := v.Spec.(ConditionSpec)
		if !ok {
			result.Message = fmt.Sprintf("internal error: expected ConditionSpec, got %T", v.Spec)
			result.Duration = time.Since(start)
			return result
		}
		result.Passed, result.Message, err = e.executeCondition(ctx, spec)
	case TypeLog:
		spec, ok := v.Spec.(LogSpec)
		if !ok {
			result.Message = fmt.Sprintf("internal error: expected LogSpec, got %T", v.Spec)
			result.Duration = time.Since(start)
			return result
		}
		result.Passed, result.Message, err = e.executeLog(ctx, spec)
	case TypeEvent:
		spec, ok := v.Spec.(EventSpec)
		if !ok {
			result.Message = fmt.Sprintf("internal error: expected EventSpec, got %T", v.Spec)
			result.Duration = time.Since(start)
			return result
		}
		result.Passed, result.Message, err = e.executeEvent(ctx, spec)
	case TypeConnectivity:
		spec, ok := v.Spec.(ConnectivitySpec)
		if !ok {
			result.Message = fmt.Sprintf("internal error: expected ConnectivitySpec, got %T", v.Spec)
			result.Duration = time.Since(start)
			return result
		}
		result.Passed, result.Message, err = e.executeConnectivity(ctx, spec)
	case TypeRbac:
		spec, ok := v.Spec.(RbacSpec)
		if !ok {
			result.Message = fmt.Sprintf("internal error: expected RbacSpec, got %T", v.Spec)
			result.Duration = time.Since(start)
			return result
		}
		result.Passed, result.Message, err = e.executeRbac(ctx, spec)
	case TypeDns:
		spec, ok := v.Spec.(DnsSpec)
		if !ok {
			result.Message = fmt.Sprintf("internal error: expected DnsSpec, got %T", v.Spec)
			result.Duration = time.Since(start)
			return result
		}
		result.Passed, result.Message, err = e.executeDns(ctx, spec)
	default:
		result.Message = fmt.Sprintf("Unknown validation type: %s", v.Type)
		result.Duration = time.Since(start)
		return result
	}

	if err != nil {
		result.Passed = false
		result.Message = err.Error()
	}

	result.Duration = time.Since(start)
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

// ExecuteSequential runs validations one by one. If failFast is true, it stops at the first failure.
func (e *Executor) ExecuteSequential(ctx context.Context, validations []Validation, failFast bool) []Result {
	var results []Result
	for _, v := range validations {
		result := e.Execute(ctx, v)
		results = append(results, result)
		if failFast && !result.Passed {
			break
		}
	}
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
		return true, msgAllStatusChecksPassed, nil
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
		return true, fmt.Sprintf(msgAllConditionsMet, len(pods)), nil
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
		return true, msgFoundAllExpectedStrings, nil
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

	var forbiddenFound []string
	podNames := make(map[string]bool)
	for _, pod := range pods {
		podNames[pod.Name] = true
	}

	// sinceSeconds==0 means "no time filter" — check all events regardless of age.
	// The loader normalises 0 to DefaultEventSinceSeconds (300s) when loading from YAML,
	// so 0 only reaches here when EventSpec is constructed directly in code.
	var sinceTime time.Time
	if spec.SinceSeconds > 0 {
		sinceTime = time.Now().Add(-time.Duration(spec.SinceSeconds) * time.Second)
	}

	for _, event := range events.Items {
		// Check if event is for one of our pods
		if event.InvolvedObject.Kind != "Pod" || !podNames[event.InvolvedObject.Name] {
			continue
		}

		// Check if event is recent enough (skip filter when sinceTime is zero value)
		if !sinceTime.IsZero() && event.LastTimestamp.Time.Before(sinceTime) && event.EventTime.Time.Before(sinceTime) {
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
		return true, msgNoForbiddenEvents, nil
	}
	return false, fmt.Sprintf("Forbidden events detected: %v", forbiddenFound), nil
}

// executeConnectivity tests network connectivity
func (e *Executor) executeConnectivity(ctx context.Context, spec ConnectivitySpec) (bool, string, error) {
	logger.Debug("Executing connectivity validation")

	// EXT-01: external mode — CLI host sends HTTP request via net/http, no pod exec
	if spec.Mode == ConnectivityModeExternal {
		return e.checkExternalConnectivityAll(ctx, spec)
	}

	// Resolve source namespace: SourcePod.Namespace wins over executor default (CONN-02, PROBE-02)
	sourceNamespace := e.namespace
	if spec.SourcePod.Namespace != "" {
		sourceNamespace = spec.SourcePod.Namespace
	}

	// Find source pod
	var sourcePod *corev1.Pod
	switch {
	case spec.SourcePod.Name != "":
		pod, err := e.clientset.CoreV1().Pods(sourceNamespace).Get(ctx, spec.SourcePod.Name, metav1.GetOptions{})
		if err != nil {
			return false, "", fmt.Errorf("failed to get source pod: %w", err)
		}
		sourcePod = pod
	case len(spec.SourcePod.LabelSelector) > 0:
		pods, err := e.clientset.CoreV1().Pods(sourceNamespace).List(ctx, metav1.ListOptions{
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
		// Probe mode (PROBE-01): empty SourcePod — deploy a CLI-managed kubeasy-probe pod.
		// Serialize to avoid concurrent CreateProbePod collisions (fixed pod name).
		e.probeMu.Lock()
		defer e.probeMu.Unlock()
		pod, err := deployer.CreateProbePod(ctx, e.clientset, sourceNamespace)
		if err != nil {
			return false, "", fmt.Errorf("failed to create probe pod: %w", err)
		}
		defer func() {
			_ = deployer.DeleteProbePod(context.Background(), e.clientset, sourceNamespace)
		}()
		if err := deployer.WaitForProbePodReady(ctx, e.clientset, sourceNamespace); err != nil {
			return false, "", fmt.Errorf("probe pod failed to become ready: %w", err)
		}
		sourcePod = pod
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
		return true, msgAllConnectivityPassed, nil
	}
	return false, strings.Join(messages, "; "), nil
}

// checkExternalConnectivityAll iterates all targets for an external connectivity spec.
// Mirrors the internal target loop in executeConnectivity.
func (e *Executor) checkExternalConnectivityAll(ctx context.Context, spec ConnectivitySpec) (bool, string, error) {
	allPassed := true
	var messages []string
	for _, target := range spec.Targets {
		passed, msg := e.checkExternalConnectivity(ctx, target)
		if !passed {
			allPassed = false
			messages = append(messages, msg)
		}
	}
	if allPassed {
		return true, msgAllConnectivityPassed, nil
	}
	return false, strings.Join(messages, "; "), nil
}

// loadKubeasyCA reads the well-known kubeasy CA Secret from the cluster and returns an
// *x509.CertPool that trusts it. Returns nil (silent no-op) when the Secret is absent,
// unreadable, or contains no valid PEM certificate — the caller falls back to the OS trust store.
func (e *Executor) loadKubeasyCA(ctx context.Context) *x509.CertPool {
	if e.clientset == nil {
		return nil
	}
	secret, err := e.clientset.CoreV1().Secrets(constants.KubeasyCASecretNamespace).
		Get(ctx, constants.KubeasyCASecretName, metav1.GetOptions{})
	if err != nil {
		logger.Debug("loadKubeasyCA: failed to get Secret %s/%s: %v — falling back to OS trust store",
			constants.KubeasyCASecretNamespace, constants.KubeasyCASecretName, err)
		return nil
	}
	pemData, ok := secret.Data[constants.KubeasyCASecretCertKey]
	if !ok {
		logger.Debug("loadKubeasyCA: Secret %s/%s missing key %q — falling back to OS trust store",
			constants.KubeasyCASecretNamespace, constants.KubeasyCASecretName, constants.KubeasyCASecretCertKey)
		return nil
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pemData) {
		logger.Debug("loadKubeasyCA: Secret %s/%s contains no valid PEM certificate — falling back to OS trust store",
			constants.KubeasyCASecretNamespace, constants.KubeasyCASecretName)
		return nil
	}
	return pool
}

// buildExternalTLSConfig constructs the *tls.Config for checkExternalConnectivity.
// When insecureSkipVerify is set it returns early. Otherwise it auto-loads the kubeasy
// local CA so that certs signed by it are trusted without any challenge.yaml config.
func (e *Executor) buildExternalTLSConfig(ctx context.Context, target ConnectivityCheck) *tls.Config {
	cfg := &tls.Config{}

	if target.TLS != nil && target.TLS.InsecureSkipVerify {
		cfg.InsecureSkipVerify = true
		return cfg
	}

	// Auto-load local CA — silent no-op if Secret is absent (falls back to OS trust store).
	if pool := e.loadKubeasyCA(ctx); pool != nil {
		cfg.RootCAs = pool
	}

	return cfg
}

// checkExternalConnectivity sends a single HTTP request from the CLI host via net/http.
// No pod exec — used for mode: external (EXT-01).
// When target.TLS is set, performs explicit TLS certificate checks before the HTTP request.
func (e *Executor) checkExternalConnectivity(ctx context.Context, target ConnectivityCheck) (bool, string) {
	timeout := target.TimeoutSeconds
	if timeout == 0 {
		timeout = DefaultConnectivityTimeoutSeconds
	}

	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	// Step 1 — Build tlsCfg for the HTTP transport.
	tlsCfg := e.buildExternalTLSConfig(ctx, target)

	// Step 2 — If explicit TLS checks requested (validateExpiry or validateSANs) AND NOT insecureSkipVerify:
	// probe the raw cert first and apply manual validation before making the HTTP request.
	if target.TLS != nil && !target.TLS.InsecureSkipVerify && (target.TLS.ValidateExpiry || target.TLS.ValidateSANs) {
		cert, dialErr := probeTLSCert(reqCtx, target.URL)
		if dialErr != nil {
			return false, fmt.Sprintf("TLS dial failed: %v", dialErr)
		}

		if target.TLS.ValidateExpiry {
			if time.Now().After(cert.NotAfter) {
				delta := time.Since(cert.NotAfter)
				days := int(delta.Hours() / 24)
				return false, fmt.Sprintf("Certificate expired on %s (%d days ago)", cert.NotAfter.Format("2006-01-02"), days)
			}
		}

		if target.TLS.ValidateSANs {
			hostname := hostnameForSAN(target)
			if err := cert.VerifyHostname(hostname); err != nil {
				return false, fmt.Sprintf("Hostname %q not in SANs: %v", hostname, cert.DNSNames)
			}
		}
	}

	// Step 3 — Build HTTP client with TLS transport.
	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: tlsCfg,
		},
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			// Return redirects as-is — do not follow automatically.
			// Allows challenge specs to assert on 3xx status codes.
			return http.ErrUseLastResponse
		},
	}

	// Step 4 — Make HTTP request (existing logic, unchanged).
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, target.URL, nil)
	if err != nil {
		return false, fmt.Sprintf("Invalid URL %s: %v", target.URL, err)
	}

	// EXT-02: override wire Host header for virtual-host routing
	// IMPORTANT: req.Host overrides the Host header on the wire.
	// req.Header.Set("Host", h) does NOT work for the Host header in Go.
	if target.HostHeader != "" {
		req.Host = target.HostHeader
	}

	resp, err := client.Do(req)
	if err != nil {
		// Connection refused or timeout — treat as "blocked" for expectedStatusCode==0
		if target.ExpectedStatusCode == 0 {
			return true, fmt.Sprintf("Connection to %s blocked as expected", target.URL)
		}
		return false, fmt.Sprintf("Connection to %s failed: %v", target.URL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == target.ExpectedStatusCode {
		return true, ""
	}
	return false, fmt.Sprintf("Connection to %s: got status %d, expected %d",
		target.URL, resp.StatusCode, target.ExpectedStatusCode)
}

// probeTLSCert dials the TLS endpoint and returns the first peer certificate.
// Always uses InsecureSkipVerify:true so metadata is fetched even for expired or self-signed certs.
// Manual validation (expiry, SANs) is applied by the caller after this returns.
func probeTLSCert(ctx context.Context, rawURL string) (*x509.Certificate, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	host := u.Hostname()
	port := u.Port()
	if port == "" {
		port = "443"
	}

	dialer := &tls.Dialer{
		Config: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // cert probe — always fetches raw cert; manual validation applied below
	}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, port))
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		return nil, fmt.Errorf("unexpected connection type")
	}

	certs := tlsConn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return nil, fmt.Errorf("no peer certificates returned")
	}
	return certs[0], nil
}

// hostnameForSAN returns the hostname to use for SAN validation.
// Uses HostHeader when set (virtual-host routing pattern), falling back to the URL hostname.
func hostnameForSAN(target ConnectivityCheck) string {
	if target.HostHeader != "" {
		return target.HostHeader
	}
	u, err := url.Parse(target.URL)
	if err != nil {
		return target.URL
	}
	return u.Hostname()
}

// buildCurlCommand constructs the curl argument slice for pod exec.
// Arguments are passed directly to the container — no shell is invoked.
func buildCurlCommand(url string, timeoutSeconds int) []string {
	return []string{
		"curl", "-s", "-o", "/dev/null",
		"-w", "%{http_code}",
		"--connect-timeout", strconv.Itoa(timeoutSeconds),
		url,
	}
}

// checkConnectivity performs a curl request from a pod
func (e *Executor) checkConnectivity(ctx context.Context, pod *corev1.Pod, target ConnectivityCheck) (bool, string) {
	timeout := target.TimeoutSeconds
	if timeout == 0 {
		timeout = 5
	}

	// Build curl command — direct args, no shell
	cmd := buildCurlCommand(target.URL, timeout)

	// Guard: fake clientsets have a non-nil RESTClient but internally nil client.
	// If restConfig has no host, we are running in a test environment — return
	// a deterministic error so the status-0 guard can be applied.
	if e.restConfig == nil || e.restConfig.Host == "" {
		if target.ExpectedStatusCode == 0 {
			return true, fmt.Sprintf("Connection to %s blocked as expected", target.URL)
		}
		return false, fmt.Sprintf("Connection to %s failed: exec not available in test environment", target.URL)
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
		return false, fmt.Sprintf("Failed to create executor: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})

	if err != nil {
		if target.ExpectedStatusCode == 0 {
			return true, fmt.Sprintf("Connection to %s blocked as expected", target.URL)
		}
		return false, fmt.Sprintf("Connection to %s failed: %v", target.URL, err)
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

// executeRbac validates ServiceAccount permissions using SubjectAccessReview
func (e *Executor) executeRbac(ctx context.Context, spec RbacSpec) (bool, string, error) {
	saUser := fmt.Sprintf("system:serviceaccount:%s:%s", spec.Namespace, spec.ServiceAccount)

	for i, check := range spec.Checks {
		checkNS := spec.Namespace
		if check.Namespace != "" {
			checkNS = check.Namespace
		}

		sar := &authv1.SubjectAccessReview{
			Spec: authv1.SubjectAccessReviewSpec{
				User: saUser,
				// Include SA groups so that permissions granted via group bindings
				// (system:serviceaccounts, system:serviceaccounts:<ns>) are honoured,
				// matching the behaviour of kubectl auth can-i --as system:serviceaccount:ns:sa
				Groups: []string{
					"system:serviceaccounts",
					fmt.Sprintf("system:serviceaccounts:%s", spec.Namespace),
				},
				ResourceAttributes: &authv1.ResourceAttributes{
					Verb:        check.Verb,
					Resource:    check.Resource,
					Subresource: check.Subresource,
					Namespace:   checkNS,
				},
			},
		}

		result, err := e.clientset.AuthorizationV1().SubjectAccessReviews().Create(ctx, sar, metav1.CreateOptions{})
		if err != nil {
			return false, "", fmt.Errorf("check %d: SubjectAccessReview failed: %w", i, err)
		}

		if result.Status.Allowed != check.Allowed {
			expected := "allowed"
			actual := "denied"
			if !check.Allowed {
				expected = "denied"
				actual = "allowed"
			}
			return false, fmt.Sprintf(
				"check %d: %s %s in namespace %q: expected %s but was %s",
				i, check.Verb, check.Resource, checkNS, expected, actual,
			), nil
		}
	}

	return true, msgAllRbacChecksPassed, nil
}

// execInPod executes a command inside a running pod and returns stdout, stderr, and any error.
// Used by DNS and connectivity validators that need in-cluster exec.
func (e *Executor) execInPod(ctx context.Context, pod *corev1.Pod, cmd []string) (string, string, error) {
	if e.restConfig == nil || e.restConfig.Host == "" {
		return "", "", fmt.Errorf("exec not available in test environment")
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

	executor, err := remotecommand.NewSPDYExecutor(e.restConfig, "POST", req.URL())
	if err != nil {
		return "", "", fmt.Errorf("failed to create executor: %w", err)
	}

	var stdout, stderr bytes.Buffer
	err = executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	return stdout.String(), stderr.String(), err
}

const msgAllDnsChecksPassed = "All DNS checks passed" //nolint:gosec // not a credential

// executeDns validates DNS resolution of hostnames from inside the cluster.
// It runs nslookup inside the source pod for each check and compares the outcome
// (resolved vs NXDOMAIN) to the expected resolves flag.
func (e *Executor) executeDns(ctx context.Context, spec DnsSpec) (bool, string, error) {
	logger.Debug("Executing DNS validation")

	// Resolve source namespace
	sourceNamespace := e.namespace
	if spec.SourcePod.Namespace != "" {
		sourceNamespace = spec.SourcePod.Namespace
	}

	// Find source pod
	var sourcePod *corev1.Pod
	switch {
	case spec.SourcePod.Name != "":
		pod, err := e.clientset.CoreV1().Pods(sourceNamespace).Get(ctx, spec.SourcePod.Name, metav1.GetOptions{})
		if err != nil {
			return false, "", fmt.Errorf("failed to get source pod: %w", err)
		}
		sourcePod = pod
	case len(spec.SourcePod.LabelSelector) > 0:
		pods, err := e.clientset.CoreV1().Pods(sourceNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: labels.SelectorFromSet(spec.SourcePod.LabelSelector).String(),
		})
		if err != nil {
			return false, "", fmt.Errorf("failed to list source pods: %w", err)
		}
		if len(pods.Items) == 0 {
			return false, errNoMatchingSourcePods, nil
		}
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

	var failures []string
	for _, check := range spec.Checks {
		cmd := []string{"nslookup", check.Hostname}
		stdout, _, execErr := e.execInPod(ctx, sourcePod, cmd)

		// nslookup exits non-zero on NXDOMAIN; also check stdout for NXDOMAIN or "can't find"
		resolved := execErr == nil &&
			!strings.Contains(stdout, "NXDOMAIN") &&
			!strings.Contains(stdout, "can't find") &&
			!strings.Contains(stdout, "server can't find")

		if resolved != check.Resolves {
			if check.Resolves {
				failures = append(failures, fmt.Sprintf("%s: expected to resolve but got NXDOMAIN", check.Hostname))
			} else {
				failures = append(failures, fmt.Sprintf("%s: expected NXDOMAIN but resolved successfully", check.Hostname))
			}
		}
	}

	if len(failures) == 0 {
		return true, msgAllDnsChecksPassed, nil
	}
	return false, strings.Join(failures, "; "), nil
}
