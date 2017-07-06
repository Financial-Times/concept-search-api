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
)

type ConceptSearchService interface {
	SetElasticClient(client *elastic.Client)
	FindAllConceptsByType(conceptType string) ([]Concept, error)
	SuggestConceptByTextAndTypes(textQuery string, conceptTypes []string) ([]Concept, error)
	SuggestConceptByTextAndTypesWithBoost(textQuery string, conceptTypes []string, boostType string) ([]Concept, error)
}

type esConceptSearchService struct {
	esClient               *elastic.Client
	index                  string
	maxSearchResults       int
	maxAutoCompleteResults int
	autoCompleteByType     map[string]struct{}
	autoCompleteTypesLock  *sync.RWMutex
	mappingRefreshTicker   *time.Ticker
	mappingRefreshInterval time.Duration
	authorsBoost           int
	clientLock             *sync.RWMutex
}

func NewEsConceptSearchService(index string, maxSearchResults int, maxAutoCompleteResults int, authorsBoost int) ConceptSearchService {
	return &esConceptSearchService{index: index,
		maxSearchResults:       maxSearchResults,
		maxAutoCompleteResults: maxAutoCompleteResults,
		autoCompleteByType:     make(map[string]struct{}),
		autoCompleteTypesLock:  &sync.RWMutex{},
		mappingRefreshInterval: 5 * time.Minute,
		authorsBoost:           authorsBoost,
		clientLock:             &sync.RWMutex{}}
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

func suggestResultToConcepts(result *elastic.SearchResult) Concepts {
	concepts := Concepts{}
	for _, c := range result.Suggest["conceptSuggestion"][0].Options {
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

func (s *esConceptSearchService) SuggestConceptByTextAndTypes(textQuery string, conceptTypes []string) ([]Concept, error) {
	if textQuery == "" {
		return nil, errEmptyTextParameter
	}

	if len(conceptTypes) == 0 {
		return nil, errNoConceptTypeParameter
	}
	if err := s.checkElasticClient(); err != nil {
		return nil, err
	}
	if len(conceptTypes) == 1 {
		return s.suggestConceptByTextAndType(textQuery, conceptTypes[0])
	}
	return s.suggestConceptByTextAndTypes(textQuery, conceptTypes)
}

func (s *esConceptSearchService) suggestConceptByTextAndType(textQuery string, conceptType string) ([]Concept, error) {
	t := esType(conceptType)
	if t == "" {
		return nil, NewInputErrorf(errInvalidConceptTypeFormat, conceptType)
	}

	if !s.isAutoCompleteType(t) {
		return nil, errInvalidConceptTypeForAutocompleteByType
	}

	typeContext := elastic.NewSuggesterCategoryQuery("typeContext", t)
	completionSuggester := elastic.NewCompletionSuggester("conceptSuggestion").Text(textQuery).Field("prefLabel.completionByContext").ContextQuery(typeContext).Size(s.maxAutoCompleteResults)
	result, err := s.esClient.Search(s.index).Suggester(completionSuggester).Do(context.Background())
	if err != nil {
		log.Errorf("error: %v", err)
		return nil, err
	}

	concepts := suggestResultToConcepts(result)
	return concepts, nil
}

func (s *esConceptSearchService) isAutoCompleteType(t string) bool {
	s.autoCompleteTypesLock.RLock()
	defer s.autoCompleteTypesLock.RUnlock()

	_, found := s.autoCompleteByType[t]
	return found
}

func (s *esConceptSearchService) suggestConceptByTextAndTypes(textQuery string, conceptTypes []string) ([]Concept, error) {
	if err := s.validateTypesForMentionsCompletion(conceptTypes); err != nil {
		return nil, err
	}

	completionSuggester := elastic.NewCompletionSuggester("conceptSuggestion").Text(textQuery).Field("prefLabel.mentionsCompletion").Size(s.maxAutoCompleteResults)
	result, err := s.esClient.Search(s.index).Suggester(completionSuggester).Do(context.Background())
	if err != nil {
		log.Errorf("error: %v", err)
		return nil, err
	}

	concepts := suggestResultToConcepts(result)
	return concepts, nil
}

func (s *esConceptSearchService) validateTypesForMentionsCompletion(conceptTypes []string) error {
	//TODO proper scan in ES of supported types
	if len(conceptTypes) != 4 {
		return errNotSupportedCombinationOfConceptTypes
	}
	for _, conceptType := range conceptTypes {
		t := esType(conceptType)
		if t == "" {
			return NewInputErrorf(errInvalidConceptTypeFormat, conceptType)
		}
		if t != "people" && t != "organisations" && t != "locations" && t != "topics" {
			return errNotSupportedCombinationOfConceptTypes
		}
	}
	return nil
}

func (s *esConceptSearchService) SuggestConceptByTextAndTypesWithBoost(textQuery string, conceptTypes []string, boostType string) ([]Concept, error) {
	if textQuery == "" {
		return nil, errEmptyTextParameter
	}
	if len(conceptTypes) == 0 {
		return nil, errNoConceptTypeParameter
	}
	if len(conceptTypes) > 1 {
		return nil, errNotSupportedCombinationOfConceptTypes
	}
	if esType(conceptTypes[0]) != "people" {
		return nil, NewInputErrorf(errInvalidConceptTypeFormat, conceptTypes[0])
	}
	if boostType != "authors" {
		return nil, errInvalidBoostTypeParameter
	}

	if err := s.checkElasticClient(); err != nil {
		return nil, err
	}

	typeContext := elastic.NewSuggesterCategoryQuery("typeContext", "people")
	authorContext := elastic.NewSuggesterCategoryQuery("authorContext").ValueWithBoost("true", s.authorsBoost)

	completionSuggester := elastic.NewCompletionSuggester("conceptSuggestion").Text(textQuery).Field("prefLabel.authorCompletionByContext").ContextQueries(typeContext, authorContext).Size(s.maxAutoCompleteResults)

	result, err := s.esClient.Search(s.index).Suggester(completionSuggester).Do(context.Background())
	if err != nil {
		log.Errorf("error: %v", err)
		return nil, err
	}

	concepts := suggestResultToConcepts(result)
	return concepts, nil
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

func (s *esConceptSearchService) initMappings(client *elastic.Client) {
	s.autoCompleteTypesLock.Lock()
	defer s.autoCompleteTypesLock.Unlock()

	s.autoCompleteByType = make(map[string]struct{})

	mapping := elastic.NewIndicesGetFieldMappingService(client)
	m, err := mapping.Index(s.index).Field("prefLabel").Do(context.Background())

	if err != nil {
		log.Errorf("unable to read ES mappings: %v", err)
		return
	}

	if len(m) != 1 {
		log.Errorf("mappings for index are unexpected size: %v", len(m))
		return
	}

	for _, v := range m {
		for conceptType, fields := range v.(map[string]interface{})["mappings"].(map[string]interface{}) {
			prefLabelFields := fields.(map[string]interface{})["prefLabel"].(map[string]interface{})["mapping"].(map[string]interface{})["prefLabel"].(map[string]interface{})["fields"].(map[string]interface{})
			if _, hasContextCompletion := prefLabelFields["completionByContext"]; hasContextCompletion {
				s.autoCompleteByType[conceptType] = struct{}{}
			}
		}
	}

	log.Infof("autocomplete by type: %v", s.autoCompleteByType)
}
