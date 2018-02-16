package meta

import (
	"github.com/google/go-cmp/cmp"
	"github.com/json-iterator/go"
	jsondiff "github.com/yudai/gojsondiff"
	"github.com/yudai/gojsondiff/formatter"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	cmpOptions = []cmp.Option{
		cmp.Comparer(func(x, y resource.Quantity) bool {
			return x.Cmp(y) == 0
		}),
		cmp.Comparer(func(x, y *metav1.Time) bool {
			if x == nil && y == nil {
				return true
			}
			if x != nil && y != nil {
				return x.Time.Equal(y.Time)
			}
			return false
		}),
	}
)

func Diff(x, y interface{}) string {
	return cmp.Diff(x, y, cmpOptions...)
}

func Equal(x, y interface{}) bool {
	return cmp.Equal(x, y, cmpOptions...)
}

func JsonDiff(old, new interface{}) (string, error) {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	oldBytes, err := json.Marshal(old)
	if err != nil {
		return "", err
	}

	newBytes, err := json.Marshal(new)
	if err != nil {
		return "", err
	}

	// Then, compare them
	differ := jsondiff.New()
	d, err := differ.Compare(oldBytes, newBytes)
	if err != nil {
		return "", err
	}

	var aJson map[string]interface{}
	if err := json.Unmarshal(oldBytes, &aJson); err != nil {
		return "", err
	}

	format := formatter.NewAsciiFormatter(aJson, formatter.AsciiFormatterConfig{
		ShowArrayIndex: true,
		Coloring:       false,
	})
	return format.Format(d)
}
