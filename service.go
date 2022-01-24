package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/Financial-Times/concept-search-api/util"
	transactionidutils "github.com/Financial-Times/transactionid-utils-go"
	"github.com/olivere/elastic/v7"
	log "github.com/sirupsen/logrus"
)

type conceptFinder interface {
	FindConcept(writer http.ResponseWriter, request *http.Request)
	SetElasticClient(client *elastic.Client)
}

type esConceptFinder struct {
	client              esClient
	defaultIndex        string
	extendedSearchIndex string

	searchResultLimit int
	lockClient        *sync.RWMutex
}

func newConceptFinder(defaultIndex string, extendedSearchIndex string, resultLimit int) conceptFinder {
	return &esConceptFinder{
		defaultIndex:        defaultIndex,
		extendedSearchIndex: extendedSearchIndex,
		searchResultLimit:   resultLimit,
		lockClient:          &sync.RWMutex{},
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

	if criteria.Term == nil && len(criteria.BestMatchTerms) == 0 {
		log.Error("The required data not provided. Check that the JSON contains the 'term' field that is used to provide " +
			"the search criteria, or the 'bestMatchTerms' value(s) for providing best match results")
		writer.WriteHeader(http.StatusBadRequest)
		return
	}
	if criteria.Term != nil && len(criteria.BestMatchTerms) > 0 {
		log.Error("Both, 'term' and 'bestMatchTerms' provided. Just one of them should be provided")
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	defer request.Body.Close()

	transactionID := transactionidutils.GetTransactionIDFromRequest(request)

	if criteria.Term != nil {
		service.findConceptsWithTerm(writer, request, &criteria, transactionID)
	} else if len(criteria.BestMatchTerms) > 0 {
		service.findConceptsWithBestMatch(writer, request, &criteria, transactionID)
	}
}

func (service *esConceptFinder) findConceptsWithTerm(writer http.ResponseWriter, request *http.Request, criteria *searchCriteria, transactionID string) {
	log.Infof("Performing concept search for term=%v, transaction_id=%v", *criteria.Term, transactionID)

	multiMatchQuery := elastic.NewMultiMatchQuery(criteria.Term, "prefLabel", "aliases").Type("most_fields")
	termQueryForPreflabelExactMatches := elastic.NewTermQuery("prefLabel.raw", criteria.Term).Boost(2)
	termQueryForAliasesExactMatches := elastic.NewTermQuery("aliases.raw", criteria.Term).Boost(2)

	finalQuery := elastic.NewBoolQuery().Should(multiMatchQuery, termQueryForPreflabelExactMatches, termQueryForAliasesExactMatches)

	// by default {include_deprecated in (nil, false)} the deprecated entities are excluded
	if !isDeprecatedIncluded(request) {
		finalQuery = finalQuery.MustNot(elastic.NewTermQuery("isDeprecated", true))
	}

	index := service.defaultIndex
	if isSearchAllAuthorities(request) {
		index = service.extendedSearchIndex
	}

	searchResult, err := service.esClient().query(index, finalQuery, service.searchResultLimit)

	if err != nil {
		log.Errorf("There was an error executing the query on ES: %s", err.Error())
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	defer func() {
		// searchResult.Hits.TotalHits call panics if the result from ES is not a valid JSON, this handles it
		if r := recover(); r != nil {
			log.WithField("Recover", r).Error("Recovered in findConcept")
			writer.WriteHeader(http.StatusInternalServerError)
		}
	}()

	if searchResult.Hits.TotalHits.Value > 0 {
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

func (service *esConceptFinder) findConceptsWithBestMatch(writer http.ResponseWriter, request *http.Request, criteria *searchCriteria, transactionID string) {
	searchWrappers, statusCode, err := createSearchRequestsForBestMatch(request, criteria, transactionID, service.searchResultLimit)
	if err != nil {
		log.WithError(err).Error("Error during query for best matching")
		writer.WriteHeader(statusCode)
		return
	}

	searchRequests := []*elastic.SearchRequest{}
	for _, searchWrapper := range searchWrappers {
		searchRequests = append(searchRequests, searchWrapper.searchRequest)
	}

	index := service.defaultIndex
	if isSearchAllAuthorities(request) {
		index = service.extendedSearchIndex
	}

	res, err := service.esClient().multiSearchQuery(index, searchRequests...)
	if err != nil {
		log.Errorf("There was an error executing the query on ES: %s", err.Error())
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	noResultsCounter := 0
	currentRespIdx := 0
	finalResults := make(map[string][]concept)
	for _, searchRequestRes := range res.Responses {
		if searchRequestRes.Hits.TotalHits.Value > 0 {
			foundConcepts := getFoundConcepts(searchRequestRes, isScoreIncluded(request), isFTAuthorIncluded(request))
			finalResults[searchWrappers[currentRespIdx].term] = foundConcepts.Results[:1]
		} else {
			finalResults[searchWrappers[currentRespIdx].term] = []concept{}
			noResultsCounter++
		}
		currentRespIdx++
	}

	if noResultsCounter == len(searchWrappers) {
		writer.WriteHeader(http.StatusNotFound)
		return
	}

	writer.Header().Add("Content-Type", "application/json")
	encoder := json.NewEncoder(writer)
	if err := encoder.Encode(finalResults); err != nil {
		log.Errorf("Cannot encode result: %s", err.Error())
		writer.WriteHeader(http.StatusInternalServerError)
	}
}

func getFoundConcepts(elasticResult *elastic.SearchResult, isScoreIncluded bool, isFTAuthorIncluded bool) searchResult {
	var foundConcepts []concept
	for _, hit := range elasticResult.Hits.Hits {
		var foundConcept concept
		err := json.Unmarshal(hit.Source, &foundConcept)
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
	includeDeprecated, _, err := util.GetBoolQueryParameter(request, "include_deprecated", false)
	if err != nil {
		return false
	}

	return includeDeprecated
}

func isSearchAllAuthorities(request *http.Request) bool {
	searchAllAuthorities, _, err := util.GetBoolQueryParameter(request, "searchAllAuthorities", false)
	if err != nil {
		return false
	}

	return searchAllAuthorities
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

func createSearchRequestsForBestMatch(request *http.Request, criteria *searchCriteria, transactionID string, size int) ([]*multiSearchWrapper, int, error) {
	log.Infof("Performing concept search for bestMatchTerms=%v, transaction_id=%v", strings.Join(criteria.BestMatchTerms, ", "), transactionID)

	requests := []*multiSearchWrapper{}
	for _, searchingTerm := range criteria.BestMatchTerms {
		finalQuery := elastic.NewBoolQuery()

		// prepare best match query
		bestMatchQ := elastic.NewMatchQuery("aliases", searchingTerm).Operator("and")
		finalQuery = finalQuery.Must(bestMatchQ)

		// add boost if it is requested
		if len(criteria.BoostType) > 0 {
			boostQ, err := getBoostQuery(criteria.BoostType, criteria.ConceptTypes)
			if err != nil {
				return nil, http.StatusBadRequest, err
			}
			finalQuery = finalQuery.Should(boostQ)
		}

		// add extra filter if it is requested
		if len(criteria.FilterType) > 0 {
			extraFilterQ, err := getExtraFilterQuery(criteria.FilterType, criteria.ConceptTypes)
			if err != nil {
				return nil, http.StatusBadRequest, err
			}
			finalQuery = finalQuery.Filter(extraFilterQ)
		}

		// filter for given concept types
		if len(criteria.ConceptTypes) > 0 {
			esTypes, _, err := util.ValidateAndConvertToEsTypes(criteria.ConceptTypes)
			if err != nil {
				return nil, http.StatusBadRequest, err
			}
			typeFilter := elastic.NewTermsQuery("type", util.ToTerms(esTypes)...) // filter by type
			finalQuery = finalQuery.Filter(typeFilter)
		}

		// filter the deprecated concepts out
		if !isDeprecatedIncluded(request) {
			finalQuery = finalQuery.MustNot(elastic.NewTermQuery("isDeprecated", true))
		}

		// requests
		ss := elastic.NewSearchSource().Size(size).Query(finalQuery)
		sq := elastic.NewSearchRequest().Source(ss)
		requests = append(requests, &multiSearchWrapper{
			term:          searchingTerm,
			searchRequest: sq,
		})
	}

	return requests, http.StatusOK, nil
}

func getBoostQuery(boostType string, conceptTypes []string) (elastic.Query, error) {
	switch boostType {
	case "authors":
		err := util.ValidateForAuthorsSearch(conceptTypes, boostType)
		if err != nil {
			return nil, err
		}
		// got from search.go#searchConceptsForMultipleTypes - not random 1.8 value. It was tunned in there
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
