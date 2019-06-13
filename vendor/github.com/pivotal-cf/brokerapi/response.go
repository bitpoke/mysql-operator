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
	"github.com/pivotal-cf/brokerapi/domain"
	"github.com/pivotal-cf/brokerapi/domain/apiresponses"
)

//Deprecated: Use github.com/pivotal-cf/brokerapi/domain/apiresponses
type EmptyResponse = apiresponses.EmptyResponse

//Deprecated: Use github.com/pivotal-cf/brokerapi/domain/apiresponses
type ErrorResponse = apiresponses.ErrorResponse

//Deprecated: Use github.com/pivotal-cf/brokerapi/domain/apiresponses
type CatalogResponse = apiresponses.CatalogResponse

//Deprecated: Use github.com/pivotal-cf/brokerapi/domain/apiresponses
type ProvisioningResponse = apiresponses.ProvisioningResponse

//Deprecated: Use github.com/pivotal-cf/brokerapi/domain/apiresponses
type GetInstanceResponse = apiresponses.GetInstanceResponse

//Deprecated: Use github.com/pivotal-cf/brokerapi/domain/apiresponses
type UpdateResponse = apiresponses.UpdateResponse

//Deprecated: Use github.com/pivotal-cf/brokerapi/domain/apiresponses
type DeprovisionResponse = apiresponses.DeprovisionResponse

//Deprecated: Use github.com/pivotal-cf/brokerapi/domain/apiresponses
type LastOperationResponse = apiresponses.LastOperationResponse

//Deprecated: Use github.com/pivotal-cf/brokerapi/domain/apiresponses
type AsyncBindResponse = apiresponses.AsyncBindResponse

//Deprecated: Use github.com/pivotal-cf/brokerapi/domain/apiresponses
type BindingResponse = apiresponses.BindingResponse

//Deprecated: Use github.com/pivotal-cf/brokerapi/domain/apiresponses
type GetBindingResponse = apiresponses.GetBindingResponse

//Deprecated: Use github.com/pivotal-cf/brokerapi/domain/apiresponses
type UnbindResponse = apiresponses.UnbindResponse

//Deprecated: Use github.com/pivotal-cf/brokerapi/domain/apiresponses
type ExperimentalVolumeMountBindingResponse = apiresponses.ExperimentalVolumeMountBindingResponse

//Deprecated: Use github.com/pivotal-cf/brokerapi/domain
type ExperimentalVolumeMount = domain.ExperimentalVolumeMount

//Deprecated: Use github.com/pivotal-cf/brokerapi/domain
type ExperimentalVolumeMountPrivate = domain.ExperimentalVolumeMountPrivate
