/*
Copyright 2019 Pressinfra SRL

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

package broker

import (
	"encoding/json"

	"github.com/alecthomas/jsonschema"

	brokerapi "github.com/pivotal-cf/brokerapi/domain"
)

func (sb *serviceBroker) plans(serviceID string) []brokerapi.ServicePlan {
	trueValue := true

	switch serviceID {
	case MysqlServiceID:
		return []brokerapi.ServicePlan{
			{
				ID:          DefaultPlanID,
				Name:        DefaultPlanName,
				Description: DefaultPlanDescription,
				Free:        &trueValue,
				Bindable:    &trueValue,
				Metadata: &brokerapi.ServicePlanMetadata{
					DisplayName: DefaultPlanName,
				},
				Schemas: &brokerapi.ServiceSchemas{
					Instance: brokerapi.ServiceInstanceSchema{
						Create: mustGetJSONSchema(&MySQLProvisionParameters{}),
					},
					Binding: brokerapi.ServiceBindingSchema{
						Create: mustGetJSONSchema(&MySQLBindParameters{}),
					},
				},
			},
		}
	}
	return []brokerapi.ServicePlan{}
}

// mustGetJSONSchema takes an struct{} and returns the related JSON schema
func mustGetJSONSchema(obj interface{}) brokerapi.Schema {
	var reflector = jsonschema.Reflector{
		ExpandedStruct: true,
	}
	var schemaBytes, err = json.Marshal(reflector.Reflect(obj))
	if err != nil {
		panic(err)
	}
	schema := brokerapi.Schema{}
	err = json.Unmarshal(schemaBytes, &schema.Parameters)
	if err != nil {
		panic(err)
	}

	return schema
}
