package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	status "github.com/Financial-Times/service-status-go/httphandlers"
	"github.com/stretchr/testify/assert"

	"strings"

	"gopkg.in/olivere/elastic.v5"
)

func TestHealthDetailsHealthyCluster(t *testing.T) {

	//create a request to pass to our handler
	req, err := http.NewRequest("GET", "/__health-details", nil)
	if err != nil {
		t.Fatal(err)
	}

	healthService := newEsHealthService()
	healthService.client = hcClient{healthy: true}

	//create a responseRecorder
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(healthService.healthDetails)

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	if contentType := rr.HeaderMap.Get("Content-Type"); contentType != "application/json" {
		t.Errorf("handler returned wrong content type: got %v want %v",
			contentType, "application/json")
	}

	var respObject *elastic.ClusterHealthResponse
	err = json.Unmarshal(rr.Body.Bytes(), &respObject)
	if err != nil {
		t.Errorf("Unmarshalling request response failed. %v", err)
	}
	if respObject.Status != "green" {
		t.Errorf("Cluster status it is not as expected, got %v want %v", respObject.Status, "green")
	}
}

func TestHealthDetailsReturnsError(t *testing.T) {

	//create a request to pass to our handler
	req, err := http.NewRequest("GET", "/__health-details", nil)
	if err != nil {
		t.Fatal(err)
	}
	healthService := newEsHealthService()
	healthService.client = hcClient{returnError: errors.New("test error")}

	//create a responseRecorder
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(healthService.healthDetails)

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)

	// Series of verifications:
	if status := rr.Code; status != http.StatusServiceUnavailable {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusServiceUnavailable)
	}

	if contentType := rr.HeaderMap.Get("Content-Type"); contentType != "application/json" {
		t.Errorf("handler returned wrong content type: got %v want %v",
			contentType, "application/json")
	}

	if rr.Body.Bytes() != nil {
		t.Error("Response body should be empty")
	}
}

func TestGTGUnhealthyCluster(t *testing.T) {
	//create a request to pass to our handler
	req := httptest.NewRequest("GET", "/__gtg", nil)

	healthService := newEsHealthService()
	healthService.client = hcClient{returnError: errors.New("test error")}
	//create a responseRecorder
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(status.NewGoodToGoHandler(healthService.GTG))

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)
	actual := rr.Result()

	// Series of verifications:
	assert.Equal(t, http.StatusServiceUnavailable, actual.StatusCode, "status code")
	assert.Equal(t, "no-cache", actual.Header.Get("Cache-Control"), "cache-control header")
	assert.Equal(t, "test error", rr.Body.String(), "GTG response body")
}

func TestGTGHealthyCluster(t *testing.T) {
	//create a request to pass to our handler
	req := httptest.NewRequest("GET", "/__gtg", nil)
	healthService := newEsHealthService()
	healthService.client = hcClient{healthy: true}
	//create a responseRecorder
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(status.NewGoodToGoHandler(healthService.GTG))

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)
	actual := rr.Result()

	// Series of verifications:
	assert.Equal(t, http.StatusOK, actual.StatusCode, "status code")
	assert.Equal(t, "no-cache", actual.Header.Get("Cache-Control"), "cache-control header")
	assert.Equal(t, "OK", rr.Body.String(), "GTG response body")
}

func TestHealthServiceConnectivityChecker(t *testing.T) {
	healthService := newEsHealthService()
	healthService.client = hcClient{healthy: true}
	hc := healthService.connectivityHealthyCheck()

	assert.Equal(t, "elasticsearch-connectivity", hc.ID, "healthcheck id")

	message, err := hc.Checker()

	assert.Equal(t, "Successfully connected to the cluster", message)
	assert.Equal(t, nil, err)
}

func TestHealthServiceConnectivityCheckerForFailedConnection(t *testing.T) {
	healthService := newEsHealthService()
	healthService.client = hcClient{returnError: errors.New("test error")}
	message, err := healthService.connectivityChecker()

	assert.Equal(t, "Could not connect to elasticsearch", message)
	assert.NotNil(t, err)
}

func TestHealthServiceConnectivityCheckerNilClient(t *testing.T) {
	healthService := newEsHealthService()

	_, err := healthService.connectivityChecker()

	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "Could not connect to elasticsearch"))
}

func TestHealthServiceHealthCheckerNilClient(t *testing.T) {
	healthService := newEsHealthService()

	_, err := healthService.healthChecker()

	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "Couldn't establish connectivity"))
}

func TestHealthServiceHealthCheckerNotHealthyClient(t *testing.T) {
	healthService := newEsHealthService()
	healthService.client = hcClient{healthy: false}

	message, err := healthService.healthChecker()

	assert.NotNil(t, err)
	assert.True(t, strings.Contains(message, "red"))
}

func TestHealthDetailsNilClient(t *testing.T) {

	//create a request to pass to our handler
	req, err := http.NewRequest("GET", "/__health-details", nil)
	if err != nil {
		t.Fatal(err)
	}
	healthService := newEsHealthService()

	//create a responseRecorder
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(healthService.healthDetails)

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)

	// Series of verifications:
	if status := rr.Code; status != http.StatusServiceUnavailable {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusServiceUnavailable)
	}

	if contentType := rr.HeaderMap.Get("Content-Type"); contentType != "application/json" {
		t.Errorf("handler returned wrong content type: got %v want %v",
			contentType, "application/json")
	}

	if rr.Body.Bytes() != nil {
		t.Error("Response body should be empty")
	}
}

func TestClusterIsHealthyChecker(t *testing.T) {
	healthService := newEsHealthService()
	healthService.client = hcClient{healthy: true}
	hc := healthService.clusterIsHealthyCheck()

	assert.Equal(t, "elasticsearch-cluster-health", hc.ID, "healthcheck id")

	message, err := hc.Checker()

	assert.Equal(t, "Cluster is healthy", message)
	assert.NoError(t, err)
}

func TestClusterIsHealthyCheckerError(t *testing.T) {
	healthService := newEsHealthService()
	expectedError := errors.New("test error")
	healthService.client = hcClient{healthy: false, returnError: expectedError}
	hc := healthService.clusterIsHealthyCheck()

	assert.Equal(t, "elasticsearch-cluster-health", hc.ID, "healthcheck id")

	message, err := hc.Checker()

	assert.Equal(t, "Cluster is not healthy: ", message)
	assert.Error(t, expectedError, err)
}

func TestClusterIsHealthyCheckerNotHealthy(t *testing.T) {
	healthService := newEsHealthService()
	healthService.client = hcClient{healthy: false}
	hc := healthService.clusterIsHealthyCheck()

	assert.Equal(t, "elasticsearch-cluster-health", hc.ID, "healthcheck id")

	message, err := hc.Checker()

	assert.Equal(t, "Cluster is red", message)
	assert.EqualError(t, err, "Cluster is red")
}

type hcClient struct {
	healthy     bool
	returnError error
}

func (c hcClient) query(indexName string, query elastic.Query, resultLimit int) (*elastic.SearchResult, error) {
	return &elastic.SearchResult{}, nil
}

func (c hcClient) getClusterHealth() (*elastic.ClusterHealthResponse, error) {
	if c.returnError != nil {
		return nil, c.returnError
	}
	if c.healthy {
		return &elastic.ClusterHealthResponse{Status: "green"}, nil
	}
	return &elastic.ClusterHealthResponse{Status: "red"}, nil

}
