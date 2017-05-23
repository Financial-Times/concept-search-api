package resources

import (
	"encoding/json"
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
	query := req.URL.Query()

	values := query["q"]
	conceptTypes := query["type"]
	modes := query["mode"]

	response := make(map[string]interface{})
	var err error
	var concepts []service.Concept

	if len(conceptTypes) > 0 && conceptTypes[0] != "" && len(values) == 0 {
		concepts, err = h.service.FindAllConceptsByType(conceptTypes[0])
	} else if len(modes) > 0 && modes[0] == "autocomplete" && len(values) > 0 && len(conceptTypes) == 0 {
		concepts, err = h.service.SuggestConceptByText(values[0])
	} else if len(modes) > 0 && modes[0] == "autocomplete" && len(values) > 0 && len(conceptTypes) >= 0 && conceptTypes[0] != "" {
		concepts, err = h.service.SuggestConceptByTextAndType(values[0], conceptTypes[0])
	} else {
		err = service.ErrInvalidConceptType
	}

	w.Header().Add("Content-Type", "application/json")
	if err != nil {
		response["message"] = err.Error()

		if err == service.ErrInvalidConceptType || err == service.ErrEmptyTextParameter {
			w.WriteHeader(http.StatusBadRequest)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}

	} else {
		response["concepts"] = concepts
	}

	json.NewEncoder(w).Encode(response)
}
