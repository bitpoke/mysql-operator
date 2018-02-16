package strings_test

import (
	"testing"

	. "github.com/appscode/go/strings"
	"github.com/stretchr/testify/assert"
)

func TestIsBothAlphaNum(t *testing.T) {
	assert.Equal(t, true, IsBothAlphaNum("a1"))
	assert.Equal(t, true, IsBothAlphaNum("a1$"))
	assert.Equal(t, true, IsBothAlphaNum("A1"))
	assert.Equal(t, true, IsBothAlphaNum("A1$"))

	assert.Equal(t, true, IsBothAlphaNum("1a"))
	assert.Equal(t, true, IsBothAlphaNum("1a$"))
	assert.Equal(t, true, IsBothAlphaNum("1A"))
	assert.Equal(t, true, IsBothAlphaNum("1A$"))

	assert.Equal(t, false, IsBothAlphaNum("A"))
	assert.Equal(t, false, IsBothAlphaNum("a"))
	assert.Equal(t, false, IsBothAlphaNum("1"))

	assert.Equal(t, false, IsBothAlphaNum("A$"))
	assert.Equal(t, false, IsBothAlphaNum("a$"))
	assert.Equal(t, false, IsBothAlphaNum("1$"))
}

func TestEqualSlice(t *testing.T) {
	a := []string{"foo", "bar", "foo"}
	b := []string{"bar", "foo", "foo"}
	assert.True(t, EqualSlice(a, b))

	a = []string{"foo", "bar", "foo2"}
	b = []string{"bar", "foo", "foo"}
	assert.False(t, EqualSlice(a, b))
}
