package main

import (
	"net/http"
	"os"

	"github.com/Financial-Times/go-fthealth/v1a"
	"github.com/Financial-Times/http-handlers-go/httphandlers"
	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/jawher/mow.cli"
	"github.com/rcrowley/go-metrics"
)

func main() {
	app := cli.App("concept-search-api", "API for searching concepts")
	port := app.String(cli.StringOpt{
		Name:   "port",
		Value:  "8080",
		Desc:   "Port to listen on",
		EnvVar: "PORT",
	})
	accessKey := app.String(cli.StringOpt{
		Name:   "aws-access-key",
		Desc:   "AWS ACCES KEY",
		EnvVar: "AWS_ACCESS_KEY_ID",
	})
	secretKey := app.String(cli.StringOpt{
		Name:   "aws-secret-access-key",
		Desc:   "AWS SECRET ACCES KEY",
		EnvVar: "AWS_SECRET_ACCESS_KEY",
	})
	esEndpoint := app.String(cli.StringOpt{
		Name:   "elasticsearch-endpoint",
		Value:  "search-concept-search-mvp-k2vkgwhfgjv63nu6jvortpggha.eu-west-1.es.amazonaws.com",
		Desc:   "AES endpoint",
		EnvVar: "ELASTICSEARCH_ENDPOINT",
	})
	esRegion := app.String(cli.StringOpt{
		Name:   "elasticsearch-region",
		Value:  "eu-west-1",
		Desc:   "AES region",
		EnvVar: "ELASTICSEARCH_REGION",
	})
	esIndex := app.String(cli.StringOpt{
		Name:   "elasticsearch-index",
		Value:  "concept",
		Desc:   "Elasticsearch index",
		EnvVar: "ELASTICSEARCH_INDEX",
	})
	searchResultLimit := app.Int(cli.IntOpt{
		Name:   "search-result-limit",
		Value:  50,
		Desc:   "The maximum number of search results returned",
		EnvVar: "RESULT_LIMIT",
	})
	app.Action = func() {

		accessConfig := esAccessConfig{
			accessKey:  *accessKey,
			secretKey:  *secretKey,
			esEndpoint: *esEndpoint,
			esRegion:   *esRegion,
		}

		elasticFinder, err := NewESSearcherService(&accessConfig, *esIndex, *searchResultLimit)

		if err != nil {
			log.Errorf("Concept search API failed to start: %v\n", err)
		}

		servicesRouter := mux.NewRouter()
		http.HandleFunc("/_search", elasticFinder.SearchConcept)

		http.HandleFunc("/__health", v1a.Handler("Amazon Elasticsearch Service Healthcheck", "Checks for AES", elasticFinder.connectivityHealthyCheck(), elasticFinder.clusterIsHealthyCheck()))
		http.HandleFunc("/__health-details", elasticFinder.HealthDetails)
		http.HandleFunc("/__gtg", elasticFinder.GoodToGo)

		var monitoringRouter http.Handler = servicesRouter
		monitoringRouter = httphandlers.TransactionAwareRequestLoggingHandler(log.StandardLogger(), monitoringRouter)
		monitoringRouter = httphandlers.HTTPMetricsHandler(metrics.DefaultRegistry, monitoringRouter)
		http.Handle("/", monitoringRouter)

		if err := http.ListenAndServe(":"+*port, nil); err != nil {
			log.Fatalf("Unable to start: %v", err)
		}
	}

	log.SetLevel(log.InfoLevel)
	err := app.Run(os.Args)
	if err != nil {
		log.Errorf("App could not start, error=[%s]\n", err)
		return
	}
}
