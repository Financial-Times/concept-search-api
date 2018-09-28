package service

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
	"time"

	"github.com/Financial-Times/concept-search-api/util"

	log "github.com/sirupsen/logrus"
	"gopkg.in/olivere/elastic.v5"
)

var (
	errEmptyTextParameter = util.NewInputError("empty text parameter")
	errEmptyIdsParameter  = util.NewInputError("empty Ids parameter")

	mentionTypes = []string{"http://www.ft.com/ontology/person/Person", "http://www.ft.com/ontology/organisation/Organisation", "http://www.ft.com/ontology/Location", "http://www.ft.com/ontology/Topic"}
)

type ConceptSearchService interface {
	SetElasticClient(client *elastic.Client)
	FindConceptsById(ids []string) ([]Concept, error)
	FindAllConceptsByType(conceptType string, includeDeprecated bool) ([]Concept, error)
	SearchConceptByTextAndTypes(textQuery string, conceptTypes []string, includeDeprecated bool) ([]Concept, error)
	SearchConceptByTextAndTypesWithBoost(textQuery string, conceptTypes []string, boostType string, includeDeprecated bool) ([]Concept, error)
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
		return util.ErrNoElasticClient
	}
	return nil
}

func (s *esConceptSearchService) FindAllConceptsByType(conceptType string, includeDeprecated bool) ([]Concept, error) {
	t := util.EsType(conceptType)
	if t == "" {
		return nil, util.NewInputErrorf(util.ErrInvalidConceptTypeFormat, conceptType)
	}

	if err := s.checkElasticClient(); err != nil {
		return nil, err
	}

	query := s.esClient.Search(s.index).Type(t).Size(s.maxSearchResults)
	if !includeDeprecated {
		deprecatedQ := elastic.NewBoolQuery().MustNot(elastic.NewTermQuery("isDeprecated", true))
		query = query.Query(deprecatedQ)
	}

	result, err := query.Do(context.Background())
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

func (s *esConceptSearchService) SearchConceptByTextAndTypes(textQuery string, conceptTypes []string, includeDeprecated bool) ([]Concept, error) {
	if textQuery == "" {
		return nil, errEmptyTextParameter
	}

	if len(conceptTypes) == 0 {
		return nil, util.ErrNoConceptTypeParameter
	}
	if err := s.checkElasticClient(); err != nil {
		return nil, err
	}
	return s.searchConceptsForMultipleTypes(textQuery, conceptTypes, "", includeDeprecated)
}

func (s *esConceptSearchService) SearchConceptByTextAndTypesWithBoost(textQuery string, conceptTypes []string, boostType string, includeDeprecated bool) ([]Concept, error) {
	if err := util.ValidateForAuthorsSearch(conceptTypes, boostType); err != nil {
		return nil, err
	}
	if textQuery == "" {
		return nil, errEmptyTextParameter
	}
	if len(conceptTypes) == 0 {
		return nil, util.ErrNoConceptTypeParameter
	}
	if err := s.checkElasticClient(); err != nil {
		return nil, err
	}
	return s.searchConceptsForMultipleTypes(textQuery, conceptTypes, boostType, includeDeprecated)
}

func (s *esConceptSearchService) searchConceptsForMultipleTypes(textQuery string, conceptTypes []string, boostType string, includeDeprecated bool) ([]Concept, error) {
	esTypes, err := util.ValidateAndConvertToEsTypes(conceptTypes)
	if err != nil {
		return nil, err
	}

	textMatch := elastic.NewMatchQuery("prefLabel.edge_ngram", textQuery)
	aliasesExactMatchMustQuery := elastic.NewMatchQuery("aliases.edge_ngram", textQuery).Boost(0.8)
	mustQuery := elastic.NewBoolQuery().Should(textMatch, aliasesExactMatchMustQuery).MinimumNumberShouldMatch(1) // All searches must either match loosely on `prefLabel`, or exactly on `aliases`

	termMatchQuery := elastic.NewMatchQuery("prefLabel", textQuery).Boost(0.1)               // Additional boost added if whole terms match, i.e. Donald Trump =returns=> Donald J Trump higher than Donald Trumpy
	exactMatchQuery := elastic.NewMatchQuery("prefLabel.exact_match", textQuery).Boost(0.95) // Further boost if the prefLabel matches exactly (barring special characters)

	topicsBoost := elastic.NewTermQuery("_type", "topics").Boost(1.5)
	locationBoost := elastic.NewTermQuery("_type", "locations").Boost(0.25)
	peopleBoost := elastic.NewTermQuery("_type", "people").Boost(0.1)

	// ES library does not support building an exists query like; {"exists": {"field":"scopeNote", "boost":1.7}}
	// Another option to provide the same functionality/boosting is via a bool query.
	scopeNoteExistBoost := elastic.NewBoolQuery().Must(elastic.NewExistsQuery("scopeNote")).Boost(1.7)

	// Phrase match to ensure that documents that contain all the typed terms (in order) are given the full popularity boost
	phraseMatchQuery := elastic.NewFunctionScoreQuery().
		Query(elastic.NewMatchPhraseQuery("prefLabel.edge_ngram", textQuery)).
		AddScoreFunc(elastic.NewWeightFactorFunction(4.5)).
		AddScoreFunc(elastic.NewFieldValueFactorFunction().Field("metrics.annotationsCount").Modifier("ln1p").Missing(0)).
		ScoreMode("multiply").
		BoostMode("replace")

	popularityBoost := elastic.NewFunctionScoreQuery().AddScoreFunc(elastic.NewFieldValueFactorFunction().Field("metrics.annotationsCount").Modifier("ln1p").Missing(0)).Boost(1.5) // smooth the annotations count

	aliasesExactMatchShouldQuery := elastic.NewMatchQuery("aliases.exact_match", textQuery).Boost(0.85) // Also boost if an alias matches exactly, but this should not precede exact matched prefLabels

	typeFilter := elastic.NewTermsQuery("_type", util.ToTerms(esTypes)...) // filter by type

	shouldMatch := []elastic.Query{termMatchQuery, exactMatchQuery, aliasesExactMatchShouldQuery, topicsBoost, locationBoost, peopleBoost, scopeNoteExistBoost, phraseMatchQuery, popularityBoost}

	if boostType != "" {
		shouldMatch = append(shouldMatch, elastic.NewTermQuery("isFTAuthor", "true").Boost(1.8))
	}

	mustNotMatch := []elastic.Query{}
	// by default (include_deprecated is false) the deprecated entities are excluded
	if !includeDeprecated {
		mustNotMatch = append(mustNotMatch, elastic.NewTermQuery("isDeprecated", true)) // exclude deprecated docs
	}

	theQuery := elastic.NewBoolQuery().Must(mustQuery).Should(shouldMatch...).MustNot(mustNotMatch...).Filter(typeFilter).MinimumNumberShouldMatch(0).Boost(1)

	search := s.esClient.Search(s.index).Size(s.maxAutoCompleteResults).Query(theQuery)

	result, err := search.SearchType("dfs_query_then_fetch").Do(context.Background())
	if err != nil {
		log.Errorf("error: %v", err)
		return nil, err
	}
	concepts := searchResultToConcepts(result)
	return concepts, nil
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
