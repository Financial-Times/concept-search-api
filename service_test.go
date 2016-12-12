package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	elastic "gopkg.in/olivere/elastic.v3"
)

var validRequestBody = `{"term":"Ferrari"}`
var invalidRequestBody = `{"term":"Ferrari}`

func TestConceptFinderErrorCases(t *testing.T) {

	testCases := []struct {
		client      esClient
		returnCode  int
		requestBody string
	}{
		{
			nil,
			http.StatusInternalServerError,
			validRequestBody,
		},
		{
			failClient{},
			http.StatusBadRequest,
			invalidRequestBody,
		},
		{
			failClient{},
			http.StatusInternalServerError,
			validRequestBody,
		},
	}

	for _, testCase := range testCases {
		conceptFinder := esConceptFinder{
			client:            testCase.client,
			indexName:         "concept",
			searchResultLimit: 50,
		}
		req, _ := http.NewRequest("POST", "http://nothing/at/all", strings.NewReader(testCase.requestBody))
		w := httptest.NewRecorder()
		conceptFinder.FindConcept(w, req)
		assert.Equal(t, testCase.returnCode, w.Code, "Expected return code %d but got %d", testCase.returnCode, w.Code)
	}

}

type failClient struct{}

func (tc failClient) query(indexName string, query elastic.Query, resultLimit int) (*elastic.SearchResult, error) {
	return &elastic.SearchResult{}, errors.New("Test ES failure")
}

func (tc failClient) getClusterHealth() (*elastic.ClusterHealthResponse, error) {
	return &elastic.ClusterHealthResponse{}, errors.New("Test ES failure")
}

type mockClient struct{}

func (mc mockClient) query(indexName string, query elastic.Query, resultLimit int) (*elastic.SearchResult, error) {
	return &elastic.SearchResult{}, nil
}

func (mc mockClient) getClusterHealth() (*elastic.ClusterHealthResponse, error) {
	return &elastic.ClusterHealthResponse{}, nil
}
