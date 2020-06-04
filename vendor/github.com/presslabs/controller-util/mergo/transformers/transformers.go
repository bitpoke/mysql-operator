/*
Copyright 2018 Pressinfra SRL.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package transformers provide mergo transformers for Kubernetes objects
package transformers

import (
	"fmt"
	"reflect"

	"github.com/imdario/mergo"

	corev1 "k8s.io/api/core/v1"
)

// TransformerMap is a mergo.Transformers implementation
type TransformerMap map[reflect.Type]func(dst, src reflect.Value) error

// PodSpec mergo transformers for corev1.PodSpec
var PodSpec TransformerMap

// nolint: gochecknoinits
func init() {
	PodSpec = TransformerMap{
		reflect.TypeOf([]corev1.Container{}):            PodSpec.MergeListByKey("Name", mergo.WithOverride),
		reflect.TypeOf([]corev1.ContainerPort{}):        PodSpec.MergeListByKey("ContainerPort", mergo.WithOverride),
		reflect.TypeOf([]corev1.EnvVar{}):               PodSpec.MergeListByKey("Name", mergo.WithOverride),
		reflect.TypeOf(corev1.EnvVar{}):                 PodSpec.OverrideFields("Value", "ValueFrom"),
		reflect.TypeOf(corev1.VolumeSource{}):           PodSpec.NilOtherFields(),
		reflect.TypeOf([]corev1.Toleration{}):           PodSpec.MergeListByKey("Key", mergo.WithOverride),
		reflect.TypeOf([]corev1.Volume{}):               PodSpec.MergeListByKey("Name", mergo.WithOverride),
		reflect.TypeOf([]corev1.LocalObjectReference{}): PodSpec.MergeListByKey("Name", mergo.WithOverride),
		reflect.TypeOf([]corev1.HostAlias{}):            PodSpec.MergeListByKey("IP", mergo.WithOverride),
		reflect.TypeOf([]corev1.VolumeMount{}):          PodSpec.MergeListByKey("MountPath", mergo.WithOverride),
		reflect.TypeOf(corev1.Affinity{}):               PodSpec.OverrideFields("NodeAffinity", "PodAffinity", "PodAntiAffinity"),
		reflect.TypeOf(corev1.ResourceList{}):           overwriteListWithNonEmptySource,
		reflect.TypeOf(""):                              overwrite,
		reflect.TypeOf(new(string)):                     overwrite,
		reflect.TypeOf(new(int32)):                      overwrite,
		reflect.TypeOf(new(int64)):                      overwrite,
	}
}

// overwrite just overrites the dst value with the source
// nolint: unparam
func overwrite(dst, src reflect.Value) error {
	if !src.IsZero() {
		if dst.CanSet() {
			dst.Set(src)
		} else {
			dst = src
		}
	}

	return nil
}

// overwrite a list only if the source is not empty
// nolint: unparam
func overwriteListWithNonEmptySource(dst, src reflect.Value) error {
	if src.Len() > 0 {
		if dst.CanSet() {
			dst.Set(src)
		} else {
			dst = src
		}
	}

	return nil
}

// Transformer implements mergo.Tansformers interface for TransformenrMap
func (s TransformerMap) Transformer(t reflect.Type) func(dst, src reflect.Value) error {
	if fn, ok := s[t]; ok {
		return fn
	}
	return nil
}

func (s *TransformerMap) mergeByKey(key string, dst, elem reflect.Value, opts ...func(*mergo.Config)) error {
	elemKey := elem.FieldByName(key)
	for i := 0; i < dst.Len(); i++ {
		dstKey := dst.Index(i).FieldByName(key)
		if elemKey.Kind() != dstKey.Kind() {
			return fmt.Errorf("cannot merge when key type differs")
		}
		eq := eq(key, elem, dst.Index(i))
		if eq {
			opts = append(opts, mergo.WithTransformers(s))
			return mergo.Merge(dst.Index(i).Addr().Interface(), elem.Interface(), opts...)
		}
	}
	dst.Set(reflect.Append(dst, elem))
	return nil
}

func eq(key string, a, b reflect.Value) bool {
	aKey := a.FieldByName(key)
	bKey := b.FieldByName(key)
	if aKey.Kind() != bKey.Kind() {
		return false
	}
	eq := false
	switch aKey.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		eq = aKey.Int() == bKey.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		eq = aKey.Uint() == bKey.Uint()
	case reflect.String:
		eq = aKey.String() == bKey.String()
	case reflect.Float32, reflect.Float64:
		eq = aKey.Float() == bKey.Float()
	}
	return eq
}

func indexByKey(key string, v reflect.Value, list reflect.Value) (int, bool) {
	for i := 0; i < list.Len(); i++ {
		if eq(key, v, list.Index(i)) {
			return i, true
		}
	}
	return -1, false
}

// MergeListByKey merges two list by element key (eg. merge []corev1.Container
// by name). If mergo.WithAppendSlice options is passed, the list is extended,
// while elemnts with same name are merged. If not, the list is filtered to
// elements in src
func (s *TransformerMap) MergeListByKey(key string, opts ...func(*mergo.Config)) func(_, _ reflect.Value) error {
	conf := &mergo.Config{}
	for _, opt := range opts {
		opt(conf)
	}
	return func(dst, src reflect.Value) error {
		entries := reflect.MakeSlice(src.Type(), src.Len(), src.Len())
		for i := 0; i < src.Len(); i++ {
			elem := src.Index(i)
			err := s.mergeByKey(key, dst, elem, opts...)
			if err != nil {
				return err
			}
			j, found := indexByKey(key, elem, dst)
			if found {
				entries.Index(i).Set(dst.Index(j))
			}
		}
		if !conf.AppendSlice {
			dst.SetLen(entries.Len())
			dst.SetCap(entries.Cap())
			dst.Set(entries)
		}

		return nil
	}
}

// NilOtherFields nils all fields not defined in src
func (s *TransformerMap) NilOtherFields(opts ...func(*mergo.Config)) func(_, _ reflect.Value) error {
	return func(dst, src reflect.Value) error {
		for i := 0; i < dst.NumField(); i++ {
			dstField := dst.Type().Field(i)
			srcValue := src.FieldByName(dstField.Name)
			dstValue := dst.FieldByName(dstField.Name)

			if srcValue.Kind() == reflect.Ptr && srcValue.IsNil() {
				dstValue.Set(srcValue)
			} else {
				if dstValue.Kind() == reflect.Ptr && dstValue.IsNil() {
					dstValue.Set(srcValue)
				} else {
					opts = append(opts, mergo.WithTransformers(s))
					return mergo.Merge(dstValue.Interface(), srcValue.Interface(), opts...)
				}
			}
		}
		return nil
	}
}

// OverrideFields when merging override fields even if they are zero values (eg. nil or empty list)
func (s *TransformerMap) OverrideFields(fields ...string) func(_, _ reflect.Value) error {
	return func(dst, src reflect.Value) error {
		for _, field := range fields {
			srcValue := src.FieldByName(field)
			dst.FieldByName(field).Set(srcValue)
		}
		return nil
	}
}
