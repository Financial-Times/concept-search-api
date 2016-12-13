package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"strings"

	"gopkg.in/olivere/elastic.v3"
)

func TestHealthDetailsHealthyCluster(t *testing.T) {

	//create a request to pass to our handler
	req, err := http.NewRequest("GET", "/__health-details", nil)
	if err != nil {
		t.Fatal(err)
	}

	healthService := newEsHealthService(hcClient{healthy: true, returnsError: false})

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
	healthService := newEsHealthService(hcClient{returnsError: true})

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

func TestGoodToGoHealthyCluster(t *testing.T) {

	//create a request to pass to our handler
	req, err := http.NewRequest("GET", "/__gtg", nil)
	if err != nil {
		t.Fatal(err)
	}

	healthService := newEsHealthService(hcClient{returnsError: true})
	//create a responseRecorder
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(healthService.goodToGo)

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)

	// Series of verifications:
	if status := rr.Code; status != http.StatusServiceUnavailable {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusServiceUnavailable)
	}

	if rr.Body.Bytes() != nil {
		t.Error("Response body should be empty")
	}
}

func TestGoodToGoUnhealthyCluster(t *testing.T) {

	//create a request to pass to our handler
	req, err := http.NewRequest("GET", "/__gtg", nil)
	if err != nil {
		t.Fatal(err)
	}
	healthService := newEsHealthService(hcClient{healthy: true, returnsError: false})

	//create a responseRecorder
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(healthService.goodToGo)

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)

	// Series of verifications:
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	if rr.Body.Bytes() != nil {
		t.Error("Response body should be empty")
	}
}

func TestHealthServiceConnectivityChecker(t *testing.T) {
	healthService := newEsHealthService(hcClient{healthy: true, returnsError: false})

	message, err := healthService.connectivityChecker()

	assert.Equal(t, "Successfully connected to the cluster", message)
	assert.Equal(t, nil, err)
}

func TestHealthServiceConnectivityCheckerForFailedConnection(t *testing.T) {
	healthService := newEsHealthService(hcClient{returnsError: true})

	message, err := healthService.connectivityChecker()

	assert.Equal(t, "Could not connect to elasticsearch", message)
	assert.NotNil(t, err)
}

func TestHealthServiceConnectivityCheckerNilClient(t *testing.T) {
	healthService := newEsHealthService(nil)

	_, err := healthService.connectivityChecker()

	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "Could not connect to elasticsearch"))
}

func TestHealthServiceHealthCheckerNilClient(t *testing.T) {
	healthService := newEsHealthService(nil)

	_, err := healthService.healthChecker()

	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "Couldn't establish connectivity"))
}

func TestHealthServiceHealthCheckerNotHealthyClient(t *testing.T) {
	healthService := newEsHealthService(hcClient{healthy: false})

	message, err := healthService.healthChecker()

	assert.Nil(t, err)
	assert.True(t, strings.Contains(message, "red"))
}

func TestHealthServiceHealthDetailClientNil(t *testing.T) {
	healthService := newEsHealthService(hcClient{healthy: false})

	message, err := healthService.healthChecker()

	assert.Nil(t, err)
	assert.True(t, strings.Contains(message, "red"))
}

func TestHealthDetailsNilClient(t *testing.T) {

	//create a request to pass to our handler
	req, err := http.NewRequest("GET", "/__health-details", nil)
	if err != nil {
		t.Fatal(err)
	}
	healthService := newEsHealthService(nil)

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
		t.Errorf("Response body should be empty")
	}
}

type hcClient struct {
	healthy      bool
	returnsError bool
}

func (c hcClient) query(indexName string, query elastic.Query, resultLimit int) (*elastic.SearchResult, error) {
	return &elastic.SearchResult{}, nil
}

func (c hcClient) getClusterHealth() (*elastic.ClusterHealthResponse, error) {
	if c.returnsError {
		return nil, errors.New("error")
	}
	if c.healthy {
		return &elastic.ClusterHealthResponse{Status: "green"}, nil
	}
	return &elastic.ClusterHealthResponse{Status: "red"}, nil

}
