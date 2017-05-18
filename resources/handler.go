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
	response := make(map[string]interface{})
	var err error

	if len(conceptTypes) > 0 && conceptTypes[0] != "" && len(values) == 0 {
		var concepts []service.Concept
		concepts, err = h.service.FindAllConceptsByType(conceptTypes[0])
		if err == nil {
			response["concepts"] = concepts
		}
	} else {
		err = service.ErrInvalidConceptType
	}

	w.Header().Add("Content-Type", "application/json")
	if err != nil {
		response["message"] = err.Error()

		if err == service.ErrInvalidConceptType {
			w.WriteHeader(http.StatusBadRequest)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}

	json.NewEncoder(w).Encode(response)
}
