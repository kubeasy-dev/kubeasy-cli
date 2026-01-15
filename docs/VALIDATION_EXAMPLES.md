# Validation Examples

This document provides comprehensive examples of all validation types supported by the Kubeasy CLI validation system.

## Table of Contents

1. [Condition Validation](#condition-validation)
2. [Status Validation](#status-validation)
3. [Advanced Field Path Syntax](#advanced-field-path-syntax)
4. [Log Validation](#log-validation)
5. [Event Validation](#event-validation)
6. [Connectivity Validation](#connectivity-validation)
7. [Complete Challenge Example](#complete-challenge-example)
8. [Metrics Validation (REMOVED)](#metrics-validation-removed)
9. [Best Practices](#best-practices)
10. [Troubleshooting](#troubleshooting)
11. [Reference](#reference)

---

## Condition Validation

Checks Kubernetes resource conditions (e.g., Pod Ready, ContainersReady). This is a shorthand for common condition checks.

### Basic Pod Ready Check

```yaml
validations:
  - key: pod-ready
    title: "Pod Ready"
    description: "The application pod must be in Ready state"
    order: 1
    type: condition
    spec:
      target:
        kind: Pod
        labelSelector:
          app: my-application
      checks:
        - type: Ready
          status: "True"
```

**When to use**: Verify that pods are running and passing readiness probes.

**What it checks**:
- Finds all pods matching the label selector
- Checks if each pod has a `Ready` condition with status `True`
- All matching pods must meet the condition

---

### Multiple Conditions

```yaml
validations:
  - key: pod-healthy
    title: "Pod Healthy"
    description: "Pod must be both Ready and Initialized"
    order: 1
    type: condition
    spec:
      target:
        kind: Pod
        labelSelector:
          app: database
      checks:
        - type: Ready
          status: "True"
        - type: Initialized
          status: "True"
```

**When to use**: Verify multiple aspects of pod health.

---

### Common Condition Types

| Resource | Condition Types |
|----------|-----------------|
| Pod | `Ready`, `ContainersReady`, `Initialized`, `PodScheduled` |
| Deployment | `Available`, `Progressing`, `ReplicaFailure` |
| StatefulSet | `Ready` |
| Job | `Complete`, `Failed` |

---

## Status Validation

Validates arbitrary status fields using operators. Use this for numeric comparisons, string values, or any status field access.

### Replica Count

```yaml
validations:
  - key: scaled-replicas
    title: "Scaled to 3 Replicas"
    description: "Deployment must have exactly 3 ready replicas"
    order: 1
    type: status
    spec:
      target:
        kind: Deployment
        name: web-app
      checks:
        - field: readyReplicas
          operator: "=="
          value: 3
        - field: availableReplicas
          operator: ">="
          value: 3
```

**When to use**: Verify horizontal scaling has been applied.

**Available operators**: `==`, `!=`, `>`, `<`, `>=`, `<=`

**Note**: Field paths are relative to `status` (no prefix needed).

---

### Restart Count with Array Access

```yaml
validations:
  - key: low-restarts
    title: "Low Restart Count"
    description: "Pod must have fewer than 3 restarts"
    order: 1
    type: status
    spec:
      target:
        kind: Pod
        labelSelector:
          app: stable-app
      checks:
        - field: containerStatuses[0].restartCount
          operator: "<"
          value: 3
```

**When to use**: Verify pod stability over time.

**Field path syntax**:
- Simple field: `readyReplicas`
- Array index: `containerStatuses[0].restartCount`
- Array filter: `conditions[type=Ready].status`

---

### Condition via Status (Advanced)

```yaml
validations:
  - key: deployment-available
    title: "Deployment Available"
    description: "The deployment must be available"
    order: 1
    type: status
    spec:
      target:
        kind: Deployment
        name: web-app
      checks:
        - field: conditions[type=Available].status
          operator: "=="
          value: "True"
```

**When to use**: Check conditions on any resource type using the flexible status validation.

**Tip**: For simple condition checks, prefer the `condition` type. Use `status` type when you need operators or complex field paths.

---

### StatefulSet Replicas

```yaml
validations:
  - key: statefulset-ready
    title: "StatefulSet Ready"
    description: "All StatefulSet replicas must be ready"
    order: 1
    type: status
    spec:
      target:
        kind: StatefulSet
        name: database
      checks:
        - field: readyReplicas
          operator: "=="
          value: 3
        - field: currentReplicas
          operator: "=="
          value: 3
```

**When to use**: Verify stateful applications are fully deployed.

---

### Boolean and String Fields

```yaml
validations:
  - key: phase-running
    title: "Pod Running"
    description: "Pod must be in Running phase"
    order: 1
    type: status
    spec:
      target:
        kind: Pod
        name: my-pod
      checks:
        - field: phase
          operator: "=="
          value: "Running"
```

**Supported value types**: string, integer, boolean, float

---

## Advanced Field Path Syntax

The `status` validation type supports advanced field path syntax for accessing nested fields, arrays, and filtering.

### Simple Field Access

Access direct fields on the status object:

```yaml
checks:
  - field: phase
    operator: "=="
    value: "Running"
  - field: readyReplicas
    operator: ">="
    value: 3
```

**Note**: Field paths are relative to `status` — no prefix needed. Internally, the system automatically prepends `status.` to your field path.

---

### Nested Field Access

Access fields within nested objects using dot notation:

```yaml
checks:
  - field: loadBalancer.ingress[0].hostname
    operator: "!="
    value: ""
```

---

### Array Index Access

Access specific array elements using `[index]` notation:

```yaml
# Access first container's restart count
checks:
  - field: containerStatuses[0].restartCount
    operator: "<"
    value: 5

# Access second container's ready state
  - field: containerStatuses[1].ready
    operator: "=="
    value: true
```

**Bounds checking**: The system validates array bounds at runtime and returns a clear error if the index is out of range.

---

### Array Filtering

Filter arrays by field value using `[field=value]` notation:

```yaml
# Find condition by type and check its status
checks:
  - field: conditions[type=Ready].status
    operator: "=="
    value: "True"

# Check Available condition
  - field: conditions[type=Available].status
    operator: "=="
    value: "True"

# Check Progressing condition reason
  - field: conditions[type=Progressing].reason
    operator: "=="
    value: "NewReplicaSetAvailable"
```

**When to use**: Array filtering is useful when the array order is not guaranteed, which is common for Kubernetes conditions.

---

### Complex Path Examples

Combining multiple syntax features:

```yaml
validations:
  - key: complex-check
    title: "Complex Status Check"
    type: status
    spec:
      target:
        kind: Pod
        name: my-pod
      checks:
        # First container must be ready
        - field: containerStatuses[0].ready
          operator: "=="
          value: true

        # Ready condition must be True
        - field: conditions[type=Ready].status
          operator: "=="
          value: "True"

        # No restarts on first container
        - field: containerStatuses[0].restartCount
          operator: "=="
          value: 0

        # Pod must be in Running phase
        - field: phase
          operator: "=="
          value: "Running"
```

---

### Supported Value Types

| Type | YAML Example | Operators |
|------|--------------|-----------|
| String | `value: "Running"` | `==`, `!=` |
| Integer | `value: 3` | `==`, `!=`, `>`, `<`, `>=`, `<=` |
| Boolean | `value: true` | `==`, `!=` |
| Float | `value: 0.95` | `==`, `!=`, `>`, `<`, `>=`, `<=` |

**Type coercion**: Integer and float values are coerced for comparison (e.g., `3` equals `3.0`).

---

### Available Operators

| Operator | Description | Supported Types |
|----------|-------------|-----------------|
| `==` | Equal | All types |
| `!=` | Not equal | All types |
| `>` | Greater than | Integer, Float |
| `<` | Less than | Integer, Float |
| `>=` | Greater than or equal | Integer, Float |
| `<=` | Less than or equal | Integer, Float |

---

### Field Validation Errors

Field paths are validated at parse time using Go reflection. If a field doesn't exist, you'll get a helpful error message:

```
Error: check 0: field "readyReplica" not found in DeploymentStatus
  Available fields: availableReplicas, collisionCount, conditions, observedGeneration, readyReplicas, replicas, unavailableReplicas, updatedReplicas
```

**Common mistakes**:
- Typo in field name: `readyReplica` instead of `readyReplicas`
- Wrong case: `ReadyReplicas` instead of `readyReplicas`
- Including `status.` prefix: `status.readyReplicas` instead of `readyReplicas`

---

## Log Validation

Searches container logs for expected strings.

### Basic Log Search

```yaml
validations:
  - key: app-started
    title: "Application Started"
    description: "Application must log startup message"
    order: 1
    type: log
    spec:
      target:
        kind: Pod
        labelSelector:
          app: web-server
      expectedStrings:
        - "Server listening on port 8080"
```

**When to use**: Verify that an application has started successfully.

**What it checks**:
- Fetches logs from all matching pods (last 5 minutes by default)
- Searches for each expected string in logs
- All expected strings must be found in at least one pod's logs

---

### Specific Container Logs

```yaml
validations:
  - key: sidecar-logs
    title: "Sidecar Running"
    description: "Sidecar container must be operational"
    order: 1
    type: log
    spec:
      target:
        kind: Pod
        labelSelector:
          app: multi-container-app
      container: logging-sidecar  # Specify exact container
      expectedStrings:
        - "Log forwarder initialized"
        - "Connected to log aggregator"
```

**When to use**: Check logs from a specific container in a multi-container pod.

---

### Custom Time Window

```yaml
validations:
  - key: recent-activity
    title: "Recent Activity"
    description: "Application must have logged activity in last minute"
    order: 1
    type: log
    spec:
      target:
        kind: Pod
        labelSelector:
          app: worker
      expectedStrings:
        - "Processing job"
      sinceSeconds: 60  # Only check last 60 seconds of logs
```

**When to use**: Verify recent activity or recent configuration changes.

**Default**: `sinceSeconds: 300` (5 minutes)

---

### Database Connection Check

```yaml
validations:
  - key: db-connected
    title: "Database Connected"
    description: "Application must successfully connect to database"
    order: 1
    type: log
    spec:
      target:
        kind: Pod
        labelSelector:
          app: api-server
      expectedStrings:
        - "Database connection established"
        - "Running migrations"
        - "Migration complete"
```

**When to use**: Verify successful database initialization.

---

## Event Validation

Detects forbidden Kubernetes events (e.g., OOMKilled, Evicted, BackOff).

### OOMKilled Detection

```yaml
validations:
  - key: no-oom
    title: "No OOM Kills"
    description: "Pod must not be killed due to out of memory"
    order: 1
    type: event
    spec:
      target:
        kind: Pod
        labelSelector:
          app: memory-intensive
      forbiddenReasons:
        - OOMKilled
      sinceSeconds: 300
```

**When to use**: Verify that pods have sufficient memory configured.

**What it checks**:
- Lists all events in the namespace
- Filters events for matching pods
- Checks if any events have forbidden reasons
- Only considers events within the time window

---

### Eviction and Scheduling Failures

```yaml
validations:
  - key: pod-stability
    title: "Pod Stability"
    description: "Pod must not be evicted or fail to schedule"
    order: 1
    type: event
    spec:
      target:
        kind: Pod
        labelSelector:
          app: critical-service
      forbiddenReasons:
        - Evicted
        - FailedScheduling
        - FailedMount
      sinceSeconds: 600  # Check last 10 minutes
```

**When to use**: Verify pod stability and resource availability.

---

### Crash Loop Detection

```yaml
validations:
  - key: no-crashes
    title: "No Crash Loops"
    description: "Pod must not be in crash loop backoff"
    order: 1
    type: event
    spec:
      target:
        kind: Pod
        labelSelector:
          app: unstable-app
      forbiddenReasons:
        - BackOff
        - CrashLoopBackOff
```

**When to use**: Verify that application starts successfully without crashes.

---

### Image Pull Failures

```yaml
validations:
  - key: image-pull-success
    title: "Image Pull Success"
    description: "Pod must successfully pull container images"
    order: 1
    type: event
    spec:
      target:
        kind: Pod
        labelSelector:
          app: new-deployment
      forbiddenReasons:
        - Failed
        - ErrImagePull
        - ImagePullBackOff
```

**When to use**: Verify that container images are accessible.

---

## Connectivity Validation

Tests HTTP connectivity between pods.

### Basic Service Connectivity

```yaml
validations:
  - key: service-reachable
    title: "Backend Service Reachable"
    description: "Frontend can reach backend service"
    order: 1
    type: connectivity
    spec:
      sourcePod:
        labelSelector:
          app: frontend
      targets:
        - url: http://backend-service:8080/health
          expectedStatusCode: 200
```

**When to use**: Verify network connectivity and service discovery.

**What it checks**:
- Finds a running pod matching sourcePod selector
- Executes `curl` (or `wget` fallback) from that pod
- Checks HTTP status code matches expected value

---

### Multiple Endpoints

```yaml
validations:
  - key: all-services-reachable
    title: "All Services Reachable"
    description: "Application can reach all dependent services"
    order: 1
    type: connectivity
    spec:
      sourcePod:
        name: app-pod-12345  # Specific pod name
      targets:
        - url: http://database-service:5432
          expectedStatusCode: 200
        - url: http://cache-service:6379
          expectedStatusCode: 200
        - url: http://api-gateway:80/health
          expectedStatusCode: 200
```

**When to use**: Verify all service dependencies are accessible.

---

### Custom Timeout

```yaml
validations:
  - key: slow-service
    title: "Slow Service Responds"
    description: "Service responds within 10 seconds"
    order: 1
    type: connectivity
    spec:
      sourcePod:
        labelSelector:
          app: client
      targets:
        - url: http://slow-service:8080/heavy-operation
          expectedStatusCode: 200
          timeoutSeconds: 10  # Custom timeout
```

**When to use**: Test connectivity to slower services.

**Default**: `timeoutSeconds: 5`

---

### External Connectivity

```yaml
validations:
  - key: internet-access
    title: "Internet Access"
    description: "Pod can reach external services"
    order: 1
    type: connectivity
    spec:
      sourcePod:
        labelSelector:
          app: worker
      targets:
        - url: https://api.github.com
          expectedStatusCode: 200
```

**When to use**: Verify egress network policies allow external access.

**Note**: Requires pods to have `curl` or `wget` available.

---

### Cross-Namespace Communication

```yaml
validations:
  - key: cross-namespace
    title: "Cross-Namespace Access"
    description: "App can reach service in another namespace"
    order: 1
    type: connectivity
    spec:
      sourcePod:
        labelSelector:
          app: frontend
      targets:
        - url: http://backend.production.svc.cluster.local:8080
          expectedStatusCode: 200
```

**When to use**: Verify multi-namespace service communication.

---

## Complete Challenge Example

Here's a complete `challenge.yaml` with multiple validation types:

```yaml
title: Microservices Deployment
description: |
  Deploy and configure a microservices application with proper resource limits,
  scaling, and network connectivity.

theme: microservices
difficulty: medium
estimated_time: 30

initial_situation: |
  You have three microservices: frontend, backend, and database.
  The deployment is failing due to configuration issues.

objective: |
  1. Fix resource limits to prevent OOM kills
  2. Scale backend to 3 replicas
  3. Ensure all services can communicate
  4. Verify application startup

objectives:
  # 1. Resource Limits Fixed
  - key: no-oom-kills
    title: "No Memory Issues"
    description: "Pods must not be killed due to out of memory"
    order: 1
    type: event
    spec:
      target:
        kind: Pod
        labelSelector:
          tier: backend
      forbiddenReasons:
        - OOMKilled
        - Evicted
      sinceSeconds: 300

  # 2. Scaling Applied
  - key: backend-scaled
    title: "Backend Scaled"
    description: "Backend must have 3 ready replicas"
    order: 2
    type: status
    spec:
      target:
        kind: Deployment
        name: backend
      checks:
        - field: readyReplicas
          operator: "=="
          value: 3
        - field: availableReplicas
          operator: ">="
          value: 3

  # 3. All Pods Ready
  - key: all-pods-ready
    title: "All Pods Ready"
    description: "Frontend, backend, and database pods must be ready"
    order: 3
    type: condition
    spec:
      target:
        kind: Pod
        labelSelector:
          app: microservices
      checks:
        - type: Ready
          status: "True"

  # 4. Application Started
  - key: app-started
    title: "Application Started"
    description: "Backend must log successful startup"
    order: 4
    type: log
    spec:
      target:
        kind: Pod
        labelSelector:
          tier: backend
      expectedStrings:
        - "Server listening on port 8080"
        - "Database connection established"
      sinceSeconds: 300

  # 5. Database Connectivity
  - key: db-connection
    title: "Database Reachable"
    description: "Backend can connect to database"
    order: 5
    type: connectivity
    spec:
      sourcePod:
        labelSelector:
          tier: backend
      targets:
        - url: http://database-service:5432
          expectedStatusCode: 200
          timeoutSeconds: 5

  # 6. Frontend to Backend
  - key: frontend-backend
    title: "Frontend to Backend"
    description: "Frontend can reach backend API"
    order: 6
    type: connectivity
    spec:
      sourcePod:
        labelSelector:
          tier: frontend
      targets:
        - url: http://backend-service:8080/api/health
          expectedStatusCode: 200

  # 7. No Crash Loops
  - key: no-crashes
    title: "No Crash Loops"
    description: "No pods should be in crash loop"
    order: 7
    type: event
    spec:
      target:
        kind: Pod
        labelSelector:
          app: microservices
      forbiddenReasons:
        - BackOff
        - CrashLoopBackOff
      sinceSeconds: 600
```

---

## Metrics Validation (REMOVED)

**Breaking Change**: The `metrics` validation type has been removed in v2.0.0.

### Why Was It Removed?

The `metrics` type was redundant with the enhanced `status` type:
- Both validated status fields with operators
- The `status` type now supports all the same functionality
- Removing `metrics` simplifies the codebase and reduces confusion

### Migration Guide

Migrate from `metrics` to `status` by:
1. Change `type: metrics` to `type: status`
2. **Remove** the `status.` prefix from field paths
3. Keep `checks` array unchanged (same structure)

**Before (v1.x):**

```yaml
type: metrics
spec:
  target:
    kind: Deployment
    name: web-app
  checks:
    - field: status.readyReplicas
      operator: ">="
      value: 3
    - field: status.availableReplicas
      operator: "=="
      value: 3
```

**After (v2.0+):**

```yaml
type: status
spec:
  target:
    kind: Deployment
    name: web-app
  checks:
    - field: readyReplicas      # No "status." prefix!
      operator: ">="
      value: 3
    - field: availableReplicas  # No "status." prefix!
      operator: "=="
      value: 3
```

### Migration Checklist

- [ ] Find all `type: metrics` in your challenge.yaml files
- [ ] Replace `type: metrics` with `type: status`
- [ ] Remove `status.` prefix from all field paths
- [ ] Test your validations with `kubeasy challenge submit`

### Automated Migration

You can use this sed command to help migrate:

```bash
# Replace type: metrics with type: status
sed -i 's/type: metrics/type: status/g' challenge.yaml

# Remove status. prefix from field paths (manual review recommended)
sed -i 's/field: status\./field: /g' challenge.yaml
```

**Note**: Always review changes manually after automated migration.

---

## Best Practices

### 1. Choosing Between Condition and Status

| Use `condition` when | Use `status` when |
|---------------------|-------------------|
| Checking standard K8s conditions | Checking numeric fields (replicas, restarts) |
| Simple Ready/Available checks | Using operators (>, <, >=, <=) |
| Targeting Pods directly | Accessing nested fields with array syntax |

### 2. Ordering Validations

Order validations from most basic to most complex:
```yaml
objectives:
  - order: 1  # Basic: Pods exist and are ready
    type: condition

  - order: 2  # Scaling: Correct replica count
    type: status

  - order: 3  # Application: Logs show startup
    type: log

  - order: 4  # Stability: No crash events
    type: event

  - order: 5  # Advanced: Network connectivity
    type: connectivity
```

### 3. Meaningful Titles

Use titles that describe success state, not the check:
- ✅ Good: "Pod Ready", "Database Connected"
- ❌ Bad: "Check Pod Status", "Validate Database"

### 4. Clear Descriptions

Explain what the user should achieve:
```yaml
description: "The application pod must be running and ready to accept traffic"
```

Not what the validation does:
```yaml
description: "Checks if pod has Ready condition set to True"
```

### 5. Label Selectors vs Names

Prefer label selectors for flexibility:
```yaml
# ✅ Good: Works with any matching pod
target:
  kind: Pod
  labelSelector:
    app: my-app

# ⚠️ Less flexible: Requires exact name
target:
  kind: Pod
  name: my-app-abc123
```

### 6. Time Windows

Adjust time windows based on application behavior:
- Fast-starting apps: `sinceSeconds: 60`
- Slow-starting apps: `sinceSeconds: 600`
- Default: `sinceSeconds: 300` (5 minutes)

---

## Troubleshooting

### Common Issues

**"No matching pods found"**
- Check label selectors match pod labels
- Verify namespace is correct
- Ensure pods are created: `kubectl get pods -l app=your-app`

**"Missing strings in logs"**
- Check container name is correct
- Verify application actually logs the expected string
- Increase `sinceSeconds` if app starts slowly

**"Invalid response from URL"**
- Ensure service returns HTTP status codes
- Check service DNS name is correct
- Verify network policies allow connectivity

**"Field not found"**
- Check field path is correct: `readyReplicas` (not `status.readyReplicas`)
- Use `kubectl get deployment -o yaml` to see available fields
- For arrays: use `[0]` for index or `[field=value]` for filtering

> **Note**: For supported Kubernetes resources (Pod, Deployment, StatefulSet, etc.), field paths are validated at parse time using reflection. Invalid field paths will cause an error when loading `challenge.yaml`, with a helpful message listing available fields. This early validation catches typos before runtime.
>
> However, some fields are conditionally populated (e.g., `containerStatuses` only exists after containers start). These fields pass parse-time validation but may return "field not found" at runtime if the resource isn't in the expected state.

---

## Reference

### Validation Types

| Type | Purpose | Common Use Cases |
|------|---------|-----------------|
| `condition` | Check K8s conditions | Pod Ready, Deployment Available |
| `status` | Check any status field | Replica counts, restart counts, phase |
| `log` | Search container logs | Startup messages, error detection |
| `event` | Detect forbidden events | OOMKilled, CrashLoopBackOff |
| `connectivity` | Test HTTP endpoints | Service discovery, network policies |

### Supported Resource Kinds

| Kind | Group | Version | Notes |
|------|-------|---------|-------|
| Pod | core | v1 | Direct pod checks |
| Deployment | apps | v1 | Checks owned pods |
| StatefulSet | apps | v1 | Checks owned pods |
| DaemonSet | apps | v1 | Checks owned pods |
| ReplicaSet | apps | v1 | Checks owned pods |
| Job | batch | v1 | Checks owned pods |
| Service | core | v1 | Connectivity targets only |

### Default Values

| Parameter | Default | Validation Type |
|-----------|---------|-----------------|
| sinceSeconds | 300 (5 min) | log, event |
| timeoutSeconds | 5 | connectivity |
| container | first container | log |

---

For migration from the old CRD-based system, see [MIGRATION.md](../MIGRATION.md).
