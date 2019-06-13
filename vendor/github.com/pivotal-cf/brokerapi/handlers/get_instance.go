package handlers

import (
	"errors"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/gorilla/mux"
	"github.com/pivotal-cf/brokerapi/domain/apiresponses"
)

const getInstanceLogKey = "getInstance"

func (h APIHandler) GetInstance(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	instanceID := vars["instance_id"]

	logger := h.logger.Session(getInstanceLogKey, lager.Data{
		instanceIDLogKey: instanceID,
	})

	version := getAPIVersion(req)
	if version.Minor < 14 {
		err := errors.New("get instance endpoint only supported starting with OSB version 2.14")
		h.respond(w, http.StatusPreconditionFailed, apiresponses.ErrorResponse{
			Description: err.Error(),
		})
		logger.Error(apiVersionInvalidKey, err)
		return
	}

	instanceDetails, err := h.serviceBroker.GetInstance(req.Context(), instanceID)
	if err != nil {
		switch err := err.(type) {
		case *apiresponses.FailureResponse:
			logger.Error(err.LoggerAction(), err)
			h.respond(w, err.ValidatedStatusCode(logger), err.ErrorResponse())
		default:
			logger.Error(unknownErrorKey, err)
			h.respond(w, http.StatusInternalServerError, apiresponses.ErrorResponse{
				Description: err.Error(),
			})
		}
		return
	}

	h.respond(w, http.StatusOK, apiresponses.GetInstanceResponse{
		ServiceID:    instanceDetails.ServiceID,
		PlanID:       instanceDetails.PlanID,
		DashboardURL: instanceDetails.DashboardURL,
		Parameters:   instanceDetails.Parameters,
	})
}
