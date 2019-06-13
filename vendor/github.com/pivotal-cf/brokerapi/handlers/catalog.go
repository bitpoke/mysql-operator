package handlers

import (
	"net/http"

	"github.com/pivotal-cf/brokerapi/domain/apiresponses"
)

func (h *APIHandler) Catalog(w http.ResponseWriter, req *http.Request) {

	services, err := h.serviceBroker.Services(req.Context())
	if err != nil {
		h.respond(w, http.StatusInternalServerError, apiresponses.ErrorResponse{
			Description: err.Error(),
		})
		return
	}

	catalog := apiresponses.CatalogResponse{
		Services: services,
	}

	h.respond(w, http.StatusOK, catalog)
}
