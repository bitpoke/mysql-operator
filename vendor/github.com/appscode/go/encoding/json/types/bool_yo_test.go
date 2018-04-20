package types_test

import (
	"testing"

	. "github.com/appscode/go/encoding/json/types"
	"github.com/stretchr/testify/assert"
)

func TestBoolYo_MarshalJSON_True(t *testing.T) {
	assert := assert.New(t)

	var a BoolYo
	a = true
	data, err := a.MarshalJSON()
	assert.Nil(err)
	assert.Equal(`"true"`, string(data))
}

func TestBoolYo_MarshalJSON_False(t *testing.T) {
	assert := assert.New(t)

	var a BoolYo
	a = false
	data, err := a.MarshalJSON()
	assert.Nil(err)
	assert.Equal(`"false"`, string(data))
}

func TestBoolYo_MarshalJSON_ZeroValue(t *testing.T) {
	assert := assert.New(t)

	var a BoolYo
	data, err := a.MarshalJSON()
	assert.Nil(err)
	assert.Equal(`"false"`, string(data))
}

func TestBoolYo_UnmarshalJSON_Empty(t *testing.T) {
	assert := assert.New(t)

	var a BoolYo
	err := a.UnmarshalJSON([]byte(`""`))
	assert.NotNil(err)
}

func TestBoolYo_UnmarshalJSON_NonEmpty(t *testing.T) {
	assert := assert.New(t)

	var a BoolYo
	err := a.UnmarshalJSON([]byte(`"false"`))
	assert.Nil(err)
	assert.Equal(BoolYo(false), a)
}

func TestBoolYo_UnmarshalJSON_True(t *testing.T) {
	assert := assert.New(t)

	var a BoolYo
	err := a.UnmarshalJSON([]byte(`true`))
	assert.Nil(err)
	assert.Equal(BoolYo(true), a)
}

func TestBoolYo_UnmarshalJSON_False(t *testing.T) {
	assert := assert.New(t)

	var a BoolYo
	err := a.UnmarshalJSON([]byte(`false`))
	assert.Nil(err)
	assert.Equal(BoolYo(false), a)
}

func TestBoolYo_UnmarshalJSON_Fail(t *testing.T) {
	assert := assert.New(t)

	var a BoolYo
	err := a.UnmarshalJSON([]byte(`True`))
	assert.Nil(err)
	assert.Equal(BoolYo(true), a)
}
