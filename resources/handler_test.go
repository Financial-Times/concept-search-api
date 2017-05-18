package resources

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/Financial-Times/concept-search-api/service"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

type dummyConceptSearchService struct {
	concepts []service.Concept
	err      error
}

func (s *dummyConceptSearchService) FindAllConceptsByType(conceptType string) ([]service.Concept, error) {
	return s.concepts, s.err
}

func TestConceptSearchByType(t *testing.T) {
	req, err := http.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fontology%2FGenre", nil)
	if err != nil {
		t.Fatal(err)
	}

	genres := []service.Concept{
		service.Concept{
			Id:          "http://api.ft.com/things/1",
			ApiUrl:      "http://api.ft.com/things/1",
			PrefLabel:   "Test Genre 1",
			ConceptType: "http://www.ft.com/ontology/Genre",
		},
		service.Concept{
			Id:          "http://api.ft.com/things/2",
			ApiUrl:      "http://api.ft.com/things/2",
			PrefLabel:   "Test Genre 2",
			ConceptType: "http://www.ft.com/ontology/Genre",
		},
	}

	dummyService := &dummyConceptSearchService{concepts: genres}
	endpoint := NewHandler(dummyService)

	router := mux.NewRouter()
	router.HandleFunc("/concepts", endpoint.ConceptSearch).Methods("GET")

	actual := httptest.NewRecorder()
	router.ServeHTTP(actual, req)

	assert.Equal(t, http.StatusOK, actual.Code, "http status")
	assert.Equal(t, "application/json", actual.Header().Get("Content-Type"), "content-type")

	respObject := make(map[string][]service.Concept)
	err = json.Unmarshal(actual.Body.Bytes(), &respObject)
	if err != nil {
		t.Errorf("Unmarshalling request response failed. %v", err)
	}

	assert.Len(t, respObject["concepts"], 2, "concepts")
	assert.True(t, reflect.DeepEqual(respObject["concepts"], genres))
}

func TestConceptSearchByTypeClientError(t *testing.T) {
	req, err := http.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fontology%2FFoo", nil)
	if err != nil {
		t.Fatal(err)
	}

	dummyService := &dummyConceptSearchService{err: service.ErrInvalidConceptType}
	endpoint := NewHandler(dummyService)

	router := mux.NewRouter()
	router.HandleFunc("/concepts", endpoint.ConceptSearch).Methods("GET")

	actual := httptest.NewRecorder()
	router.ServeHTTP(actual, req)

	assert.Equal(t, http.StatusBadRequest, actual.Code, "http status")
	assert.Equal(t, "application/json", actual.Header().Get("Content-Type"), "content-type")

	respObject := make(map[string]string)
	err = json.Unmarshal(actual.Body.Bytes(), &respObject)
	if err != nil {
		t.Errorf("Unmarshalling request response failed. %v", err)
	}

	assert.Equal(t, service.ErrInvalidConceptType.Error(), respObject["message"], "error message")
}

func TestConceptSearchByTypeServerError(t *testing.T) {
	req, err := http.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fontology%2FGenre", nil)
	if err != nil {
		t.Fatal(err)
	}

	expectedError := errors.New("Test error")
	dummyService := &dummyConceptSearchService{err: expectedError}
	endpoint := NewHandler(dummyService)

	router := mux.NewRouter()
	router.HandleFunc("/concepts", endpoint.ConceptSearch).Methods("GET")

	actual := httptest.NewRecorder()
	router.ServeHTTP(actual, req)

	assert.Equal(t, http.StatusInternalServerError, actual.Code, "http status")
	assert.Equal(t, "application/json", actual.Header().Get("Content-Type"), "content-type")

	respObject := make(map[string]string)
	err = json.Unmarshal(actual.Body.Bytes(), &respObject)
	if err != nil {
		t.Errorf("Unmarshalling request response failed. %v", err)
	}

	assert.Equal(t, expectedError.Error(), respObject["message"], "error message")
}
