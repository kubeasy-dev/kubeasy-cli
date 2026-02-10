package deployer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppendUnique(t *testing.T) {
	t.Run("no duplicates", func(t *testing.T) {
		dst := []string{"a", "b"}
		src := []string{"c", "d"}
		result := appendUnique(dst, src)
		assert.Equal(t, []string{"a", "b", "c", "d"}, result)
	})

	t.Run("with duplicates", func(t *testing.T) {
		dst := []string{"a", "b"}
		src := []string{"b", "c"}
		result := appendUnique(dst, src)
		assert.Equal(t, []string{"a", "b", "c"}, result)
	})

	t.Run("empty src", func(t *testing.T) {
		dst := []string{"a", "b"}
		result := appendUnique(dst, nil)
		assert.Equal(t, []string{"a", "b"}, result)
	})

	t.Run("empty dst", func(t *testing.T) {
		var dst []string
		src := []string{"a", "b"}
		result := appendUnique(dst, src)
		assert.Equal(t, []string{"a", "b"}, result)
	})
}
