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
	ErrNoElasticClient                         = errors.New("no ElasticSearch client available")
	errNoConceptTypeParameter                  = NewInputError("no concept type specified")
	errInvalidConceptTypeFormat                = "invalid concept type %v"
	errInvalidConceptTypeForAutocompleteByType = NewInputError("invalid concept type for this search")
	errEmptyTextParameter                      = NewInputError("empty text parameter")
	errNotSupportedCombinationOfConceptTypes   = NewInputError("the combination of concept types is not supported")
	errInvalidBoostTypeParameter               = NewInputError("invalid boost type")
	mentionTypes                               = []string{"http://www.ft.com/ontology/person/Person", "http://www.ft.com/ontology/organisation/Organisation", "http://www.ft.com/ontology/Location", "http://www.ft.com/ontology/Topic"}
)

type ConceptSearchService interface {
	SetElasticClient(client *elastic.Client)
	FindAllConceptsByType(conceptType string) ([]Concept, error)
	SuggestConceptByTextAndTypes(textQuery string, conceptTypes []string) ([]Concept, error)
	SuggestConceptByTextAndTypesWithBoost(textQuery string, conceptTypes []string, boostType string) ([]Concept, error)
	SearchConceptByTextAndTypes(textQuery string, conceptTypes []string) ([]Concept, error)
}

type esConceptSearchService struct {
	esClient               *elastic.Client
	index                  string
	maxSearchResults       int
	maxAutoCompleteResults int
	autoCompleteTypes      *typeSet
	mentionTypes           *typeSet
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
		autoCompleteTypes:      newTypeSet(),
		mentionTypes:           newTypeSet(),
		mappingRefreshInterval: 5 * time.Minute,
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

func searchResultToConcepts(result *elastic.SearchResult) Concepts {
	concepts := Concepts{}
	for _, c := range result.Hits.Hits {
		concept, err := transformToConcept(c.Source, c.Type)
		if err != nil {
			log.Warnf("unmarshallable response from ElasticSearch: %v", err)
			continue
		}
		concepts = append(concepts, concept)
	}

	return concepts
}

func transformToConcept(source *json.RawMessage, esType string) (Concept, error) {
	esConcept := EsConceptModel{}
	err := json.Unmarshal(*source, &esConcept)
	if err != nil {
		return Concept{}, err
	}

	return ConvertToSimpleConcept(esConcept, esType), nil
}

func (s *esConceptSearchService) isAutoCompleteType(t string) bool {
	return s.autoCompleteTypes.contains(t)
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

	return s.searchConceptsForMultipleTypes(textQuery, conceptTypes)
}

func (s *esConceptSearchService) searchConceptsForMultipleTypes(textQuery string, conceptTypes []string) ([]Concept, error) {
	esTypes, err := validateAndConvertToEsTypes(conceptTypes)
	if err != nil {
		return nil, err
	}

	textMatch := elastic.NewMatchQuery("prefLabel.edge_ngram", textQuery)
	mentionsFilter := elastic.NewTermsQuery("_type", toTerms(esTypes)...)
	mentionsQuery := elastic.NewBoolQuery().Must(textMatch).Filter(mentionsFilter).Boost(1)

	result, err := s.esClient.Search(s.index).Size(s.maxAutoCompleteResults).Query(mentionsQuery).Do(context.Background())
	if err != nil {
		log.Errorf("error: %v", err)
		return nil, err
	}

	concepts := searchResultToConcepts(result)
	return concepts, nil
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

func (s *esConceptSearchService) SetElasticClient(client *elastic.Client) {
	s.clientLock.Lock()
	defer s.clientLock.Unlock()
	s.esClient = client

	if s.mappingRefreshTicker != nil {
		s.mappingRefreshTicker.Stop()
	}

	s.initMappings(client)
	s.mappingRefreshTicker = time.NewTicker(s.mappingRefreshInterval)
	go func() {
		for range s.mappingRefreshTicker.C {
			s.initMappings(client)
		}
	}()
}

func (s *esConceptSearchService) elasticClient() *elastic.Client {
	s.clientLock.RLock()
	defer s.clientLock.RUnlock()
	return s.esClient
}
