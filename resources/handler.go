package resources

import (
	"encoding/json"
	"net/http"

	"github.com/Financial-Times/concept-search-api/util"

	"strings"

	"github.com/Financial-Times/concept-search-api/service"
	"github.com/olivere/elastic/v7"
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

	mode, foundMode, modeErr := util.GetSingleValueQueryParameter(req, "mode", "search", "text")
	q, foundQ, qErr := util.GetSingleValueQueryParameter(req, "q")
	conceptTypes, foundConceptTypes := util.GetMultipleValueQueryParameter(req, "type")
	boostType, foundBoostType, boostTypeErr := util.GetSingleValueQueryParameter(req, "boost") // we currently only accept authors, so ignoring the actual boost value
	ids, foundIds := util.GetMultipleValueQueryParameter(req, "ids")
	includeDeprecated, _, includeDeprecatedErr := util.GetBoolQueryParameter(req, "include_deprecated", false)
	searchAllAuthorities, _, searchAllErr := util.GetBoolQueryParameter(req, "searchAllAuthorities", false)

	err = util.FirstError(modeErr, qErr, boostTypeErr, includeDeprecatedErr, searchAllErr)
	if err != nil {
		writeHTTPError(w, http.StatusBadRequest, err)
		return
	}
	if foundIds {
		if foundBoostType || foundQ || foundConceptTypes || foundMode {
			err = NewValidationError("invalid parameters, 'ids' cannot be combined with any other parameter")
		} else {
			concepts, err = h.service.FindConceptsById(ids)
		}
	} else {
		if foundMode {
			if !foundConceptTypes {
				err = NewValidationError("invalid or missing parameters for concept search (require type)")
			} else {
				if mode == "search" {
					concepts, err = h.searchConcepts(foundBoostType, boostType, foundQ, q, conceptTypes, searchAllAuthorities, includeDeprecated)
				} else if mode == "text" {
					validationErr := util.ValidateConceptTypesForTextModeSearch(conceptTypes)
					if validationErr != nil {
						err = validationErr
					} else {
						concepts, err = h.searchConceptsInTextMode(foundQ, q, conceptTypes, searchAllAuthorities, includeDeprecated)
					}
				}
			}
		} else {
			if foundQ {
				err = NewValidationError("invalid or missing parameters for concept search (q but no mode)")
			} else if foundBoostType {
				err = NewValidationError("invalid or missing parameters for concept search (boost but no mode)")
			} else if foundConceptTypes {
				concepts, err = h.findConceptsByType(conceptTypes, includeDeprecated, searchAllAuthorities)
			} else {
				err = NewValidationError("invalid or missing parameters for concept search")
			}
		}
	}

	if err != nil {
		switch err.(type) {

		case validationError, util.InputError:

			writeHTTPError(w, http.StatusBadRequest, err)

		default:
			if err == util.ErrNoElasticClient || err == elastic.ErrNoClient {
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

func (h *Handler) searchConcepts(foundBoostType bool, boostType string, foundQ bool, q string, conceptTypes []string, searchAllAuthorities bool, includeDeprecated bool) ([]service.Concept, error) {
	if !foundQ {
		return nil, NewValidationError("invalid or missing parameters for concept search (require q)")
	} else if foundBoostType {
		return h.service.SearchConceptByTextAndTypesWithBoost(q, conceptTypes, boostType, searchAllAuthorities, includeDeprecated)
	}
	return h.service.SearchConceptByTextAndTypes(q, conceptTypes, searchAllAuthorities, includeDeprecated)
}

func (h *Handler) searchConceptsInTextMode(foundQ bool, q string, conceptTypes []string, searchAllAuthorities bool, includeDeprecated bool) ([]service.Concept, error) {
	if !foundQ {
		return nil, NewValidationError("invalid or missing parameters for concept search (require q)")
	}
	return h.service.SearchConceptByTextAndTypesInTextMode(q, conceptTypes, searchAllAuthorities, includeDeprecated)
}

func (h *Handler) findConceptsByType(conceptTypes []string, includeDeprecated bool, searchAllAuthorities bool) ([]service.Concept, error) {
	if len(conceptTypes) == 0 {
		return []service.Concept{}, nil
	}

	if len(conceptTypes) > 1 {
		return nil, NewValidationError("only a single type is supported by this kind of request")
	}

	if strings.Contains(conceptTypes[0], "PublicCompany") {
		return h.service.FindAllConceptsByDirectType(conceptTypes[0], searchAllAuthorities, includeDeprecated)
	}

	return h.service.FindAllConceptsByType(conceptTypes[0], searchAllAuthorities, includeDeprecated)
}

func writeHTTPError(w http.ResponseWriter, status int, err error) {
	response := make(map[string]interface{})
	response["message"] = err.Error()
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(response)
}
