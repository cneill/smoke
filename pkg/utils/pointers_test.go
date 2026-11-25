package utils_test

import (
	"testing"

	"github.com/cneill/smoke/pkg/utils"
	"github.com/stretchr/testify/assert"
)

func TestDeref(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 0, utils.Deref[int](nil))
	assert.Equal(t, 42, utils.Deref(new(42)))
	assert.Empty(t, utils.Deref[string](nil))
	assert.Equal(t, "hello", utils.Deref(new("hello")))
	assert.False(t, utils.Deref[bool](nil))
	assert.True(t, utils.Deref(new(true)))
}
