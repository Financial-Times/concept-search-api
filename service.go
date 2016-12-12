package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	log "github.com/Sirupsen/logrus"
	"gopkg.in/olivere/elastic.v3"
)

type conceptFinder interface {
	findConcept(writer http.ResponseWriter, request *http.Request)
}

type esConceptFinder struct {
	elasticClient     *elastic.Client
	indexName         string
	searchResultLimit int
}

type esAccessConfig struct {
	accessKey  string
	secretKey  string
	esEndpoint string
	esRegion   string
}

func newEsConceptFinder(accessConfig *esAccessConfig, indexName string, searchResultLimit int) (*esConceptFinder, error) {
	elasticClient, err := newElasticClient(accessConfig.accessKey, accessConfig.secretKey, &accessConfig.esEndpoint, &accessConfig.esRegion)
	if err != nil {
		return &esConceptFinder{}, fmt.Errorf("creating elasticsearch client failed with error=[%v]", err)
	}
	elasticSearcher := esConceptFinder{elasticClient: elasticClient, indexName: indexName, searchResultLimit: searchResultLimit}
	return &elasticSearcher, nil
}

func (service *esConceptFinder) FindConcept(writer http.ResponseWriter, request *http.Request) {
	if service.elasticClient == nil {
		log.Errorf("Elasticsearch client is not created.")
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	var criteria searchCriteria
	decoder := json.NewDecoder(request.Body)
	err := decoder.Decode(&criteria)

	if err != nil {
		log.Errorf("There was an error parsing the search request: %s", err.Error())
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	defer request.Body.Close()

	query := elastic.NewMultiMatchQuery(criteria.Term, "prefLabel.raw", "aliases.raw", "prefLabel", "aliases").Type("most_fields")
	searchResult, err := service.elasticClient.Search().
		Index(service.indexName).
		Query(query).
		Size(service.searchResultLimit).
		Do()

	if err != nil {
		log.Errorf("There was an error executing the query on ES: %s", err.Error())
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	if searchResult.Hits.TotalHits > 0 {
		writer.Header().Add("Content-Type", "application/json")
		foundConcepts := getFoundConcepts(searchResult, isScoreIncluded(request))
		encoder := json.NewEncoder(writer)
		if err := encoder.Encode(&foundConcepts); err != nil {
			log.Errorf("Cannot encode result: %s", err.Error())
			writer.WriteHeader(http.StatusInternalServerError)
		}
	} else {
		writer.WriteHeader(http.StatusNotFound)
	}
}

func getFoundConcepts(searchResult *elastic.SearchResult, isScoreIncluded bool) []concept {
	var foundConcepts []concept
	for _, hit := range searchResult.Hits.Hits {
		var foundConcept concept
		err := json.Unmarshal(*hit.Source, &foundConcept)
		if err != nil {
			log.Errorf("Unable to unmarshall concept, error=[%s]\n", err)
		} else {
			if isScoreIncluded {
				score := *hit.Score
				foundConcept.Score = score
			}
			foundConcepts = append(foundConcepts, foundConcept)
		}
	}
	return foundConcepts
}

func isScoreIncluded(request *http.Request) bool {
	queryParam := request.URL.Query().Get("include_score")
	includeScore, err := strconv.ParseBool(queryParam)
	if err != nil {
		return false
	}
	return includeScore
}
