package validation

import (
	"testing"

	"github.com/kubeasy-dev/registry/pkg/challenges"
	"github.com/stretchr/testify/assert"
)

func TestFromObjective_UnknownSpec(t *testing.T) {
	obj := challenges.Objective{
		Key:   "unknown-obj",
		Type:  "unknown-type",
		Spec:  struct{ Foo string }{Foo: "bar"}, // Not one of the supported pointer types
	}

	v := fromObjective(obj)

	assert.Equal(t, "unknown-obj", v.Key)
	assert.Equal(t, "unknown-type", string(v.Type))
	assert.Nil(t, v.Spec, "Spec should be nil for unknown spec type")
}

func TestFromObjective_SupportedSpec(t *testing.T) {
	spec := &StatusSpec{
		Target: Target{Kind: "Pod", Name: "test-pod"},
	}
	obj := challenges.Objective{
		Key:  "status-obj",
		Type: "status",
		Spec: spec,
	}

	v := fromObjective(obj)

	assert.Equal(t, "status-obj", v.Key)
	assert.Equal(t, TypeStatus, v.Type)
	assert.Equal(t, *spec, v.Spec)
}
