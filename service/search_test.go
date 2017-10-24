package service

import (
	"context"
	"fmt"
	"io/ioutil"
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
	apiBaseURL         = "http://test.api.ft.com"
	testIndexName      = "test-index"
	esGenreType        = "genres"
	esBrandType        = "brands"
	esPeopleType       = "people"
	esOrganisationType = "organisations"
	esLocationType     = "locations"
	esTopicType        = "topics"
	ftGenreType        = "http://www.ft.com/ontology/Genre"
	ftBrandType        = "http://www.ft.com/ontology/product/Brand"
	ftPeopleType       = "http://www.ft.com/ontology/person/Person"
	ftOrganisationType = "http://www.ft.com/ontology/organisation/Organisation"
	ftLocationType     = "http://www.ft.com/ontology/Location"
	ftTopicType        = "http://www.ft.com/ontology/Topic"
	testMappingFile    = "test/mapping.json"
)

func TestNoElasticClient(t *testing.T) {
	service := NewEsConceptSearchService("test", 50, 10, 2)

	_, err := service.FindAllConceptsByType(ftGenreType)
	assert.EqualError(t, err, ErrNoElasticClient.Error(), "error response")

	_, err = service.SuggestConceptByTextAndTypes("lucy", []string{ftBrandType})
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

	err = createIndex(s.ec, testMappingFile)
	require.NoError(s.T(), err, "expected no error in creating index")

	writeTestConcepts(s.ec, esGenreType, ftGenreType, 4)
	require.NoError(s.T(), err, "expected no error in adding genres")
	err = writeTestConcepts(s.ec, esBrandType, ftBrandType, 4)
	require.NoError(s.T(), err, "expected no error in adding brands")
	err = writeTestConcepts(s.ec, esPeopleType, ftPeopleType, 4)
	require.NoError(s.T(), err, "expected no error in adding people")
	err = writeTestAuthors(s.ec, 4)
	require.NoError(s.T(), err, "expected no error in adding authors")
	err = writeTestConcepts(s.ec, esOrganisationType, ftOrganisationType, 1)
	require.NoError(s.T(), err, "expected no error in adding organisations")
	err = writeTestConcepts(s.ec, esLocationType, ftLocationType, 2)
	require.NoError(s.T(), err, "expected no error in adding locations")
	err = writeTestConcepts(s.ec, esTopicType, ftTopicType, 2)
	require.NoError(s.T(), err, "expected no error in adding topics")
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

func createIndex(ec *elastic.Client, mappingFile string) error {
	mapping, err := ioutil.ReadFile(mappingFile)
	if err != nil {
		return err
	}
	_, err = ec.CreateIndex(testIndexName).Body(string(mapping)).Do(context.Background())
	if err != nil {
		return err
	}
	return nil
}

func writeTestAuthors(ec *elastic.Client, amount int) error {
	for i := 0; i < amount; i++ {
		uuid := uuid.NewV4().String()

		ftAuthor := "true"
		payload := EsConceptModel{
			Id:         uuid,
			ApiUrl:     fmt.Sprintf("%s/%s/%s", apiBaseURL, esPeopleType, uuid),
			PrefLabel:  fmt.Sprintf("Test concept %s %s", esPeopleType, uuid),
			Types:      []string{ftPeopleType},
			DirectType: ftPeopleType,
			Aliases:    []string{},
			IsFTAuthor: &ftAuthor,
		}

		_, err := ec.Index().
			Index(testIndexName).
			Type(esPeopleType).
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

func writeTestConcepts(ec *elastic.Client, esConceptType string, ftConceptType string, amount int) error {
	for i := 0; i < amount; i++ {
		uuid := uuid.NewV4().String()
		err := writeTestConcept(ec, uuid, esConceptType, ftConceptType, fmt.Sprintf("Test concept %s %s", esConceptType, uuid))
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

func writeTestConcept(ec *elastic.Client, uuid string, esConceptType string, ftConceptType string, prefLabel string) error {
	payload := EsConceptModel{
		Id:         uuid,
		ApiUrl:     fmt.Sprintf("%s/%s/%s", apiBaseURL, esConceptType, uuid),
		PrefLabel:  prefLabel,
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
	return nil
}

func (s *EsConceptSearchServiceTestSuite) TestFindAllConceptsByType() {
	service := NewEsConceptSearchService(testIndexName, 10, 10, 2)
	service.SetElasticClient(s.ec)

	concepts, err := service.FindAllConceptsByType(ftGenreType)

	assert.NoError(s.T(), err, "expected no error for ES read")
	assert.Len(s.T(), concepts, 4, "there should be four genres")

	var prev string
	for i := range concepts {
		if i > 0 {
			assert.Equal(s.T(), -1, strings.Compare(prev, concepts[i].PrefLabel), "concepts should be ordered")
		}
		assert.Equal(s.T(), ftGenreType, concepts[i].ConceptType, "Results should be of type FT Genre")
		prev = concepts[i].PrefLabel
	}
}

func (s *EsConceptSearchServiceTestSuite) TestFindAllConceptsByTypeResultSize() {
	service := NewEsConceptSearchService(testIndexName, 3, 10, 2)
	service.SetElasticClient(s.ec)
	concepts, err := service.FindAllConceptsByType(ftGenreType)

	assert.NoError(s.T(), err, "expected no error for ES read")
	assert.Len(s.T(), concepts, 3, "there should be three genres")

	var prev string
	for i := range concepts {
		if i > 0 {
			assert.Equal(s.T(), -1, strings.Compare(prev, concepts[i].PrefLabel), "concepts should be ordered")
		}
		assert.Equal(s.T(), ftGenreType, concepts[i].ConceptType, "Results should be of type FT Genre")
		prev = concepts[i].PrefLabel
	}
}

func (s *EsConceptSearchServiceTestSuite) TestFindAllConceptsByTypeInvalid() {
	service := NewEsConceptSearchService(testIndexName, 10, 10, 2)
	service.SetElasticClient(s.ec)

	_, err := service.FindAllConceptsByType("http://www.ft.com/ontology/Foo")

	assert.EqualError(s.T(), err, fmt.Sprintf(errInvalidConceptTypeFormat, "http://www.ft.com/ontology/Foo"), "expected error")
}

func (s *EsConceptSearchServiceTestSuite) TestSearchConceptByTextAndTypes() {
	service := NewEsConceptSearchService(testIndexName, 10, 10, 2)
	service.SetElasticClient(s.ec)

	concepts, err := service.SearchConceptByTextAndTypes("test", []string{ftPeopleType})
	assert.NoError(s.T(), err)
	assert.Len(s.T(), concepts, 8)

	for _, concept := range concepts {
		assert.Equal(s.T(), ftPeopleType, concept.ConceptType, "expect the results to only contain people")
	}
}

func (s *EsConceptSearchServiceTestSuite) TestSearchConceptByTextAndTypesMultipleTypes() {
	service := NewEsConceptSearchService(testIndexName, 10, 10, 2)
	service.SetElasticClient(s.ec)

	concepts, err := service.SearchConceptByTextAndTypes("test", []string{ftBrandType, ftGenreType})
	assert.NoError(s.T(), err)
	assert.Len(s.T(), concepts, 8)

	for _, concept := range concepts {
		assert.True(s.T(), concept.ConceptType == ftBrandType || concept.ConceptType == ftGenreType, "expect concept to be either brand or genre")
	}
}

func (s *EsConceptSearchServiceTestSuite) TestSearchConceptByTextAndTypesNoText() {
	service := NewEsConceptSearchService(testIndexName, 10, 10, 2)
	service.SetElasticClient(s.ec)

	_, err := service.SearchConceptByTextAndTypes("", []string{ftPeopleType})
	assert.EqualError(s.T(), err, errEmptyTextParameter.Error())
}

func (s *EsConceptSearchServiceTestSuite) TestFindConceptsByIdsSingle() {
	uuid1 := uuid.NewV4().String()
	err := writeTestConcept(s.ec, uuid1, esPeopleType, ftPeopleType, "Eric Phillips inc")
	require.NoError(s.T(), err)
	_, err = s.ec.Refresh(testIndexName).Do(context.Background())
	require.NoError(s.T(), err)

	service := NewEsConceptSearchService(testIndexName, 10, 10, 2)
	service.SetElasticClient(s.ec)

	concepts, err := service.FindConceptsById([]string{uuid1})

	assert.NoError(s.T(), err, "expected no error for ES read")
	assert.Len(s.T(), concepts, 1, "there should be one concept")
	assert.Equal(s.T(), uuid1, concepts[0].Id, "retrieved concepts should have id %s ", uuid1)
}

func (s *EsConceptSearchServiceTestSuite) TestFindConceptsByIdsMultiple() {
	uuid1 := uuid.NewV4().String()
	err := writeTestConcept(s.ec, uuid1, esOrganisationType, ftOrganisationType, "Matilda Phillips")
	require.NoError(s.T(), err)

	uuid2 := uuid.NewV4().String()
	err = writeTestConcept(s.ec, uuid2, esLocationType, ftLocationType, "little pond")
	require.NoError(s.T(), err)

	_, err = s.ec.Refresh(testIndexName).Do(context.Background())
	require.NoError(s.T(), err)

	service := NewEsConceptSearchService(testIndexName, 10, 10, 2)
	service.SetElasticClient(s.ec)

	testIds := []string{uuid1, uuid2}

	concepts, err := service.FindConceptsById(testIds)

	assert.NoError(s.T(), err, "expected no error for ES read")
	assert.Len(s.T(), concepts, 2, "there should be two concepts")
	conceptIds := []string{}
	for _, concept := range concepts {
		conceptIds = append(conceptIds, concept.Id)
	}
	assert.Contains(s.T(), conceptIds, uuid1, "retrieved concepts should contain id %s ", uuid1)
	assert.Contains(s.T(), conceptIds, uuid2, "retrieved concepts should contain id %s ", uuid2)
}

func (s *EsConceptSearchServiceTestSuite) TestFindConceptsByIdsSingleInvalidUUID() {

	service := NewEsConceptSearchService(testIndexName, 10, 10, 2)
	service.SetElasticClient(s.ec)

	concepts, err := service.FindConceptsById([]string{"uuid1"})

	assert.NoError(s.T(), err, "expected no error for ES read")
	assert.Len(s.T(), concepts, 0, "there should be no concepts")
}

func (s *EsConceptSearchServiceTestSuite) TestFindConceptsByIdsMultipleMixValidInvalid() {
	uuid1 := uuid.NewV4().String()
	err := writeTestConcept(s.ec, uuid1, esOrganisationType, ftOrganisationType, "Betty Phillips")
	require.NoError(s.T(), err)

	uuid2 := uuid.NewV4().String()
	err = writeTestConcept(s.ec, uuid2, esLocationType, ftLocationType, "big pond")
	require.NoError(s.T(), err)

	_, err = s.ec.Refresh(testIndexName).Do(context.Background())
	require.NoError(s.T(), err)

	service := NewEsConceptSearchService(testIndexName, 10, 10, 2)
	service.SetElasticClient(s.ec)

	testIds := []string{uuid1, "xxx", uuid2, "zzzz"}

	concepts, err := service.FindConceptsById(testIds)

	assert.NoError(s.T(), err, "expected no error for ES read")
	assert.Len(s.T(), concepts, 2, "there should be two concepts")
	conceptIds := []string{}
	for _, concept := range concepts {
		conceptIds = append(conceptIds, concept.Id)
	}
	assert.Contains(s.T(), conceptIds, uuid1, "retrieved concepts should contain id %s ", uuid1)
	assert.Contains(s.T(), conceptIds, uuid2, "retrieved concepts should contain id %s ", uuid2)
}

func (s *EsConceptSearchServiceTestSuite) TestFindConceptsByIdsEmptyStringValue() {
	service := NewEsConceptSearchService(testIndexName, 10, 10, 2)
	service.SetElasticClient(s.ec)

	_, err := service.FindConceptsById([]string{""})
	assert.EqualError(s.T(), err, errEmptyIdsParameter.Error())
}

func (s *EsConceptSearchServiceTestSuite) TestFindConceptsByIdsEmptySlice() {
	service := NewEsConceptSearchService(testIndexName, 10, 10, 2)
	service.SetElasticClient(s.ec)

	_, err := service.FindConceptsById([]string{})
	assert.EqualError(s.T(), err, errEmptyIdsParameter.Error())
}

func (s *EsConceptSearchServiceTestSuite) TestFindConceptsByIdsNilSlice() {
	service := NewEsConceptSearchService(testIndexName, 10, 10, 2)
	service.SetElasticClient(s.ec)

	_, err := service.FindConceptsById(nil)
	assert.EqualError(s.T(), err, errEmptyIdsParameter.Error())
}

func (s *EsConceptSearchServiceTestSuite) TestSearchConceptByTextAndTypesNoConceptTypes() {
	service := NewEsConceptSearchService(testIndexName, 10, 10, 2)
	service.SetElasticClient(s.ec)

	_, err := service.SearchConceptByTextAndTypes("pippo", []string{})
	assert.EqualError(s.T(), err, errNoConceptTypeParameter.Error())
}

func (s *EsConceptSearchServiceTestSuite) TestSearchConceptByTextAndTypesInvalidConceptType() {
	service := NewEsConceptSearchService(testIndexName, 10, 10, 2)
	service.SetElasticClient(s.ec)

	_, err := service.SearchConceptByTextAndTypes("pippo", []string{"http://www.ft.com/ontology/Foo"})
	assert.EqualError(s.T(), err, fmt.Sprintf(errInvalidConceptTypeFormat, "http://www.ft.com/ontology/Foo"))
}

func (s *EsConceptSearchServiceTestSuite) TestSearchConceptByTextAndTypesExactMatchBoosted() {
	service := NewEsConceptSearchService(testIndexName, 10, 10, 2)
	service.SetElasticClient(s.ec)

	uuid1 := uuid.NewV4().String()
	err := writeTestConcept(s.ec, uuid1, esPeopleType, ftPeopleType, "Donaldo Trump")
	require.NoError(s.T(), err)

	uuid2 := uuid.NewV4().String()
	err = writeTestConcept(s.ec, uuid2, esPeopleType, ftPeopleType, "Donald J Trump")
	require.NoError(s.T(), err)

	_, err = s.ec.Refresh(testIndexName).Do(context.Background())
	require.NoError(s.T(), err)

	concepts, err := service.SearchConceptByTextAndTypes("donald trump", []string{ftPeopleType})
	assert.NoError(s.T(), err)
	assert.Len(s.T(), concepts, 2)

	elPresidente := concepts[0]
	donaldo := concepts[1]

	assert.Equal(s.T(), elPresidente.PrefLabel, "Donald J Trump")
	assert.Equal(s.T(), donaldo.PrefLabel, "Donaldo Trump")
}
