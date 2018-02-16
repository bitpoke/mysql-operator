package types_test

import (
	"testing"

	. "github.com/appscode/go/encoding/json/types"
	"github.com/stretchr/testify/assert"
)

func TestStrToBool_MarshalJSON_True(t *testing.T) {
	assert := assert.New(t)

	var a StrToBool
	a = true
	data, err := a.MarshalJSON()
	assert.Nil(err)
	assert.Equal(`true`, string(data))
}

func TestStrToBool_MarshalJSON_False(t *testing.T) {
	assert := assert.New(t)

	var a StrToBool
	a = false
	data, err := a.MarshalJSON()
	assert.Nil(err)
	assert.Equal(`false`, string(data))
}

func TestStrToBool_MarshalJSON_ZeroValue(t *testing.T) {
	assert := assert.New(t)

	var a StrToBool
	data, err := a.MarshalJSON()
	assert.Nil(err)
	assert.Equal(`false`, string(data))
}

func TestStrToBool_UnmarshalJSON_Empty(t *testing.T) {
	assert := assert.New(t)

	var a StrToBool
	err := a.UnmarshalJSON([]byte(`""`))
	assert.Nil(err)
	assert.Equal(StrToBool(false), a)
}

func TestStrToBool_UnmarshalJSON_NonEmpty(t *testing.T) {
	assert := assert.New(t)

	var a StrToBool
	err := a.UnmarshalJSON([]byte(`"false"`))
	assert.Nil(err)
	assert.Equal(StrToBool(true), a)
}

func TestStrToBool_UnmarshalJSON_True(t *testing.T) {
	assert := assert.New(t)

	var a StrToBool
	err := a.UnmarshalJSON([]byte(`true`))
	assert.Nil(err)
	assert.Equal(StrToBool(true), a)
}

func TestStrToBool_UnmarshalJSON_False(t *testing.T) {
	assert := assert.New(t)

	var a StrToBool
	err := a.UnmarshalJSON([]byte(`false`))
	assert.Nil(err)
	assert.Equal(StrToBool(false), a)
}

func TestStrToBool_UnmarshalJSON_Fail(t *testing.T) {
	assert := assert.New(t)

	var a StrToBool
	err := a.UnmarshalJSON([]byte(`True`))
	assert.NotNil(err)
}
