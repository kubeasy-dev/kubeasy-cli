# Migration Guide: CRD-Based to CLI-Based Validation System

## Overview

As of **February 2025** (commit `03d1a85`), Kubeasy has transitioned from a Kubernetes operator-based validation system to a CLI-based validation executor. This migration simplifies the architecture, reduces dependencies, and makes the system easier to understand and maintain.

This guide explains the changes, their impact, and how to migrate existing challenges.

---

## What Changed?

### Before: Operator-Based Validation (≤ v1.3.0)

```
┌─────────────────────────────────────────────────────────────┐
│ 1. Challenge started                                        │
│    → ArgoCD deploys manifests to cluster                   │
│    → Manifests include validation CRD instances             │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ 2. Operator reconciles CRDs                                 │
│    → Reads LogValidation, StatusValidation, etc.            │
│    → Executes checks against cluster                        │
│    → Updates CRD .status.allPassed field                    │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ 3. User submits solution                                    │
│    → CLI reads CRD status fields from cluster               │
│    → Sends grouped results to backend                       │
└─────────────────────────────────────────────────────────────┘
```

### After: CLI-Based Validation (≥ v1.4.0)

```
┌─────────────────────────────────────────────────────────────┐
│ 1. Challenge started                                        │
│    → ArgoCD deploys manifests (no CRDs)                     │
│    → Validation definitions in challenge.yaml               │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ 2. User submits solution                                    │
│    → CLI loads validations from challenge.yaml              │
│    → Executes checks directly against cluster               │
│    → Sends flat results to backend                          │
└─────────────────────────────────────────────────────────────┘
```

---

## Key Differences

| Aspect | Old (Operator-Based) | New (CLI-Based) |
|--------|---------------------|-----------------|
| **Validation Source** | CRD instances in cluster | `challenge.yaml` in challenges repo |
| **Execution** | Operator reconciliation loop | Direct CLI execution on submit |
| **Dependencies** | `challenge-operator` package | None (self-contained) |
| **Setup** | 4 components (ArgoCD, Kyverno, Operator, CLI) | 3 components (ArgoCD, Kyverno, CLI) |
| **CRD Types** | 6 types (LogValidation, StatusValidation, EventValidation, MetricsValidation, RBACValidation, ConnectivityValidation) | 0 CRDs |
| **Validation Types** | 6 types | 5 types (RBAC removed for security) |
| **API Payload** | Grouped by type with raw CRD status | Flat list of ObjectiveResult |
| **Execution Time** | Async (operator reconciliation) | Synchronous (on submit) |

---

## Timeline

- **v1.3.0 and earlier**: Operator-based system
- **v1.4.0 (commit 03d1a85)**: CLI-based system introduced
- **Future**: Operator completely deprecated

---

## Impact Assessment

### ✅ What Still Works

1. **User Workflow**: No changes to CLI commands
   ```bash
   kubeasy setup
   kubeasy challenge start <slug>
   # ... work on challenge
   kubeasy challenge submit <slug>
   ```

2. **Backend API**: Still accepts submissions at `/api/cli/challenge/[slug]/submit`

3. **Challenge Structure**: Manifests, policies, and images still work the same way

4. **Local Development**: Local `challenge.yaml` detection still works

5. **Existing Installations**: Old operator can remain installed (safely ignored)

### ⚠️ What Changed

1. **No CRD Instances**: Challenges no longer generate CRD resources in the cluster

2. **Validation Definitions**: Must be in `challenge.yaml` instead of separate CRD manifests

3. **RBAC Validation Removed**: For security reasons, `RBACValidation` type is no longer supported

4. **Submission Timing**: Validations run on submit, not continuously via operator

5. **Setup Speed**: Faster setup (one fewer component to deploy and wait for)

### ❌ What Breaks

1. **Direct CRD Inspection**: Can no longer `kubectl get logvalidation` to see status

2. **CRD-Based Tooling**: Any external tools reading validation CRDs will fail

3. **Operator Lifecycle Management**: No operator to manage anymore

---

## Migration Steps for Challenge Authors

### Step 1: Convert CRD Manifests to challenge.yaml

**Old Format** (separate files in `manifests/validations/`):

```yaml
# manifests/validations/pod-ready.yaml
apiVersion: challenge.kubeasy.dev/v1alpha1
kind: StatusValidation
metadata:
  name: pod-ready-check
  namespace: challenge-pod-evicted
spec:
  target:
    kind: Pod
    labelSelector:
      app: data-processor
  conditions:
    - type: Ready
      status: "True"
```

**New Format** (in `challenge.yaml`):

```yaml
# challenge.yaml
title: Pod Evicted
description: |
  A data processing pod keeps crashing...
# ... other metadata

validations:
  - key: pod-ready-check
    title: "Pod Ready"
    description: "The pod must be running and ready"
    order: 1
    type: status
    spec:
      target:
        kind: Pod
        labelSelector:
          app: data-processor
      conditions:
        - type: Ready
          status: "True"
```

### Step 2: Update Validation Types

Map old CRD types to new validation types:

| Old CRD Kind | New Type | Notes |
|-------------|----------|-------|
| `LogValidation` | `log` | Unchanged behavior |
| `StatusValidation` | `status` | Unchanged behavior |
| `EventValidation` | `event` | Unchanged behavior |
| `MetricsValidation` | `metrics` | Unchanged behavior |
| `ConnectivityValidation` | `connectivity` | Unchanged behavior |
| `RBACValidation` | ❌ Removed | Use Kyverno policies instead |

### Step 3: Remove CRD Manifests

Delete the following directories/files:
- `manifests/validations/` (entire directory)
- Any references to CRD instances in ArgoCD applications

### Step 4: Test Locally

```bash
# Ensure your challenge.yaml is in the correct location
cd challenges/your-challenge/

# Test validation loading
kubeasy-cli challenge start your-challenge

# Make the required fixes
kubectl edit deployment your-app

# Test validation execution
kubeasy-cli challenge submit your-challenge
```

---

## Complete Example: Before and After

### Before (Operator-Based)

**Directory Structure**:
```
pod-evicted/
├── challenge.yaml           # Metadata only
├── manifests/
│   ├── deployment.yaml
│   └── validations/
│       ├── pod-ready.yaml   # StatusValidation CRD
│       └── no-oom.yaml      # EventValidation CRD
└── policies/
    └── protect.yaml
```

**challenge.yaml** (old):
```yaml
title: Pod Evicted
description: A pod keeps getting evicted
difficulty: easy
# No validations here
```

**manifests/validations/pod-ready.yaml**:
```yaml
apiVersion: challenge.kubeasy.dev/v1alpha1
kind: StatusValidation
metadata:
  name: pod-ready-check
spec:
  target:
    kind: Pod
    labelSelector: {app: data-processor}
  conditions:
    - type: Ready
      status: "True"
```

**manifests/validations/no-oom.yaml**:
```yaml
apiVersion: challenge.kubeasy.dev/v1alpha1
kind: EventValidation
metadata:
  name: no-oom-check
spec:
  target:
    kind: Pod
    labelSelector: {app: data-processor}
  forbiddenReasons:
    - OOMKilled
  sinceSeconds: 300
```

---

### After (CLI-Based)

**Directory Structure**:
```
pod-evicted/
├── challenge.yaml           # Metadata + validations
├── manifests/
│   └── deployment.yaml      # No validations/ subdirectory
└── policies/
    └── protect.yaml
```

**challenge.yaml** (new):
```yaml
title: Pod Evicted
description: A data processing pod keeps crashing and getting evicted
theme: resources-scaling
difficulty: easy
estimated_time: 15
initial_situation: |
  A data processing application is deployed as a single pod.
  The pod starts but gets killed after a few seconds.
objective: |
  Make the pod run stably without being evicted.

validations:
  - key: pod-ready-check
    title: "Pod Ready"
    description: "The application pod must be running and ready"
    order: 1
    type: status
    spec:
      target:
        kind: Pod
        labelSelector:
          app: data-processor
      conditions:
        - type: Ready
          status: "True"

  - key: no-oom-check
    title: "Stable Operation"
    description: "The pod must not have any OOMKilled events"
    order: 2
    type: event
    spec:
      target:
        kind: Pod
        labelSelector:
          app: data-processor
      forbiddenReasons:
        - OOMKilled
        - Evicted
      sinceSeconds: 300
```

---

## Validation Type Reference

### 1. Status Validation

Checks Kubernetes resource conditions (e.g., Pod Ready, Deployment Available).

```yaml
- key: pod-ready
  title: "Pod Ready"
  type: status
  spec:
    target:
      kind: Pod
      labelSelector: {app: my-app}
    conditions:
      - type: Ready
        status: "True"
```

### 2. Log Validation

Searches container logs for expected strings.

```yaml
- key: logs-check
  title: "Application Started"
  type: log
  spec:
    target:
      kind: Pod
      labelSelector: {app: my-app}
    container: app  # Optional, defaults to first container
    expectedStrings:
      - "Server listening on port 8080"
      - "Database connection established"
    sinceSeconds: 300  # Optional, default: 300 (5 minutes)
```

### 3. Event Validation

Detects forbidden Kubernetes events (e.g., OOMKilled, Evicted).

```yaml
- key: no-crashes
  title: "No Crashes"
  type: event
  spec:
    target:
      kind: Pod
      labelSelector: {app: my-app}
    forbiddenReasons:
      - OOMKilled
      - Evicted
      - BackOff
    sinceSeconds: 300  # Optional, default: 300 (5 minutes)
```

### 4. Metrics Validation

Validates pod/deployment numeric fields (e.g., replicas, restart count).

```yaml
- key: scaled-replicas
  title: "Correct Replica Count"
  type: metrics
  spec:
    target:
      kind: Deployment
      name: my-app
    checks:
      - field: status.readyReplicas
        operator: ">="
        value: 3
      - field: status.availableReplicas
        operator: "=="
        value: 3
```

### 5. Connectivity Validation

Tests HTTP connectivity between pods.

```yaml
- key: service-reachable
  title: "Service Reachable"
  type: connectivity
  spec:
    sourcePod:
      labelSelector: {app: client}
    targets:
      - url: http://backend-service:8080/health
        expectedStatusCode: 200
        timeoutSeconds: 5  # Optional, default: 5
```

---

## RBAC Validation Removed

The `RBACValidation` CRD type has been **intentionally removed** for security reasons:

**Why?**
- `SubjectAccessReview` checks could grant unintended permissions if misconfigured
- Reduces blast radius of validation errors
- Safer to enforce permissions via Kyverno policies

**Migration Path**:
If your challenge required RBAC validation, use Kyverno policies instead:

```yaml
# policies/rbac.yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: require-specific-permissions
spec:
  validationFailureAction: Enforce
  rules:
    - name: check-service-account-permissions
      match:
        any:
          - resources:
              kinds:
                - Pod
              namespaces:
                - challenge-*
      validate:
        message: "Pod must use authorized ServiceAccount"
        pattern:
          spec:
            serviceAccountName: "allowed-sa"
```

---

## Troubleshooting

### Issue: "No matching pods found" error

**Cause**: The validation can't find pods matching the target selector.

**Fix**:
```yaml
spec:
  target:
    kind: Pod
    labelSelector:
      app: your-app  # Ensure this matches your pod labels
```

Verify with:
```bash
kubectl get pods -l app=your-app -n challenge-<slug>
```

---

### Issue: "Invalid response from URL" in connectivity check

**Cause**: The HTTP response isn't a valid status code.

**Fix**: Ensure the target service responds with proper HTTP codes:
```yaml
spec:
  sourcePod:
    labelSelector: {app: client}
  targets:
    - url: http://service:8080/health  # Must return HTTP status code
      expectedStatusCode: 200
```

---

### Issue: "Missing strings in logs" with log errors

**Cause**: Container doesn't exist or logs aren't available.

**Fix**:
```yaml
spec:
  target:
    kind: Pod
    labelSelector: {app: my-app}
  container: main-app  # Specify exact container name
  expectedStrings:
    - "Expected log message"
```

---

## FAQ

### Q: Can I still use the operator?

**A**: The operator is no longer maintained or required. It will be ignored if present but won't be used for validation.

### Q: Do I need to update existing challenge installations?

**A**: No. Existing cluster installations are unaffected. The CLI will automatically use the new validation system when you update the CLI binary.

### Q: Will old CRDs in my cluster cause problems?

**A**: No. Old CRD instances are harmlessly ignored. You can safely delete them:
```bash
kubectl delete logvalidations,statusvalidations,eventvalidations,metricsvalidations,rbacvalidations,connectivityvalidations --all -n challenge-<slug>
```

### Q: How do I test validations locally during development?

**A**: Place `challenge.yaml` in a standard location:
```
~/Workspace/kubeasy/challenges/<slug>/challenge.yaml
```

The CLI will automatically detect and use it.

### Q: What happens if validation times out?

**A**: Currently, validations inherit the command's context timeout. Individual checks have built-in timeouts:
- Connectivity: 5 seconds (configurable)
- Logs: No timeout (reads available logs)
- Metrics: API client default (~30s)

---

## Getting Help

If you encounter issues during migration:

1. **Check challenge.yaml syntax**: Run `kubeasy challenge start <slug>` and check for parsing errors
2. **Enable debug logging**: `kubeasy challenge submit <slug> --debug`
3. **Report issues**: https://github.com/kubeasy-dev/kubeasy-cli/issues

---

## Summary

The migration from operator-based to CLI-based validation:
- ✅ **Simplifies** the architecture (fewer moving parts)
- ✅ **Speeds up** setup (one fewer component)
- ✅ **Improves** maintainability (validation logic in one place)
- ✅ **Preserves** user workflow (commands unchanged)
- ✅ **Maintains** backward compatibility (old clusters safe)

**Action Required**: Update `challenge.yaml` to include validation definitions. Remove CRD manifest files.

For complete examples, see [VALIDATION_EXAMPLES.md](./docs/VALIDATION_EXAMPLES.md).
