//go:build integration
// +build integration

package main

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	"log"

	"github.com/stretchr/testify/assert"
	"gopkg.in/olivere/elastic.v5"
)

func TestConceptFinder(t *testing.T) {

	testCases := []struct {
		client        esClient
		returnCode    int
		requestURL    string
		requestBody   string
		expectedUUIDs []string
		expectedScore []float64
		assertFields  map[string]func(concept)
	}{
		{
			returnCode:  http.StatusInternalServerError,
			requestURL:  defaultRequestURL,
			requestBody: validRequestBody,
		},
		{
			client:      failClient{},
			returnCode:  http.StatusBadRequest,
			requestURL:  defaultRequestURL,
			requestBody: invalidRequestBody,
		},
		{
			client:      failClient{},
			returnCode:  http.StatusInternalServerError,
			requestURL:  defaultRequestURL,
			requestBody: validRequestBody,
		},
		{
			client: mockClient{
				queryResponse: validResponse,
			},
			returnCode:    http.StatusOK,
			requestURL:    defaultRequestURL,
			requestBody:   validRequestBody,
			expectedUUIDs: []string{"9a0dd8b8-2ae4-34ca-8639-cfef69711eb9", "6084734d-f4c2-3375-b298-dbbc6c00a680"},
			assertFields: map[string]func(concept){
				"9a0dd8b8-2ae4-34ca-8639-cfef69711eb9": func(c concept) {
					assert.Equal(t, "Foobar SpA", c.PrefLabel)
					assert.Equal(t, "", c.ScopeNote)
					assert.Equal(t, "http://www.ft.com/ontology/company/PublicCompany", c.DirectType)
					assert.Equal(t, "CA", c.CountryCode)
					assert.Equal(t, "US", c.CountryOfIncorporation)
				}},
		},
		{
			client: mockClient{
				queryResponse: emptyResponse,
			},
			returnCode:  http.StatusNotFound,
			requestURL:  defaultRequestURL,
			requestBody: validRequestBody,
		},
		{
			client: mockClient{
				queryResponse: validResponse,
			},
			returnCode:    http.StatusOK,
			requestURL:    requestURLWithScore,
			requestBody:   validRequestBody,
			expectedUUIDs: []string{"9a0dd8b8-2ae4-34ca-8639-cfef69711eb9", "6084734d-f4c2-3375-b298-dbbc6c00a680"},
			expectedScore: []float64{9.992676, 2.68152},
		},
		{
			client: mockClient{
				queryResponse: validResponseDeprecated,
			},
			returnCode:    http.StatusOK,
			requestURL:    requestURLWithScoreAndDeprecated,
			requestBody:   validRequestBodyForDeprecated,
			expectedUUIDs: []string{"74877f31-6c39-4e07-a85a-39236354a93e"},
			expectedScore: []float64{113.70959},
		},
		{
			client: mockClient{
				queryResponse: validResponse,
			},
			returnCode:    http.StatusOK,
			requestURL:    requestURLWithAllAuthorities,
			requestBody:   validRequestBody,
			expectedUUIDs: []string{"9a0dd8b8-2ae4-34ca-8639-cfef69711eb9", "6084734d-f4c2-3375-b298-dbbc6c00a680"},
			assertFields: map[string]func(concept){
				"9a0dd8b8-2ae4-34ca-8639-cfef69711eb9": func(c concept) {
					assert.Equal(t, "Foobar SpA", c.PrefLabel)
					assert.Equal(t, "", c.ScopeNote)
					assert.Equal(t, "http://www.ft.com/ontology/company/PublicCompany", c.DirectType)
					assert.Equal(t, "CA", c.CountryCode)
					assert.Equal(t, "US", c.CountryOfIncorporation)
				}},
		},
		{
			client: mockClient{
				queryResponse: invalidResponseBadHits,
			},
			returnCode:  http.StatusInternalServerError,
			requestURL:  defaultRequestURL,
			requestBody: validRequestBody,
		},
		{
			client: mockClient{
				queryResponse: invvalidResponseBadConcept,
			},
			returnCode:  http.StatusInternalServerError,
			requestURL:  defaultRequestURL,
			requestBody: validRequestBody,
		},
		{
			client:      failClient{},
			returnCode:  http.StatusBadRequest,
			requestURL:  defaultRequestURL,
			requestBody: missingTermRequestBody,
		},
	}

	for _, testCase := range testCases {
		conceptFinder := &esConceptFinder{
			defaultIndex:      "concept",
			searchResultLimit: 50,
			lockClient:        &sync.RWMutex{},
		}
		conceptFinder.client = testCase.client

		req, _ := http.NewRequest("POST", testCase.requestURL, strings.NewReader(testCase.requestBody))
		w := httptest.NewRecorder()

		conceptFinder.FindConcept(w, req)

		assert.Equal(t, testCase.returnCode, w.Code, "Expected return code %d but got %d", testCase.returnCode, w.Code)
		if testCase.returnCode != http.StatusOK {
			continue
		}

		var searchResults searchResult
		err := json.Unmarshal(w.Body.Bytes(), &searchResults)
		assert.Equal(t, nil, err)
		assert.Equal(t, len(testCase.expectedUUIDs), len(searchResults.Results))

		for i, uuid := range testCase.expectedUUIDs {
			assert.True(t, strings.Contains(searchResults.Results[i].ID, uuid))
			if testCase.requestURL == requestURLWithScoreAndDeprecated {
				assert.True(t, searchResults.Results[i].IsDeprecated)
			}
			if testCase.assertFields != nil {
				assertFields, found := testCase.assertFields[uuid]
				if found {
					assertFields(searchResults.Results[i])
				}
			}
		}

		if testCase.requestURL == requestURLWithScore ||
			testCase.requestURL == requestURLWithScoreAndDeprecated {
			for i, score := range testCase.expectedScore {
				assert.Equal(t, score, searchResults.Results[i].Score)
			}
		}

	}
}

func TestConceptFinderForBestMatch(t *testing.T) {

	testCases := []struct {
		testName            string
		client              esClient
		returnCode          int
		requestURL          string
		requestBody         string
		expectedUUIDs       map[string][]string
		extraAssertionLogic func(t *testing.T, searchResults map[string][]concept)
	}{
		{
			testName:    "TooMuchDataInPayload",
			client:      mockClient{},
			returnCode:  http.StatusBadRequest,
			requestURL:  defaultRequestURL,
			requestBody: `{"term":"Foobar", "bestMatchTerms":["testTerm"]}`,
		},
		{
			testName:    "WrongType",
			client:      mockClient{},
			returnCode:  http.StatusBadRequest,
			requestURL:  defaultRequestURL,
			requestBody: `{"bestMatchTerms":["testTerm"], "conceptTypes": ["http://www.ft.com/ontology/organisation/NotExisting"]}`,
		},
		{
			testName:    "WrongBoost",
			client:      mockClient{},
			returnCode:  http.StatusBadRequest,
			requestURL:  defaultRequestURL,
			requestBody: `{"bestMatchTerms":["testTerm"], "boost":"wrong_boost"}`,
		},
		{
			testName:    "WrongBoostTypeCombination",
			client:      mockClient{},
			returnCode:  http.StatusBadRequest,
			requestURL:  defaultRequestURL,
			requestBody: `{"bestMatchTerms":["testTerm"], "boost":"wrong_boost","conceptTypes": ["http://www.ft.com/ontology/organisation/Organisation"]}`,
		},
		{
			testName:    "WrongFilterTypeCombination",
			client:      mockClient{},
			returnCode:  http.StatusBadRequest,
			requestURL:  defaultRequestURL,
			requestBody: `{"bestMatchTerms":["testTerm"], "filter": "authors", "conceptTypes": ["http://www.ft.com/ontology/organisation/Organisation"]}`,
		},
		{
			testName:    "BoostWithoutType",
			client:      mockClient{},
			returnCode:  http.StatusBadRequest,
			requestURL:  defaultRequestURL,
			requestBody: `{"bestMatchTerms":["testTerm"], "boost": "authors"}`,
		},
		{
			testName:    "ErrorFromES",
			client:      failClient{},
			returnCode:  http.StatusInternalServerError,
			requestURL:  defaultRequestURL,
			requestBody: `{"bestMatchTerms":["testTerm"]}`,
		},
		{
			testName: "OkPeopleType",
			client: mockClient{
				queryResponse: validResponseBestMatch,
			},
			returnCode:  http.StatusOK,
			requestURL:  defaultRequestURL,
			requestBody: `{"bestMatchTerms":["Adam Samson", "Eric Platt", "Michael Hunter"], "conceptTypes": ["http://www.ft.com/ontology/person/Person"]}`,
			expectedUUIDs: map[string][]string{
				"Adam Samson": []string{
					"f758ef56-c40a-3162-91aa-3e8a3aabc494",
				},
				"Eric Platt": []string{
					"40281396-8369-4699-ae48-1ccc0c931a72",
				},
				"Michael Hunter": []string{
					"9332270e-f959-3f55-9153-d30acd0d0a51",
				},
			},
			extraAssertionLogic: func(t *testing.T, searchResults map[string][]concept) {
				for _, concepts := range searchResults {
					for _, res := range concepts {
						_, err := strconv.ParseBool(res.IsFTAuthor)
						assert.Error(t, err, "isFtAuthor shouldn't be included")
					}
				}
			},
		},
		{
			testName: "OkPeopleTypePartialResults",
			client: mockClient{
				queryResponse: validResponseBestMatchPartialResults,
			},
			returnCode:  http.StatusOK,
			requestURL:  defaultRequestURL,
			requestBody: `{"bestMatchTerms":["Adam Samson", "Eric Platt", "Michael Hunter"], "conceptTypes": ["http://www.ft.com/ontology/person/Person"]}`,
			expectedUUIDs: map[string][]string{
				"Adam Samson": []string{
					"f758ef56-c40a-3162-91aa-3e8a3aabc494",
				},
				"Eric Platt": []string{},
				"Michael Hunter": []string{
					"9332270e-f959-3f55-9153-d30acd0d0a51",
				},
			},
			extraAssertionLogic: func(t *testing.T, searchResults map[string][]concept) {
				for _, concepts := range searchResults {
					for _, res := range concepts {
						_, err := strconv.ParseBool(res.IsFTAuthor)
						assert.Error(t, err, "isFtAuthor shouldn't be included")
					}
				}
			},
		},
		{
			testName: "OkPeopleTypeNoResults",
			client: mockClient{
				queryResponse: validResponseBestMatchNoResults,
			},
			returnCode:  http.StatusNotFound,
			requestURL:  defaultRequestURL,
			requestBody: `{"bestMatchTerms":["Adam Samson", "Eric Platt", "Michael Hunter"], "conceptTypes": ["http://www.ft.com/ontology/person/Person"]}`,
			expectedUUIDs: map[string][]string{
				"Adam Samson":    []string{},
				"Eric Platt":     []string{},
				"Michael Hunter": []string{},
			},
			extraAssertionLogic: func(t *testing.T, searchResults map[string][]concept) {
				for _, concepts := range searchResults {
					for _, res := range concepts {
						_, err := strconv.ParseBool(res.IsFTAuthor)
						assert.Error(t, err, "isFtAuthor shouldn't be included")
					}
				}
			},
		},
		{
			testName: "IncludeFtAuthorQueryParam",
			client: mockClient{
				queryResponse: validResponseBestMatch,
			},
			returnCode:  http.StatusOK,
			requestURL:  defaultRequestURL + "?include_field=authors",
			requestBody: `{"bestMatchTerms":["Adam Samson", "Eric Platt", "Michael Hunter"], "conceptTypes": ["http://www.ft.com/ontology/person/Person"]}`,
			expectedUUIDs: map[string][]string{
				"Adam Samson": []string{
					"f758ef56-c40a-3162-91aa-3e8a3aabc494",
				},
				"Eric Platt": []string{
					"40281396-8369-4699-ae48-1ccc0c931a72",
				},
				"Michael Hunter": []string{
					"9332270e-f959-3f55-9153-d30acd0d0a51",
				},
			},
			extraAssertionLogic: func(t *testing.T, searchResults map[string][]concept) {
				notAuthorCounter := 0
				authorCounter := 0
				for _, concepts := range searchResults {
					for _, res := range concepts {
						isFtAuthor, err := strconv.ParseBool(res.IsFTAuthor)
						assert.NoError(t, err)
						if isFtAuthor {
							authorCounter++
						} else {
							notAuthorCounter++
						}
					}
				}
				assert.Equal(t, 1, notAuthorCounter)
				assert.Equal(t, 2, authorCounter)
			},
		},
	}

	for _, testCase := range testCases {
		conceptFinder := &esConceptFinder{
			defaultIndex:      "concept",
			searchResultLimit: 50,
			lockClient:        &sync.RWMutex{},
		}
		conceptFinder.client = testCase.client

		req, _ := http.NewRequest("POST", testCase.requestURL, strings.NewReader(testCase.requestBody))
		w := httptest.NewRecorder()

		conceptFinder.FindConcept(w, req)

		assert.Equal(t, testCase.returnCode, w.Code, "%s -> Expected return code %d but got %d", testCase.testName, testCase.returnCode, w.Code)
		if testCase.returnCode != http.StatusOK {
			continue
		}

		var searchResults map[string][]concept
		err := json.Unmarshal(w.Body.Bytes(), &searchResults)
		assert.Equal(t, nil, err, "%s -> expected no error", testCase.testName)
		assert.Equal(t, len(testCase.expectedUUIDs), len(searchResults), "%s -> different no. of results", testCase.testName)
		for searchTerm, searchTermExpectedUUIDs := range testCase.expectedUUIDs {
			actualTermUUIDs, ok := searchResults[searchTerm]
			assert.True(t, ok, "%s -> expected values for %s", testCase.testName, searchTerm)
			for i, uuid := range searchTermExpectedUUIDs {
				assert.Contains(t, actualTermUUIDs[i].ID, uuid, "%s -> uuid expectation", testCase.testName)
			}
		}

		if testCase.extraAssertionLogic != nil {
			testCase.extraAssertionLogic(t, searchResults)
		}

	}
}

// during concept deprecation story an issue was encountered during calling FindConcept.
// The filtering was applied in a way that the data was returned even when the query did not match the doc.
func TestEsQueryScore(t *testing.T) {
	// create ES client
	ec, err := elastic.NewClient(
		elastic.SetURL(getElasticSearchTestURL(t)),
		elastic.SetSniff(false),
	)
	assert.NoError(t, err, "expected no error for ES client")

	// cleanup for accuracy
	ec.DeleteIndex(filterScoreTestingIndexName).Do(context.Background())

	// store testing data
	for uuid, conceptBody := range filterScoreTestingData {
		_, e := ec.Index().
			Index(filterScoreTestingIndexName).
			Type("people").
			BodyString(conceptBody).
			Id(uuid).
			Do(context.Background())
		assert.NoError(t, e, "expected no error for ES client")
	}
	ec.Refresh(filterScoreTestingIndexName).Do(context.TODO())

	// prepare request and trigger this
	req, _ := http.NewRequest("POST", "http://dummy_host/concepts?include_score=true", strings.NewReader(`{"term": "Anna"}`))
	w := httptest.NewRecorder()
	conceptFinder := newConceptFinder(filterScoreTestingIndexName, "", 10)
	conceptFinder.SetElasticClient(ec)
	conceptFinder.FindConcept(w, req)

	// check
	assert.Equal(t, http.StatusOK, w.Code)
	var searchResults searchResult
	err = json.Unmarshal(w.Body.Bytes(), &searchResults)
	assert.Equal(t, nil, err)
	assert.Len(t, searchResults.Results, 1)
	assert.Equal(t, "Anna Whitwham", searchResults.Results[0].PrefLabel)
}

func TestEsBestMatchImpl(t *testing.T) {
	// create ES client
	ec, err := elastic.NewClient(
		elastic.SetURL(getElasticSearchTestURL(t)),
		elastic.SetSniff(false),
	)
	assert.NoError(t, err, "expected no error for ES client")

	// cleanup for accuracy
	ec.DeleteIndex(bestMatchIndexName).Do(context.Background())
	err = createIndex(ec, "service/test/mapping.json", bestMatchIndexName)

	// store testing data
	for uuid, conceptBody := range bestMatchTestingData {
		_, e := ec.Index().
			Index(bestMatchIndexName).
			Type("people").
			BodyString(conceptBody).
			Id(uuid).
			Do(context.Background())
		assert.NoError(t, e, "expected no error for ES client")
	}
	ec.Refresh(bestMatchIndexName).Do(context.TODO())

	// prepare request and trigger this
	req, _ := http.NewRequest("POST", "http://dummy_host/concepts?include_deprecated=true", strings.NewReader(`
		{
			"bestMatchTerms":[
				"Platt Eric",
				"Michael Hunter",
				"Samson Adam",
				"Rick And Morty"
			],
			"conceptTypes": ["http://www.ft.com/ontology/person/Person"]
		}`))
	w := httptest.NewRecorder()
	conceptFinder := newConceptFinder(bestMatchIndexName, "", 10)
	conceptFinder.SetElasticClient(ec)
	conceptFinder.FindConcept(w, req)

	// check
	assert.Equal(t, http.StatusOK, w.Code)
	var searchResults map[string][]concept
	err = json.Unmarshal(w.Body.Bytes(), &searchResults)
	assert.Equal(t, nil, err)
	assert.Len(t, searchResults, 4)

	// assert for uuids
	ericPlattConcepts, ok := searchResults["Platt Eric"]
	assert.True(t, ok, "expected results for Platt Eric")
	assert.Len(t, ericPlattConcepts, 1, "expected 1 concept for Platt Eric")
	assert.Equal(t, "http://api.ft.com/things/64302452-e369-4ddb-88fa-9adc5124a38c", ericPlattConcepts[0].ID)

	adamSamsonConcepts, ok := searchResults["Samson Adam"]
	assert.True(t, ok, "expected results for Adam Samson")
	assert.Len(t, adamSamsonConcepts, 1, "expected 1 concept for Adam Samson")
	assert.Equal(t, "http://api.ft.com/things/f758ef56-c40a-3162-91aa-3e8a3aabc494", adamSamsonConcepts[0].ID)

	michaelHunterConcepts, ok := searchResults["Michael Hunter"]
	assert.True(t, ok, "expected results for Michael Hunter")
	assert.Len(t, michaelHunterConcepts, 1, "expected 1 concept for Michael Hunter")
	assert.Equal(t, "http://api.ft.com/things/9332270e-f959-3f55-9153-d30acd0d0a51", michaelHunterConcepts[0].ID)

	rickAndMortyConcepts, ok := searchResults["Rick And Morty"]
	assert.True(t, ok, "expected results for Rick And Morty")
	assert.Len(t, rickAndMortyConcepts, 1, "expected 1 concept for Rick And Morty")
	assert.Equal(t, "http://api.ft.com/things/40281396-8369-4699-ae48-1ccc0c931b55", rickAndMortyConcepts[0].ID)

	// check for `boost`
	req, _ = http.NewRequest("POST", "http://dummy_host/concepts", strings.NewReader(`
		{
			"bestMatchTerms":[
				"Platt Eric",
				"Michael Hunter",
				"Samson Adam",
				"Rick And Morty"
			],
			"conceptTypes": ["http://www.ft.com/ontology/person/Person"],
			"boost": "authors"
		}`))
	w = httptest.NewRecorder()
	conceptFinder.FindConcept(w, req)

	// check
	assert.Equal(t, http.StatusOK, w.Code)
	searchResults = make(map[string][]concept)
	err = json.Unmarshal(w.Body.Bytes(), &searchResults)
	assert.Equal(t, nil, err)
	assert.Len(t, searchResults, 4)

	ericPlattConcepts, ok = searchResults["Platt Eric"]
	assert.True(t, ok, "expected results for Platt Eric")
	assert.Len(t, ericPlattConcepts, 1, "expected 1 concept for Platt Eric")
	assert.Equal(t, "http://api.ft.com/things/64302452-e369-4ddb-88fa-9adc5124a38c", ericPlattConcepts[0].ID)

	adamSamsonConcepts, ok = searchResults["Samson Adam"]
	assert.True(t, ok, "expected results for Adam Samson")
	assert.Len(t, adamSamsonConcepts, 1, "expected 1 concept for Adam Samson")
	assert.Equal(t, "http://api.ft.com/things/f758ef56-c40a-3162-91aa-3e8a3aabc494", adamSamsonConcepts[0].ID)

	michaelHunterConcepts, ok = searchResults["Michael Hunter"]
	assert.True(t, ok, "expected results for Michael Hunter")
	assert.Len(t, michaelHunterConcepts, 1, "expected 1 concept for Michael Hunter")
	assert.Equal(t, "http://api.ft.com/things/9332270e-f959-3f55-9153-d30acd0d0a51", michaelHunterConcepts[0].ID)

	rickAndMortyConcepts, ok = searchResults["Rick And Morty"]
	assert.True(t, ok, "expected results for Rick And Morty")
	assert.Len(t, rickAndMortyConcepts, 0, "expected 0 concept for Rick And Morty")

	// check for `filter`
	req, _ = http.NewRequest("POST", "http://dummy_host/concepts", strings.NewReader(`
		{
			"bestMatchTerms":[
				"Platt Eric",
				"Michael Hunter",
				"Samson Adam"
			],
			"conceptTypes": ["http://www.ft.com/ontology/person/Person"],
			"filter": "authors"
		}`))
	w = httptest.NewRecorder()
	conceptFinder.FindConcept(w, req)

	// check
	assert.Equal(t, http.StatusOK, w.Code)
	searchResults = make(map[string][]concept)
	err = json.Unmarshal(w.Body.Bytes(), &searchResults)
	assert.Equal(t, nil, err)
	assert.Len(t, searchResults, 3)

	ericPlattConcepts, ok = searchResults["Platt Eric"]
	assert.True(t, ok, "expected results for Platt Eric")
	assert.Len(t, ericPlattConcepts, 1, "expected 1 concept for Platt Eric")
	assert.Equal(t, "http://api.ft.com/things/64302452-e369-4ddb-88fa-9adc5124a38c", ericPlattConcepts[0].ID)

	adamSamsonConcepts, ok = searchResults["Samson Adam"]
	assert.True(t, ok, "expected results for Adam Samson")
	assert.Len(t, adamSamsonConcepts, 1, "expected 1 concept for Adam Samson")
	assert.Equal(t, "http://api.ft.com/things/f758ef56-c40a-3162-91aa-3e8a3aabc494", adamSamsonConcepts[0].ID)

	michaelHunterConcepts, ok = searchResults["Michael Hunter"]
	assert.True(t, ok, "expected results for Michael Hunter")
	assert.Len(t, michaelHunterConcepts, 1, "expected 1 concept for Michael Hunter")
	assert.Equal(t, "http://api.ft.com/things/9332270e-f959-3f55-9153-d30acd0d0a51", michaelHunterConcepts[0].ID)
}

func getElasticSearchTestURL(t *testing.T) string {
	if testing.Short() {
		t.Skip("ElasticSearch integration for long tests only.")
	}

	esURL := os.Getenv("ELASTICSEARCH_TEST_URL")
	if strings.TrimSpace(esURL) == "" {
		esURL = "http://localhost:9200"
	}

	return esURL
}

func createIndex(ec *elastic.Client, mappingFile string, indexName string) error {
	mapping, err := ioutil.ReadFile(mappingFile)
	if err != nil {
		return err
	}
	_, err = ec.CreateIndex(indexName).Body(string(mapping)).Do(context.Background())
	if err != nil {
		return err
	}
	return nil
}

type failClient struct{}

func (tc failClient) query(indexName string, query elastic.Query, resultLimit int) (*elastic.SearchResult, error) {
	return &elastic.SearchResult{}, errors.New("Test ES failure")
}

func (tc failClient) multiSearchQuery(indexName string, searchRequests ...*elastic.SearchRequest) (*elastic.MultiSearchResult, error) {
	return &elastic.MultiSearchResult{}, errors.New("Test ES failure")
}

func (tc failClient) getClusterHealth() (*elastic.ClusterHealthResponse, error) {
	return &elastic.ClusterHealthResponse{}, errors.New("Test ES failure")
}

type mockClient struct {
	queryResponse string
}

func (mc mockClient) query(indexName string, query elastic.Query, resultLimit int) (*elastic.SearchResult, error) {
	var searchResult elastic.SearchResult
	err := json.Unmarshal([]byte(mc.queryResponse), &searchResult)
	if err != nil {
		log.Printf("%v \n", err.Error())
	}
	return &searchResult, nil
}

func (mc mockClient) multiSearchQuery(indexName string, searchRequests ...*elastic.SearchRequest) (*elastic.MultiSearchResult, error) {
	var searchResult elastic.MultiSearchResult
	err := json.Unmarshal([]byte(mc.queryResponse), &searchResult)
	if err != nil {
		log.Printf("%v \n", err.Error())
	}
	return &searchResult, nil
}

func (mc mockClient) getClusterHealth() (*elastic.ClusterHealthResponse, error) {
	return &elastic.ClusterHealthResponse{}, nil
}

const (
	validRequestBody              = `{"term":"Foobar"}`
	validRequestBodyForDeprecated = `{"term": "Rick And Morty"}`
	invalidRequestBody            = `{"term":"Foobar}`
	missingTermRequestBody        = `{"ter":"Foobar"}`

	defaultRequestURL                = "http://nothing/at/all"
	requestURLWithScore              = "http://nothing/at/all?include_score=true"
	requestURLWithScoreAndDeprecated = "http://nothing/at/all?include_score=true&include_deprecated=true"
	requestURLWithAllAuthorities     = "http://nothing/at/all?searchAllAuthorities=true"
)

const validResponse = `{
  "took": 111,
  "timed_out": false,
  "_shards": {
    "total": 5,
    "successful": 5,
    "failed": 0
  },
  "hits": {
    "total": 540,
    "max_score": 9.992676,
    "hits": [
      {
        "_index": "concept",
        "_type": "organisations",
        "_id": "9a0dd8b8-2ae4-34ca-8639-cfef69711eb9",
        "_score": 9.992676,
        "_source": {
          "id": "http://api.ft.com/things/9a0dd8b8-2ae4-34ca-8639-cfef69711eb9",
          "apiUrl": "http://api.ft.com/organisations/9a0dd8b8-2ae4-34ca-8639-cfef69711eb9",
          "prefLabel": "Foobar SpA",
          "types": [
            "http://www.ft.com/ontology/core/Thing",
            "http://www.ft.com/ontology/concept/Concept",
            "http://www.ft.com/ontology/organisation/Organisation",
            "http://www.ft.com/ontology/company/Company",
            "http://www.ft.com/ontology/company/PublicCompany"
          ],
          "directType": "http://www.ft.com/ontology/company/PublicCompany",
          "aliases": [
            "Foobar SpA"
          ],
          "countryCode": "CA",
          "countryOfIncorporation": "US"
        }
      },
      {
        "_index": "concept",
        "_type": "organisations",
        "_id": "6084734d-f4c2-3375-b298-dbbc6c00a680",
        "_score": 2.68152,
        "_source": {
          "id": "http://api.ft.com/things/6084734d-f4c2-3375-b298-dbbc6c00a680",
          "apiUrl": "http://api.ft.com/organisations/6084734d-f4c2-3375-b298-dbbc6c00a680",
          "prefLabel": "Foobar GmbH",
          "types": [
            "http://www.ft.com/ontology/core/Thing",
            "http://www.ft.com/ontology/concept/Concept",
            "http://www.ft.com/ontology/organisation/Organisation"
          ],
          "directType": "http://www.ft.com/ontology/organisation/Organisation",
          "aliases": [
            "Foobar GMBH"
          ]}}]}
}`
const validResponseDeprecated = `{
  "took": 111,
  "timed_out": false,
  "_shards": {
    "total": 5,
    "successful": 5,
    "failed": 0
  },
  "hits": {
    "total": 1,
    "max_score": 113.70959,
    "hits": [
			{
				"_index": "concept",
				"_type": "genres",
				"_id": "74877f31-6c39-4e07-a85a-39236354a93e",
				"_score": 113.70959,
				"_source": {
						"id": "http://api.ft.com/things/74877f31-6c39-4e07-a85a-39236354a93e",
						"apiUrl": "http://api.ft.com/things/74877f31-6c39-4e07-a85a-39236354a93e",
						"prefLabel": "Rick And Morty",
						"types": [
								"http://www.ft.com/ontology/core/Thing",
								"http://www.ft.com/ontology/concept/Concept",
								"http://www.ft.com/ontology/classification/Classification",
								"http://www.ft.com/ontology/Genre"
						],
						"authorities": [
								"TME"
						],
						"directType": "http://www.ft.com/ontology/Genre",
						"aliases": [
								"Rick And Morty"
						],
						"isDeprecated": true
				}
			}]}
}`

const emptyResponse = `{
  "took": 38,
  "timed_out": false,
  "_shards": {
    "total": 5,
    "successful": 5,
    "failed": 0
  },
  "hits": {
    "total": 0,
    "max_score": null,
    "hits": []
  }
}`

const invalidResponseBadHits = `{
  "took": 222,
  "timed_out": false,
  "_shards: {
    "total": 5,
    "successful": 5,
    "failed": 0
  },
  "hits: {
    "total": 999,
    "max_score": 9.992676,
    "hits": [
      {
        "_index": "concept",
        "_type": "organisations",
        "_id": "9a0dd8b8-2ae4-34ca-8639-cfef69711eb9",
}`

const invvalidResponseBadConcept = `{
  "took": 111,
  "timed_out": false,
  "_shards": {
    "total": 5,
    "successful": 5,
    "failed": 0
  },
  "hits": {
    "total": 540,
    "max_score": 9.992676,
    "hits": [
      {
        "_index": "concept",
        "_type": "organisations",
        "_id": "9a0dd8b8-2ae4-34ca-8639-cfef69711eb9",
        "_score: 9.992676,
        }}]}
}`

var filterScoreTestingIndexName = "concepts_score_test"
var filterScoreTestingData = map[string]string{
	"08147da5-8110-407c-a51c-a91855e6b073": `{
	"id": "http://api.ft.com/things/08147da5-8110-407c-a51c-a91855e6b073",
	"apiUrl": "http://api.ft.com/people/08147da5-8110-407c-a51c-a91855e6b073",
	"prefLabel": "Anna Whitwham",
	"types": [
			"http://www.ft.com/ontology/core/Thing",
			"http://www.ft.com/ontology/concept/Concept",
			"http://www.ft.com/ontology/person/Person"
	],
	"authorities": [
			"Smartlogic",
			"TME",
			"TME"
	],
	"directType": "http://www.ft.com/ontology/person/Person",
	"aliases": [
			"Anna Whitwham"
	],
	"lastModified": "2018-05-17T16:10:11+03:00",
	"publishReference": "tid_PEqDS576xe",
	"isFTAuthor": "true"
}`,
	"a0ec2c50-1174-48f2-b804-d1f346bb7256": `{
	"id": "http://api.ft.com/things/a0ec2c50-1174-48f2-b804-d1f346bb7256",
	"apiUrl": "http://api.ft.com/things/a0ec2c50-1174-48f2-b804-d1f346bb7256",
	"prefLabel": "Onyx Pike Broader Transitive",
	"types": [
			"http://www.ft.com/ontology/core/Thing",
			"http://www.ft.com/ontology/concept/Concept",
			"http://www.ft.com/ontology/Topic"
	],
	"authorities": [
			"Smartlogic"
	],
	"directType": "http://www.ft.com/ontology/Topic",
	"aliases": [
			"Onyx Pike Broader Transitive Business & Economy",
			"Onyx Pike Broader Transitive"
	],
	"lastModified": "2018-05-17T14:38:40+03:00",
	"publishReference": "tid_aNgmualoBB",
	"isFTAuthor": "false"
}`,
}

var bestMatchIndexName = "exact_match_index"
var bestMatchTestingData = map[string]string{
	"f758ef56-c40a-3162-91aa-3e8a3aabc494": `{
		"id": "http://api.ft.com/things/f758ef56-c40a-3162-91aa-3e8a3aabc494",
		"apiUrl": "http://api.ft.com/people/f758ef56-c40a-3162-91aa-3e8a3aabc494",
		"prefLabel": "Adam Samson",
		"types": [
			"http://www.ft.com/ontology/core/Thing",
			"http://www.ft.com/ontology/concept/Concept",
			"http://www.ft.com/ontology/person/Person"
		],
		"authorities": [
			"TME"
		],
		"directType": "http://www.ft.com/ontology/person/Person",
		"aliases": [
			"Adam Samson"
		],
		"lastModified": "2018-06-08T14:34:22Z",
		"publishReference": "job_dNZnTv32iM",
		"isFTAuthor": "true"}`,

	"64302452-e369-4ddb-88fa-9adc5124a38c": `{
		"id": "http://api.ft.com/things/64302452-e369-4ddb-88fa-9adc5124a38c",
		"apiUrl": "http://api.ft.com/people/64302452-e369-4ddb-88fa-9adc5124a38c",
		"prefLabel": "Eric Platt",
		"types": [
			"http://www.ft.com/ontology/core/Thing",
			"http://www.ft.com/ontology/concept/Concept",
			"http://www.ft.com/ontology/person/Person"
		],
		"authorities": [
			"TME",
			"Smartlogic"
		],
		"directType": "http://www.ft.com/ontology/person/Person",
		"aliases": [
			"Eric Platt"
		],
		"lastModified": "2018-06-08T14:34:29Z",
		"publishReference": "tid_fQ3qCMiEvC",
		"isFTAuthor": "true"}`,

	"9332270e-f959-3f55-9153-d30acd0d0a51": `{
		"id": "http://api.ft.com/things/9332270e-f959-3f55-9153-d30acd0d0a51",
		"apiUrl": "http://api.ft.com/people/9332270e-f959-3f55-9153-d30acd0d0a51",
		"prefLabel": "Michael Hunter",
		"types": [
			"http://www.ft.com/ontology/core/Thing",
			"http://www.ft.com/ontology/concept/Concept",
			"http://www.ft.com/ontology/person/Person"
		],
		"authorities": [
			"TME"
		],
		"directType": "http://www.ft.com/ontology/person/Person",
		"aliases": [
			"Michael Hunter"
		],
		"lastModified": "2018-06-08T14:34:27Z",
		"publishReference": "job_dNZnTv32iM",
		"isFTAuthor": "true"}`,

	"40281396-8369-4699-ae48-1ccc0c931a72": `{
		"id": "http://api.ft.com/things/40281396-8369-4699-ae48-1ccc0c931a72",
		"apiUrl": "http://api.ft.com/people/40281396-8369-4699-ae48-1ccc0c931a72",
		"prefLabel": "Eric Platt",
		"types": [
			"http://www.ft.com/ontology/core/Thing",
			"http://www.ft.com/ontology/concept/Concept",
			"http://www.ft.com/ontology/person/Person"
		],
		"authorities": [
			"TME",
			"Smartlogic"
		],
		"directType": "http://www.ft.com/ontology/person/Person",
		"aliases": [
			"Eric Platt"
		],
		"isFTAuthor": "false",
		"isDeprecated": true
	}`,

	"40281396-8369-4699-ae48-1ccc0c931b50": `{
		"id": "http://api.ft.com/things/40281396-8369-4699-ae48-1ccc0c931b50",
		"apiUrl": "http://api.ft.com/people/40281396-8369-4699-ae48-1ccc0c931b50",
		"prefLabel": "Eric Andrew",
		"types": [
			"http://www.ft.com/ontology/core/Thing",
			"http://www.ft.com/ontology/concept/Concept",
			"http://www.ft.com/ontology/person/Person"
		],
		"authorities": [
			"TME",
			"Smartlogic"
		],
		"directType": "http://www.ft.com/ontology/person/Person",
		"aliases": [
			"Eric Andrew"
		],
		"isFTAuthor": "false"}`,

	"40281396-8369-4699-ae48-1ccc0c931b55": `{
		"id": "http://api.ft.com/things/40281396-8369-4699-ae48-1ccc0c931b55",
		"apiUrl": "http://api.ft.com/people/40281396-8369-4699-ae48-1ccc0c931b55",
		"prefLabel": "Rick And Morty",
		"types": [
			"http://www.ft.com/ontology/core/Thing",
			"http://www.ft.com/ontology/concept/Concept",
			"http://www.ft.com/ontology/person/Person"
		],
		"authorities": [
			"TME",
			"Smartlogic"
		],
		"directType": "http://www.ft.com/ontology/person/Person",
		"aliases": [
			"Rick And Morty"
		],
		"isFTAuthor": "true",
		"isDeprecated": true}`,
}
var validResponseBestMatch = `{
    "responses": [
        {
            "took": 46,
            "timed_out": false,
            "_shards": {
                "total": 5,
                "successful": 5,
                "failed": 0
            },
            "hits": {
                "total": 1,
                "max_score": 16.835419,
                "hits": [
                    {
                        "_index": "concepts-0.2.2",
                        "_type": "people",
                        "_id": "f758ef56-c40a-3162-91aa-3e8a3aabc494",
                        "_score": 16.835419,
                        "_source": {
                            "id": "http://api.ft.com/things/f758ef56-c40a-3162-91aa-3e8a3aabc494",
                            "apiUrl": "http://api.ft.com/people/f758ef56-c40a-3162-91aa-3e8a3aabc494",
                            "prefLabel": "Adam Samson",
                            "types": [
                                "http://www.ft.com/ontology/core/Thing",
                                "http://www.ft.com/ontology/concept/Concept",
                                "http://www.ft.com/ontology/person/Person"
                            ],
                            "authorities": [
                                "TME"
                            ],
                            "directType": "http://www.ft.com/ontology/person/Person",
                            "aliases": [
                                "Adam Samson"
                            ],
                            "lastModified": "2018-06-08T14:34:22Z",
                            "publishReference": "job_dNZnTv32iM",
                            "isFTAuthor": "true"
                        }
                    }
                ]
            },
            "status": 200
        },
        {
            "took": 41,
            "timed_out": false,
            "_shards": {
                "total": 5,
                "successful": 5,
                "failed": 0
            },
            "hits": {
                "total": 2,
                "max_score": 16.62907,
                "hits": [
                    {
                        "_index": "concepts-0.2.2",
                        "_type": "people",
                        "_id": "40281396-8369-4699-ae48-1ccc0c931a72",
                        "_score": 16.62907,
                        "_source": {
                            "id": "http://api.ft.com/things/40281396-8369-4699-ae48-1ccc0c931a72",
                            "apiUrl": "http://api.ft.com/people/40281396-8369-4699-ae48-1ccc0c931a72",
                            "prefLabel": "Eric Platt",
                            "types": [
                                "http://www.ft.com/ontology/core/Thing",
                                "http://www.ft.com/ontology/concept/Concept",
                                "http://www.ft.com/ontology/person/Person"
                            ],
                            "authorities": [
                                "TME",
                                "Smartlogic"
                            ],
                            "directType": "http://www.ft.com/ontology/person/Person",
                            "aliases": [
                                "Eric Platt"
                            ],
                            "isFTAuthor": "false"
                        }
                    },
                    {
                        "_index": "concepts-0.2.2",
                        "_type": "people",
                        "_id": "64302452-e369-4ddb-88fa-9adc5124a38c",
                        "_score": 16.264492,
                        "_source": {
                            "id": "http://api.ft.com/things/64302452-e369-4ddb-88fa-9adc5124a38c",
                            "apiUrl": "http://api.ft.com/people/64302452-e369-4ddb-88fa-9adc5124a38c",
                            "prefLabel": "Eric Platt",
                            "types": [
                                "http://www.ft.com/ontology/core/Thing",
                                "http://www.ft.com/ontology/concept/Concept",
                                "http://www.ft.com/ontology/person/Person"
                            ],
                            "authorities": [
                                "TME",
                                "Smartlogic"
                            ],
                            "directType": "http://www.ft.com/ontology/person/Person",
                            "aliases": [
                                "Eric Platt"
                            ],
                            "lastModified": "2018-06-08T14:34:29Z",
                            "publishReference": "tid_fQ3qCMiEvC",
                            "isFTAuthor": "true"
                        }
                    }
                ]
            },
            "status": 200
        },
        {
            "took": 8,
            "timed_out": false,
            "_shards": {
                "total": 5,
                "successful": 5,
                "failed": 0
            },
            "hits": {
                "total": 1,
                "max_score": 12.8185625,
                "hits": [
                    {
                        "_index": "concepts-0.2.2",
                        "_type": "people",
                        "_id": "9332270e-f959-3f55-9153-d30acd0d0a51",
                        "_score": 12.8185625,
                        "_source": {
                            "id": "http://api.ft.com/things/9332270e-f959-3f55-9153-d30acd0d0a51",
                            "apiUrl": "http://api.ft.com/people/9332270e-f959-3f55-9153-d30acd0d0a51",
                            "prefLabel": "Michael Hunter",
                            "types": [
                                "http://www.ft.com/ontology/core/Thing",
                                "http://www.ft.com/ontology/concept/Concept",
                                "http://www.ft.com/ontology/person/Person"
                            ],
                            "authorities": [
                                "TME"
                            ],
                            "directType": "http://www.ft.com/ontology/person/Person",
                            "aliases": [
                                "Michael Hunter"
                            ],
                            "lastModified": "2018-06-08T14:34:27Z",
                            "publishReference": "job_dNZnTv32iM",
                            "isFTAuthor": "true"
                        }
                    }
                ]
            },
            "status": 200
        }
    ]
}`

var validResponseBestMatchPartialResults = `{
    "responses": [
        {
            "took": 46,
            "timed_out": false,
            "_shards": {
                "total": 5,
                "successful": 5,
                "failed": 0
            },
            "hits": {
                "total": 1,
                "max_score": 16.835419,
                "hits": [
                    {
                        "_index": "concepts-0.2.2",
                        "_type": "people",
                        "_id": "f758ef56-c40a-3162-91aa-3e8a3aabc494",
                        "_score": 16.835419,
                        "_source": {
                            "id": "http://api.ft.com/things/f758ef56-c40a-3162-91aa-3e8a3aabc494",
                            "apiUrl": "http://api.ft.com/people/f758ef56-c40a-3162-91aa-3e8a3aabc494",
                            "prefLabel": "Adam Samson",
                            "types": [
                                "http://www.ft.com/ontology/core/Thing",
                                "http://www.ft.com/ontology/concept/Concept",
                                "http://www.ft.com/ontology/person/Person"
                            ],
                            "authorities": [
                                "TME"
                            ],
                            "directType": "http://www.ft.com/ontology/person/Person",
                            "aliases": [
                                "Adam Samson"
                            ],
                            "lastModified": "2018-06-08T14:34:22Z",
                            "publishReference": "job_dNZnTv32iM",
                            "isFTAuthor": "true"
                        }
                    }
                ]
            },
            "status": 200
        },
        {
            "took": 41,
            "timed_out": false,
            "_shards": {
                "total": 5,
                "successful": 5,
                "failed": 0
            },
            "hits": {
                "total": 0,
                "max_score": null,
                "hits": []
            },
            "status": 200
        },
        {
            "took": 8,
            "timed_out": false,
            "_shards": {
                "total": 5,
                "successful": 5,
                "failed": 0
            },
            "hits": {
                "total": 1,
                "max_score": 12.8185625,
                "hits": [
                    {
                        "_index": "concepts-0.2.2",
                        "_type": "people",
                        "_id": "9332270e-f959-3f55-9153-d30acd0d0a51",
                        "_score": 12.8185625,
                        "_source": {
                            "id": "http://api.ft.com/things/9332270e-f959-3f55-9153-d30acd0d0a51",
                            "apiUrl": "http://api.ft.com/people/9332270e-f959-3f55-9153-d30acd0d0a51",
                            "prefLabel": "Michael Hunter",
                            "types": [
                                "http://www.ft.com/ontology/core/Thing",
                                "http://www.ft.com/ontology/concept/Concept",
                                "http://www.ft.com/ontology/person/Person"
                            ],
                            "authorities": [
                                "TME"
                            ],
                            "directType": "http://www.ft.com/ontology/person/Person",
                            "aliases": [
                                "Michael Hunter"
                            ],
                            "lastModified": "2018-06-08T14:34:27Z",
                            "publishReference": "job_dNZnTv32iM",
                            "isFTAuthor": "true"
                        }
                    }
                ]
            },
            "status": 200
        }
    ]
}`
var validResponseBestMatchNoResults = `{
    "responses": [
        {
            "took": 46,
            "timed_out": false,
            "_shards": {
                "total": 5,
                "successful": 5,
                "failed": 0
            },
            "hits": {
                "total": 0,
                "max_score": null,
                "hits": []
            },
            "status": 200
        },
        {
            "took": 41,
            "timed_out": false,
            "_shards": {
                "total": 5,
                "successful": 5,
                "failed": 0
            },
            "hits": {
                "total": 0,
                "max_score": null,
                "hits": []
            },
            "status": 200
        },
        {
            "took": 8,
            "timed_out": false,
            "_shards": {
                "total": 5,
                "successful": 5,
                "failed": 0
            },
            "hits": {
                "total": 0,
                "max_score": null,
                "hits": []
            },
            "status": 200
        }
    ]
}`
