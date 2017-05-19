package service

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"gopkg.in/olivere/elastic.v5"
)

const (
	apiBaseUrl    = "http://test.api.ft.com"
	testIndexName = "test-index"
	conceptType   = "genres"
	ftGenreType   = "http://www.ft.com/ontology/Genre"
)

func TestNoElasticClient(t *testing.T) {
	service := esConceptSearchService{nil, "test"}

	_, err := service.FindAllConceptsByType(ftGenreType)

	assert.Equal(t, ErrNoElasticClient, err, "error response")
}

type EsConceptSearchServiceTestSuite struct {
	suite.Suite
	esURL string
	ec    *elastic.Client
}

func TestEsConceptSearchServiceSuite(t *testing.T) {
	suite.Run(t, new(EsConceptSearchServiceTestSuite))
}

func (s *EsConceptSearchServiceTestSuite) SetupSuite() {
	s.esURL = getElasticSearchTestURL(s.T())

	ec, err := elastic.NewClient(
		elastic.SetURL(s.esURL),
		elastic.SetSniff(false),
	)
	assert.NoError(s.T(), err, "expected no error for ES client")

	s.ec = ec
	_ = writeTestConcepts(s.ec)
}

func (s *EsConceptSearchServiceTestSuite) TearDownSuite() {
	s.ec.DeleteIndex(testIndexName).Do(context.Background())
}

func getElasticSearchTestURL(t *testing.T) string {
	if testing.Short() {
		t.Skip("ElasticSearch integration for long tests only.")
	}

	esURL := os.Getenv("ELASTICSEARCH_TEST_URL")
	if strings.TrimSpace(esURL) == "" {
		t.Fatal("Please set the environment variable ELASTICSEARCH_TEST_URL to run ElasticSearch integration tests (e.g. export ELASTICSEARCH_TEST_URL=http://localhost:9200). Alternatively, run `go test -short` to skip them.")
	}

	return esURL
}

func writeTestConcepts(ec *elastic.Client) []string {
	uuids := []string{}

	for i := 0; i < 2; i++ {
		u := uuid.NewV4().String()

		payload := EsConceptModel{
			Id:         u,
			ApiUrl:     fmt.Sprintf("%s/%ss/%s", apiBaseUrl, conceptType, u),
			PrefLabel:  fmt.Sprintf("Test concept %s %s", conceptType, u),
			Types:      []string{ftGenreType},
			DirectType: ftGenreType,
			Aliases:    []string{},
		}

		ec.Index().
			Index(testIndexName).
			Type(conceptType).
			Id(u).
			BodyJson(payload).
			Do(context.Background())

		uuids = append(uuids, u)
	}

	// ensure test data is immediately available from the index
	ec.Refresh(testIndexName).Do(context.Background())

	return uuids
}

func (s *EsConceptSearchServiceTestSuite) TestFindAllConceptsByType() {
	service := NewEsConceptSearchService(s.ec, testIndexName)
	concepts, err := service.FindAllConceptsByType(ftGenreType)

	assert.NoError(s.T(), err, "expected no error for ES read")
	assert.True(s.T(), len(concepts) > 1, "there should be at least two genres")

	var prev string
	for i := range concepts {
		if i > 0 {
			assert.Equal(s.T(), -1, strings.Compare(prev, concepts[i].PrefLabel), "concepts should be ordered")
		}

		prev = concepts[i].PrefLabel
	}
}

func (s *EsConceptSearchServiceTestSuite) TestFindAllConceptsByTypeInvalid() {
	service := NewEsConceptSearchService(s.ec, testIndexName)
	_, err := service.FindAllConceptsByType("http://www.ft.com/ontology/Foo")

	assert.Equal(s.T(), ErrInvalidConceptType, err, "expected error for ES read")
}
