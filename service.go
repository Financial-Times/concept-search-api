package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Financial-Times/transactionid-utils-go"
	log "github.com/Sirupsen/logrus"
	"gopkg.in/olivere/elastic.v3"
)

type conceptFinder interface {
	FindConcept(writer http.ResponseWriter, request *http.Request)
}

type esConceptFinder struct {
	client            esClient
	indexName         string
	searchResultLimit int
}

func (service esConceptFinder) FindConcept(writer http.ResponseWriter, request *http.Request) {
	if service.client == nil {
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

	if criteria.Term == nil {
		log.Error("The search criteria was not provided. Check that the JSON contains the 'term' field that is used to provide " +
			"the search criteria")
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	defer request.Body.Close()

	transactionID := transactionidutils.GetTransactionIDFromRequest(request)
	log.Infof("Performing concept search for term=%v, transaction_id=%v", *criteria.Term, transactionID)

	multiMatchQuery := elastic.NewMultiMatchQuery(criteria.Term, "prefLabel", "aliases").Type("most_fields")
	termQueryForPreflabelExactMatches := elastic.NewTermQuery("prefLabel.raw", criteria.Term).Boost(2)
	termQueryForAliasesExactMatches := elastic.NewTermQuery("aliases.raw", criteria.Term).Boost(2)

	finalQuery := elastic.NewBoolQuery().Should(multiMatchQuery, termQueryForPreflabelExactMatches, termQueryForAliasesExactMatches)

	searchResult, err := service.client.query(service.indexName, finalQuery, service.searchResultLimit)

	if err != nil {
		log.Errorf("There was an error executing the query on ES: %s", err.Error())
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	defer func() {
		// searchResult.Hits.TotalHits call panics if the result from ES is not a valid JSON, this handles it
		if r := recover(); r != nil {
			fmt.Println("Recovered in findConcept", r)
			writer.WriteHeader(http.StatusInternalServerError)
		}
	}()

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

func getFoundConcepts(elasticResult *elastic.SearchResult, isScoreIncluded bool) searchResult {
	var foundConcepts []concept
	for _, hit := range elasticResult.Hits.Hits {
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
	return searchResult{Results: foundConcepts}
}

func isScoreIncluded(request *http.Request) bool {
	queryParam := request.URL.Query().Get("include_score")
	includeScore, err := strconv.ParseBool(queryParam)
	if err != nil {
		return false
	}
	return includeScore
}
