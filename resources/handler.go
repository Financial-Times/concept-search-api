package resources

import (
	"encoding/json"
	"net/http"

	"github.com/Financial-Times/concept-search-api/service"
	log "github.com/Sirupsen/logrus"
)

type Handler struct {
	service service.ConceptSearchService
}

func NewHandler(service service.ConceptSearchService) *Handler {
	return &Handler{service}
}

func (h *Handler) ConceptSearch(w http.ResponseWriter, req *http.Request) {
	query := req.URL.Query()
	log.Infof("query: %v", query)

	values := query["q"]
	conceptTypes := query["type"]
	var concepts []service.Concept
	var err error

	if len(conceptTypes) > 0 && conceptTypes[0] != "" && len(values) == 0 {
		concepts, err = h.service.FindAllConceptsByType(conceptTypes[0])
	} else {
		err = service.ErrInvalidConceptType
	}

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)

		by, e2 := json.Marshal(map[string]string{"error": err.Error()})
		if e2 != nil {
			w.Write(by)
		}

		return
	}

	json.NewEncoder(w).Encode(concepts)
}
