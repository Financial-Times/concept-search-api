package main

import (
	"net/http"
	"os"
	"time"

	api "github.com/Financial-Times/api-endpoint"
	"github.com/Financial-Times/concept-search-api/resources"
	"github.com/Financial-Times/concept-search-api/service"
	fthealth "github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/Financial-Times/http-handlers-go/httphandlers"
	status "github.com/Financial-Times/service-status-go/httphandlers"
	log "github.com/Sirupsen/logrus"
	"github.com/husobee/vestigo"
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
		Value:  "http://localhost:9200",
		Desc:   "AES endpoint",
		EnvVar: "ELASTICSEARCH_ENDPOINT",
	})
	esAuth := app.String(cli.StringOpt{
		Name:   "auth",
		Value:  "none",
		Desc:   "Authentication method for ES cluster (aws or none)",
		EnvVar: "AUTH",
	})
	esIndex := app.String(cli.StringOpt{
		Name:   "elasticsearch-index",
		Value:  "concepts",
		Desc:   "Elasticsearch index",
		EnvVar: "ELASTICSEARCH_INDEX",
	})
	apiYml := app.String(cli.StringOpt{
		Name:   "api-yml",
		Value:  "./api.yml",
		Desc:   "Location of the API Swagger YML file.",
		EnvVar: "API_YML",
	})
	searchResultLimit := app.Int(cli.IntOpt{
		Name:   "search-result-limit",
		Value:  50,
		Desc:   "The maximum number of search results returned",
		EnvVar: "RESULT_LIMIT",
	})

	log.SetLevel(log.InfoLevel)

	app.Action = func() {
		logStartupConfig(port, esEndpoint, esAuth, esIndex, searchResultLimit)

		search := service.NewEsConceptSearchService(*esIndex)
		conceptFinder := newConceptFinder(*esIndex, *searchResultLimit)
		healthcheck := newEsHealthService()

		if *esAuth == "aws" {
			go service.AWSClientSetup(*accessKey, *secretKey, *esEndpoint, time.Minute, search, conceptFinder, healthcheck)
		} else {
			go service.SimpleClientSetup(*esEndpoint, time.Minute, search, conceptFinder, healthcheck)
		}

		handler := resources.NewHandler(search)
		routeRequest(port, apiYml, conceptFinder, handler, healthcheck)
	}

	log.SetLevel(log.InfoLevel)
	err := app.Run(os.Args)
	if err != nil {
		log.Errorf("App could not start, error=[%s]\n", err)
		return
	}
}

func logStartupConfig(port, esEndpoint, esAuth, esIndex *string, searchResultLimit *int) {
	log.Info("Concept Search API uses the following configurations:")
	log.Infof("port: %v", *port)
	log.Infof("elasticsearch-endpoint: %v", *esEndpoint)
	log.Infof("elasticsearch-auth: %v", *esAuth)
	log.Infof("elasticsearch-index: %v", *esIndex)
	log.Infof("search-result-limit: %v", *searchResultLimit)
}

func routeRequest(port *string, apiYml *string, conceptFinder conceptFinder, handler *resources.Handler, healthService *esHealthService) {
	servicesRouter := vestigo.NewRouter()
	servicesRouter.Post("/concept/search", conceptFinder.FindConcept)
	servicesRouter.Get("/concepts", handler.ConceptSearch, &resources.AcceptInterceptor{})

	if apiYml != nil {
		apiEndpoint, err := api.NewAPIEndpointForFile(*apiYml)
		if err != nil {
			log.WithError(err).WithField("file", apiYml).Warn("Failed to serve the API Endpoint for this service. Please validate the Swagger YML and the file location.")
		} else {
			servicesRouter.Get(api.DefaultPath, apiEndpoint.ServeHTTP)
		}
	}

	var monitoringRouter http.Handler = servicesRouter
	monitoringRouter = httphandlers.TransactionAwareRequestLoggingHandler(log.StandardLogger(), monitoringRouter)
	monitoringRouter = httphandlers.HTTPMetricsHandler(metrics.DefaultRegistry, monitoringRouter)

	healthCheck := fthealth.HealthCheck{
		SystemCode:  "up-csa",
		Name:        "Amazon Elasticsearch Service Healthcheck",
		Description: "Checks for AES",
		Checks: []fthealth.Check{
			healthService.connectivityHealthyCheck(),
			healthService.clusterIsHealthyCheck(),
		},
	}
	http.HandleFunc("/__health", fthealth.Handler(healthCheck))
	http.HandleFunc("/__health-details", healthService.healthDetails)

	http.HandleFunc(status.GTGPath, healthService.goodToGo)
	http.HandleFunc(status.BuildInfoPath, status.BuildInfoHandler)

	http.Handle("/", monitoringRouter)

	log.Infof("Concept Search API listening on port %v...", *port)
	if err := http.ListenAndServe(":"+*port, nil); err != nil {
		log.Fatalf("Unable to start: %v", err)
	}
}
