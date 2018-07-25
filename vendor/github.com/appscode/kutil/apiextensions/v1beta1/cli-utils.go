// Copyright 2018
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1beta1

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/appscode/go/types"
	"github.com/ghodss/yaml"
	"github.com/spf13/pflag"
	extensionsobj "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/kube-openapi/pkg/common"
)

// Config stores the user configuration input
type Config struct {
	SpecDefinitionName      string
	EnableValidation        bool
	OutputFormat            string
	Labels                  Labels
	Annotations             Labels
	ResourceScope           string
	Group                   string
	Kind                    string
	Version                 string
	Plural                  string
	Singular                string
	ShortNames              []string
	GetOpenAPIDefinitions   GetAPIDefinitions
	EnableStatusSubresource bool
	EnableScaleSubresource  bool
}

type Labels struct {
	LabelsString string
	LabelsMap    map[string]string
}

func (labels *Labels) Type() string { return "Labels" }

// Implement the flag.Value interface
func (labels *Labels) String() string {
	return labels.LabelsString
}

// Merge labels create a new map with labels merged.
func (labels *Labels) Merge(otherLabels map[string]string) map[string]string {
	mergedLabels := map[string]string{}

	for key, value := range otherLabels {
		mergedLabels[key] = value
	}

	for key, value := range labels.LabelsMap {
		mergedLabels[key] = value
	}
	return mergedLabels
}

// Implement the flag.Set interface
func (labels *Labels) Set(value string) error {
	m := map[string]string{}
	if value != "" {
		splited := strings.Split(value, ",")
		for _, pair := range splited {
			sp := strings.Split(pair, "=")
			m[sp[0]] = sp[1]
		}
	}
	(*labels).LabelsMap = m
	(*labels).LabelsString = value
	return nil
}

func NewCustomResourceDefinition(config Config, options ...func(map[string]common.OpenAPIDefinition)) *extensionsobj.CustomResourceDefinition {
	crd := &extensionsobj.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:        config.Plural + "." + config.Group,
			Labels:      config.Labels.LabelsMap,
			Annotations: config.Annotations.LabelsMap,
		},
		TypeMeta: CustomResourceDefinitionTypeMeta,
		Spec: extensionsobj.CustomResourceDefinitionSpec{
			Group:   config.Group,
			Version: config.Version,
			Scope:   extensionsobj.ResourceScope(config.ResourceScope),
			Names: extensionsobj.CustomResourceDefinitionNames{
				Plural:     config.Plural,
				Singular:   config.Singular,
				Kind:       config.Kind,
				ShortNames: config.ShortNames,
			},
		},
	}
	if config.SpecDefinitionName != "" && config.EnableValidation == true {
		crd.Spec.Validation = GetCustomResourceValidation(config.SpecDefinitionName, config.GetOpenAPIDefinitions, options)
	}
	if config.EnableStatusSubresource || config.EnableScaleSubresource {
		crd.Spec.Subresources = &extensionsobj.CustomResourceSubresources{}
		if config.EnableStatusSubresource {
			crd.Spec.Subresources.Status = &extensionsobj.CustomResourceSubresourceStatus{}
		}
		if config.EnableScaleSubresource {
			crd.Spec.Subresources.Scale = &extensionsobj.CustomResourceSubresourceScale{
				SpecReplicasPath:   ".spec.replicas",
				StatusReplicasPath: ".status.replicas",
				LabelSelectorPath:  types.StringP(".status.labelSelector"),
			}
		}
	}
	return crd
}

func MarshallCrd(w io.Writer, crd *extensionsobj.CustomResourceDefinition, outputFormat string) error {
	jsonBytes, err := json.Marshal(crd)
	if err != nil {
		return err
	}

	var r unstructured.Unstructured
	if err := json.Unmarshal(jsonBytes, &r.Object); err != nil {
		return err
	}

	unstructured.RemoveNestedField(r.Object, "status")

	jsonBytes, err = json.MarshalIndent(r.Object, "", "    ")
	if err != nil {
		return err
	}

	if outputFormat == "json" {
		_, err = w.Write(jsonBytes)
		if err != nil {
			return err
		}
	} else {
		yamlBytes, err := yaml.JSONToYAML(jsonBytes)
		if err != nil {
			return err
		}

		_, err = w.Write([]byte("---\n"))
		if err != nil {
			return err
		}

		_, err = w.Write(yamlBytes)
		if err != nil {
			return err
		}
	}

	return nil
}

// InitFlags prepares command line flags parser
func InitFlags(cfg *Config, fs *pflag.FlagSet) *pflag.FlagSet {
	fs.Var(&cfg.Labels, "labels", "Labels")
	fs.Var(&cfg.Annotations, "annotations", "Annotations")
	fs.BoolVar(&cfg.EnableValidation, "with-validation", true, "Add CRD validation field, default: true")
	fs.StringVar(&cfg.Group, "apigroup", "custom.example.com", "CRD api group")
	fs.StringVar(&cfg.SpecDefinitionName, "spec-name", "", "CRD spec definition name")
	fs.StringVar(&cfg.OutputFormat, "output", "yaml", "output format: json|yaml")
	fs.StringVar(&cfg.Kind, "kind", "", "CRD Kind")
	fs.StringVar(&cfg.ResourceScope, "scope", string(extensionsobj.NamespaceScoped), "CRD scope: 'Namespaced' | 'Cluster'.  Default: Namespaced")
	fs.StringVar(&cfg.Version, "version", "v1", "CRD version, default: 'v1'")
	fs.StringVar(&cfg.Plural, "plural", "", "CRD plural name")
	fs.StringVar(&cfg.Singular, "singular", "", "CRD singular name")
	fs.StringSliceVar(&cfg.ShortNames, "short-names", nil, "CRD short names")
	return fs
}
