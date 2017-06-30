package service

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"gopkg.in/olivere/elastic.v5"
)

var (
	ErrNoElasticClient                         = errors.New("no ElasticSearch client available")
	ErrInvalidConceptType                      = errors.New("invalid concept type")
	ErrInvalidConceptTypeForAutocompleteByType = errors.New("invalid concept type for this search")
	ErrEmptyTextParameter                      = errors.New("empty text parameter")
)

type ConceptSearchService interface {
	FindAllConceptsByType(conceptType string) ([]Concept, error)
	SuggestConceptByTextAndType(textQuery string, conceptType string) ([]Concept, error)
	SuggestConceptByText(textQuery string) ([]Concept, error)
}

type esConceptSearchService struct {
	esClient               *elastic.Client
	index                  string
	maxSearchResults       int
	maxAutoCompleteResults int
	autoCompleteByType     map[string]struct{}
	mappingRefreshTicker   *time.Ticker
	clientLock             *sync.RWMutex
}

func NewEsConceptSearchService(index string, maxSearchResults int, maxAutoCompleteResults int) *esConceptSearchService {
	return &esConceptSearchService{index: index,
		maxSearchResults:       maxSearchResults,
		maxAutoCompleteResults: maxAutoCompleteResults,
		autoCompleteByType:     make(map[string]struct{}),
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
		return nil, ErrInvalidConceptType
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

func (s *esConceptSearchService) SuggestConceptByTextAndType(textQuery string, conceptType string) ([]Concept, error) {
	if textQuery == "" {
		return nil, ErrEmptyTextParameter
	}

	if err := s.checkElasticClient(); err != nil {
		return nil, err
	}

	t := esType(conceptType)
	if t == "" {
		return nil, ErrInvalidConceptType
	}
	if _, found := s.autoCompleteByType[t]; !found {
		return nil, ErrInvalidConceptTypeForAutocompleteByType
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

func (s *esConceptSearchService) SuggestConceptByText(textQuery string) ([]Concept, error) {
	if textQuery == "" {
		return nil, ErrEmptyTextParameter
	}

	if err := s.checkElasticClient(); err != nil {
		return nil, err
	}

	completionSuggester := elastic.NewCompletionSuggester("conceptSuggestion").Text(textQuery).Field("prefLabel.mentionsCompletion").Size(10)
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
	s.mappingRefreshTicker = time.NewTicker(5 * time.Minute)
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
