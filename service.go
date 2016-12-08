package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	log "github.com/Sirupsen/logrus"
	"gopkg.in/olivere/elastic.v3"
)

type esSearcherService struct {
	elasticClient *elastic.Client
	indexName     string
}

type esAccessConfig struct {
	accessKey  string
	secretKey  string
	esEndpoint string
	esRegion   string
}

func newESSearcherService(accessConfig *esAccessConfig, indexName string) (*esSearcherService, error) {
	elasticClient, err := newElasticClient(accessConfig.accessKey, accessConfig.secretKey, &accessConfig.esEndpoint, &accessConfig.esRegion)
	if err != nil {
		return &esSearcherService{}, fmt.Errorf("creating elasticsearch client failed with error=[%v]", err)
	}

	elasticSearcher := esSearcherService{elasticClient: elasticClient, indexName: indexName}

	return &elasticSearcher, nil
}

func (service *esSearcherService) SearchConcept(writer http.ResponseWriter, request *http.Request) {

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
		Size(50).
		Do()

	if err != nil {
		log.Errorf("There was an error executing the query on ES: %s", err.Error())
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	if searchResult.Hits.TotalHits > 0 {
		writer.Header().Add("Content-Type", "application/json")
		searchedConcepts := getSearchedConcepts(searchResult, isScoreIncluded(request))
		encoder := json.NewEncoder(writer)
		encoder.Encode(&searchedConcepts)
	} else {
		writer.WriteHeader(http.StatusNotFound)
	}
}

func getSearchedConcepts(searchResult *elastic.SearchResult, isScoreIncluded bool) []concept {
	var searchedConcepts []concept
	for _, hit := range searchResult.Hits.Hits {
		var searchedConcept concept
		err := json.Unmarshal(*hit.Source, &searchedConcept)
		if err != nil {
			log.Errorf("Unable to unmarshall concept, error=[%s]\n", err)
		} else {
			if isScoreIncluded {
				score := *hit.Score
				searchedConcept.Score = score
			}
			searchedConcepts = append(searchedConcepts, searchedConcept)
		}
	}
	return searchedConcepts
}

func isScoreIncluded(request *http.Request) bool {
	queryParam := request.URL.Query().Get("include_score")
	includeScore, err := strconv.ParseBool(queryParam)
	if err != nil {
		return false
	}
	return includeScore
}
