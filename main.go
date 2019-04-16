package main

import (
	"net/http"
	"os"
	"time"

	"github.com/Financial-Times/api-endpoint"
	"github.com/Financial-Times/concept-search-api/resources"
	"github.com/Financial-Times/concept-search-api/service"
	fthealth "github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/Financial-Times/http-handlers-go/httphandlers"
	status "github.com/Financial-Times/service-status-go/httphandlers"
	"github.com/husobee/vestigo"
	"github.com/jawher/mow.cli"
	"github.com/rcrowley/go-metrics"
	log "github.com/sirupsen/logrus"
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
		Desc:   "AWS ACCESS KEY",
		EnvVar: "AWS_ACCESS_KEY_ID",
	})
	secretKey := app.String(cli.StringOpt{
		Name:   "aws-secret-access-key",
		Desc:   "AWS SECRET ACCESS KEY",
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
	esDefaultIndex := app.String(cli.StringOpt{
		Name:   "elasticsearch-default-index",
		Value:  "concepts",
		Desc:   "Elasticsearch default index",
		EnvVar: "ELASTICSEARCH_DEFAULT_INDEX",
	})
	esExtendedSearchIndex := app.String(cli.StringOpt{
		Name:   "elasticsearch-extended-index",
		Value:  "all-concepts",
		Desc:   "Elasticsearch extended index",
		EnvVar: "ELASTICSEARCH_EXTENDED_SEARCH_INDEX",
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
	autoCompleteResultLimit := app.Int(cli.IntOpt{
		Name:   "autocomplete-result-limit",
		Value:  10,
		Desc:   "The maximum number of autocomplete results returned",
		EnvVar: "AUTOCOMPLETE_LIMIT",
	})
	authorsBoost := app.Int(cli.IntOpt{
		Name:   "authors-boost",
		Value:  10,
		Desc:   "The boost to apply to authors during a /concepts?boost=author typeahead search.",
		EnvVar: "AUTHORS_BOOST",
	})
	esTraceLogging := app.Bool(cli.BoolOpt{
		Name:   "elasticsearch-trace",
		Value:  false,
		Desc:   "Whether to log ElasticSearch HTTP requests and responses",
		EnvVar: "ELASTICSEARCH_TRACE",
	})

	log.SetLevel(log.InfoLevel)

	app.Action = func() {
		logStartupConfig(port, esEndpoint, esAuth, esDefaultIndex, esExtendedSearchIndex, searchResultLimit)

		search := service.NewEsConceptSearchService(*esDefaultIndex, *esExtendedSearchIndex, *searchResultLimit, *autoCompleteResultLimit, *authorsBoost)
		conceptFinder := newConceptFinder(*esDefaultIndex, *esExtendedSearchIndex, *searchResultLimit)
		healthcheck := newEsHealthService()

		if *esAuth == "aws" {
			go service.AWSClientSetup(*accessKey, *secretKey, *esEndpoint, *esTraceLogging, time.Minute, search, conceptFinder, healthcheck)
		} else {
			go service.SimpleClientSetup(*esEndpoint, *esTraceLogging, time.Minute, search, conceptFinder, healthcheck)
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

func logStartupConfig(port, esEndpoint, esAuth, esDefaultIndex *string, esExtendedSearchIndex *string, searchResultLimit *int) {
	log.Info("Concept Search API uses the following configurations:")
	log.Infof("port: %v", *port)
	log.Infof("elasticsearch-endpoint: %v", *esEndpoint)
	log.Infof("elasticsearch-auth: %v", *esAuth)
	log.Infof("elasticsearch-index: %v", *esDefaultIndex)
	log.Infof("elasticsearch-extended-index: %v", *esExtendedSearchIndex)
	log.Infof("search-result-limit: %v", *searchResultLimit)
}

func routeRequest(port *string, apiYml *string, conceptFinder conceptFinder, handler *resources.Handler, healthService *esHealthService) {
	servicesRouter := vestigo.NewRouter()
	servicesRouter.Post("/concept/search", conceptFinder.FindConcept)
	servicesRouter.Get("/concepts", handler.ConceptSearch, resources.AcceptInterceptor)

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

	healthCheck := fthealth.TimedHealthCheck{
		HealthCheck: fthealth.HealthCheck{
			SystemCode:  "up-csa",
			Name:        "Amazon Elasticsearch Service Healthcheck",
			Description: "Checks for AES",
			Checks: []fthealth.Check{
				healthService.connectivityHealthyCheck(),
				healthService.clusterIsHealthyCheck(),
			},
		},
		Timeout: 10 * time.Second,
	}
	http.HandleFunc("/__health", fthealth.Handler(healthCheck))
	http.HandleFunc("/__health-details", healthService.healthDetails)

	http.HandleFunc(status.GTGPath, status.NewGoodToGoHandler(healthService.GTG))
	http.HandleFunc(status.BuildInfoPath, status.BuildInfoHandler)

	http.Handle("/", monitoringRouter)

	log.Infof("Concept Search API listening on port %v...", *port)
	if err := http.ListenAndServe(":"+*port, nil); err != nil {
		log.Fatalf("Unable to start: %v", err)
	}
}
