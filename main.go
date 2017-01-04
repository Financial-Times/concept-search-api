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

	accessConfig := esAccessConfig{
		accessKey:  *accessKey,
		secretKey:  *secretKey,
		esEndpoint: *esEndpoint,
		esRegion:   *esRegion,
	}

	app.Action = func() {
		logStartupConfig(port, esEndpoint, esRegion, esIndex, searchResultLimit)
		client, err := newElasticClient(accessConfig.accessKey, accessConfig.secretKey, &accessConfig.esEndpoint, &accessConfig.esRegion)
		if err != nil {
			log.Fatalf("Creating elasticsearch client failed with error=[%v]", err)
		}
		conceptFinder := esConceptFinder{
			client:            client,
			indexName:         *esIndex,
			searchResultLimit: *searchResultLimit,
		}

		routeRequest(port, conceptFinder, newEsHealthService(client))
	}

	log.SetLevel(log.InfoLevel)
	err := app.Run(os.Args)
	if err != nil {
		log.Errorf("App could not start, error=[%s]\n", err)
		return
	}
}

func logStartupConfig(port, esEndpoint, esRegion, esIndex *string, searchResultLimit *int) {
	log.Info("Concept Search API uses the following configurations:")
	log.Infof("port: %v", *port)
	log.Infof("elasticsearch-endpoint: %v", *esEndpoint)
	log.Infof("elasticsearch-region: %v", *esRegion)
	log.Infof("elasticsearch-index: %v", *esIndex)
	log.Infof("search-result-limit: %v", *searchResultLimit)
}

func routeRequest(port *string, conceptFinder conceptFinder, healthService *esHealthService) {
	servicesRouter := mux.NewRouter()
	servicesRouter.HandleFunc("/concept/search", conceptFinder.FindConcept).Methods("POST")

	var monitoringRouter http.Handler = servicesRouter
	monitoringRouter = httphandlers.TransactionAwareRequestLoggingHandler(log.StandardLogger(), monitoringRouter)
	monitoringRouter = httphandlers.HTTPMetricsHandler(metrics.DefaultRegistry, monitoringRouter)

	http.HandleFunc("/__health", v1a.Handler("Amazon Elasticsearch Service Healthcheck", "Checks for AES", healthService.connectivityHealthyCheck(), healthService.clusterIsHealthyCheck()))
	http.HandleFunc("/__health-details", healthService.healthDetails)
	http.HandleFunc("/__gtg", healthService.goodToGo)

	http.Handle("/", monitoringRouter)

	log.Info("Concept Search API starting up...")
	if err := http.ListenAndServe(":"+*port, nil); err != nil {
		log.Fatalf("Unable to start: %v", err)
	}
}
