package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sort"

	log "github.com/Sirupsen/logrus"
	"gopkg.in/olivere/elastic.v5"
)

var (
	ErrNoElasticClient    error = errors.New("no ElasticSearch client available")
	ErrInvalidConceptType       = errors.New("invalid concept type")
	ErrEmptyTextParameter       = errors.New("empty text parameter")
)

type ConceptSearchService interface {
	FindAllConceptsByType(conceptType string) ([]Concept, error)
	SuggestConceptByText(textQuery string) ([]Concept, error)
}

type esConceptSearchService struct {
	esClient *elastic.Client
	index    string
}

func NewEsConceptSearchService(client *elastic.Client, index string) *esConceptSearchService {
	return &esConceptSearchService{client, index}
}

func (s *esConceptSearchService) checkElasticClient() error {
	if s.esClient == nil {
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

	result, err := s.esClient.Search(s.index).Type(t).Size(50).Do(context.Background())
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
		by, err := (*c.Source).MarshalJSON()
		if err != nil {
			log.Warnf("unmarshallable response from ElasticSearch: %v", err)
			continue
		}

		esConcept := EsConceptModel{}
		json.NewDecoder(bytes.NewReader(by)).Decode(&esConcept)

		concept := ConvertToSimpleConcept(esConcept, c.Type)
		concepts = append(concepts, concept)
	}
	return concepts
}

func (s *esConceptSearchService) SuggestConceptByText(textQuery string) ([]Concept, error) {
	if textQuery == "" {
		return nil, ErrEmptyTextParameter
	}

	if err := s.checkElasticClient(); err != nil {
		return nil, err
	}

	completionSuggester := elastic.NewCompletionSuggester("conceptSuggestion").Text(textQuery).Field("prefLabel.indexCompletion").Size(50)
	result, err := s.esClient.Search(s.index).Suggester(completionSuggester).Do(context.Background())
	if err != nil {
		log.Errorf("error: %v", err)
		return nil, err
	}

	concepts := searchResultToConcepts(result)
	return concepts, nil
}
