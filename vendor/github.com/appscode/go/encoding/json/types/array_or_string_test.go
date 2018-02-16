package types_test

import (
	"testing"

	. "github.com/appscode/go/encoding/json/types"
	"github.com/stretchr/testify/assert"
)

func TestArrayOrString_MarshalJSON_Nil(t *testing.T) {
	assert := assert.New(t)

	var a ArrayOrString
	a = nil
	data, err := a.MarshalJSON()
	assert.Nil(err)
	assert.Equal(`[]`, string(data))
}

func TestArrayOrString_MarshalJSON_Empty(t *testing.T) {
	assert := assert.New(t)

	var a ArrayOrString
	data, err := a.MarshalJSON()
	assert.Nil(err)
	assert.Equal(`[]`, string(data))
}

func TestArrayOrString_MarshalJSON_Single(t *testing.T) {
	assert := assert.New(t)

	var a ArrayOrString
	a = []string{"x"}
	data, err := a.MarshalJSON()
	assert.Nil(err)
	assert.Equal(`"x"`, string(data))
}

func TestArrayOrString_MarshalJSON_Multiple(t *testing.T) {
	assert := assert.New(t)

	var a ArrayOrString
	a = []string{"x", "y"}
	data, err := a.MarshalJSON()
	assert.Nil(err)
	assert.Equal(`["x","y"]`, string(data))
}

func TestArrayOrString_UnmarshalJSON_Empty(t *testing.T) {
	assert := assert.New(t)

	var a ArrayOrString
	err := a.UnmarshalJSON([]byte(`[]`))
	assert.Nil(err)
	assert.Empty(a)
}

func TestArrayOrString_UnmarshalJSON_Single(t *testing.T) {
	assert := assert.New(t)

	var a ArrayOrString
	err := a.UnmarshalJSON([]byte(`"x"`))
	assert.Nil(err)
	assert.Len(a, 1)
	assert.Equal("x", a[0])
}

func TestArrayOrString_UnmarshalJSON_Multiple(t *testing.T) {
	assert := assert.New(t)

	var a ArrayOrString
	err := a.UnmarshalJSON([]byte(`["x","y"]`))
	assert.Nil(err)
	assert.Len(a, 2)
	assert.Equal("x", a[0])
	assert.Equal("y", a[1])
}
