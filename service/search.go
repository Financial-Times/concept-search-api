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
	return s.suggestConceptForMentions(textQuery, conceptTypes)
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
	return s.autoCompleteTypes.contains(t)
}

func (s *esConceptSearchService) suggestConceptForMentions(textQuery string, conceptTypes []string) ([]Concept, error) {
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
	if len(conceptTypes) != s.mentionTypes.len() {
		return errNotSupportedCombinationOfConceptTypes
	}
	for _, conceptType := range conceptTypes {
		t := esType(conceptType)
		if t == "" {
			return NewInputErrorf(errInvalidConceptTypeFormat, conceptType)
		}
		if !s.mentionTypes.contains(t) {
			return errNotSupportedCombinationOfConceptTypes
		}
	}
	return nil
}

func (s *esConceptSearchService) SuggestConceptByTextAndTypesWithBoost(textQuery string, conceptTypes []string, boostType string) ([]Concept, error) {
	if err := validateForAuthorsSearch(conceptTypes, boostType); err != nil {
		return nil, err
	}
	return s.suggestAuthors(textQuery)
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

func (s *esConceptSearchService) suggestAuthors(textQuery string) ([]Concept, error) {
	if textQuery == "" {
		return nil, errEmptyTextParameter
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

	autoCompleteTypes := []string{}
	mentionTypes := []string{}
	for _, v := range m {
		for conceptType, fields := range v.(map[string]interface{})["mappings"].(map[string]interface{}) {
			prefLabelFields := fields.(map[string]interface{})["prefLabel"].(map[string]interface{})["mapping"].(map[string]interface{})["prefLabel"].(map[string]interface{})["fields"].(map[string]interface{})
			if _, hasContextCompletion := prefLabelFields["completionByContext"]; hasContextCompletion {
				autoCompleteTypes = append(autoCompleteTypes, conceptType)
			}
			if _, hasMentionCompletion := prefLabelFields["mentionsCompletion"]; hasMentionCompletion {
				mentionTypes = append(mentionTypes, conceptType)
			}
		}
	}

	log.Infof("autocomplete by type: %v", autoCompleteTypes)
	s.autoCompleteTypes.updateTypes(autoCompleteTypes)
	log.Infof("mention types: %v", mentionTypes)
	s.mentionTypes.updateTypes(mentionTypes)
}

type typeSet struct {
	sync.RWMutex
	types map[string]struct{}
}

func newTypeSet() *typeSet {
	return &typeSet{types: make(map[string]struct{})}
}

func (s *typeSet) updateTypes(types []string) {
	s.Lock()
	defer s.Unlock()
	s.types = make(map[string]struct{})
	for _, t := range types {
		s.types[t] = struct{}{}
	}
}

func (s *typeSet) contains(t string) bool {
	s.RLock()
	defer s.RUnlock()
	_, found := s.types[t]
	return found
}

func (s *typeSet) len() int {
	s.RLock()
	defer s.RUnlock()
	return len(s.types)
}
