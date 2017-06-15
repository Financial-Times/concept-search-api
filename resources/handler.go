package resources

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/Financial-Times/concept-search-api/service"
	"github.com/olivere/elastic"
)

type Handler struct {
	service service.ConceptSearchService
}

func NewHandler(service service.ConceptSearchService) *Handler {
	return &Handler{service}
}

func (h *Handler) ConceptSearch(w http.ResponseWriter, req *http.Request) {
	response := make(map[string]interface{})
	var searchErr error
	var concepts []service.Concept

	q, foundQ, qErr := getSingleValueQueryParameter(req, "q")
	conceptType, foundConceptType, conceptTypeErr := getSingleValueQueryParameter(req, "type")

	if isAutocompleteRequest(req) {
		if foundQ && foundConceptType {
			ok := checkAndHandleParamErrors(w, qErr, conceptTypeErr)
			if !ok {
				return
			}
			concepts, searchErr = h.service.SuggestConceptByTextAndType(q, conceptType)
		} else {
			writeHTTPError(w, http.StatusBadRequest, errors.New("invalid or missing parameters for autocomplete concept search"))
			return
		}
	} else {
		if foundConceptType && !foundQ {
			ok := checkAndHandleParamErrors(w, conceptTypeErr)
			if !ok {
				return
			}
			concepts, searchErr = h.service.FindAllConceptsByType(conceptType)
		} else {
			writeHTTPError(w, http.StatusBadRequest, errors.New("invalid or missing parameters for concept search"))
			return
		}
	}

	if searchErr != nil {
		if searchErr == service.ErrInvalidConceptType || searchErr == service.ErrEmptyTextParameter {
			writeHTTPError(w, http.StatusBadRequest, searchErr)
		} else if searchErr == elastic.ErrNoClient {
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

func isAutocompleteRequest(req *http.Request) bool {
	mode, foundMode, _ := getSingleValueQueryParameter(req, "mode")
	return foundMode && mode == "autocomplete"
}

func getSingleValueQueryParameter(req *http.Request, param string) (string, bool, error) {
	query := req.URL.Query()
	values, found := query[param]
	if len(values) > 1 {
		return "", found, fmt.Errorf("specified multiple %v query parameters in the URL", param)
	}
	if len(values) < 1 {
		return "", found, nil
	}
	return values[0], found, nil
}

func checkAndHandleParamErrors(w http.ResponseWriter, paramErrs ...error) bool {
	for _, err := range paramErrs {
		if err != nil {
			writeHTTPError(w, http.StatusBadRequest, err)
			return false
		}
	}
	return true
}

func writeHTTPError(w http.ResponseWriter, status int, err error) {
	response := make(map[string]interface{})
	response["message"] = err.Error()
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(response)
}
