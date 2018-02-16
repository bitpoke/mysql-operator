package arrays_test

import (
	"testing"

	. "github.com/appscode/go/arrays"
	"github.com/stretchr/testify/assert"
)

func TestReverse(t *testing.T) {
	a := []int{1, 2, 3, 4, 5}
	ar, err := Reverse(a)

	assert.Nil(t, err)
	assert.Equal(t, ar, []interface{}{5, 4, 3, 2, 1})
}

func TestFilter(t *testing.T) {
	a := []int{1, 2, 3, 4, 5}

	filterFuncs := func(i interface{}) bool {
		v := i.(int)
		if v%2 == 1 {
			return true
		}
		return false
	}

	ar, err := Filter(a, filterFuncs)

	assert.Nil(t, err)
	assert.Equal(t, ar, []interface{}{1, 3, 5})
}

func TestContains(t *testing.T) {
	a := []int{1, 2, 3, 4, 5}
	ok, pos := Contains(a, 2)
	assert.Equal(t, ok, true)
	assert.Equal(t, pos, 1)

	ok, pos = Contains(a, 9)
	assert.Equal(t, ok, false)
	assert.Equal(t, pos, -1)

	b := []string{"hello", "world"}
	ok, pos = Contains(b, "world")
	assert.Equal(t, ok, true)
	assert.Equal(t, pos, 1)

	ok, pos = Contains(a, "not found")
	assert.Equal(t, ok, false)
	assert.Equal(t, pos, -1)
}
