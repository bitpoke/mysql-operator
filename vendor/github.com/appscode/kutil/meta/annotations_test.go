package meta

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetMap(t *testing.T) {
	in := map[string]string{
		"k1": `{"o1": "v1"}`,
	}

	actual, _ := GetMap(in, "k1")
	assert.Equal(t, map[string]string{"o1": "v1"}, actual)
}
