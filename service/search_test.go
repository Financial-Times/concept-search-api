package service

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	"gopkg.in/olivere/elastic.v5"
)

const (
	apiBaseUrl  = "http://test.api.ft.com"
	indexName   = "concept"
	conceptType = "genres"
	ftGenreType = "http://www.ft.com/ontology/Genre"
)

func TestNoElasticClient(t *testing.T) {
	service := esConceptSearchService{nil, "test"}

	_, err := service.FindAllConceptsByType(ftGenreType)

	assert.Equal(t, ErrNoElasticClient, err, "error response")
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
			Index(indexName).
			Type(conceptType).
			Id(u).
			BodyJson(payload).
			Do(context.Background())

		uuids = append(uuids, u)
	}

	// ensure test data is immediately available from the index
	ec.Refresh(indexName).Do(context.Background())

	return uuids
}

func TestFindAllConceptsByType(t *testing.T) {
	esURL := getElasticSearchTestURL(t)

	ec, err := elastic.NewClient(
		elastic.SetURL(esURL),
		elastic.SetSniff(false),
	)
	assert.NoError(t, err, "expected no error for ES client")

	_ = writeTestConcepts(ec)

	service := NewEsConceptSearchService(ec, indexName)
	concepts, err := service.FindAllConceptsByType(ftGenreType)

	assert.NoError(t, err, "expected no error for ES read")
	assert.True(t, len(concepts) > 1, "there should be at least two genres")

	var prev string
	for i := range concepts {
		if i > 0 {
			assert.Equal(t, -1, strings.Compare(prev, concepts[i].PrefLabel), "concepts should be ordered")
		}

		prev = concepts[i].PrefLabel
	}
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
