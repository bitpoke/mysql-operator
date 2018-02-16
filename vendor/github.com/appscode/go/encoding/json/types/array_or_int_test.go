package types_test

import (
	"testing"

	. "github.com/appscode/go/encoding/json/types"
	"github.com/stretchr/testify/assert"
)

func TestArrayOrInt_MarshalJSON_Nil(t *testing.T) {
	assert := assert.New(t)

	var a ArrayOrInt
	a = nil
	data, err := a.MarshalJSON()
	assert.Nil(err)
	assert.Equal(`[]`, string(data))
}

func TestArrayOrInt_MarshalJSON_Empty(t *testing.T) {
	assert := assert.New(t)

	var a ArrayOrInt
	data, err := a.MarshalJSON()
	assert.Nil(err)
	assert.Equal(`[]`, string(data))
}

func TestArrayOrInt_MarshalJSON_Single(t *testing.T) {
	assert := assert.New(t)

	var a ArrayOrInt
	a = []int{1}
	data, err := a.MarshalJSON()
	assert.Nil(err)
	assert.Equal(`1`, string(data))
}

func TestArrayOrInt_MarshalJSON_Multiple(t *testing.T) {
	assert := assert.New(t)

	var a ArrayOrInt
	a = []int{1, 2}
	data, err := a.MarshalJSON()
	assert.Nil(err)
	assert.Equal(`[1,2]`, string(data))
}

func TestArrayOrInt_UnmarshalJSON_Empty(t *testing.T) {
	assert := assert.New(t)

	var a ArrayOrInt
	err := a.UnmarshalJSON([]byte(`[]`))
	assert.Nil(err)
	assert.Empty(a)
}

func TestArrayOrInt_UnmarshalJSON_Single(t *testing.T) {
	assert := assert.New(t)

	var a ArrayOrInt
	err := a.UnmarshalJSON([]byte(`1`))
	assert.Nil(err)
	assert.Len(a, 1)
	assert.Equal(1, a[0])
}

func TestArrayOrInt_UnmarshalJSON_Multiple(t *testing.T) {
	assert := assert.New(t)

	var a ArrayOrInt
	err := a.UnmarshalJSON([]byte(`[1,2]`))
	assert.Nil(err)
	assert.Len(a, 2)
	assert.Equal(1, a[0])
	assert.Equal(2, a[1])
}
