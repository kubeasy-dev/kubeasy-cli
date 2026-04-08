package audit

import (
	"os"
	"path/filepath"

	"github.com/kubeasy-dev/kubeasy-cli/internal/constants"
)

// AuditPolicyYAML is the Kubernetes audit policy applied to the Kind API server.
// It filters system noise and captures kubernetes-admin actions at Request level.
const AuditPolicyYAML = `apiVersion: audit.k8s.io/v1
kind: Policy
omitStages:
  - RequestReceived
rules:
  # Ignore read-only requests from system components
  - level: None
    users:
      - system:kube-scheduler
      - system:kube-controller-manager
      - system:node
      - system:apiserver
    verbs:
      - get
      - list
      - watch

  # Ignore noisy system namespaces
  - level: None
    namespaces:
      - kube-system
      - kube-public
      - kube-node-lease
      - local-path-storage
      - kyverno
      - cert-manager

  # Ignore health checks and metrics
  - level: None
    nonResourceURLs:
      - /healthz*
      - /readyz*
      - /livez*
      - /metrics
      - /version

  # Ignore watch requests (too noisy)
  - level: None
    verbs:
      - watch

  # Capture kubernetes-admin (user) actions at Request level
  - level: Request
    users:
      - kubernetes-admin
    verbs:
      - create
      - update
      - patch
      - delete
      - deletecollection
      - apply

  # Default: ignore everything else
  - level: None
`

// GetAuditDir returns the path to the audit directory (~/.kubeasy/audit).
func GetAuditDir() string {
	return filepath.Join(constants.GetKubeasyConfigDir(), "audit")
}

// GetAuditPolicyPath returns the path to the audit policy file.
func GetAuditPolicyPath() string {
	return filepath.Join(GetAuditDir(), "audit-policy.yaml")
}

// GetAuditLogPath returns the path to the audit log file written by the API server.
func GetAuditLogPath() string {
	return filepath.Join(GetAuditDir(), "audit.log")
}

// EnsureAuditPolicy creates the audit directory and writes the policy file.
// It is idempotent — safe to call on every setup.
func EnsureAuditPolicy() error {
	if err := os.MkdirAll(GetAuditDir(), 0o750); err != nil {
		return err
	}
	return os.WriteFile(GetAuditPolicyPath(), []byte(AuditPolicyYAML), 0o640)
}
