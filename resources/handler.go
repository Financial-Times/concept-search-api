package resources

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/Financial-Times/concept-search-api/service"
)

type Handler struct {
	service service.ConceptSearchService
}

func NewHandler(service service.ConceptSearchService) *Handler {
	return &Handler{service}
}

func (h *Handler) ConceptSearch(w http.ResponseWriter, req *http.Request) {
	response := make(map[string]interface{})
	var searchErr, validationErr error
	var concepts []service.Concept

	_, isAutocomplete, modeErr := getSingleValueQueryParameter(req, "mode", "autocomplete")
	q, foundQ, qErr := getSingleValueQueryParameter(req, "q")
	conceptType, foundConceptType, conceptTypeErr := getSingleValueQueryParameter(req, "type")

	err := firstError(modeErr, qErr, conceptTypeErr)
	if err != nil {
		writeHTTPError(w, http.StatusBadRequest, err)
		return
	}

	if isAutocomplete {
		if foundQ && foundConceptType {
			concepts, searchErr = h.service.SuggestConceptByTextAndType(q, conceptType)
		} else {
			validationErr = errors.New("invalid or missing parameters for autocomplete concept search (require type and q)")
		}
	} else {
		if foundConceptType {
			if !foundQ {
				concepts, searchErr = h.service.FindAllConceptsByType(conceptType)
			} else {
				validationErr = errors.New("invalid or missing parameters for concept search (no mode)")
			}
		} else {
			validationErr = errors.New("invalid or missing parameters for concept search (no type)")
		}
	}

	if validationErr != nil {
		writeHTTPError(w, http.StatusBadRequest, validationErr)
		return
	}

	if searchErr != nil {
		if searchErr == service.ErrInvalidConceptType || searchErr == service.ErrEmptyTextParameter {
			writeHTTPError(w, http.StatusBadRequest, searchErr)
		} else if searchErr == service.ErrNoElasticClient {
			writeHTTPError(w, http.StatusServiceUnavailable, searchErr)
		} else {
			writeHTTPError(w, http.StatusInternalServerError, searchErr)
		}
		return
	}

	response["concepts"] = concepts
	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func getSingleValueQueryParameter(req *http.Request, param string, allowed ...string) (string, bool, error) {
	query := req.URL.Query()
	values, found := query[param]
	if len(values) > 1 {
		return "", found, fmt.Errorf("specified multiple %v query parameters in the URL", param)
	}
	if len(values) < 1 {
		return "", found, nil
	}

	v := values[0]
	if len(allowed) > 0 {
		for _, a := range allowed {
			if v == a {
				return v, found, nil
			}
		}

		return "", found, fmt.Errorf("'%s' is not a valid value for parameter '%s'", v, param)
	}

	return v, found, nil
}

func firstError(errors ...error) error {
	for _, err := range errors {
		if err != nil {
			return err
		}
	}

	return nil
}

func writeHTTPError(w http.ResponseWriter, status int, err error) {
	response := make(map[string]interface{})
	response["message"] = err.Error()
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(response)
}
