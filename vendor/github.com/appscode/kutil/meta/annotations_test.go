package meta

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetMap(t *testing.T) {
	in := map[string]string{
		"k1": `{"o1": "v1"}`,
	}

	actual, _ := GetMap(in, "k1")
	assert.Equal(t, map[string]string{"o1": "v1"}, actual)
}

func TestGetFloat(t *testing.T) {
	in := map[string]string{
		"k1": "17.33",
	}
	actual, _ := GetFloat(in, "k1")
	assert.Equal(t, 17.33, actual)
}

func TestGetDuration(t *testing.T) {
	in := map[string]string{
		"k1": "30s",
	}
	actual, _ := GetDuration(in, "k1")
	assert.Equal(t, time.Second*30, actual)
}
