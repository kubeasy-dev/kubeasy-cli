package validation

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/executors/condition"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/executors/connectivity"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/executors/event"
	executorlog "github.com/kubeasy-dev/kubeasy-cli/internal/validation/executors/log"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/executors/rbac"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/executors/spec"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/executors/status"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/executors/triggered"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/shared"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/vtypes"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Executor executes validations against a Kubernetes cluster.
type Executor struct {
	deps    shared.Deps
	probeMu sync.Mutex // serializes probe-mode connectivity checks
}

// NewExecutor creates a new validation executor.
func NewExecutor(clientset kubernetes.Interface, dynamicClient dynamic.Interface, restConfig *rest.Config, namespace string) *Executor {
	e := &Executor{}
	e.deps = shared.Deps{
		Clientset:     clientset,
		DynamicClient: dynamicClient,
		RestConfig:    restConfig,
		Namespace:     namespace,
		ProbeMu:       &e.probeMu,
	}
	return e
}

// Execute runs a single validation and returns the result.
func (e *Executor) Execute(ctx context.Context, v vtypes.Validation) vtypes.Result {
	start := time.Now()
	result := vtypes.Result{
		Key:     v.Key,
		Passed:  false,
		Message: "Unknown validation type",
	}

	var (
		passed bool
		msg    string
		err    error
	)

	switch v.Type {
	case TypeStatus:
		s, ok := v.Spec.(vtypes.StatusSpec)
		if !ok {
			result.Message = fmt.Sprintf("internal error: expected StatusSpec, got %T", v.Spec)
			result.Duration = time.Since(start)
			return result
		}
		passed, msg, err = status.Execute(ctx, s, e.deps)

	case TypeCondition:
		s, ok := v.Spec.(vtypes.ConditionSpec)
		if !ok {
			result.Message = fmt.Sprintf("internal error: expected ConditionSpec, got %T", v.Spec)
			result.Duration = time.Since(start)
			return result
		}
		passed, msg, err = condition.Execute(ctx, s, e.deps)

	case TypeLog:
		s, ok := v.Spec.(vtypes.LogSpec)
		if !ok {
			result.Message = fmt.Sprintf("internal error: expected LogSpec, got %T", v.Spec)
			result.Duration = time.Since(start)
			return result
		}
		passed, msg, err = executorlog.Execute(ctx, s, e.deps)

	case TypeEvent:
		s, ok := v.Spec.(vtypes.EventSpec)
		if !ok {
			result.Message = fmt.Sprintf("internal error: expected EventSpec, got %T", v.Spec)
			result.Duration = time.Since(start)
			return result
		}
		passed, msg, err = event.Execute(ctx, s, e.deps)

	case TypeConnectivity:
		s, ok := v.Spec.(vtypes.ConnectivitySpec)
		if !ok {
			result.Message = fmt.Sprintf("internal error: expected ConnectivitySpec, got %T", v.Spec)
			result.Duration = time.Since(start)
			return result
		}
		passed, msg, err = connectivity.Execute(ctx, s, e.deps)

	case TypeRbac:
		s, ok := v.Spec.(vtypes.RbacSpec)
		if !ok {
			result.Message = fmt.Sprintf("internal error: expected RbacSpec, got %T", v.Spec)
			result.Duration = time.Since(start)
			return result
		}
		passed, msg, err = rbac.Execute(ctx, s, e.deps)

	case TypeSpec:
		s, ok := v.Spec.(vtypes.SpecSpec)
		if !ok {
			result.Message = fmt.Sprintf("internal error: expected SpecSpec, got %T", v.Spec)
			result.Duration = time.Since(start)
			return result
		}
		passed, msg, err = spec.Execute(ctx, s, e.deps)

	case TypeTriggered:
		s, ok := v.Spec.(vtypes.TriggeredSpec)
		if !ok {
			result.Message = fmt.Sprintf("internal error: expected TriggeredSpec, got %T", v.Spec)
			result.Duration = time.Since(start)
			return result
		}
		passed, msg, err = triggered.Execute(ctx, s, e.deps, e.Execute)

	default:
		result.Message = fmt.Sprintf("Unknown validation type: %s", v.Type)
		result.Duration = time.Since(start)
		return result
	}

	if err != nil {
		result.Passed = false
		result.Message = err.Error()
	} else {
		result.Passed = passed
		result.Message = msg
	}

	result.Duration = time.Since(start)
	return result
}

// ExecuteAll runs all validations in parallel and returns results in input order.
func (e *Executor) ExecuteAll(ctx context.Context, validations []vtypes.Validation) []vtypes.Result {
	results := make([]vtypes.Result, len(validations))
	var wg sync.WaitGroup

	for i, v := range validations {
		wg.Add(1)
		go func(idx int, val vtypes.Validation) {
			defer wg.Done()
			results[idx] = e.Execute(ctx, val)
		}(i, v)
	}

	wg.Wait()
	return results
}

// ExecuteSequential runs validations one by one.
// If failFast is true, it stops at the first failure.
func (e *Executor) ExecuteSequential(ctx context.Context, validations []vtypes.Validation, failFast bool) []vtypes.Result {
	var results []vtypes.Result
	for _, v := range validations {
		result := e.Execute(ctx, v)
		results = append(results, result)
		if failFast && !result.Passed {
			break
		}
	}
	return results
}
