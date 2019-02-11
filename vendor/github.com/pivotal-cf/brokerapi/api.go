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
	"errors"
	"fmt"
	"github.com/pivotal-cf/brokerapi/middlewares/originating_identity_header"
	"net/http"
	"strconv"

	"code.cloudfoundry.org/lager"
	"github.com/gorilla/mux"
	"github.com/pivotal-cf/brokerapi/auth"
)

const (
	provisionLogKey            = "provision"
	deprovisionLogKey          = "deprovision"
	bindLogKey                 = "bind"
	getBindLogKey              = "getBinding"
	getInstanceLogKey          = "getInstance"
	unbindLogKey               = "unbind"
	updateLogKey               = "update"
	lastOperationLogKey        = "lastOperation"
	lastBindingOperationLogKey = "lastBindingOperation"
	catalogLogKey              = "catalog"

	instanceIDLogKey      = "instance-id"
	instanceDetailsLogKey = "instance-details"
	bindingIDLogKey       = "binding-id"

	invalidServiceDetailsErrorKey = "invalid-service-details"
	invalidBindDetailsErrorKey    = "invalid-bind-details"
	instanceLimitReachedErrorKey  = "instance-limit-reached"
	instanceAlreadyExistsErrorKey = "instance-already-exists"
	bindingAlreadyExistsErrorKey  = "binding-already-exists"
	instanceMissingErrorKey       = "instance-missing"
	bindingMissingErrorKey        = "binding-missing"
	bindingNotFoundErrorKey       = "binding-not-found"
	asyncRequiredKey              = "async-required"
	planChangeNotSupportedKey     = "plan-change-not-supported"
	unknownErrorKey               = "unknown-error"
	invalidRawParamsKey           = "invalid-raw-params"
	appGuidNotProvidedErrorKey    = "app-guid-not-provided"
	apiVersionInvalidKey          = "broker-api-version-invalid"
	serviceIdMissingKey           = "service-id-missing"
	planIdMissingKey              = "plan-id-missing"
	invalidServiceID              = "invalid-service-id"
	invalidPlanID                 = "invalid-plan-id"
	concurrentAccessKey           = "get-instance-during-update"
	maintenanceInfoConflictKey    = "maintenance-info-conflict"
)

var (
	serviceIdError        = errors.New("service_id missing")
	planIdError           = errors.New("plan_id missing")
	invalidServiceIDError = errors.New("service-id not in the catalog")
	invalidPlanIDError    = errors.New("plan-id not in the catalog")
)

type BrokerCredentials struct {
	Username string
	Password string
}

func New(serviceBroker ServiceBroker, logger lager.Logger, brokerCredentials BrokerCredentials) http.Handler {
	router := mux.NewRouter()
	AttachRoutes(router, serviceBroker, logger)

	authMiddleware := auth.NewWrapper(brokerCredentials.Username, brokerCredentials.Password).Wrap
	router.Use(authMiddleware)
	router.Use(originating_identity_header.AddToContext)

	return router
}

func AttachRoutes(router *mux.Router, serviceBroker ServiceBroker, logger lager.Logger) {
	handler := serviceBrokerHandler{serviceBroker: serviceBroker, logger: logger}
	router.HandleFunc("/v2/catalog", handler.catalog).Methods("GET")

	router.HandleFunc("/v2/service_instances/{instance_id}", handler.getInstance).Methods("GET")
	router.HandleFunc("/v2/service_instances/{instance_id}", handler.provision).Methods("PUT")
	router.HandleFunc("/v2/service_instances/{instance_id}", handler.deprovision).Methods("DELETE")
	router.HandleFunc("/v2/service_instances/{instance_id}/last_operation", handler.lastOperation).Methods("GET")
	router.HandleFunc("/v2/service_instances/{instance_id}", handler.update).Methods("PATCH")

	router.HandleFunc("/v2/service_instances/{instance_id}/service_bindings/{binding_id}", handler.getBinding).Methods("GET")
	router.HandleFunc("/v2/service_instances/{instance_id}/service_bindings/{binding_id}", handler.bind).Methods("PUT")
	router.HandleFunc("/v2/service_instances/{instance_id}/service_bindings/{binding_id}", handler.unbind).Methods("DELETE")

	router.HandleFunc("/v2/service_instances/{instance_id}/service_bindings/{binding_id}/last_operation", handler.lastBindingOperation).Methods("GET")
}

type serviceBrokerHandler struct {
	serviceBroker ServiceBroker
	logger        lager.Logger
}

func (h serviceBrokerHandler) catalog(w http.ResponseWriter, req *http.Request) {
	logger := h.logger.Session(catalogLogKey, lager.Data{})

	if _, err := checkBrokerAPIVersionHdr(req); err != nil {
		logger.Error("Check failed", err)
		h.respond(w, http.StatusPreconditionFailed, ErrorResponse{
			Description: err.Error(),
		})
		logger.Error(apiVersionInvalidKey, err)
		return
	}

	services, err := h.serviceBroker.Services(req.Context())
	if err != nil {
		h.respond(w, http.StatusInternalServerError, ErrorResponse{
			Description: err.Error(),
		})
		return
	}

	catalog := CatalogResponse{
		Services: services,
	}

	h.respond(w, http.StatusOK, catalog)
}

func (h serviceBrokerHandler) provision(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	instanceID := vars["instance_id"]

	logger := h.logger.Session(provisionLogKey, lager.Data{
		instanceIDLogKey: instanceID,
	})

	if _, err := checkBrokerAPIVersionHdr(req); err != nil {
		h.respond(w, http.StatusPreconditionFailed, ErrorResponse{
			Description: err.Error(),
		})
		logger.Error(apiVersionInvalidKey, err)
		return
	}

	var details ProvisionDetails
	if err := json.NewDecoder(req.Body).Decode(&details); err != nil {
		logger.Error(invalidServiceDetailsErrorKey, err)
		h.respond(w, http.StatusUnprocessableEntity, ErrorResponse{
			Description: err.Error(),
		})
		return
	}

	if details.ServiceID == "" {
		logger.Error(serviceIdMissingKey, serviceIdError)
		h.respond(w, http.StatusBadRequest, ErrorResponse{
			Description: serviceIdError.Error(),
		})
		return
	}

	if details.PlanID == "" {
		logger.Error(planIdMissingKey, planIdError)
		h.respond(w, http.StatusBadRequest, ErrorResponse{
			Description: planIdError.Error(),
		})
		return
	}

	valid := false
	services, _ := h.serviceBroker.Services(req.Context())
	for _, service := range services {
		if service.ID == details.ServiceID {
			valid = true
			break
		}
	}
	if !valid {
		logger.Error(invalidServiceID, invalidServiceIDError)
		h.respond(w, http.StatusBadRequest, ErrorResponse{
			Description: invalidServiceIDError.Error(),
		})
		return
	}

	valid = false
	for _, service := range services {
		for _, plan := range service.Plans {
			if plan.ID == details.PlanID {
				valid = true
				break
			}
		}
	}
	if !valid {
		logger.Error(invalidPlanID, invalidPlanIDError)
		h.respond(w, http.StatusBadRequest, ErrorResponse{
			Description: invalidPlanIDError.Error(),
		})
		return
	}

	asyncAllowed := req.FormValue("accepts_incomplete") == "true"

	logger = logger.WithData(lager.Data{
		instanceDetailsLogKey: details,
	})

	provisionResponse, err := h.serviceBroker.Provision(req.Context(), instanceID, details, asyncAllowed)

	if err != nil {
		switch err := err.(type) {
		case *FailureResponse:
			logger.Error(err.LoggerAction(), err)
			h.respond(w, err.ValidatedStatusCode(logger), err.ErrorResponse())
		default:
			logger.Error(unknownErrorKey, err)
			h.respond(w, http.StatusInternalServerError, ErrorResponse{
				Description: err.Error(),
			})
		}
		return
	}

	if provisionResponse.IsAsync {
		h.respond(w, http.StatusAccepted, ProvisioningResponse{
			DashboardURL:  provisionResponse.DashboardURL,
			OperationData: provisionResponse.OperationData,
		})
	} else {
		h.respond(w, http.StatusCreated, ProvisioningResponse{
			DashboardURL: provisionResponse.DashboardURL,
		})
	}
}

func (h serviceBrokerHandler) update(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	instanceID := vars["instance_id"]

	logger := h.logger.Session(updateLogKey, lager.Data{
		instanceIDLogKey: instanceID,
	})

	if _, err := checkBrokerAPIVersionHdr(req); err != nil {
		h.respond(w, http.StatusPreconditionFailed, ErrorResponse{
			Description: err.Error(),
		})
		logger.Error(apiVersionInvalidKey, err)
		return
	}

	var details UpdateDetails
	if err := json.NewDecoder(req.Body).Decode(&details); err != nil {
		h.logger.Error(invalidServiceDetailsErrorKey, err)
		h.respond(w, http.StatusUnprocessableEntity, ErrorResponse{
			Description: err.Error(),
		})
		return
	}

	if details.ServiceID == "" {
		logger.Error(serviceIdMissingKey, serviceIdError)
		h.respond(w, http.StatusBadRequest, ErrorResponse{
			Description: serviceIdError.Error(),
		})
		return
	}

	acceptsIncompleteFlag, _ := strconv.ParseBool(req.URL.Query().Get("accepts_incomplete"))

	updateServiceSpec, err := h.serviceBroker.Update(req.Context(), instanceID, details, acceptsIncompleteFlag)
	if err != nil {
		switch err := err.(type) {
		case *FailureResponse:
			h.logger.Error(err.LoggerAction(), err)
			h.respond(w, err.ValidatedStatusCode(h.logger), err.ErrorResponse())
		default:
			h.logger.Error(unknownErrorKey, err)
			h.respond(w, http.StatusInternalServerError, ErrorResponse{
				Description: err.Error(),
			})
		}
		return
	}

	statusCode := http.StatusOK
	if updateServiceSpec.IsAsync {
		statusCode = http.StatusAccepted
	}
	h.respond(w, statusCode, UpdateResponse{
		OperationData: updateServiceSpec.OperationData,
		DashboardURL:  updateServiceSpec.DashboardURL,
	})
}

func (h serviceBrokerHandler) deprovision(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	instanceID := vars["instance_id"]
	logger := h.logger.Session(deprovisionLogKey, lager.Data{
		instanceIDLogKey: instanceID,
	})

	if _, err := checkBrokerAPIVersionHdr(req); err != nil {
		h.respond(w, http.StatusPreconditionFailed, ErrorResponse{
			Description: err.Error(),
		})
		logger.Error(apiVersionInvalidKey, err)
		return
	}

	details := DeprovisionDetails{
		PlanID:    req.FormValue("plan_id"),
		ServiceID: req.FormValue("service_id"),
	}

	if details.ServiceID == "" {
		h.respond(w, http.StatusBadRequest, ErrorResponse{
			Description: serviceIdError.Error(),
		})
		logger.Error(serviceIdMissingKey, serviceIdError)
		return
	}

	if details.PlanID == "" {
		h.respond(w, http.StatusBadRequest, ErrorResponse{
			Description: planIdError.Error(),
		})
		logger.Error(planIdMissingKey, planIdError)
		return
	}

	asyncAllowed := req.FormValue("accepts_incomplete") == "true"

	deprovisionSpec, err := h.serviceBroker.Deprovision(req.Context(), instanceID, details, asyncAllowed)
	if err != nil {
		switch err := err.(type) {
		case *FailureResponse:
			logger.Error(err.LoggerAction(), err)
			h.respond(w, err.ValidatedStatusCode(logger), err.ErrorResponse())
		default:
			logger.Error(unknownErrorKey, err)
			h.respond(w, http.StatusInternalServerError, ErrorResponse{
				Description: err.Error(),
			})
		}
		return
	}

	if deprovisionSpec.IsAsync {
		h.respond(w, http.StatusAccepted, DeprovisionResponse{OperationData: deprovisionSpec.OperationData})
	} else {
		h.respond(w, http.StatusOK, EmptyResponse{})
	}
}

func (h serviceBrokerHandler) getInstance(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	instanceID := vars["instance_id"]

	logger := h.logger.Session(getInstanceLogKey, lager.Data{
		instanceIDLogKey: instanceID,
	})

	versionCompatibility, err := checkBrokerAPIVersionHdr(req)
	if err != nil {
		h.respond(w, http.StatusPreconditionFailed, ErrorResponse{
			Description: err.Error(),
		})
		logger.Error(apiVersionInvalidKey, err)
		return
	}
	if versionCompatibility.Minor < 14 {
		err = errors.New("get instance endpoint only supported starting with OSB version 2.14")
		h.respond(w, http.StatusPreconditionFailed, ErrorResponse{
			Description: err.Error(),
		})
		logger.Error(apiVersionInvalidKey, err)
		return
	}

	instanceDetails, err := h.serviceBroker.GetInstance(req.Context(), instanceID)
	if err != nil {
		switch err := err.(type) {
		case *FailureResponse:
			logger.Error(err.LoggerAction(), err)
			h.respond(w, err.ValidatedStatusCode(logger), err.ErrorResponse())
		default:
			logger.Error(unknownErrorKey, err)
			h.respond(w, http.StatusInternalServerError, ErrorResponse{
				Description: err.Error(),
			})
		}
		return
	}

	h.respond(w, http.StatusOK, GetInstanceResponse{
		ServiceID:    instanceDetails.ServiceID,
		PlanID:       instanceDetails.PlanID,
		DashboardURL: instanceDetails.DashboardURL,
		Parameters:   instanceDetails.Parameters,
	})
}

func (h serviceBrokerHandler) getBinding(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	instanceID := vars["instance_id"]
	bindingID := vars["binding_id"]

	logger := h.logger.Session(getBindLogKey, lager.Data{
		instanceIDLogKey: instanceID,
		bindingIDLogKey:  bindingID,
	})

	versionCompatibility, err := checkBrokerAPIVersionHdr(req)
	if err != nil {
		h.respond(w, http.StatusPreconditionFailed, ErrorResponse{
			Description: err.Error(),
		})
		logger.Error(apiVersionInvalidKey, err)
		return
	}
	if versionCompatibility.Minor < 14 {
		h.respond(w, http.StatusPreconditionFailed, ErrorResponse{
			Description: "get binding endpoint only supported starting with OSB version 2.14",
		})
		logger.Error(apiVersionInvalidKey, err)
		return
	}

	binding, err := h.serviceBroker.GetBinding(req.Context(), instanceID, bindingID)
	if err != nil {
		switch err := err.(type) {
		case *FailureResponse:
			logger.Error(err.LoggerAction(), err)
			h.respond(w, err.ValidatedStatusCode(logger), err.ErrorResponse())
		default:
			logger.Error(unknownErrorKey, err)
			h.respond(w, http.StatusInternalServerError, ErrorResponse{
				Description: err.Error(),
			})
		}
		return
	}

	h.respond(w, http.StatusOK, GetBindingResponse{
		BindingResponse: BindingResponse{
			Credentials:     binding.Credentials,
			SyslogDrainURL:  binding.SyslogDrainURL,
			RouteServiceURL: binding.RouteServiceURL,
			VolumeMounts:    binding.VolumeMounts,
		},
		Parameters: binding.Parameters,
	})
}

func (h serviceBrokerHandler) bind(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	instanceID := vars["instance_id"]
	bindingID := vars["binding_id"]

	logger := h.logger.Session(bindLogKey, lager.Data{
		instanceIDLogKey: instanceID,
		bindingIDLogKey:  bindingID,
	})

	versionCompatibility, err := checkBrokerAPIVersionHdr(req)
	if err != nil {
		h.respond(w, http.StatusPreconditionFailed, ErrorResponse{
			Description: err.Error(),
		})
		logger.Error(apiVersionInvalidKey, err)
		return
	}

	var details BindDetails
	if err := json.NewDecoder(req.Body).Decode(&details); err != nil {
		logger.Error(invalidBindDetailsErrorKey, err)
		h.respond(w, http.StatusUnprocessableEntity, ErrorResponse{
			Description: err.Error(),
		})
		return
	}

	if details.ServiceID == "" {
		logger.Error(serviceIdMissingKey, serviceIdError)
		h.respond(w, http.StatusBadRequest, ErrorResponse{
			Description: serviceIdError.Error(),
		})
		return
	}

	if details.PlanID == "" {
		logger.Error(planIdMissingKey, planIdError)
		h.respond(w, http.StatusBadRequest, ErrorResponse{
			Description: planIdError.Error(),
		})
		return
	}

	asyncAllowed := false
	if versionCompatibility.Minor >= 14 {
		asyncAllowed = req.FormValue("accepts_incomplete") == "true"
	}

	binding, err := h.serviceBroker.Bind(req.Context(), instanceID, bindingID, details, asyncAllowed)
	if err != nil {
		switch err := err.(type) {
		case *FailureResponse:
			statusCode := err.ValidatedStatusCode(logger)
			errorResponse := err.ErrorResponse()
			if err == ErrInstanceDoesNotExist {
				// work around ErrInstanceDoesNotExist having different pre-refactor behaviour to other actions
				errorResponse = ErrorResponse{
					Description: err.Error(),
				}
				statusCode = http.StatusNotFound
			}
			logger.Error(err.LoggerAction(), err)
			h.respond(w, statusCode, errorResponse)
		default:
			logger.Error(unknownErrorKey, err)
			h.respond(w, http.StatusInternalServerError, ErrorResponse{
				Description: err.Error(),
			})
		}
		return
	}

	if binding.IsAsync {
		h.respond(w, http.StatusAccepted, AsyncBindResponse{
			OperationData: binding.OperationData,
		})
		return
	}

	if versionCompatibility.Minor == 8 || versionCompatibility.Minor == 9 {
		experimentalVols := []ExperimentalVolumeMount{}

		for _, vol := range binding.VolumeMounts {
			experimentalConfig, err := json.Marshal(vol.Device.MountConfig)
			if err != nil {
				logger.Error(unknownErrorKey, err)
				h.respond(w, http.StatusInternalServerError, ErrorResponse{Description: err.Error()})
				return
			}

			experimentalVols = append(experimentalVols, ExperimentalVolumeMount{
				ContainerPath: vol.ContainerDir,
				Mode:          vol.Mode,
				Private: ExperimentalVolumeMountPrivate{
					Driver:  vol.Driver,
					GroupID: vol.Device.VolumeId,
					Config:  string(experimentalConfig),
				},
			})
		}

		experimentalBinding := ExperimentalVolumeMountBindingResponse{
			Credentials:     binding.Credentials,
			RouteServiceURL: binding.RouteServiceURL,
			SyslogDrainURL:  binding.SyslogDrainURL,
			VolumeMounts:    experimentalVols,
		}
		h.respond(w, http.StatusCreated, experimentalBinding)
		return
	}

	h.respond(w, http.StatusCreated, BindingResponse{
		Credentials:     binding.Credentials,
		SyslogDrainURL:  binding.SyslogDrainURL,
		RouteServiceURL: binding.RouteServiceURL,
		VolumeMounts:    binding.VolumeMounts,
	})
}

func (h serviceBrokerHandler) unbind(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	instanceID := vars["instance_id"]
	bindingID := vars["binding_id"]

	logger := h.logger.Session(unbindLogKey, lager.Data{
		instanceIDLogKey: instanceID,
		bindingIDLogKey:  bindingID,
	})

	versionCompatibility, err := checkBrokerAPIVersionHdr(req)
	if err != nil {
		h.respond(w, http.StatusPreconditionFailed, ErrorResponse{
			Description: err.Error(),
		})
		logger.Error(apiVersionInvalidKey, err)
		return
	}

	details := UnbindDetails{
		PlanID:    req.FormValue("plan_id"),
		ServiceID: req.FormValue("service_id"),
	}

	if details.ServiceID == "" {
		h.respond(w, http.StatusBadRequest, ErrorResponse{
			Description: serviceIdError.Error(),
		})
		logger.Error(serviceIdMissingKey, serviceIdError)
		return
	}

	if details.PlanID == "" {
		h.respond(w, http.StatusBadRequest, ErrorResponse{
			Description: planIdError.Error(),
		})
		logger.Error(planIdMissingKey, planIdError)
		return
	}

	asyncAllowed := req.FormValue("accepts_incomplete") == "true"
	if asyncAllowed && versionCompatibility.Minor < 14 {
		h.respond(w, http.StatusUnprocessableEntity, ErrorResponse{
			Description: "async unbinding only supported from OSB version 2.14 and up",
		})
		logger.Error(apiVersionInvalidKey, err)
		return
	}

	unbindResponse, err := h.serviceBroker.Unbind(req.Context(), instanceID, bindingID, details, asyncAllowed)
	if err != nil {
		switch err := err.(type) {
		case *FailureResponse:
			logger.Error(err.LoggerAction(), err)
			h.respond(w, err.ValidatedStatusCode(logger), err.ErrorResponse())
		default:
			logger.Error(unknownErrorKey, err)
			h.respond(w, http.StatusInternalServerError, ErrorResponse{
				Description: err.Error(),
			})
		}
		return
	}

	if unbindResponse.IsAsync {
		h.respond(w, http.StatusAccepted, UnbindResponse{
			OperationData: unbindResponse.OperationData,
		})
	} else {
		h.respond(w, http.StatusOK, EmptyResponse{})
	}

}

func (h serviceBrokerHandler) lastBindingOperation(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	instanceID := vars["instance_id"]
	bindingID := vars["binding_id"]
	pollDetails := PollDetails{
		PlanID:        req.FormValue("plan_id"),
		ServiceID:     req.FormValue("service_id"),
		OperationData: req.FormValue("operation"),
	}

	logger := h.logger.Session(lastBindingOperationLogKey, lager.Data{
		instanceIDLogKey: instanceID,
	})

	versionCompatibility, err := checkBrokerAPIVersionHdr(req)
	if err != nil {
		h.respond(w, http.StatusPreconditionFailed, ErrorResponse{
			Description: err.Error(),
		})
		logger.Error(apiVersionInvalidKey, err)
		return
	}
	if versionCompatibility.Minor < 14 {
		h.respond(w, http.StatusPreconditionFailed, ErrorResponse{
			Description: "get binding endpoint only supported starting with OSB version 2.14",
		})
		logger.Error(apiVersionInvalidKey, err)
		return
	}

	logger.Info("starting-check-for-binding-operation")

	lastOperation, err := h.serviceBroker.LastBindingOperation(req.Context(), instanceID, bindingID, pollDetails)

	if err != nil {
		switch err := err.(type) {
		case *FailureResponse:
			logger.Error(err.LoggerAction(), err)
			h.respond(w, err.ValidatedStatusCode(logger), err.ErrorResponse())
		default:
			logger.Error(unknownErrorKey, err)
			h.respond(w, http.StatusInternalServerError, ErrorResponse{
				Description: err.Error(),
			})
		}
		return
	}

	logger.WithData(lager.Data{"state": lastOperation.State}).Info("done-check-for-binding-operation")

	lastOperationResponse := LastOperationResponse{
		State:       lastOperation.State,
		Description: lastOperation.Description,
	}
	h.respond(w, http.StatusOK, lastOperationResponse)

}

func (h serviceBrokerHandler) lastOperation(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	instanceID := vars["instance_id"]
	pollDetails := PollDetails{
		PlanID:        req.FormValue("plan_id"),
		ServiceID:     req.FormValue("service_id"),
		OperationData: req.FormValue("operation"),
	}

	logger := h.logger.Session(lastOperationLogKey, lager.Data{
		instanceIDLogKey: instanceID,
	})

	if _, err := checkBrokerAPIVersionHdr(req); err != nil {
		h.respond(w, http.StatusPreconditionFailed, ErrorResponse{
			Description: err.Error(),
		})
		logger.Error(apiVersionInvalidKey, err)
		return
	}

	logger.Info("starting-check-for-operation")

	lastOperation, err := h.serviceBroker.LastOperation(req.Context(), instanceID, pollDetails)

	if err != nil {
		switch err := err.(type) {
		case *FailureResponse:
			logger.Error(err.LoggerAction(), err)
			h.respond(w, err.ValidatedStatusCode(logger), err.ErrorResponse())
		default:
			logger.Error(unknownErrorKey, err)
			h.respond(w, http.StatusInternalServerError, ErrorResponse{
				Description: err.Error(),
			})
		}
		return
	}

	logger.WithData(lager.Data{"state": lastOperation.State}).Info("done-check-for-operation")

	lastOperationResponse := LastOperationResponse{
		State:       lastOperation.State,
		Description: lastOperation.Description,
	}

	h.respond(w, http.StatusOK, lastOperationResponse)
}

func (h serviceBrokerHandler) respond(w http.ResponseWriter, status int, response interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	encoder := json.NewEncoder(w)
	err := encoder.Encode(response)
	if err != nil {
		h.logger.Error("encoding response", err, lager.Data{"status": status, "response": response})
	}
}

type brokerVersion struct {
	Major int
	Minor int
}

func checkBrokerAPIVersionHdr(req *http.Request) (brokerVersion, error) {
	var version brokerVersion
	apiVersion := req.Header.Get("X-Broker-API-Version")
	if apiVersion == "" {
		return version, errors.New("X-Broker-API-Version Header not set")
	}
	if n, err := fmt.Sscanf(apiVersion, "%d.%d", &version.Major, &version.Minor); err != nil || n < 2 {
		return version, errors.New("X-Broker-API-Version Header must contain a version")
	}

	if version.Major != 2 {
		return version, errors.New("X-Broker-API-Version Header must be 2.x")
	}
	return version, nil
}