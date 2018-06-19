package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/Financial-Times/concept-search-api/util"
	"github.com/Financial-Times/transactionid-utils-go"
	log "github.com/Sirupsen/logrus"
	"gopkg.in/olivere/elastic.v5"
)

type conceptFinder interface {
	FindConcept(writer http.ResponseWriter, request *http.Request)
	SetElasticClient(client *elastic.Client)
}

type esConceptFinder struct {
	client            esClient
	indexName         string
	searchResultLimit int
	lockClient        *sync.RWMutex
}

func newConceptFinder(index string, resultLimit int) conceptFinder {
	return &esConceptFinder{
		indexName:         index,
		searchResultLimit: resultLimit,
		lockClient:        &sync.RWMutex{},
	}
}

func (service *esConceptFinder) FindConcept(writer http.ResponseWriter, request *http.Request) {
	if service.esClient() == nil {
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

	if criteria.Term == nil && len(criteria.ExactMatchTerms) == 0 {
		log.Error("The required data not provided. Check that the JSON contains the 'term' field that is used to provide " +
			"the search criteria, or the 'exactMatchTerms' value(s) for providing exact match results")
		writer.WriteHeader(http.StatusBadRequest)
		return
	}
	if criteria.Term != nil && len(criteria.ExactMatchTerms) > 0 {
		log.Error("Both, 'term' and 'exactMatchTerms' provided. Just one of them should be provided")
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	defer request.Body.Close()

	finalQuery := elastic.NewBoolQuery()
	transactionID := transactionidutils.GetTransactionIDFromRequest(request)

	if criteria.Term != nil {
		log.Infof("Performing concept search for term=%v, transaction_id=%v", *criteria.Term, transactionID)

		multiMatchQuery := elastic.NewMultiMatchQuery(criteria.Term, "prefLabel", "aliases").Type("most_fields")
		termQueryForPreflabelExactMatches := elastic.NewTermQuery("prefLabel.raw", criteria.Term).Boost(2)
		termQueryForAliasesExactMatches := elastic.NewTermQuery("aliases.raw", criteria.Term).Boost(2)

		finalQuery = finalQuery.Should(multiMatchQuery, termQueryForPreflabelExactMatches, termQueryForAliasesExactMatches)
	} else if len(criteria.ExactMatchTerms) > 0 {
		q, statusCode, err := createQueryForExactMatch(request, &criteria, transactionID)
		if err != nil {
			log.WithError(err).Error("Error during creating query for exact matching")
			writer.WriteHeader(statusCode)
			return
		}
		finalQuery = q
	}

	// by default {include_deprecated in (nil, false)} the deprecated entities are excluded
	if !isDeprecatedIncluded(request) {
		finalQuery = finalQuery.MustNot(elastic.NewTermQuery("isDeprecated", true))
	}

	searchResult, err := service.esClient().query(service.indexName, finalQuery, service.searchResultLimit)

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
		foundConcepts := getFoundConcepts(searchResult, isScoreIncluded(request), isFTAuthorIncluded(request))
		encoder := json.NewEncoder(writer)
		if err := encoder.Encode(&foundConcepts); err != nil {
			log.Errorf("Cannot encode result: %s", err.Error())
			writer.WriteHeader(http.StatusInternalServerError)
		}
	} else {
		writer.WriteHeader(http.StatusNotFound)
	}
}

func getFoundConcepts(elasticResult *elastic.SearchResult, isScoreIncluded bool, isFTAuthorIncluded bool) searchResult {
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
			if !isFTAuthorIncluded {
				foundConcept.IsFTAuthor = ""
			}
			foundConcepts = append(foundConcepts, foundConcept)
		}
	}
	return searchResult{Results: foundConcepts}
}

func isDeprecatedIncluded(request *http.Request) bool {
	queryParam := request.URL.Query().Get("include_deprecated")
	if len(queryParam) == 0 {
		return false
	}
	includeDeprecated, err := strconv.ParseBool(queryParam)
	if err != nil {
		return false
	}
	return includeDeprecated
}

func isFTAuthorIncluded(request *http.Request) bool {
	return isFieldIncluded(request, "authors")
}

func isFieldIncluded(request *http.Request, fieldValue string) bool {
	for _, field := range request.URL.Query()["include_field"] {
		if field == fieldValue {
			return true
		}
	}
	return false
}

func isScoreIncluded(request *http.Request) bool {
	queryParam := request.URL.Query().Get("include_score")
	includeScore, err := strconv.ParseBool(queryParam)
	if err != nil {
		return false
	}
	return includeScore
}

func (service *esConceptFinder) SetElasticClient(client *elastic.Client) {
	service.lockClient.Lock()
	defer service.lockClient.Unlock()
	service.client = &esClientWrapper{elasticClient: client}
}

func (service *esConceptFinder) esClient() esClient {
	service.lockClient.RLock()
	defer service.lockClient.RUnlock()
	return service.client
}

func createQueryForExactMatch(request *http.Request, criteria *searchCriteria, transactionID string) (*elastic.BoolQuery, int, error) {
	log.Infof("Performing concept search for aliases=%v, transaction_id=%v", strings.Join(criteria.ExactMatchTerms, ", "), transactionID)

	finalQuery := elastic.NewBoolQuery()

	conceptTypes, conceptTypesFound := util.GetMultipleValueQueryParameter(request, "type")
	boostType, boostTypeFound, boostTypeErr := util.GetSingleValueQueryParameter(request, "boost", "authors")
	extraFilterType, extraFilterTypeFound, extraFilterTypeErr := util.GetSingleValueQueryParameter(request, "filter", "authors")
	if err := util.FirstError(boostTypeErr, extraFilterTypeErr); err != nil {
		return nil, http.StatusBadRequest, err
	}

	// prepare exact match query
	exactMatchQ := []elastic.Query{}
	for _, termFields := range criteria.ExactMatchTerms {
		currentTermFieldsQueries := []elastic.Query{}
		for _, term := range strings.Fields(termFields) {
			currentTermFieldsQueries = append(currentTermFieldsQueries, elastic.NewMatchQuery("aliases", term))
		}
		exactMatchQ = append(exactMatchQ, elastic.NewBoolQuery().Must(currentTermFieldsQueries...))
	}
	finalQuery = finalQuery.Must(elastic.NewBoolQuery().Should(exactMatchQ...))

	// add boost if it is requested
	if boostTypeFound {
		boostQ, err := getBoostQuery(boostType, conceptTypes)
		if err != nil {
			return nil, http.StatusBadRequest, err
		}
		finalQuery = finalQuery.Should(boostQ)
	}

	// add extra filter if it is requested
	if extraFilterTypeFound {
		extraFilterQ, err := getExtraFilterQuery(extraFilterType, conceptTypes)
		if err != nil {
			return nil, http.StatusBadRequest, err
		}
		finalQuery = finalQuery.Must(extraFilterQ)
	}

	// filter for given concept types
	if conceptTypesFound {
		esTypes, err := util.ValidateAndConvertToEsTypes(conceptTypes)
		if err != nil {
			return nil, http.StatusBadRequest, err
		}
		typeFilter := elastic.NewTermsQuery("_type", util.ToTerms(esTypes)...) // filter by type
		finalQuery = finalQuery.Filter(typeFilter)
	}

	return finalQuery, http.StatusOK, nil
}

func getBoostQuery(boostType string, conceptTypes []string) (elastic.Query, error) {
	switch boostType {
	case "authors":
		err := util.ValidateForAuthorsSearch(conceptTypes, boostType)
		if err != nil {
			return nil, err
		}
		return elastic.NewTermQuery("isFTAuthor", "true").Boost(1.8), nil
	default:
		return nil, util.ErrInvalidBoostTypeParameter
	}
}

func getExtraFilterQuery(extraFilterType string, conceptTypes []string) (elastic.Query, error) {
	switch extraFilterType {
	case "authors":
		err := util.ValidateForAuthorsSearch(conceptTypes, extraFilterType)
		if err != nil {
			return nil, err
		}
		return elastic.NewTermQuery("isFTAuthor", "true"), nil
	default:
		return nil, util.ErrInvalidBoostTypeParameter
	}
}
