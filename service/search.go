package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"gopkg.in/olivere/elastic.v5"
)

type InputError struct {
	msg string
}

func NewInputError(msg string) InputError {
	return InputError{msg}
}

func NewInputErrorf(format string, args ...interface{}) InputError {
	return InputError{fmt.Sprintf(format, args...)}
}

func (e InputError) Error() string {
	return e.msg
}

var (
	ErrNoElasticClient                       = errors.New("no ElasticSearch client available")
	errNoConceptTypeParameter                = NewInputError("no concept type specified")
	errInvalidConceptTypeFormat              = "invalid concept type %v"
	errEmptyTextParameter                    = NewInputError("empty text parameter")
	errEmptyIdsParameter                     = NewInputError("empty Ids parameter")
	errNotSupportedCombinationOfConceptTypes = NewInputError("the combination of concept types is not supported")
	errInvalidBoostTypeParameter             = NewInputError("invalid boost type")
	mentionTypes                             = []string{"http://www.ft.com/ontology/person/Person", "http://www.ft.com/ontology/organisation/Organisation", "http://www.ft.com/ontology/Location", "http://www.ft.com/ontology/Topic"}
)

type ConceptSearchService interface {
	SetElasticClient(client *elastic.Client)
	FindConceptsById(ids []string) ([]Concept, error)
	FindAllConceptsByType(conceptType string) ([]Concept, error)
	SearchConceptByTextAndTypes(textQuery string, conceptTypes []string) ([]Concept, error)
	SearchConceptByTextAndTypesWithBoost(textQuery string, conceptTypes []string, boostType string) ([]Concept, error)
}

type esConceptSearchService struct {
	esClient               *elastic.Client
	index                  string
	maxSearchResults       int
	maxAutoCompleteResults int
	mappingRefreshTicker   *time.Ticker
	mappingRefreshInterval time.Duration
	authorsBoost           int
	clientLock             *sync.RWMutex
}

func NewEsConceptSearchService(index string, maxSearchResults int, maxAutoCompleteResults int, authorsBoost int) ConceptSearchService {
	return &esConceptSearchService{
		index:                  index,
		maxSearchResults:       maxSearchResults,
		maxAutoCompleteResults: maxAutoCompleteResults,
		authorsBoost:           authorsBoost,
		clientLock:             &sync.RWMutex{},
	}
}

func (s *esConceptSearchService) checkElasticClient() error {
	if s.elasticClient() == nil {
		return ErrNoElasticClient
	}
	return nil
}

func (s *esConceptSearchService) FindAllConceptsByType(conceptType string) ([]Concept, error) {
	t := esType(conceptType)
	if t == "" {
		return nil, NewInputErrorf(errInvalidConceptTypeFormat, conceptType)
	}

	if err := s.checkElasticClient(); err != nil {
		return nil, err
	}

	result, err := s.esClient.Search(s.index).Type(t).Size(s.maxSearchResults).Do(context.Background())
	if err != nil {
		log.Errorf("error: %v", err)
		return nil, err
	}
	concepts := searchResultToConcepts(result)
	sort.Sort(concepts)
	return concepts, nil
}

func (s *esConceptSearchService) FindConceptsById(ids []string) ([]Concept, error) {
	if ids == nil || len(ids) == 0 || containsOnlyEmptyValues(ids) {
		return nil, errEmptyIdsParameter
	}
	if err := s.checkElasticClient(); err != nil {
		return nil, err
	}
	idsQuery := elastic.NewIdsQuery("_all").Ids(ids...)
	result, err := s.esClient.Search(s.index).Size(s.maxSearchResults).Query(idsQuery).Do(context.Background())
	if err != nil {
		log.Errorf("error: %v", err)
		return nil, err
	}
	concepts := searchResultToConcepts(result)
	return concepts, nil
}

func searchResultToConcepts(result *elastic.SearchResult) Concepts {
	concepts := Concepts{}
	for _, c := range result.Hits.Hits {
		concept, err := transformToConcept(c.Source)
		if err != nil {
			log.Warnf("unmarshallable response from ElasticSearch: %v", err)
			continue
		}
		concepts = append(concepts, concept)
	}
	return concepts
}

func transformToConcept(source *json.RawMessage) (Concept, error) {
	esConcept := EsConceptModel{}
	err := json.Unmarshal(*source, &esConcept)
	if err != nil {
		return Concept{}, err
	}
	return ConvertToSimpleConcept(esConcept), nil
}

func (s *esConceptSearchService) SearchConceptByTextAndTypes(textQuery string, conceptTypes []string) ([]Concept, error) {
	if textQuery == "" {
		return nil, errEmptyTextParameter
	}

	if len(conceptTypes) == 0 {
		return nil, errNoConceptTypeParameter
	}
	if err := s.checkElasticClient(); err != nil {
		return nil, err
	}
	return s.searchConceptsForMultipleTypes(textQuery, conceptTypes, "")
}

func (s *esConceptSearchService) SearchConceptByTextAndTypesWithBoost(textQuery string, conceptTypes []string, boostType string) ([]Concept, error) {
	if err := validateForAuthorsSearch(conceptTypes, boostType); err != nil {
		return nil, err
	}
	if textQuery == "" {
		return nil, errEmptyTextParameter
	}
	if len(conceptTypes) == 0 {
		return nil, errNoConceptTypeParameter
	}
	if err := s.checkElasticClient(); err != nil {
		return nil, err
	}
	return s.searchConceptsForMultipleTypes(textQuery, conceptTypes, boostType)
}

func (s *esConceptSearchService) searchConceptsForMultipleTypes(textQuery string, conceptTypes []string, boostType string) ([]Concept, error) {
	esTypes, err := validateAndConvertToEsTypes(conceptTypes)
	if err != nil {
		return nil, err
	}

	textMatch := elastic.NewMatchQuery("prefLabel.edge_ngram", textQuery)
	aliasesExactMatchMustQuery := elastic.NewMatchQuery("aliases.exact_match", textQuery)
	mustQuery := elastic.NewBoolQuery().Should(textMatch, aliasesExactMatchMustQuery).MinimumNumberShouldMatch(1) // All searches must either match loosely on `prefLabel`, or exactly on `aliases`

	termMatchQuery := elastic.NewMatchQuery("prefLabel", textQuery).Boost(0.1)               // Additional boost added if whole terms match, i.e. Donald Trump =returns=> Donald J Trump higher than Donald Trumpy
	exactMatchQuery := elastic.NewMatchQuery("prefLabel.exact_match", textQuery).Boost(0.75) // Further boost if the prefLabel matches exactly (barring special characters)

	topicsBoost := elastic.NewTermQuery("_type", "topics").Boost(1)
	locationBoost := elastic.NewTermQuery("_type", "locations").Boost(0.65)
	peopleBoost := elastic.NewTermQuery("_type", "people").Boost(0.65)

	aliasesExactMatchShouldQuery := elastic.NewMatchQuery("aliases.exact_match", textQuery).Boost(0.65) // Also boost if an alias matches exactly, but this should not precede exact matched prefLabels

	typeFilter := elastic.NewTermsQuery("_type", toTerms(esTypes)...) // filter by type

	shouldMatch := []elastic.Query{termMatchQuery, exactMatchQuery, aliasesExactMatchShouldQuery, topicsBoost, locationBoost, peopleBoost}

	if boostType != "" {
		shouldMatch = append(shouldMatch, elastic.NewTermQuery("isFTAuthor", "true").Boost(1.8))
	}

	theQuery := elastic.NewBoolQuery().Must(mustQuery).Should(shouldMatch...).Filter(typeFilter).MinimumNumberShouldMatch(0).Boost(1)

	result, err := s.esClient.Search(s.index).Size(s.maxAutoCompleteResults).Query(theQuery).SearchType("dfs_query_then_fetch").Do(context.Background())
	if err != nil {
		log.Errorf("error: %v", err)
		return nil, err
	}
	concepts := searchResultToConcepts(result)
	return concepts, nil
}

func validateForAuthorsSearch(conceptTypes []string, boostType string) error {
	if len(conceptTypes) == 0 {
		return errNoConceptTypeParameter
	}
	if len(conceptTypes) > 1 {
		return errNotSupportedCombinationOfConceptTypes
	}
	if esType(conceptTypes[0]) != "people" {
		return NewInputErrorf(errInvalidConceptTypeFormat, conceptTypes[0])
	}
	if boostType != "authors" {
		return errInvalidBoostTypeParameter
	}
	return nil
}

func validateAndConvertToEsTypes(conceptTypes []string) ([]string, error) {
	esTypes := make([]string, len(conceptTypes))
	for _, t := range conceptTypes {
		esT := esType(t)
		if esT == "" {
			return esTypes, NewInputErrorf(errInvalidConceptTypeFormat, t)
		}
		esTypes = append(esTypes, esT)
	}
	return esTypes, nil
}

func toTerms(types []string) []interface{} {
	i := make([]interface{}, 0)
	for _, v := range types {
		i = append(i, v)
	}
	return i
}

func containsOnlyEmptyValues(ids []string) bool {
	for _, v := range ids {
		if v != "" {
			return false
		}
	}
	return true
}

func (s *esConceptSearchService) SetElasticClient(client *elastic.Client) {
	s.clientLock.Lock()
	defer s.clientLock.Unlock()
	s.esClient = client
}

func (s *esConceptSearchService) elasticClient() *elastic.Client {
	s.clientLock.RLock()
	defer s.clientLock.RUnlock()
	return s.esClient
}
