# Validation Examples

This document provides comprehensive examples of all validation types supported by the Kubeasy CLI validation system.

## Table of Contents

1. [Status Validation](#status-validation)
2. [Log Validation](#log-validation)
3. [Event Validation](#event-validation)
4. [Metrics Validation](#metrics-validation)
5. [Connectivity Validation](#connectivity-validation)
6. [Complete Challenge Example](#complete-challenge-example)

---

## Status Validation

Checks Kubernetes resource status conditions (e.g., Pod Ready, Deployment Available).

### Basic Pod Ready Check

```yaml
validations:
  - key: pod-ready
    title: "Pod Ready"
    description: "The application pod must be in Ready state"
    order: 1
    type: status
    spec:
      target:
        kind: Pod
        labelSelector:
          app: my-application
      conditions:
        - type: Ready
          status: "True"
```

**When to use**: Verify that pods are running and passing readiness probes.

**What it checks**:
- Finds all pods matching the label selector
- Checks if each pod has a `Ready` condition with status `True`
- All matching pods must meet the condition

---

### Deployment Available Check

```yaml
validations:
  - key: deployment-available
    title: "Deployment Available"
    description: "The deployment must have available replicas"
    order: 1
    type: status
    spec:
      target:
        kind: Deployment
        name: web-app
      conditions:
        - type: Available
          status: "True"
```

**When to use**: Verify that a deployment has successfully rolled out.

**What it checks**:
- Finds the deployment by name
- Gets pods owned by the deployment (via label selector)
- Checks pod conditions

---

### Multiple Conditions

```yaml
validations:
  - key: pod-healthy
    title: "Pod Healthy"
    description: "Pod must be both Ready and Initialized"
    order: 1
    type: status
    spec:
      target:
        kind: Pod
        labelSelector:
          app: database
      conditions:
        - type: Ready
          status: "True"
        - type: Initialized
          status: "True"
```

**When to use**: Verify multiple aspects of pod health.

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

## Metrics Validation

Validates numeric fields in pod/deployment status (e.g., replicas, restart count).

### Replica Count

```yaml
validations:
  - key: scaled-replicas
    title: "Scaled to 3 Replicas"
    description: "Deployment must have exactly 3 ready replicas"
    order: 1
    type: metrics
    spec:
      target:
        kind: Deployment
        name: web-app
      checks:
        - field: status.readyReplicas
          operator: "=="
          value: 3
        - field: status.availableReplicas
          operator: ">="
          value: 3
```

**When to use**: Verify horizontal scaling has been applied.

**Available operators**: `==`, `!=`, `>`, `<`, `>=`, `<=`

---

### Restart Count

```yaml
validations:
  - key: low-restarts
    title: "Low Restart Count"
    description: "Pod must have fewer than 3 restarts"
    order: 1
    type: metrics
    spec:
      target:
        kind: Pod
        labelSelector:
          app: stable-app
      checks:
        - field: status.containerStatuses.0.restartCount
          operator: "<"
          value: 3
```

**When to use**: Verify pod stability over time.

**Note**: Field paths use dot notation. Array indices are 0-based.

---

### StatefulSet Replicas

```yaml
validations:
  - key: statefulset-ready
    title: "StatefulSet Ready"
    description: "All StatefulSet replicas must be ready"
    order: 1
    type: metrics
    spec:
      target:
        kind: StatefulSet
        name: database
      checks:
        - field: status.readyReplicas
          operator: "=="
          value: 3
        - field: status.currentReplicas
          operator: "=="
          value: 3
```

**When to use**: Verify stateful applications are fully deployed.

---

### Label Selector with Metrics

```yaml
validations:
  - key: deployment-replicas
    title: "Deployment Replicas"
    description: "Any matching deployment must have 2+ replicas"
    order: 1
    type: metrics
    spec:
      target:
        kind: Deployment
        labelSelector:
          tier: backend
      checks:
        - field: status.readyReplicas
          operator: ">="
          value: 2
```

**When to use**: Check metrics across multiple resources matching a label.

**Note**: If multiple resources match, only the first is checked.

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

validations:
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
    type: metrics
    spec:
      target:
        kind: Deployment
        name: backend
      checks:
        - field: status.readyReplicas
          operator: "=="
          value: 3
        - field: status.availableReplicas
          operator: ">="
          value: 3

  # 3. All Pods Ready
  - key: all-pods-ready
    title: "All Pods Ready"
    description: "Frontend, backend, and database pods must be ready"
    order: 3
    type: status
    spec:
      target:
        kind: Pod
        labelSelector:
          app: microservices
      conditions:
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

## Best Practices

### 1. Ordering Validations

Order validations from most basic to most complex:
```yaml
validations:
  - order: 1  # Basic: Pods exist and are ready
    type: status

  - order: 2  # Application: Logs show startup
    type: log

  - order: 3  # Stability: No crash events
    type: event

  - order: 4  # Advanced: Network connectivity
    type: connectivity
```

### 2. Meaningful Titles

Use titles that describe success state, not the check:
- ✅ Good: "Pod Ready", "Database Connected"
- ❌ Bad: "Check Pod Status", "Validate Database"

### 3. Clear Descriptions

Explain what the user should achieve:
```yaml
description: "The application pod must be running and ready to accept traffic"
```

Not what the validation does:
```yaml
description: "Checks if pod has Ready condition set to True"
```

### 4. Label Selectors vs Names

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

### 5. Time Windows

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

**"Field not found or invalid"**
- Check field path is correct: `status.readyReplicas`
- Use `kubectl get deployment -o yaml` to see available fields

---

## Reference

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
