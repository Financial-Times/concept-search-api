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
	"gopkg.in/olivere/elastic.v5"
)

const (
	apiBaseURL   = "http://test.api.ft.com"
	indexName    = "concepts"
	esGenreType  = "genres"
	esBrandType  = "brands"
	esPeopleType = "people"
	ftGenreType  = "http://www.ft.com/ontology/Genre"
	ftBrandType  = "http://www.ft.com/ontology/product/Brand"
	ftPeopleType = "http://www.ft.com/ontology/person/Person"
	mappingURL   = "https://raw.githubusercontent.com/Financial-Times/concept-rw-elasticsearch/Add_mappings_for_brand_people_typeahed/mapping.json"
)

func TestNoElasticClient(t *testing.T) {
	service := esConceptSearchService{nil, "test"}

	_, err := service.FindAllConceptsByType(ftGenreType)
	assert.EqualError(t, err, ErrNoElasticClient.Error(), "error response")

	_, err = service.SuggestConceptByText("lucy")
	assert.EqualError(t, err, ErrNoElasticClient.Error(), "error response")

	_, err = service.SuggestConceptByTextAndType("lucy", ftBrandType)
	assert.EqualError(t, err, ErrNoElasticClient.Error(), "error response")
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
	_, err = ec.CreateIndex(indexName).Body(string(body)).Do(context.Background())
	fmt.Println(err)
	return nil
}

func writeTestConcepts(ec *elastic.Client, esConceptType string, ftConceptType string, amount int) []string {
	uuids := []string{}

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

		ec.Index().
			Index(indexName).
			Type(esConceptType).
			Id(uuid).
			BodyJson(payload).
			Do(context.Background())

		uuids = append(uuids, uuid)
	}

	// ensure test data is immediately available from the index
	ec.Refresh(indexName).Do(context.Background())

	return uuids
}

func cleanElasticSearch(ec *elastic.Client) {
	ec.DeleteIndex(indexName).Do(context.Background())
	ec.Refresh(indexName).Do(context.Background())
}

func TestFindAllConceptsByType(t *testing.T) {
	esURL := getElasticSearchTestURL(t)

	ec, err := elastic.NewClient(
		elastic.SetURL(esURL),
		elastic.SetSniff(false),
	)
	assert.NoError(t, err, "expected no error for ES client")

	_ = createIndex(ec, mappingURL)
	_ = writeTestConcepts(ec, esGenreType, ftGenreType, 2)
	_ = writeTestConcepts(ec, esBrandType, ftBrandType, 2)

	service := NewEsConceptSearchService(ec, indexName)
	concepts, err := service.FindAllConceptsByType(ftGenreType)

	assert.NoError(t, err, "expected no error for ES read")
	assert.Len(t, concepts, 2, "there should be two genres")

	var prev string
	for i := range concepts {
		if i > 0 {
			assert.Equal(t, -1, strings.Compare(prev, concepts[i].PrefLabel), "concepts should be ordered")
		}

		prev = concepts[i].PrefLabel
	}

	cleanElasticSearch(ec)
}

func TestFindAllConceptsByTypeInvalid(t *testing.T) {
	esURL := getElasticSearchTestURL(t)

	ec, err := elastic.NewClient(
		elastic.SetURL(esURL),
		elastic.SetSniff(false),
	)
	assert.NoError(t, err, "expected no error for ES client")

	service := NewEsConceptSearchService(ec, indexName)
	_, err = service.FindAllConceptsByType("http://www.ft.com/ontology/Foo")

	assert.Equal(t, ErrInvalidConceptType, err, "expected error for ES read")
}

func TestSuggestConceptByTextInvalidTextParameter(t *testing.T) {
	esURL := getElasticSearchTestURL(t)

	ec, err := elastic.NewClient(
		elastic.SetURL(esURL),
		elastic.SetSniff(false),
	)
	assert.NoError(t, err, "expected no error for ES client")

	service := NewEsConceptSearchService(ec, indexName)

	_, err = service.SuggestConceptByText("")
	assert.EqualError(t, err, ErrEmptyTextParameter.Error(), "error response")
}

func TestSuggestConceptByTextAndTypeInvalidTextParameter(t *testing.T) {
	esURL := getElasticSearchTestURL(t)

	ec, err := elastic.NewClient(
		elastic.SetURL(esURL),
		elastic.SetSniff(false),
	)
	assert.NoError(t, err, "expected no error for ES client")

	service := NewEsConceptSearchService(ec, indexName)

	_, err = service.SuggestConceptByTextAndType("", ftBrandType)
	assert.EqualError(t, err, ErrEmptyTextParameter.Error(), "error response")
}

func TestSuggestConceptByText(t *testing.T) {
	esURL := getElasticSearchTestURL(t)

	ec, err := elastic.NewClient(
		elastic.SetURL(esURL),
		elastic.SetSniff(false),
	)
	assert.NoError(t, err, "expected no error for ES client")

	_ = createIndex(ec, mappingURL)
	_ = writeTestConcepts(ec, esGenreType, ftGenreType, 4)
	_ = writeTestConcepts(ec, esBrandType, ftBrandType, 4)
	_ = writeTestConcepts(ec, esPeopleType, ftPeopleType, 4)

	service := NewEsConceptSearchService(ec, indexName)
	concepts, err := service.SuggestConceptByText("test")

	assert.NoError(t, err, "expected no error for ES read")
	assert.Len(t, concepts, 8, "there should be eight results")

	cleanElasticSearch(ec)
}

func TestSuggestConceptByTextAndType(t *testing.T) {
	esURL := getElasticSearchTestURL(t)

	ec, err := elastic.NewClient(
		elastic.SetURL(esURL),
		elastic.SetSniff(false),
	)
	assert.NoError(t, err, "expected no error for ES client")

	_ = createIndex(ec, mappingURL)
	_ = writeTestConcepts(ec, esGenreType, ftGenreType, 4)
	_ = writeTestConcepts(ec, esBrandType, ftBrandType, 4)
	_ = writeTestConcepts(ec, esPeopleType, ftPeopleType, 4)

	service := NewEsConceptSearchService(ec, indexName)
	concepts, err := service.SuggestConceptByTextAndType("test", ftBrandType)

	assert.NoError(t, err, "expected no error for ES read")
	assert.Len(t, concepts, 4, "there should be four results")

	for _, c := range concepts {
		assert.Equal(t, ftBrandType, c.ConceptType, "Results should be of type FT Brand")
	}

	cleanElasticSearch(ec)
}
