package resources

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/Financial-Times/concept-search-api/service"
	"github.com/gorilla/mux"
	"github.com/husobee/vestigo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	elastic "gopkg.in/olivere/elastic.v5"
)

var (
	expectedInputErr = service.NewInputError("computer says no")
)

type mockConceptSearchService struct {
	mock.Mock
}

func (s *mockConceptSearchService) FindAllConceptsByType(conceptType string) ([]service.Concept, error) {
	args := s.Called(conceptType)
	return args.Get(0).([]service.Concept), args.Error(1)
}

func (s *mockConceptSearchService) SuggestConceptByTextAndTypes(textQuery string, conceptTypes []string) ([]service.Concept, error) {
	args := s.Called(textQuery, conceptTypes)
	return args.Get(0).([]service.Concept), args.Error(1)
}

func (s *mockConceptSearchService) SuggestConceptByTextAndTypesWithBoost(textQuery string, conceptTypes []string, boostType string) ([]service.Concept, error) {
	args := s.Called(textQuery, conceptTypes, boostType)
	return args.Get(0).([]service.Concept), args.Error(1)
}

func (s *mockConceptSearchService) SetElasticClient(client *elastic.Client) {
	s.Called(client)
}

func dummyConcepts() []service.Concept {
	return []service.Concept{
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
}

func TestConceptSearchByType(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fontology%2FGenre", nil)

	concepts := dummyConcepts()
	svc := mockConceptSearchService{}
	svc.On("FindAllConceptsByType", "http://www.ft.com/ontology/Genre").Return(concepts, nil)
	endpoint := NewHandler(&svc)

	router := vestigo.NewRouter()
	router.Get("/concepts", endpoint.ConceptSearch)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	actual := w.Result()

	assert.Equal(t, http.StatusOK, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := make(map[string][]service.Concept)
	actualBody, _ := ioutil.ReadAll(actual.Body)
	err := json.Unmarshal(actualBody, &respObject)
	if err != nil {
		t.Errorf("Unmarshalling request response failed. %v", err)
	}

	assert.Len(t, respObject["concepts"], 2, "concepts")
	assert.True(t, reflect.DeepEqual(respObject["concepts"], concepts))
}

func TestConceptSearchByTypeInputError(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fontology%2FFoo", nil)

	svc := mockConceptSearchService{}
	svc.On("FindAllConceptsByType", mock.AnythingOfType("string")).Return([]service.Concept{}, expectedInputErr)
	endpoint := NewHandler(&svc)

	router := vestigo.NewRouter()
	router.Get("/concepts", endpoint.ConceptSearch)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	actual := w.Result()

	assert.Equal(t, http.StatusBadRequest, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := make(map[string]string)
	actualBody, _ := ioutil.ReadAll(actual.Body)
	err := json.Unmarshal(actualBody, &respObject)
	if err != nil {
		t.Errorf("Unmarshalling request response failed. %v", err)
	}

	assert.Equal(t, expectedInputErr.Error(), respObject["message"], "error message")
}

func TestConceptSearchByTypeNoElasticsearchError(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fontology%2FFoo", nil)

	svc := mockConceptSearchService{}
	svc.On("FindAllConceptsByType", mock.AnythingOfType("string")).Return([]service.Concept{}, elastic.ErrNoClient)
	endpoint := NewHandler(&svc)

	router := mux.NewRouter()
	router.HandleFunc("/concepts", endpoint.ConceptSearch).Methods("GET")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	actual := w.Result()

	assert.Equal(t, http.StatusServiceUnavailable, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := make(map[string]string)
	actualBody, _ := ioutil.ReadAll(actual.Body)
	err := json.Unmarshal(actualBody, &respObject)
	if err != nil {
		t.Errorf("Unmarshalling request response failed. %v", err)
	}

	assert.Equal(t, elastic.ErrNoClient.Error(), respObject["message"], "error message")
}

func TestConceptSearchByTypeNoElasticsearchClientError(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fontology%2FFoo", nil)

	svc := mockConceptSearchService{}
	svc.On("FindAllConceptsByType", mock.AnythingOfType("string")).Return([]service.Concept{}, service.ErrNoElasticClient)
	endpoint := NewHandler(&svc)

	router := mux.NewRouter()
	router.HandleFunc("/concepts", endpoint.ConceptSearch).Methods("GET")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	actual := w.Result()

	assert.Equal(t, http.StatusServiceUnavailable, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := make(map[string]string)
	actualBody, _ := ioutil.ReadAll(actual.Body)
	err := json.Unmarshal(actualBody, &respObject)
	if err != nil {
		t.Errorf("Unmarshalling request response failed. %v", err)
	}

	assert.Equal(t, service.ErrNoElasticClient.Error(), respObject["message"], "error message")
}

func TestConceptSearchByTypeServerError(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fontology%2FGenre", nil)

	expectedError := errors.New("Test error")
	svc := mockConceptSearchService{}
	svc.On("FindAllConceptsByType", mock.AnythingOfType("string")).Return([]service.Concept{}, expectedError)
	endpoint := NewHandler(&svc)

	router := vestigo.NewRouter()
	router.Get("/concepts", endpoint.ConceptSearch)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	actual := w.Result()

	assert.Equal(t, http.StatusInternalServerError, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := make(map[string]string)
	actualBody, _ := ioutil.ReadAll(actual.Body)
	err := json.Unmarshal(actualBody, &respObject)
	if err != nil {
		t.Errorf("Unmarshalling request response failed. %v", err)
	}

	assert.Equal(t, expectedError.Error(), respObject["message"], "error message")
}

func TestConceptSeachByTypeNoType(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts", nil)

	svc := mockConceptSearchService{}
	endpoint := NewHandler(&svc)

	router := vestigo.NewRouter()
	router.Get("/concepts", endpoint.ConceptSearch)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	actual := w.Result()

	assert.Equal(t, http.StatusBadRequest, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := make(map[string]string)
	actualBody, _ := ioutil.ReadAll(actual.Body)
	err := json.Unmarshal(actualBody, &respObject)
	if err != nil {
		t.Errorf("Unmarshalling request response failed. %v", err)
	}

	assert.Equal(t, "invalid or missing parameters for concept search (no type)", respObject["message"], "error message")
	svc.AssertExpectations(t)
}

func TestConceptSeachByTypeMultipleTypes(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fontology%2Fperson%2FPerson&type=http%3A%2F%2Fwww.ft.com%2Fontology%2FGenre", nil)

	svc := mockConceptSearchService{}
	endpoint := NewHandler(&svc)

	router := vestigo.NewRouter()
	router.Get("/concepts", endpoint.ConceptSearch)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	actual := w.Result()

	assert.Equal(t, http.StatusBadRequest, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := make(map[string]string)
	actualBody, _ := ioutil.ReadAll(actual.Body)
	err := json.Unmarshal(actualBody, &respObject)
	if err != nil {
		t.Errorf("Unmarshalling request response failed. %v", err)
	}

	assert.Equal(t, "only a single type is supported by this kind of request", respObject["message"], "error message")
	svc.AssertExpectations(t)
}

func TestConceptSeachByTypeAndValue(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fontology%2FGenre&q=fast", nil)

	svc := mockConceptSearchService{}
	endpoint := NewHandler(&svc)

	router := vestigo.NewRouter()
	router.Get("/concepts", endpoint.ConceptSearch)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	actual := w.Result()

	assert.Equal(t, http.StatusBadRequest, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := make(map[string]string)
	actualBody, _ := ioutil.ReadAll(actual.Body)
	err := json.Unmarshal(actualBody, &respObject)
	if err != nil {
		t.Errorf("Unmarshalling request response failed. %v", err)
	}

	assert.Equal(t, "invalid or missing parameters for concept search (no mode)", respObject["message"], "error message")
	svc.AssertExpectations(t)
}

func TestTypeaheadConceptSearchErrorMissingType(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?q=lucy&mode=autocomplete", nil)

	svc := mockConceptSearchService{}
	endpoint := NewHandler(&svc)

	router := vestigo.NewRouter()
	router.Get("/concepts", endpoint.ConceptSearch)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	actual := w.Result()

	assert.Equal(t, http.StatusBadRequest, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := make(map[string]string)
	actualBody, _ := ioutil.ReadAll(actual.Body)
	err := json.Unmarshal(actualBody, &respObject)
	if err != nil {
		t.Errorf("Unmarshalling request response failed. %v", err)
	}

	assert.Equal(t, "invalid or missing parameters for concept search (no type)", respObject["message"], "error message")
	svc.AssertExpectations(t)
}

func TestTypeaheadConceptSearchByTextAndType(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fproduct%2FBrand&q=lucy&mode=autocomplete", nil)

	concepts := dummyConcepts()
	svc := mockConceptSearchService{}
	svc.On("SuggestConceptByTextAndTypes", "lucy", []string{"http://www.ft.com/product/Brand"}).Return(concepts, nil)
	endpoint := NewHandler(&svc)

	router := vestigo.NewRouter()
	router.Get("/concepts", endpoint.ConceptSearch)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	actual := w.Result()

	assert.Equal(t, http.StatusOK, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := make(map[string][]service.Concept)
	actualBody, _ := ioutil.ReadAll(actual.Body)
	err := json.Unmarshal(actualBody, &respObject)
	if err != nil {
		t.Errorf("Unmarshalling request response failed. %v", err)
	}

	assert.Len(t, respObject["concepts"], 2, "concepts")
	assert.True(t, reflect.DeepEqual(respObject["concepts"], concepts))
}

func TestTypeaheadConceptSearchByTextAndMultipleTypesError(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fontology%2Fperson%2FPerson&type=http%3A%2F%2Fwww.ft.com%2Fontology%2FGenre&q=lucy&mode=autocomplete", nil)

	svc := mockConceptSearchService{}
	expectedTypes := []string{"http://www.ft.com/ontology/person/Person", "http://www.ft.com/ontology/Genre"}
	svc.On("SuggestConceptByTextAndTypes", "lucy", expectedTypes).Return([]service.Concept{}, expectedInputErr)
	endpoint := NewHandler(&svc)

	router := vestigo.NewRouter()
	router.Get("/concepts", endpoint.ConceptSearch)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	actual := w.Result()

	assert.Equal(t, http.StatusBadRequest, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := make(map[string]string)
	actualBody, _ := ioutil.ReadAll(actual.Body)
	err := json.Unmarshal(actualBody, &respObject)
	if err != nil {
		t.Errorf("Unmarshalling request response failed. %v", err)
	}

	assert.Equal(t, "computer says no", respObject["message"], "error message")
	svc.AssertExpectations(t)
}

func TestTypeaheadConceptSearchByMultipleTextAndType(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fontology%2Fperson%2FPerson&q=pippo&q=lucy&mode=autocomplete", nil)

	svc := mockConceptSearchService{}
	endpoint := NewHandler(&svc)

	router := vestigo.NewRouter()
	router.Get("/concepts", endpoint.ConceptSearch)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	actual := w.Result()

	assert.Equal(t, http.StatusBadRequest, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := make(map[string]string)
	actualBody, _ := ioutil.ReadAll(actual.Body)
	err := json.Unmarshal(actualBody, &respObject)
	if err != nil {
		t.Errorf("Unmarshalling request response failed. %v", err)
	}

	assert.Equal(t, "specified multiple q query parameters in the URL", respObject["message"], "error message")
	svc.AssertExpectations(t)
}

func TestTypeaheadConceptSearchForAuthors(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fontology%2Fperson%2FPerson&q=pippo&mode=autocomplete&boost=authors", nil)

	svc := mockConceptSearchService{}
	endpoint := NewHandler(&svc)

	router := vestigo.NewRouter()
	router.Get("/concepts", endpoint.ConceptSearch)

	concepts := dummyConcepts()
	svc.On("SuggestConceptByTextAndTypesWithBoost", "pippo", []string{"http://www.ft.com/ontology/person/Person"}, "authors").Return(concepts, nil)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	actual := w.Result()

	assert.Equal(t, http.StatusOK, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := make(map[string][]service.Concept)
	actualBody, _ := ioutil.ReadAll(actual.Body)
	err := json.Unmarshal(actualBody, &respObject)
	if err != nil {
		t.Errorf("Unmarshalling request response failed. %v", err)
	}

	assert.Len(t, respObject["concepts"], 2, "concepts")
	assert.True(t, reflect.DeepEqual(respObject["concepts"], concepts))
	svc.AssertExpectations(t)
}

func TestTypeaheadInvalidBoost(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fontology%2Fperson%2FPerson&q=pippo&mode=autocomplete&boost=somethingThatWeDontSupport", nil)

	svc := mockConceptSearchService{}
	svc.On("SuggestConceptByTextAndTypesWithBoost", "pippo", []string{"http://www.ft.com/ontology/person/Person"}, "somethingThatWeDontSupport").Return([]service.Concept{}, expectedInputErr)
	endpoint := NewHandler(&svc)

	router := vestigo.NewRouter()
	router.Get("/concepts", endpoint.ConceptSearch)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	actual := w.Result()

	assert.Equal(t, http.StatusBadRequest, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := make(map[string]string)
	actualBody, _ := ioutil.ReadAll(actual.Body)
	err := json.Unmarshal(actualBody, &respObject)
	if err != nil {
		t.Errorf("Unmarshalling request response failed. %v", err)
	}

	assert.Equal(t, expectedInputErr.Error(), respObject["message"])
	svc.AssertExpectations(t)
}

func TestTypeaheadMultipleBoostValues(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fontology%2Fperson%2FPerson&q=pippo&mode=autocomplete&boost=somethingThatWeDontSupport&boost=anotherThingWeDontSupport", nil)

	svc := mockConceptSearchService{}
	endpoint := NewHandler(&svc)

	router := vestigo.NewRouter()
	router.Get("/concepts", endpoint.ConceptSearch)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	actual := w.Result()

	assert.Equal(t, http.StatusBadRequest, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := make(map[string]string)
	actualBody, _ := ioutil.ReadAll(actual.Body)
	err := json.Unmarshal(actualBody, &respObject)
	if err != nil {
		t.Errorf("Unmarshalling request response failed. %v", err)
	}

	assert.Equal(t, "specified multiple boost query parameters in the URL", respObject["message"])
	svc.AssertExpectations(t)
}
