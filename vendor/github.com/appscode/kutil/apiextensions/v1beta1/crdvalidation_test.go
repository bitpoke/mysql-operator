package v1beta1_test

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/appscode/kutil/apiextensions/v1beta1"
	"github.com/go-openapi/spec"
	extensionsobj "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/kube-openapi/pkg/common"
)

func TestConvertSchematoJsonProp(t *testing.T) {
	ref := new(string)
	*ref = "k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta"
	schema := spec.Schema{
		SchemaProps: spec.SchemaProps{
			Description: "Standard object’s metadata. More info: https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#metadata",
			Ref:         v1beta1.OpenAPIRefCallBack(*ref),
		},
	}

	expected := extensionsobj.JSONSchemaProps{
		Description: "Standard object’s metadata. More info: https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#metadata",
		Ref:         ref,
	}
	var def map[string]common.OpenAPIDefinition
	props := v1beta1.SchemaPropsToJSONProps(&schema, def, false)

	if props.Description != expected.Description {
		t.Errorf("Description: expected %s, got %s", schema.Description, expected.Description)
	}

	if *props.Ref != schema.Ref.String() {
		t.Errorf("Ref: expected '%s', got '%s'", schema.Ref.String(), *props.Ref)
	}
}

func TestConvertFullSchematoJsonProp(t *testing.T) {
	schema := spec.Schema{SchemaProps: spec.SchemaProps{
		Description: "Describes an Alertmanager cluster.",
		Properties: map[string]spec.Schema{
			"kind": {
				SchemaProps: spec.SchemaProps{
					Description: "Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#types-kinds",
					Type:        []string{"string"},
					Format:      "",
				},
			},
			"items": {
				SchemaProps: spec.SchemaProps{
					Description: "List of Alertmanagers",
					Type:        []string{"array"},
					Items: &spec.SchemaOrArray{
						Schema: &spec.Schema{
							SchemaProps: spec.SchemaProps{
								Ref: v1beta1.OpenAPIRefCallBack("github.com/coreos/prometheus-operator/pkg/client/monitoring/v1.Alertmanager"),
							},
						},
					},
				},
			},
			"metadata": {
				SchemaProps: spec.SchemaProps{
					Description: "Standard object’s metadata. More info: https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#metadata",
					Ref:         v1beta1.OpenAPIRefCallBack("k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta"),
				},
			},
		},
	},
	}
	var def map[string]common.OpenAPIDefinition
	props := v1beta1.SchemaPropsToJSONProps(&schema, def, false)
	jsonBytes, err := json.MarshalIndent(props, "", "  ")
	if err != nil {
		fmt.Println("error:", err)
	}
	os.Stdout.Write(jsonBytes)

}
