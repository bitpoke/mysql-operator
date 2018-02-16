package types_test

import (
	"encoding/json"
	"fmt"
	"testing"

	. "github.com/appscode/go/encoding/json/types"
	"github.com/json-iterator/go"
	"github.com/stretchr/testify/assert"
)

func TestStrYo(t *testing.T) {
	assert := assert.New(t)

	type Example struct {
		A StrYo
		B StrYo
		C StrYo
		D StrYo
		E StrYo
		F StrYo `json:",omitempty"`
		G StrYo
	}
	s := `{
		"A": "str\\",
		"B": 1,
		"C": 2.5,
		"D": false,
		"E": true,
		"F": null,
		"G": "8.0"
	}`

	var e Example
	err := json.Unmarshal([]byte(s), &e)

	assert.Nil(err)
	b, err := json.Marshal(&e)
	fmt.Println(string(b))
	assert.Equal(`{"A":"str\\","B":"1","C":"2.5","D":"false","E":"true","G":"8.0"}`, string(b))
}

func TestStrYo2(t *testing.T) {
	assert := assert.New(t)
	json := jsoniter.ConfigCompatibleWithStandardLibrary

	type Example struct {
		A StrYo
		B StrYo
		C StrYo
		D StrYo
		E StrYo
		F StrYo `json:",omitempty"`
		G StrYo
	}
	s := `{
		"A": "str\\",
		"B": 1,
		"C": 2.5,
		"D": false,
		"E": true,
		"F": null,
		"G": "8.0"
	}`

	var e Example
	err := json.Unmarshal([]byte(s), &e)

	assert.Nil(err)
	b, err := json.Marshal(&e)
	fmt.Println(string(b))
	assert.Equal(`{"A":"str\\","B":"1","C":"2.5","D":"false","E":"true","G":"8.0"}`, string(b))
}
