package resources

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Financial-Times/concept-search-api/service"
	"gopkg.in/olivere/elastic.v5"
)

type Handler struct {
	service service.ConceptSearchService
}

type validationError struct {
	msg string
}

func NewValidationError(msg string) validationError {
	return validationError{msg}
}

func (e validationError) Error() string {
	return e.msg
}

func NewHandler(service service.ConceptSearchService) *Handler {
	return &Handler{service}
}

func (h *Handler) ConceptSearch(w http.ResponseWriter, req *http.Request) {
	response := make(map[string]interface{})
	var err error
	var concepts []service.Concept

	mode, foundMode, modeErr := getSingleValueQueryParameter(req, "mode", "autocomplete", "search")
	q, foundQ, qErr := getSingleValueQueryParameter(req, "q")
	conceptTypes, foundConceptTypes := getMultipleValueQueryParameter(req, "type")
	boostType, foundBoostType, boostTypeErr := getSingleValueQueryParameter(req, "boost") // we currently only accept authors, so ignoring the actual boost value
	ids, foundIds := getMultipleValueQueryParameter(req, "ids")

	err = firstError(modeErr, qErr, boostTypeErr)
	if err != nil {
		writeHTTPError(w, http.StatusBadRequest, err)
		return
	}

	if foundMode {
		if !foundConceptTypes {
			err = NewValidationError("invalid or missing parameters for concept search (require type)")
		} else {
			if mode == "search" {
				concepts, err = h.searchConcepts(foundBoostType, foundQ, q, conceptTypes)
			} else if mode == "autocomplete" {
				concepts, err = h.suggestConcepts(foundQ, q, conceptTypes, foundBoostType, boostType)
			}
		}
	} else {
		if foundQ {
			err = NewValidationError("invalid or missing parameters for concept search (q but no mode)")
		} else if foundBoostType {
			err = NewValidationError("invalid or missing parameters for concept search (boost but no mode)")
		} else if foundConceptTypes {
			concepts, err = h.findConceptsByType(conceptTypes)
		} else if foundIds {
			concepts, err = h.service.FindConceptsById(ids)
		} else {
			err = NewValidationError("invalid or missing parameters for concept search")
		}
	}

	if err != nil {
		switch err.(type) {

		case validationError, service.InputError:

			writeHTTPError(w, http.StatusBadRequest, err)

		default:
			if err == service.ErrNoElasticClient || err == elastic.ErrNoClient {
				writeHTTPError(w, http.StatusServiceUnavailable, err)
			} else {

				writeHTTPError(w, http.StatusInternalServerError, err)
			}
		}
		return
	}

	response["concepts"] = concepts
	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) searchConcepts(foundBoostType bool, foundQ bool, q string, conceptTypes []string) ([]service.Concept, error) {
	if foundBoostType {
		return nil, NewValidationError("invalid parameters for concept search (boost not supported for mode=search)")
	} else if foundQ {
		return h.service.SearchConceptByTextAndTypes(q, conceptTypes)
	}
	return nil, NewValidationError("invalid or missing parameters for concept search (require q)")
}

func (h *Handler) suggestConcepts(foundQ bool, q string, conceptTypes []string, foundBoostType bool, boostType string) ([]service.Concept, error) {
	if !foundQ {
		return nil, NewValidationError("invalid or missing parameters for autocomplete concept search (require q)")
	} else if foundBoostType {
		return h.service.SuggestConceptByTextAndTypesWithBoost(q, conceptTypes, boostType)
	}
	return h.service.SuggestConceptByTextAndTypes(q, conceptTypes)
}

func (h *Handler) findConceptsByType(conceptTypes []string) ([]service.Concept, error) {
	if len(conceptTypes) == 1 {
		return h.service.FindAllConceptsByType(conceptTypes[0])
	} else if len(conceptTypes) > 1 {
		return nil, NewValidationError("only a single type is supported by this kind of request")
	}
	return []service.Concept{}, nil
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
