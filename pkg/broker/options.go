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

var (
	// MysqlServiceID is the ID of the service
	MysqlServiceID = "79f8df87-658d-4056-803b-d66c17b6e437"
	// MysqlServiceName is the name of the service
	MysqlServiceName = "MySQL"
	// MysqlServiceDescription service description
	MysqlServiceDescription = `A MySQL cluster deployed in K8s cluster using Presslabs MySQL Operator`

	// DefaultPlanID is the ID of the plan
	DefaultPlanID = "d488e51c-8de1-11e9-bc42-526af7764f64"
	// DefaultPlanName is the name of the default plan
	DefaultPlanName = "default"
	// DefaultPlanDescription
	DefaultPlanDescription = `MySQL cluster default plan`

	// DefaultNamespace where objects such as Secrets will be created
	DefaultNamespace = "default"
)
