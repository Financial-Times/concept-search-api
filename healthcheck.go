package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	fthealth "github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/Financial-Times/service-status-go/gtg"
	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"gopkg.in/olivere/elastic.v5"
)

const (
	deweyURL = "https://dewey.ft.com/up-csa.html"
)

type esHealthService struct {
	client     esClient
	clientLock *sync.RWMutex
}

func (service *esHealthService) getClusterHealth() (*elastic.ClusterHealthResponse, error) {
	return service.esClient().getClusterHealth()
}

func newEsHealthService() *esHealthService {
	return &esHealthService{
		clientLock: &sync.RWMutex{},
	}
}

func (service *esHealthService) clusterIsHealthyCheck() fthealth.Check {
	return fthealth.Check{
		ID:               "elasticsearch-cluster-health",
		BusinessImpact:   "Full or partial degradation in serving requests from Elasticsearch",
		Name:             "Check Elasticsearch cluster health",
		PanicGuide:       deweyURL,
		Severity:         1,
		TechnicalSummary: "Elasticsearch cluster is not healthy. Details on /__health-details",
		Checker:          service.healthChecker,
	}
}

func (service *esHealthService) healthChecker() (string, error) {
	if service.esClient() != nil {
		output, err := service.getClusterHealth()
		if err != nil {
			return "Cluster is not healthy: ", err
		} else if output.Status != "green" {
			return fmt.Sprintf("Cluster is %v", output.Status), fmt.Errorf("Cluster is %v", output.Status)
		}
		return "Cluster is healthy", nil
	}

	return "Couldn't check the cluster's health", errors.New("Couldn't establish connectivity")
}

func (service *esHealthService) connectivityHealthyCheck() fthealth.Check {
	return fthealth.Check{
		ID:               "elasticsearch-connectivity",
		BusinessImpact:   "Could not connect to Elasticsearch",
		Name:             "Check connectivity to the Elasticsearch cluster",
		PanicGuide:       deweyURL,
		Severity:         1,
		TechnicalSummary: "Connection to Elasticsearch cluster could not be created. Please check your AWS credentials.",
		Checker:          service.connectivityChecker,
	}
}

func (service *esHealthService) connectivityChecker() (string, error) {
	if service.esClient() == nil {
		return "", errors.New("Could not connect to elasticsearch, please check the application parameters/env variables, and restart the service")
	}

	_, err := service.getClusterHealth()
	if err != nil {
		return "Could not connect to elasticsearch", err
	}
	return "Successfully connected to the cluster", nil
}

func (service *esHealthService) GTG() gtg.Status {
	statusCheck := func() gtg.Status {
		return gtgCheck(service.healthChecker)
	}

	return gtg.FailFastParallelCheck([]gtg.StatusChecker{statusCheck})()
}

func gtgCheck(handler func() (string, error)) gtg.Status {
	if _, err := handler(); err != nil {
		return gtg.Status{GoodToGo: false, Message: err.Error()}
	}
	return gtg.Status{GoodToGo: true}
}

//HealthDetails returns the response from elasticsearch service /__health endpoint - describing the cluster health
func (service *esHealthService) healthDetails(writer http.ResponseWriter, req *http.Request) {
	writer.Header().Set("Content-Type", "application/json")

	if writer == nil || service.esClient() == nil {
		writer.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	output, err := service.getClusterHealth()
	if err != nil {
		writer.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	var response []byte
	response, err = json.Marshal(*output)
	if err != nil {
		response = []byte(err.Error())
	}

	_, err = writer.Write(response)
	if err != nil {
		log.Errorf(err.Error())
	}
}

func (service *esHealthService) SetElasticClient(client *elastic.Client) {
	service.clientLock.Lock()
	defer service.clientLock.Unlock()
	service.client = &esClientWrapper{elasticClient: client}
}

func (service *esHealthService) esClient() esClient {
	service.clientLock.RLock()
	defer service.clientLock.RUnlock()
	return service.client
}
