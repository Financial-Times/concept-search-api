package service

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gopkg.in/olivere/elastic.v5"
)

const (
	apiBaseURL    = "http://test.api.ft.com"
	testIndexName = "test-index"
	esGenreType   = "genres"
	esBrandType   = "brands"
	esPeopleType  = "people"
	ftGenreType   = "http://www.ft.com/ontology/Genre"
	ftBrandType   = "http://www.ft.com/ontology/product/Brand"
	ftPeopleType  = "http://www.ft.com/ontology/person/Person"
	mappingURL    = "https://raw.githubusercontent.com/Financial-Times/concept-rw-elasticsearch/Add_mappings_for_brand_people_typeahed/mapping.json"
)

func TestNoElasticClient(t *testing.T) {
	service := esConceptSearchService{nil, "test"}

	_, err := service.FindAllConceptsByType(ftGenreType)
	assert.EqualError(t, err, ErrNoElasticClient.Error(), "error response")

	_, err = service.SuggestConceptByTextAndType("lucy", ftBrandType)
	assert.EqualError(t, err, ErrNoElasticClient.Error(), "error response")
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
	require.NoError(s.T(), err, "expected no error for ES client")

	s.ec = ec

	err = createIndex(s.ec, mappingURL)
	require.NoError(s.T(), err, "expected no error in creating index")

	writeTestConcepts(s.ec, esGenreType, ftGenreType, 4)
	require.NoError(s.T(), err, "expected no error in adding genres")
	err = writeTestConcepts(s.ec, esBrandType, ftBrandType, 4)
	require.NoError(s.T(), err, "expected no error in adding brands")
	err = writeTestConcepts(s.ec, esPeopleType, ftPeopleType, 4)
	require.NoError(s.T(), err, "expected no error in adding people")
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

func createIndex(ec *elastic.Client, mappingURL string) error {
	resp, err := http.Get(mappingURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	_, err = ec.CreateIndex(testIndexName).Body(string(body)).Do(context.Background())
	if err != nil {
		return err
	}
	return nil
}

func writeTestConcepts(ec *elastic.Client, esConceptType string, ftConceptType string, amount int) error {
	for i := 0; i < amount; i++ {
		uuid := uuid.NewV4().String()

		payload := EsConceptModel{
			Id:         uuid,
			ApiUrl:     fmt.Sprintf("%s/%s/%s", apiBaseURL, esConceptType, uuid),
			PrefLabel:  fmt.Sprintf("Test concept %s %s", esConceptType, uuid),
			Types:      []string{ftConceptType},
			DirectType: ftConceptType,
			Aliases:    []string{},
		}

		_, err := ec.Index().
			Index(testIndexName).
			Type(esConceptType).
			Id(uuid).
			BodyJson(payload).
			Do(context.Background())
		if err != nil {
			return err
		}
	}

	// ensure test data is immediately available from the index
	_, err := ec.Refresh(testIndexName).Do(context.Background())
	if err != nil {
		return err
	}
	return nil
}

func (s *EsConceptSearchServiceTestSuite) TestFindAllConceptsByType() {
	service := NewEsConceptSearchService(s.ec, testIndexName)
	concepts, err := service.FindAllConceptsByType(ftGenreType)

	assert.NoError(s.T(), err, "expected no error for ES read")
	assert.Len(s.T(), concepts, 4, "there should be four genres")

	var prev string
	for i := range concepts {
		if i > 0 {
			assert.Equal(s.T(), -1, strings.Compare(prev, concepts[i].PrefLabel), "concepts should be ordered")
		}
		assert.Equal(s.T(), ftGenreType, concepts[i].ConceptType, "Results should be of type FT Brand")
		prev = concepts[i].PrefLabel
	}
}

func (s *EsConceptSearchServiceTestSuite) TestFindAllConceptsByTypeInvalid() {
	service := NewEsConceptSearchService(s.ec, testIndexName)
	_, err := service.FindAllConceptsByType("http://www.ft.com/ontology/Foo")

	assert.Equal(s.T(), ErrInvalidConceptType, err, "expected error for ES read")
}

func (s *EsConceptSearchServiceTestSuite) TestSuggestConceptByTextAndTypeInvalidTextParameter() {
	service := NewEsConceptSearchService(s.ec, testIndexName)
	_, err := service.SuggestConceptByTextAndType("", ftBrandType)
	assert.EqualError(s.T(), err, ErrEmptyTextParameter.Error(), "error response")
}

func (s *EsConceptSearchServiceTestSuite) TestSuggestConceptByTextAndType() {
	service := NewEsConceptSearchService(s.ec, testIndexName)
	concepts, err := service.SuggestConceptByTextAndType("test", ftBrandType)
	assert.NoError(s.T(), err, "expected no error for ES read")
	assert.Len(s.T(), concepts, 4, "there should be four results")
	for _, c := range concepts {
		assert.Equal(s.T(), ftBrandType, c.ConceptType, "Results should be of type FT Brand")
	}
}
