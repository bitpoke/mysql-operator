package types_test

import (
	"testing"

	. "github.com/appscode/go/encoding/json/types"
	"github.com/stretchr/testify/assert"
)

func TestURLMap_MarshalJSON_Nil(t *testing.T) {
	assert := assert.New(t)

	var a *URLMap
	a = nil
	data, err := a.MarshalJSON()
	assert.Nil(err)
	assert.Equal(`""`, string(data))
}

func TestURLMap_MarshalJSON_Empty(t *testing.T) {
	assert := assert.New(t)

	var a URLMap
	data, err := a.MarshalJSON()
	assert.Nil(err)
	assert.Equal(`""`, string(data))
}

func TestURLMap_MarshalJSON_Single(t *testing.T) {
	assert := assert.New(t)

	a := NewURLMap("https", 2380)
	a.Insert("s1", "127.0.0.1")
	data, err := a.MarshalJSON()
	assert.Nil(err)
	assert.Equal(`"s1=https://127.0.0.1:2380"`, string(data))
}

func TestURLMap_MarshalJSON_Multiple(t *testing.T) {
	assert := assert.New(t)

	a := NewURLMap("https", 2380)
	a.Insert("s1", "127.0.0.1")
	a.Insert("s2", "127.0.0.2")
	data, err := a.MarshalJSON()
	assert.Nil(err)
	assert.Equal(`"s1=https://127.0.0.1:2380,s2=https://127.0.0.2:2380"`, string(data))
}

func TestURLMap_UnmarshalJSON_Empty(t *testing.T) {
	assert := assert.New(t)

	var a URLMap
	err := a.UnmarshalJSON([]byte(`""`))
	assert.Nil(err)
	assert.True(a.Equal(URLMap{
		Scheme: "",
		Hosts:  nil,
		Port:   0,
	}))
}

func TestURLMap_UnmarshalJSON_Single(t *testing.T) {
	assert := assert.New(t)

	var a URLMap
	err := a.UnmarshalJSON([]byte(`"s1=https://127.0.0.1:2380"`))
	assert.Nil(err)
	assert.True(a.Equal(URLMap{
		Scheme: "https",
		Hosts:  map[string]string{"s1": "127.0.0.1"},
		Port:   2380,
	}))
}

func TestURLMap_UnmarshalJSON_Multiple(t *testing.T) {
	assert := assert.New(t)

	var a URLMap
	err := a.UnmarshalJSON([]byte(`"s1=https://127.0.0.1:2380,s2=https://127.0.0.2:2380"`))
	assert.Nil(err)
	assert.True(a.Equal(URLMap{
		Scheme: "https",
		Hosts: map[string]string{
			"s1": "127.0.0.1",
			"s2": "127.0.0.2",
		},
		Port: 2380,
	}))
}
