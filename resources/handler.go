package resources

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/Financial-Times/concept-search-api/service"
	elastic "gopkg.in/olivere/elastic.v5"
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
	conceptTypes, foundConceptTypes := getMultipleValueQueryParameter(req, "type")
	boostType, foundBoostType, boostTypeErr := getSingleValueQueryParameter(req, "boost") // we currently only accept authors, so ignoring the actual boost value

	err := firstError(modeErr, qErr, boostTypeErr)
	if err != nil {
		writeHTTPError(w, http.StatusBadRequest, err)
		return
	}

	if foundConceptTypes {
		if isAutocomplete {
			if foundQ {
				if foundBoostType {
					concepts, searchErr = h.service.SuggestConceptByTextAndTypesWithBoost(q, conceptTypes, boostType)
				} else {
					concepts, searchErr = h.service.SuggestConceptByTextAndTypes(q, conceptTypes)
				}
			} else {
				validationErr = errors.New("invalid or missing parameters for autocomplete concept search (require q)")
			}
		} else {
			if !foundQ && len(conceptTypes) == 1 && !foundBoostType {
				concepts, searchErr = h.service.FindAllConceptsByType(conceptTypes[0])
			} else if len(conceptTypes) > 1 {
				validationErr = errors.New("only a single type is supported by this kind of request")
			} else if foundBoostType {
				validationErr = errors.New("invalid or missing parameters for concept search (boost but no mode)")
			} else {
				validationErr = errors.New("invalid or missing parameters for concept search (q but no mode)")
			}
		}
	} else {
		validationErr = errors.New("invalid or missing parameters for concept search (no type)")
	}

	if validationErr != nil {
		writeHTTPError(w, http.StatusBadRequest, validationErr)
		return
	}

	if searchErr != nil {
		switch searchErr.(type) {
		case service.InputError:
			writeHTTPError(w, http.StatusBadRequest, searchErr)
		default:
			if searchErr == service.ErrNoElasticClient || searchErr == elastic.ErrNoClient {
				writeHTTPError(w, http.StatusServiceUnavailable, searchErr)
			} else {
				writeHTTPError(w, http.StatusInternalServerError, searchErr)
			}
		}
		return
	}

	response["concepts"] = concepts
	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func getSingleValueQueryParameter(req *http.Request, param string, allowed ...string) (string, bool, error) {
	values, found := getMultipleValueQueryParameter(req, param)
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

func getMultipleValueQueryParameter(req *http.Request, param string) ([]string, bool) {
	query := req.URL.Query()
	values, found := query[param]
	return values, found
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
