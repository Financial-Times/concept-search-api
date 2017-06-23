package service

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"sync"

	log "github.com/Sirupsen/logrus"
	"gopkg.in/olivere/elastic.v5"
)

var (
	ErrNoElasticClient    = errors.New("no ElasticSearch client available")
	ErrInvalidConceptType = errors.New("invalid concept type")
	ErrEmptyTextParameter = errors.New("empty text parameter")
)

type ConceptSearchService interface {
	FindAllConceptsByType(conceptType string) ([]Concept, error)
	SuggestConceptByTextAndType(textQuery string, conceptType string) ([]Concept, error)
	SuggestAuthorsByText(textQuery string, conceptType string) ([]Concept, error)
}

type esConceptSearchService struct {
	esClient               *elastic.Client
	index                  string
	maxSearchResults       int
	maxAutoCompleteResults int
	authorsBoost           int
	clientLock             *sync.RWMutex
}

func NewEsConceptSearchService(index string, maxSearchResults int, maxAutoCompleteResults int, authorsBoost int) *esConceptSearchService {
	return &esConceptSearchService{nil, index, maxSearchResults, maxAutoCompleteResults, authorsBoost, &sync.RWMutex{}}
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

	t := esType(conceptType)
	if t == "" {
		return nil, ErrInvalidConceptType
	}

	if err := s.checkElasticClient(); err != nil {
		return nil, err
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

func (s *esConceptSearchService) SuggestAuthorsByText(textQuery string, conceptType string) ([]Concept, error) {
	if textQuery == "" {
		return nil, ErrEmptyTextParameter
	}

	if esType(conceptType) != "people" {
		return nil, ErrInvalidConceptType
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
}

func (s *esConceptSearchService) elasticClient() *elastic.Client {
	s.clientLock.RLock()
	defer s.clientLock.RUnlock()
	return s.esClient
}
