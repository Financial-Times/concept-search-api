package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"log"

	"github.com/stretchr/testify/assert"
	elastic "gopkg.in/olivere/elastic.v3"
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
		conceptFinder := esConceptFinder{
			client:            testCase.client,
			indexName:         "concept",
			searchResultLimit: 50,
		}
		req, _ := http.NewRequest("POST", testCase.requestURL, strings.NewReader(testCase.requestBody))
		w := httptest.NewRecorder()

		conceptFinder.FindConcept(w, req)

		assert.Equal(t, testCase.returnCode, w.Code, "Expected return code %d but got %d", testCase.returnCode, w.Code)
		if testCase.returnCode != http.StatusOK {
			continue
		}

		var searchResults []concept
		err := json.Unmarshal(w.Body.Bytes(), &searchResults)
		assert.Equal(t, nil, err)
		assert.Equal(t, len(testCase.expectedUUIDs), len(searchResults))

		for i, uuid := range testCase.expectedUUIDs {
			assert.True(t, strings.Contains(searchResults[i].ID, uuid))
		}

		if testCase.requestURL == requestURLWithScore {
			for i, score := range testCase.expectedScore {
				assert.Equal(t, score, searchResults[i].Score)
			}
		}

	}

}

type failClient struct{}

func (tc failClient) query(indexName string, query elastic.Query, resultLimit int) (*elastic.SearchResult, error) {
	return &elastic.SearchResult{}, errors.New("Test ES failure")
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

func (mc mockClient) getClusterHealth() (*elastic.ClusterHealthResponse, error) {
	return &elastic.ClusterHealthResponse{}, nil
}

const validRequestBody = `{"term":"Foobar"}`
const invalidRequestBody = `{"term":"Foobar}`
const missingTermRequestBody = `{"ter":"Foobar"}`

const defaultRequestURL = "http://nothing/at/all"
const requestURLWithScore = "http://nothing/at/all?include_score=true"

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
