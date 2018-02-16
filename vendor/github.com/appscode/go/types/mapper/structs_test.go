package mapper_test

import (
	"fmt"
	"testing"

	"github.com/appscode/go/types/mapper"
)

type testA struct {
	M       map[string]interface{} `mapper:"name=map"`
	A       string                 `mapper:"name=a"`
	AA      string                 `mapper:"name=nothing"`
	I       int                    `mapper:"name=i"`
	Ignore  int                    `mapper:"-"`
	NoTag   int
	private int `mapper:"name=private"`
}

type testB struct {
	M2         map[string]interface{} `mapper:"name=map"`
	A2         string                 `mapper:"name=a"`
	AA2        string                 `mapper:"name=nothing"`
	I2         int                    `mapper:"name=i"`
	Ignore2    int                    `mapper:"-"`
	NoTag2     int
	private2   int    `mapper:"name=private"`
	Extra      string `mapper:"extra"`
	IgnoreHere int    `mapper:"-"`
}

func TestMapperByNameKey(t *testing.T) {
	src := &testA{
		M: map[string]interface{}{
			"hello": "world",
		},
		A:       "nothing",
		AA:      "test at all",
		I:       10,
		Ignore:  3,
		NoTag:   11,
		private: 4,
	}
	dest := &testB{}
	mapper.ByNameKey(src, dest)
	fmt.Println(src)
	fmt.Println(dest)
}

type testC struct {
	M       map[string]interface{} `mapper:"target=M2"`
	A       string                 `mapper:"target=A2"`
	AA      string                 `mapper:"target=AA2"`
	I       int                    `mapper:"target=Ignore2"`
	Ignore  int                    `mapper:"-"`
	NoTag   int
	Extra   string `mapper:"nofield"`
	private int    `mapper:"target=private2"`
}

type testD struct {
	M2         map[string]interface{}
	A2         string
	AA2        string
	I2         int
	Ignore2    int
	NoTag2     int
	private2   int
	Extra      string
	IgnoreHere int
}

func TestMapperByField(t *testing.T) {
	src := &testC{
		M: map[string]interface{}{
			"hello": "world",
		},
		A:       "nothing",
		AA:      "test at all",
		I:       10,
		Ignore:  3,
		NoTag:   11,
		private: 4,
		Extra:   "not in dest",
	}
	dest := &testD{}
	mapper.ByField(src, dest)
	fmt.Println(src)
	fmt.Println(dest)
}
