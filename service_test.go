package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"log"

	"github.com/stretchr/testify/assert"
	elastic "gopkg.in/olivere/elastic.v5"
)

func TestConceptFinder(t *testing.T) {

	testCases := []struct {
		client        esClient
		returnCode    int
		requestURL    string
		requestBody   string
		expectedUUIDs []string
		expectedScore []float64
	}{
		{
			nil,
			http.StatusInternalServerError,
			defaultRequestURL,
			validRequestBody,
			nil, nil,
		},
		{
			failClient{},
			http.StatusBadRequest,
			defaultRequestURL,
			invalidRequestBody,
			nil, nil,
		},
		{
			failClient{},
			http.StatusInternalServerError,
			defaultRequestURL,
			validRequestBody,
			nil, nil,
		},
		{
			mockClient{
				queryResponse: validResponse,
			},
			http.StatusOK,
			defaultRequestURL,
			validRequestBody,
			[]string{"9a0dd8b8-2ae4-34ca-8639-cfef69711eb9", "6084734d-f4c2-3375-b298-dbbc6c00a680"},
			nil,
		},
		{
			mockClient{
				queryResponse: emptyResponse,
			},
			http.StatusNotFound,
			defaultRequestURL,
			validRequestBody,
			nil, nil,
		},
		{
			mockClient{
				queryResponse: validResponse,
			},
			http.StatusOK,
			requestURLWithScore,
			validRequestBody,
			[]string{"9a0dd8b8-2ae4-34ca-8639-cfef69711eb9", "6084734d-f4c2-3375-b298-dbbc6c00a680"},
			[]float64{9.992676, 2.68152},
		},
		{
			mockClient{
				queryResponse: validResponseDeprecated,
			},
			http.StatusOK,
			requestURLWithScoreAndDeprecated,
			validRequestBodyForDeprecated,
			[]string{"74877f31-6c39-4e07-a85a-39236354a93e"},
			[]float64{113.70959},
		},
		{
			mockClient{
				queryResponse: invalidResponseBadHits,
			},
			http.StatusInternalServerError,
			defaultRequestURL,
			validRequestBody,
			nil, nil,
		},
		{
			mockClient{
				queryResponse: invvalidResponseBadConcept,
			},
			http.StatusInternalServerError,
			defaultRequestURL,
			validRequestBody,
			nil, nil,
		},
		{
			failClient{},
			http.StatusBadRequest,
			defaultRequestURL,
			missingTermRequestBody,
			nil, nil,
		},
	}

	for _, testCase := range testCases {
		conceptFinder := &esConceptFinder{
			indexName:         "concept",
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
		}

		if testCase.requestURL == requestURLWithScore ||
			testCase.requestURL == requestURLWithScoreAndDeprecated {
			for i, score := range testCase.expectedScore {
				assert.Equal(t, score, searchResults.Results[i].Score)
			}
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

	// prepare request and triger this
	req, _ := http.NewRequest("POST", "http://dummy_host/concepts?include_score=true", strings.NewReader(`{"term": "Anna"}`))
	w := httptest.NewRecorder()
	conceptFinder := newConceptFinder(filterScoreTestingIndexName, 10)
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

func getElasticSearchTestURL(t *testing.T) string {
	if testing.Short() {
		t.Skip("ElasticSearch integration for long tests only.")
	}

	esURL := os.Getenv("ELASTICSEARCH_TEST_URL")
	if strings.TrimSpace(esURL) == "" {
		t.Fatal("Please set the environment variable ELASTICSEARCH_TEST_URL to run ElasticSearch integration tests (e.g. export ELASTICSEARCH_TEST_URL=http://localhost:9200). Alternatively, run `go test -short` to skip them.")
	}

	return esURL
}

type failClient struct{}

func (tc failClient) query(indexName string, query elastic.Query, resultLimit int, minScore float64) (*elastic.SearchResult, error) {
	return &elastic.SearchResult{}, errors.New("Test ES failure")
}

func (tc failClient) getClusterHealth() (*elastic.ClusterHealthResponse, error) {
	return &elastic.ClusterHealthResponse{}, errors.New("Test ES failure")
}

type mockClient struct {
	queryResponse string
}

func (mc mockClient) query(indexName string, query elastic.Query, resultLimit int, minScore float64) (*elastic.SearchResult, error) {
	var searchResult elastic.SearchResult
	err := json.Unmarshal([]byte(mc.queryResponse), &searchResult)
	if err != nil {
		log.Printf("%v \n", err.Error())
	}
	return &searchResult, nil
}

func (mc mockClient) getClusterHealth() (*elastic.ClusterHealthResponse, error) {
	return &elastic.ClusterHealthResponse{}, nil
}

const validRequestBody = `{"term":"Foobar"}`
const validRequestBodyForDeprecated = `{"term": "Rick And Morty"}`
const invalidRequestBody = `{"term":"Foobar}`
const missingTermRequestBody = `{"ter":"Foobar"}`

const defaultRequestURL = "http://nothing/at/all"
const requestURLWithScore = "http://nothing/at/all?include_score=true"
const requestURLWithScoreAndDeprecated = "http://nothing/at/all?include_score=true&include_deprecated=true"

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
            "http://www.ft.com/ontology/organisation/Organisation"
          ],
          "directType": "http://www.ft.com/ontology/organisation/Organisation",
          "aliases": [
            "Foobar SpA"
          ]
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
