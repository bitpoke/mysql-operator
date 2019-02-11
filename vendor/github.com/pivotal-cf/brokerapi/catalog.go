// Copyright (C) 2015-Present Pivotal Software, Inc. All rights reserved.

// This program and the accompanying materials are made available under
// the terms of the under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

// http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package brokerapi

import (
	"encoding/json"
	"reflect"
	"strings"

	"github.com/pkg/errors"
)

type Service struct {
	ID                   string                  `json:"id"`
	Name                 string                  `json:"name"`
	Description          string                  `json:"description"`
	Bindable             bool                    `json:"bindable"`
	InstancesRetrievable bool                    `json:"instances_retrievable,omitempty"`
	BindingsRetrievable  bool                    `json:"bindings_retrievable,omitempty"`
	Tags                 []string                `json:"tags,omitempty"`
	PlanUpdatable        bool                    `json:"plan_updateable"`
	Plans                []ServicePlan           `json:"plans"`
	Requires             []RequiredPermission    `json:"requires,omitempty"`
	Metadata             *ServiceMetadata        `json:"metadata,omitempty"`
	DashboardClient      *ServiceDashboardClient `json:"dashboard_client,omitempty"`
}

type ServiceDashboardClient struct {
	ID          string `json:"id"`
	Secret      string `json:"secret"`
	RedirectURI string `json:"redirect_uri"`
}

type ServicePlan struct {
	ID              string               `json:"id"`
	Name            string               `json:"name"`
	Description     string               `json:"description"`
	Free            *bool                `json:"free,omitempty"`
	Bindable        *bool                `json:"bindable,omitempty"`
	Metadata        *ServicePlanMetadata `json:"metadata,omitempty"`
	Schemas         *ServiceSchemas      `json:"schemas,omitempty"`
	MaintenanceInfo *MaintenanceInfo     `json:"maintenance_info,omitempty"`
}

type ServiceSchemas struct {
	Instance ServiceInstanceSchema `json:"service_instance,omitempty"`
	Binding  ServiceBindingSchema  `json:"service_binding,omitempty"`
}

type ServiceInstanceSchema struct {
	Create Schema `json:"create,omitempty"`
	Update Schema `json:"update,omitempty"`
}

type ServiceBindingSchema struct {
	Create Schema `json:"create,omitempty"`
}

type Schema struct {
	Parameters map[string]interface{} `json:"parameters"`
}

type ServicePlanMetadata struct {
	DisplayName        string            `json:"displayName,omitempty"`
	Bullets            []string          `json:"bullets,omitempty"`
	Costs              []ServicePlanCost `json:"costs,omitempty"`
	AdditionalMetadata map[string]interface{}
}

type ServicePlanCost struct {
	Amount map[string]float64 `json:"amount"`
	Unit   string             `json:"unit"`
}

type ServiceMetadata struct {
	DisplayName         string `json:"displayName,omitempty"`
	ImageUrl            string `json:"imageUrl,omitempty"`
	LongDescription     string `json:"longDescription,omitempty"`
	ProviderDisplayName string `json:"providerDisplayName,omitempty"`
	DocumentationUrl    string `json:"documentationUrl,omitempty"`
	SupportUrl          string `json:"supportUrl,omitempty"`
	Shareable           *bool  `json:"shareable,omitempty"`
	AdditionalMetadata  map[string]interface{}
}

type MaintenanceInfo struct {
	Public  map[string]string `json:"public,omitempty"`
	Private string            `json:"private,omitempty"`
}

func FreeValue(v bool) *bool {
	return &v
}

func BindableValue(v bool) *bool {
	return &v
}

type RequiredPermission string

const (
	PermissionRouteForwarding = RequiredPermission("route_forwarding")
	PermissionSyslogDrain     = RequiredPermission("syslog_drain")
	PermissionVolumeMount     = RequiredPermission("volume_mount")

	additionalMetadataName = "AdditionalMetadata"
)

func (spm ServicePlanMetadata) MarshalJSON() ([]byte, error) {
	type Alias ServicePlanMetadata

	b, err := json.Marshal(Alias(spm))
	if err != nil {
		return []byte{}, errors.Wrap(err, "unmarshallable content in AdditionalMetadata")
	}

	var m map[string]interface{}
	json.Unmarshal(b, &m)
	delete(m, additionalMetadataName)

	for k, v := range spm.AdditionalMetadata {
		m[k] = v
	}

	return json.Marshal(m)
}

func (spm *ServicePlanMetadata) UnmarshalJSON(data []byte) error {
	type Alias ServicePlanMetadata

	if err := json.Unmarshal(data, (*Alias)(spm)); err != nil {
		return err
	}

	additionalMetadata := map[string]interface{}{}
	if err := json.Unmarshal(data, &additionalMetadata); err != nil {
		return err
	}

	s := reflect.ValueOf(spm).Elem()
	for _, jsonName := range GetJsonNames(s) {
		if jsonName == additionalMetadataName {
			continue
		}
		delete(additionalMetadata, jsonName)
	}

	if len(additionalMetadata) > 0 {
		spm.AdditionalMetadata = additionalMetadata
	}
	return nil
}

func GetJsonNames(s reflect.Value) (res []string) {
	valType := s.Type()
	for i := 0; i < s.NumField(); i++ {
		field := valType.Field(i)
		tag := field.Tag
		jsonVal := tag.Get("json")
		if jsonVal != "" {
			components := strings.Split(jsonVal, ",")
			jsonName := components[0]
			res = append(res, jsonName)
		} else {
			res = append(res, field.Name)
		}
	}
	return res
}

func (sm ServiceMetadata) MarshalJSON() ([]byte, error) {
	type Alias ServiceMetadata

	b, err := json.Marshal(Alias(sm))
	if err != nil {
		return []byte{}, errors.Wrap(err, "unmarshallable content in AdditionalMetadata")
	}

	var m map[string]interface{}
	json.Unmarshal(b, &m)
	delete(m, additionalMetadataName)

	for k, v := range sm.AdditionalMetadata {
		m[k] = v
	}
	return json.Marshal(m)
}

func (sm *ServiceMetadata) UnmarshalJSON(data []byte) error {
	type Alias ServiceMetadata

	if err := json.Unmarshal(data, (*Alias)(sm)); err != nil {
		return err
	}

	additionalMetadata := map[string]interface{}{}
	if err := json.Unmarshal(data, &additionalMetadata); err != nil {
		return err
	}

	for _, jsonName := range GetJsonNames(reflect.ValueOf(sm).Elem()) {
		if jsonName == additionalMetadataName {
			continue
		}
		delete(additionalMetadata, jsonName)
	}

	if len(additionalMetadata) > 0 {
		sm.AdditionalMetadata = additionalMetadata
	}
	return nil
}
