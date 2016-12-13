package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Financial-Times/go-fthealth/v1a"
	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"gopkg.in/olivere/elastic.v3"
)

type esHealthService struct {
	client esClient
}

func (service esHealthService) getClusterHealth() (*elastic.ClusterHealthResponse, error) {
	return service.client.getClusterHealth()
}

func newEsHealthService(client esClient) *esHealthService {
	return &esHealthService{client: client}
}

func (service *esHealthService) clusterIsHealthyCheck() v1a.Check {
	return v1a.Check{
		BusinessImpact:   "Full or partial degradation in serving requests from Elasticsearch",
		Name:             "Check Elasticsearch cluster health",
		PanicGuide:       "todo",
		Severity:         1,
		TechnicalSummary: "Elasticsearch cluster is not healthy. Details on /__health-details",
		Checker:          service.healthChecker,
	}
}

func (service *esHealthService) healthChecker() (string, error) {
	if service.client != nil {
		output, err := service.getClusterHealth()
		if err != nil {
			return "Cluster is not healthy: ", err
		} else if output.Status != "green" {
			return fmt.Sprintf("Cluster is %v", output.Status), nil
		}
		return "Cluster is healthy", nil
	}

	return "Couldn't check the cluster's health.", errors.New("Couldn't establish connectivity.")
}

func (service *esHealthService) connectivityHealthyCheck() v1a.Check {
	return v1a.Check{
		BusinessImpact:   "Could not connect to Elasticsearch",
		Name:             "Check connectivity to the Elasticsearch cluster",
		PanicGuide:       "todo",
		Severity:         1,
		TechnicalSummary: "Connection to Elasticsearch cluster could not be created. Please check your AWS credentials.",
		Checker:          service.connectivityChecker,
	}
}

func (service *esHealthService) connectivityChecker() (string, error) {
	if service.client == nil {
		return "", errors.New("Could not connect to elasticsearch, please check the application parameters/env variables, and restart the service.")
	}

	_, err := service.getClusterHealth()
	if err != nil {
		return "Could not connect to elasticsearch", err
	}
	return "Successfully connected to the cluster", nil
}

//GoodToGo returns a 503 if the healthcheck fails - suitable for use from varnish to check availability of a node
func (service *esHealthService) goodToGo(writer http.ResponseWriter, req *http.Request) {
	if _, err := service.healthChecker(); err != nil {
		writer.WriteHeader(http.StatusServiceUnavailable)
	}
}

//HealthDetails returns the response from elasticsearch service /__health endpoint - describing the cluster health
func (service *esHealthService) healthDetails(writer http.ResponseWriter, req *http.Request) {

	writer.Header().Set("Content-Type", "application/json")

	if writer == nil || service.client == nil {
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
