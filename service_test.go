package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	log "github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	elastic "gopkg.in/olivere/elastic.v3"
)

var requestBody = `{"term":"Ferrari"}`
var responseBody = `{ "hits": { "total": 540 } }`

func TestConceptFinder(t *testing.T) {

	tc := testClient{}
	conceptFinder := esConceptFinder{
		client:            tc,
		indexName:         "concept",
		searchResultLimit: 50,
	}
	req, _ := http.NewRequest("POST", "http://nothing/at/all", strings.NewReader(requestBody))
	w := httptest.NewRecorder()
	conceptFinder.FindConcept(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

type testClient struct {
}

func (tc testClient) query(indexName string, query elastic.Query, resultLimit int) (*elastic.SearchResult, error) {
	log.Info("query")
	return &elastic.SearchResult{}, errors.New("FAIL")
}

func (tc testClient) getClusterHealth() (*elastic.ClusterHealthResponse, error) {
	log.Info("getClusterHealth")
	return &elastic.ClusterHealthResponse{}, nil
}
