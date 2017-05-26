package service

import (
	"context"
	"encoding/json"
	"errors"
	"sort"

	log "github.com/Sirupsen/logrus"
	"gopkg.in/olivere/elastic.v5"
)

var (
	ErrNoElasticClient    error = errors.New("No ElasticSearch client available")
	ErrInvalidConceptType       = errors.New("Invalid concept type")
)

type ConceptSearchService interface {
	FindAllConceptsByType(conceptType string) ([]Concept, error)
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

	concepts := Concepts{}
	result, err := s.esClient.Search(s.index).Type(t).Size(50).Do(context.Background())
	if err != nil {
		log.Errorf("error: %v", err)
	} else {
		for _, c := range result.Hits.Hits {
			esConcept := EsConceptModel{}
			err := json.Unmarshal(*c.Source, &esConcept)
			if err != nil {
				log.Warnf("unmarshallable response from ElasticSearch: %v", err)
				continue
			}

			concept := ConvertToSimpleConcept(esConcept, c.Type)
			concepts = append(concepts, concept)
		}
	}
	sort.Sort(concepts)

	return concepts, err
}
