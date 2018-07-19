/*
Copyright 2018 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package apply

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/json"
)

func TestMerge(t *testing.T) {
	table := []struct {
		name, observed, lastApplied, desired, want string
	}{
		{
			name:        "empty",
			observed:    `{}`,
			lastApplied: `{}`,
			desired:     `{}`,
			want:        `{}`,
		},
		{
			name: "scalars",
			observed: `{
				"keep": "other",
				"remove": "other",
				"replace": "other"
			}`,
			lastApplied: `{"remove": "old", "replace": "old"}`,
			desired:     `{"replace": "new", "add": "new" }`,
			want: `{
				"replace": "new",
				"add": "new",
				"keep": "other"
			}`,
		},
		{
			name: "nested object",
			observed: `{
				"update": {"keep": "other", "remove": "other"}
			}`,
			lastApplied: `{"update": {"remove": "old", "replace": "old"}}`,
			desired:     `{"update": {"replace": "new", "add": "new"}}`,
			want: `{
				"update": {"replace": "new", "add": "new", "keep": "other"}
			}`,
		},
		{
			name:        "replace list",
			observed:    `{"list": [1,2,3,{"a":true}]}`,
			lastApplied: `{"list": [4,5,6]}`,
			desired:     `{"list": [7,8,9,{"b":false}]}`,
			want:        `{"list": [7,8,9,{"b":false}]}`,
		},
		{
			name: "merge list-map",
			observed: `{
        "listMap": [
          {"name": "keep", "value": "other"},
          {"name": "remove", "value": "other"},
          {"name": "merge", "nested": {"keep": "other"}}
        ],
				"ports1": [
					{"port": 80, "keep": "other"}
				],
				"ports2": [
					{"containerPort": 80, "keep": "other"}
				]
      }`,
			lastApplied: `{
        "listMap": [
          {"name": "remove", "value": "old"}
        ],
				"ports1": [
					{"port": 80, "remove": "old"}
				]
      }`,
			desired: `{
        "listMap": [
          {"name": "add", "value": "new"},
          {"name": "merge", "nested": {"add": "new"}}
        ],
				"ports1": [
					{"port": 80, "add": "new"},
					{"port": 90}
				],
				"ports2": [
					{"containerPort": 80},
					{"containerPort": 90}
				]
      }`,
			want: `{
        "listMap": [
          {"name": "keep", "value": "other"},
          {"name": "merge", "nested": {"keep": "other", "add": "new"}},
          {"name": "add", "value": "new"}
        ],
				"ports1": [
					{"port": 80, "keep": "other", "add": "new"},
					{"port": 90}
				],
				"ports2": [
					{"containerPort": 80, "keep": "other"},
					{"containerPort": 90}
				]
      }`,
		},
	}

	for _, tc := range table {
		observed := make(map[string]interface{})
		if err := json.Unmarshal([]byte(tc.observed), &observed); err != nil {
			t.Errorf("%v: can't unmarshal tc.observed: %v", tc.name, err)
			continue
		}
		lastApplied := make(map[string]interface{})
		if err := json.Unmarshal([]byte(tc.lastApplied), &lastApplied); err != nil {
			t.Errorf("%v: can't unmarshal tc.lastApplied: %v", tc.name, err)
			continue
		}
		desired := make(map[string]interface{})
		if err := json.Unmarshal([]byte(tc.desired), &desired); err != nil {
			t.Errorf("%v: can't unmarshal tc.desired: %v", tc.name, err)
			continue
		}
		want := make(map[string]interface{})
		if err := json.Unmarshal([]byte(tc.want), &want); err != nil {
			t.Errorf("%v: can't unmarshal tc.want: %v", tc.name, err)
			continue
		}

		got, err := Merge(observed, lastApplied, desired)
		if err != nil {
			t.Errorf("%v: Merge error: %v", tc.name, err)
			continue
		}

		if !reflect.DeepEqual(got, want) {
			t.Logf("reflect diff: a=got, b=want:\n%s", diff.ObjectReflectDiff(got, want))
			t.Errorf("%v: Merge() = %#v, want %#v", tc.name, got, want)
		}
	}
}
