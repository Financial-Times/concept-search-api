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
	"github.com/husobee/vestigo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gopkg.in/olivere/elastic.v5"
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

func (s *mockConceptSearchService) FindConceptsById(ids []string) ([]service.Concept, error) {
	args := s.Called(ids)
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

func (s *mockConceptSearchService) SearchConceptByTextAndTypes(textQuery string, conceptTypes []string) ([]service.Concept, error) {
	args := s.Called(textQuery, conceptTypes)
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

func TestAllConceptsByType(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fontology%2FGenre", nil)

	concepts := dummyConcepts()
	svc := &mockConceptSearchService{}
	svc.On("FindAllConceptsByType", "http://www.ft.com/ontology/Genre").Return(concepts, nil)

	actual := doHttpCall(svc, req)

	assert.Equal(t, http.StatusOK, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := unmarshallResponse(t, actual)

	assert.Len(t, respObject["concepts"], 2, "concepts")
	assert.True(t, reflect.DeepEqual(respObject["concepts"], concepts))
}

func TestAllConceptsByTypeInputError(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fontology%2FFoo", nil)
	svc := &mockConceptSearchService{}
	svc.On("FindAllConceptsByType", mock.AnythingOfType("string")).Return([]service.Concept{}, expectedInputErr)

	actual := doHttpCall(svc, req)

	assert.Equal(t, http.StatusBadRequest, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := unmarshallResponseMessage(t, actual)

	assert.Equal(t, expectedInputErr.Error(), respObject["message"], "error message")
}

func TestAllConceptByTypeNoElasticsearchError(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fontology%2FFoo", nil)
	svc := &mockConceptSearchService{}
	svc.On("FindAllConceptsByType", mock.AnythingOfType("string")).Return([]service.Concept{}, elastic.ErrNoClient)

	actual := doHttpCall(svc, req)

	assert.Equal(t, http.StatusServiceUnavailable, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := unmarshallResponseMessage(t, actual)

	assert.Equal(t, elastic.ErrNoClient.Error(), respObject["message"], "error message")
}

func TestAllConceptByTypeNoElasticsearchClientError(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fontology%2FFoo", nil)
	svc := &mockConceptSearchService{}
	svc.On("FindAllConceptsByType", mock.AnythingOfType("string")).Return([]service.Concept{}, service.ErrNoElasticClient)

	actual := doHttpCall(svc, req)

	assert.Equal(t, http.StatusServiceUnavailable, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := unmarshallResponseMessage(t, actual)

	assert.Equal(t, service.ErrNoElasticClient.Error(), respObject["message"], "error message")
}

func TestAllConceptByTypeServerError(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fontology%2FGenre", nil)

	expectedError := errors.New("Test error")
	svc := &mockConceptSearchService{}
	svc.On("FindAllConceptsByType", mock.AnythingOfType("string")).Return([]service.Concept{}, expectedError)

	actual := doHttpCall(svc, req)

	assert.Equal(t, http.StatusInternalServerError, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := unmarshallResponseMessage(t, actual)

	assert.Equal(t, expectedError.Error(), respObject["message"], "error message")
}

func TestConceptSearchNoParams(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts", nil)
	svc := &mockConceptSearchService{}

	actual := doHttpCall(svc, req)

	assert.Equal(t, http.StatusBadRequest, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := unmarshallResponseMessage(t, actual)

	assert.Equal(t, "invalid or missing parameters for concept search", respObject["message"], "error message")
	svc.AssertExpectations(t)
}

func TestAllConceptsByTypeMultipleTypes(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fontology%2Fperson%2FPerson&type=http%3A%2F%2Fwww.ft.com%2Fontology%2FGenre", nil)
	svc := &mockConceptSearchService{}

	actual := doHttpCall(svc, req)

	assert.Equal(t, http.StatusBadRequest, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := unmarshallResponseMessage(t, actual)

	assert.Equal(t, "only a single type is supported by this kind of request", respObject["message"], "error message")
	svc.AssertExpectations(t)
}

func TestConceptSeachByTypeAndValue(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fontology%2FGenre&q=fast", nil)

	svc := &mockConceptSearchService{}

	actual := doHttpCall(svc, req)

	assert.Equal(t, http.StatusBadRequest, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := unmarshallResponseMessage(t, actual)

	assert.Equal(t, "invalid or missing parameters for concept search (q but no mode)", respObject["message"], "error message")
	svc.AssertExpectations(t)
}

func TestTypeaheadConceptSearchErrorMissingType(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?q=lucy&mode=autocomplete", nil)
	svc := &mockConceptSearchService{}

	actual := doHttpCall(svc, req)

	assert.Equal(t, http.StatusBadRequest, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := unmarshallResponseMessage(t, actual)

	assert.Equal(t, "invalid or missing parameters for concept search (require type)", respObject["message"], "error message")
	svc.AssertExpectations(t)
}

func TestTypeaheadConceptSearchErrorMissingQ(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fproduct%2FBrand&mode=autocomplete", nil)

	svc := &mockConceptSearchService{}

	actual := doHttpCall(svc, req)

	assert.Equal(t, http.StatusBadRequest, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := unmarshallResponseMessage(t, actual)

	assert.Equal(t, "invalid or missing parameters for autocomplete concept search (require q)", respObject["message"], "error message")
	svc.AssertExpectations(t)
}

func TestTypeaheadConceptSearchErrorBoostButModeSearch(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fproduct%2FBrand&mode=search&boost=authors", nil)

	svc := &mockConceptSearchService{}
	actual := doHttpCall(svc, req)

	assert.Equal(t, http.StatusBadRequest, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := unmarshallResponseMessage(t, actual)

	assert.Equal(t, "invalid parameters for concept search (boost not supported for mode=search)", respObject["message"], "error message")
	svc.AssertExpectations(t)
}

func TestTypeaheadConceptSearchErrorInvalidMode(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?q=lucy&type=http%3A%2F%2Fwww.ft.com%2Fproduct%2FBrand&mode=pippo", nil)

	svc := &mockConceptSearchService{}
	actual := doHttpCall(svc, req)

	assert.Equal(t, http.StatusBadRequest, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := unmarshallResponseMessage(t, actual)

	assert.Equal(t, "'pippo' is not a valid value for parameter 'mode'", respObject["message"], "error message")
	svc.AssertExpectations(t)
}

func TestTypeaheadConceptSearchByTextAndSingleType(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fproduct%2FBrand&q=lucy&mode=autocomplete", nil)

	concepts := dummyConcepts()
	svc := &mockConceptSearchService{}
	svc.On("SuggestConceptByTextAndTypes", "lucy", []string{"http://www.ft.com/product/Brand"}).Return(concepts, nil)

	actual := doHttpCall(svc, req)

	assert.Equal(t, http.StatusOK, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := unmarshallResponse(t, actual)

	assert.Len(t, respObject["concepts"], 2, "concepts")
	assert.True(t, reflect.DeepEqual(respObject["concepts"], concepts))
}

func TestTypeaheadConceptSearchByTextAndMultipleTypes(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fontology%2Fperson%2FPerson&type=http%3A%2F%2Fwww.ft.com%2Fontology%2FGenre&q=lucy&mode=autocomplete", nil)

	concepts := dummyConcepts()
	expectedTypes := []string{"http://www.ft.com/ontology/person/Person", "http://www.ft.com/ontology/Genre"}

	svc := &mockConceptSearchService{}
	svc.On("SuggestConceptByTextAndTypes", "lucy", expectedTypes).Return(concepts, nil)

	actual := doHttpCall(svc, req)

	assert.Equal(t, http.StatusOK, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := unmarshallResponse(t, actual)

	assert.Len(t, respObject["concepts"], 2, "concepts")
	assert.True(t, reflect.DeepEqual(respObject["concepts"], concepts))
}

func TestTypeaheadConceptSearchByTextAndMultipleTypesServerError(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fontology%2Fperson%2FPerson&type=http%3A%2F%2Fwww.ft.com%2Fontology%2FGenre&q=lucy&mode=autocomplete", nil)

	svc := &mockConceptSearchService{}
	expectedErr := errors.New("test error")
	expectedTypes := []string{"http://www.ft.com/ontology/person/Person", "http://www.ft.com/ontology/Genre"}
	svc.On("SuggestConceptByTextAndTypes", "lucy", expectedTypes).Return([]service.Concept{}, expectedErr)

	actual := doHttpCall(svc, req)

	assert.Equal(t, http.StatusInternalServerError, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := unmarshallResponseMessage(t, actual)

	assert.Equal(t, "test error", respObject["message"], "error message")
	svc.AssertExpectations(t)
}

func TestTypeaheadConceptSearchByTextAndMultipleTypesInputError(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fontology%2Fperson%2FPerson&type=http%3A%2F%2Fwww.ft.com%2Fontology%2FGenre&q=lucy&mode=autocomplete", nil)

	svc := &mockConceptSearchService{}
	expectedTypes := []string{"http://www.ft.com/ontology/person/Person", "http://www.ft.com/ontology/Genre"}
	svc.On("SuggestConceptByTextAndTypes", "lucy", expectedTypes).Return([]service.Concept{}, expectedInputErr)

	actual := doHttpCall(svc, req)

	assert.Equal(t, http.StatusBadRequest, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := unmarshallResponseMessage(t, actual)

	assert.Equal(t, "computer says no", respObject["message"], "error message")
	svc.AssertExpectations(t)
}

func TestTypeaheadConceptSearchForAuthors(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fontology%2Fperson%2FPerson&q=pippo&mode=autocomplete&boost=authors", nil)
	svc := &mockConceptSearchService{}

	concepts := dummyConcepts()
	svc.On("SuggestConceptByTextAndTypesWithBoost", "pippo", []string{"http://www.ft.com/ontology/person/Person"}, "authors").Return(concepts, nil)

	actual := doHttpCall(svc, req)

	assert.Equal(t, http.StatusOK, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := unmarshallResponse(t, actual)

	assert.Len(t, respObject["concepts"], 2, "concepts")
	assert.True(t, reflect.DeepEqual(respObject["concepts"], concepts))
	svc.AssertExpectations(t)
}

func TestTypeaheadConceptSearchForAuthorsServerError(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fproduct%2FBrand&q=pippo&mode=autocomplete&boost=authors", nil)
	svc := &mockConceptSearchService{}

	expectedErr := errors.New("test error")
	svc.On("SuggestConceptByTextAndTypesWithBoost", "pippo", []string{"http://www.ft.com/product/Brand"}, "authors").Return([]service.Concept{}, expectedErr)

	actual := doHttpCall(svc, req)

	assert.Equal(t, http.StatusInternalServerError, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := unmarshallResponseMessage(t, actual)

	assert.Equal(t, expectedErr.Error(), respObject["message"])
	svc.AssertExpectations(t)
}

func TestTypeaheadInvalidBoost(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fontology%2Fperson%2FPerson&q=pippo&mode=autocomplete&boost=somethingThatWeDontSupport", nil)

	svc := &mockConceptSearchService{}
	svc.On("SuggestConceptByTextAndTypesWithBoost", "pippo", []string{"http://www.ft.com/ontology/person/Person"}, "somethingThatWeDontSupport").Return([]service.Concept{}, expectedInputErr)

	actual := doHttpCall(svc, req)

	assert.Equal(t, http.StatusBadRequest, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := unmarshallResponseMessage(t, actual)

	assert.Equal(t, expectedInputErr.Error(), respObject["message"])
	svc.AssertExpectations(t)
}

func TestTypeaheadMultipleBoostValues(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fontology%2Fperson%2FPerson&q=pippo&mode=autocomplete&boost=somethingThatWeDontSupport&boost=anotherThingWeDontSupport", nil)
	svc := &mockConceptSearchService{}

	actual := doHttpCall(svc, req)

	assert.Equal(t, http.StatusBadRequest, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := unmarshallResponseMessage(t, actual)

	assert.Equal(t, "specified multiple boost query parameters in the URL", respObject["message"])
	svc.AssertExpectations(t)
}

func TestTypeaheadConceptSearchNoText(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?mode=autocomplete&type=http%3A%2F%2Fwww.ft.com%2Fontology%2Fperson%2FPerson", nil)
	svc := &mockConceptSearchService{}

	actual := doHttpCall(svc, req)

	assert.Equal(t, http.StatusBadRequest, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := unmarshallResponseMessage(t, actual)

	assert.Equal(t, "invalid or missing parameters for autocomplete concept search (require q)", respObject["message"], "error message")
	svc.AssertExpectations(t)
}

func TestTypeaheadConceptSearchBoostButNoType(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?mode=autocomplete&boost=authors&q=blah", nil)
	svc := &mockConceptSearchService{}

	actual := doHttpCall(svc, req)

	assert.Equal(t, http.StatusBadRequest, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := unmarshallResponseMessage(t, actual)

	assert.Equal(t, "invalid or missing parameters for concept search (require type)", respObject["message"], "error message")
	svc.AssertExpectations(t)
}

func TestTypeaheadConceptSearchBoostButNoMode(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?type=http://www.ft.com/ontology/Genre&boost=authors", nil)
	svc := &mockConceptSearchService{}

	actual := doHttpCall(svc, req)

	assert.Equal(t, http.StatusBadRequest, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := unmarshallResponseMessage(t, actual)

	assert.Equal(t, "invalid or missing parameters for concept search (boost but no mode)", respObject["message"], "error message")
	svc.AssertExpectations(t)
}

func TestSearchMode(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?type=http://www.ft.com/ontology/person/Person&mode=search&q=pippo", nil)
	svc := &mockConceptSearchService{}

	concepts := dummyConcepts()
	svc.On("SearchConceptByTextAndTypes", "pippo", []string{"http://www.ft.com/ontology/person/Person"}).Return(concepts, nil)

	actual := doHttpCall(svc, req)

	assert.Equal(t, http.StatusOK, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := unmarshallResponse(t, actual)

	assert.Len(t, respObject["concepts"], 2, "concepts")
	assert.True(t, reflect.DeepEqual(respObject["concepts"], concepts))
	svc.AssertExpectations(t)
}

func TestSearchModeWithNoQ(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?type=http://www.ft.com/ontology/Genre&mode=search", nil)
	svc := &mockConceptSearchService{}

	actual := doHttpCall(svc, req)

	assert.Equal(t, http.StatusBadRequest, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := unmarshallResponseMessage(t, actual)

	assert.Equal(t, "invalid or missing parameters for concept search (require q)", respObject["message"], "error message")
	svc.AssertExpectations(t)
}

func TestConceptsById(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?ids=1&ids=2", nil)

	concepts := dummyConcepts()
	svc := &mockConceptSearchService{}
	svc.On("FindConceptsById", []string{"1", "2"}).Return(concepts, nil)

	actual := doHttpCall(svc, req)

	assert.Equal(t, http.StatusOK, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := unmarshallResponse(t, actual)

	assert.Len(t, respObject["concepts"], 2, "concepts")
	assert.True(t, reflect.DeepEqual(respObject["concepts"], concepts))
}

func TestConceptsByIdInputError(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?ids=", nil)

	svc := &mockConceptSearchService{}
	svc.On("FindConceptsById", []string{""}).Return([]service.Concept{}, expectedInputErr)

	actual := doHttpCall(svc, req)

	assert.Equal(t, http.StatusBadRequest, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := unmarshallResponseMessage(t, actual)

	assert.Equal(t, expectedInputErr.Error(), respObject["message"], "error message")
}



func TestConceptsByIdParameterCombinationError(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?ids=1&q=xyz", nil)

	svc := &mockConceptSearchService{}

	actual := doHttpCall(svc, req)

	assert.Equal(t, http.StatusBadRequest, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := unmarshallResponseMessage(t, actual)

	assert.Equal(t, "invalid parameters, 'ids' cannot be combined with any other parameter" , respObject["message"])
	svc.AssertExpectations(t)
}


func TestConceptsByIdNoElasticsearchError(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?ids=1&ids=2", nil)
	svc := &mockConceptSearchService{}
	svc.On("FindConceptsById", []string{"1", "2"}).Return([]service.Concept{}, elastic.ErrNoClient)

	actual := doHttpCall(svc, req)

	assert.Equal(t, http.StatusServiceUnavailable, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := unmarshallResponseMessage(t, actual)

	assert.Equal(t, elastic.ErrNoClient.Error(), respObject["message"], "error message")
}

func TestConceptsByIdNoElasticsearchClientError(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?ids=1&ids=2", nil)
	svc := &mockConceptSearchService{}
	svc.On("FindConceptsById", []string{"1", "2"}).Return([]service.Concept{}, service.ErrNoElasticClient)

	actual := doHttpCall(svc, req)

	assert.Equal(t, http.StatusServiceUnavailable, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := unmarshallResponseMessage(t, actual)

	assert.Equal(t, service.ErrNoElasticClient.Error(), respObject["message"], "error message")
}

func TestConceptsByIdServerError(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?ids=1&ids=2", nil)
	expectedError := errors.New("Test error")
	svc := &mockConceptSearchService{}
	svc.On("FindConceptsById", []string{"1", "2"}).Return([]service.Concept{}, expectedError)

	actual := doHttpCall(svc, req)

	assert.Equal(t, http.StatusInternalServerError, actual.StatusCode, "http status")
	assert.Equal(t, "application/json", actual.Header.Get("Content-Type"), "content-type")

	respObject := unmarshallResponseMessage(t, actual)

	assert.Equal(t, expectedError.Error(), respObject["message"], "error message")
}

func doHttpCall(svc *mockConceptSearchService, req *http.Request) *http.Response {
	endpoint := NewHandler(svc)

	router := vestigo.NewRouter()
	router.Get("/concepts", endpoint.ConceptSearch)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w.Result()
}

func unmarshallResponseMessage(t *testing.T, resp *http.Response) map[string]string {
	respObject := make(map[string]string)
	actualBody, _ := ioutil.ReadAll(resp.Body)
	err := json.Unmarshal(actualBody, &respObject)
	if err != nil {
		t.Errorf("Unmarshalling request response failed. %v", err)
	}
	return respObject
}

func unmarshallResponse(t *testing.T, resp *http.Response) map[string][]service.Concept {
	respObject := make(map[string][]service.Concept)
	actualBody, _ := ioutil.ReadAll(resp.Body)
	err := json.Unmarshal(actualBody, &respObject)
	if err != nil {
		t.Errorf("Unmarshalling request response failed. %v", err)
	}
	return respObject
}
