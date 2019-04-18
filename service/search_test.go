package service

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/Financial-Times/concept-search-api/util"
	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gopkg.in/olivere/elastic.v5"
)

const (
	apiBaseURL             = "http://test.api.ft.com"
	testDefaultIndex       = "test-index"
	testExtendedIndex      = "test-extended-index"
	esGenreType            = "genres"
	esBrandType            = "brands"
	esPeopleType           = "people"
	esOrganisationType     = "organisations"
	esLocationType         = "locations"
	esTopicType            = "topics"
	esAlphavilleSeriesType = "alphaville-series"
	ftGenreType            = "http://www.ft.com/ontology/Genre"
	ftBrandType            = "http://www.ft.com/ontology/product/Brand"
	ftPeopleType           = "http://www.ft.com/ontology/person/Person"
	ftOrganisationType     = "http://www.ft.com/ontology/organisation/Organisation"
	ftLocationType         = "http://www.ft.com/ontology/Location"
	ftTopicType            = "http://www.ft.com/ontology/Topic"
	ftAlphavilleSeriesType = "http://www.ft.com/ontology/AlphavilleSeries"
	ftPublicCompanies      = "http://www.ft.com/ontology/company/PublicCompany"
	testMappingFile        = "test/mapping.json"
)

func TestNoElasticClient(t *testing.T) {
	service := NewEsConceptSearchService("test", "", 50, 10, 2)

	_, err := service.FindAllConceptsByType(ftGenreType, false, true)
	assert.EqualError(t, err, util.ErrNoElasticClient.Error(), "error response")

	_, err = service.SearchConceptByTextAndTypes("lucy", []string{ftBrandType}, false, true)
	assert.EqualError(t, err, util.ErrNoElasticClient.Error(), "error response")
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

	err = createIndex(s.ec, testDefaultIndex, testMappingFile)
	require.NoError(s.T(), err, "expected no error in creating index")

	err = createIndex(s.ec, testExtendedIndex, testMappingFile)
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
	err = writeTestConcepts(s.ec, esAlphavilleSeriesType, ftAlphavilleSeriesType, 1)
	require.NoError(s.T(), err, "expected no error in adding topics")
	err = writeTestConcepts(s.ec, esOrganisationType, ftPublicCompanies, 4)
	require.NoError(s.T(), err, "expected no error in adding public companies")
}

func (s *EsConceptSearchServiceTestSuite) TearDownSuite() {
	s.ec.DeleteIndex(testDefaultIndex).Do(context.Background())
	s.ec.DeleteIndex(testExtendedIndex).Do(context.Background())
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

func createIndex(ec *elastic.Client, indexName string, mappingFile string) error {
	mapping, err := ioutil.ReadFile(mappingFile)
	if err != nil {
		return err
	}
	_, err = ec.CreateIndex(indexName).Body(string(mapping)).Do(context.Background())
	if err != nil {
		return err
	}
	return nil
}

func cleanup(t *testing.T, ec *elastic.Client, esType string, uuids ...string) {
	for _, uuid := range uuids {
		_, err := ec.Delete().
			Index(testDefaultIndex).
			Type(esType).
			Id(uuid).
			Do(context.TODO())
		assert.NoError(t, err)
	}
	_, err := ec.Refresh(testDefaultIndex).Do(context.TODO())
	assert.NoError(t, err)
}

func writeTestAuthors(ec *elastic.Client, amount int) error {
	for i := 0; i < amount; i++ {
		uuid := uuid.NewV4().String()

		ftAuthor := "true"
		prefLabel := fmt.Sprintf("Test concept %s %s", esPeopleType, uuid)
		payload := EsConceptModel{
			Id:         uuid,
			ApiUrl:     fmt.Sprintf("%s/%s/%s", apiBaseURL, esPeopleType, uuid),
			PrefLabel:  prefLabel,
			Types:      []string{ftPeopleType},
			DirectType: ftPeopleType,
			Aliases:    []string{prefLabel},
			IsFTAuthor: &ftAuthor,
		}

		_, err := ec.Index().
			Index(testDefaultIndex).
			Type(esPeopleType).
			Id(uuid).
			BodyJson(payload).
			Do(context.Background())
		if err != nil {
			return err
		}
	}

	// ensure test data is immediately available from the index
	_, err := ec.Refresh(testDefaultIndex).Do(context.Background())
	if err != nil {
		return err
	}
	return nil
}

func writeTestConcepts(ec *elastic.Client, esConceptType string, ftConceptType string, amount int) error {
	for i := 0; i < amount; i++ {
		uuid := uuid.NewV4().String()
		prefLabel := fmt.Sprintf("Test concept %s %s", esConceptType, uuid)
		err := writeTestConcept(ec, uuid, esConceptType, ftConceptType, prefLabel, []string{prefLabel}, nil)
		if err != nil {
			return err
		}
	}

	// ensure test data is immediately available from the index
	_, err := ec.Refresh(testDefaultIndex).Do(context.Background())
	if err != nil {
		return err
	}
	return nil
}

func writeTestPerson(ec *elastic.Client, uuid string, prefLabel string, ftAuthor string) error {
	payload := EsConceptModel{
		Id:         uuid,
		ApiUrl:     fmt.Sprintf("%s/%s/%s", apiBaseURL, esPeopleType, uuid),
		PrefLabel:  fmt.Sprintf(prefLabel),
		Types:      []string{ftPeopleType},
		DirectType: ftPeopleType,
		Aliases:    []string{prefLabel},
		IsFTAuthor: &ftAuthor,
	}

	_, err := ec.Index().
		Index(testDefaultIndex).
		Type(esPeopleType).
		Id(uuid).
		BodyJson(payload).
		Do(context.Background())
	if err != nil {
		return err
	}
	return nil
}

func writeTestConcept(ec *elastic.Client, uuid string, esConceptType string, ftConceptType string, prefLabel string, aliases []string, metrics *ConceptMetrics) error {
	payload := EsConceptModel{
		Id:         uuid,
		ApiUrl:     fmt.Sprintf("%s/%s/%s", apiBaseURL, esConceptType, uuid),
		PrefLabel:  prefLabel,
		Types:      []string{ftConceptType},
		DirectType: ftConceptType,
		Aliases:    aliases,
		Metrics:    metrics,
	}

	_, err := ec.Index().
		Index(testDefaultIndex).
		Type(esConceptType).
		Id(uuid).
		BodyJson(payload).
		Do(context.Background())

	if err != nil {
		return err
	}
	return nil
}

func writeTestConceptWithScopeNote(ec *elastic.Client, uuid string, esConceptType string,
	ftConceptType string, prefLabel string, aliases []string, scopeNote string) error {

	payload := EsConceptModel{
		Id:         uuid,
		ApiUrl:     fmt.Sprintf("%s/%s/%s", apiBaseURL, esConceptType, uuid),
		PrefLabel:  prefLabel,
		Types:      []string{ftConceptType},
		DirectType: ftConceptType,
		Aliases:    aliases,
		ScopeNote:  scopeNote,
	}

	_, err := ec.Index().
		Index(testDefaultIndex).
		Type(esConceptType).
		Id(uuid).
		BodyJson(payload).
		Do(context.Background())

	if err != nil {
		return err
	}
	return nil
}

func writeTestConceptWithCountryCodeAndCountryOfIncorporation(ec *elastic.Client, uuid string, esConceptType string,
	ftConceptType string, prefLabel string, aliases []string, countryCode string, countryOfIncorporation string) error {

	payload := EsConceptModel{
		Id:                     uuid,
		ApiUrl:                 fmt.Sprintf("%s/%s/%s", apiBaseURL, esConceptType, uuid),
		PrefLabel:              prefLabel,
		Types:                  []string{ftConceptType},
		DirectType:             ftConceptType,
		Aliases:                aliases,
		CountryCode:            countryCode,
		CountryOfIncorporation: countryOfIncorporation,
	}

	_, err := ec.Index().
		Index(testDefaultIndex).
		Type(esConceptType).
		Id(uuid).
		BodyJson(payload).
		Do(context.Background())

	if err != nil {
		return err
	}
	return nil
}

func writeTestConceptModel(ec *elastic.Client, esConceptType string, model EsConceptModel) error {
	_, err := ec.Index().
		Index(testDefaultIndex).
		Type(esConceptType).
		Id(model.Id).
		BodyJson(model).
		Do(context.Background())

	if err != nil {
		return err
	}
	return nil
}

func (s *EsConceptSearchServiceTestSuite) TestFindAllConceptsByType() {
	service := NewEsConceptSearchService(testDefaultIndex, "", 10, 10, 2)
	service.SetElasticClient(s.ec)

	concepts, err := service.FindAllConceptsByType(ftGenreType, false, true)

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
	service := NewEsConceptSearchService(testDefaultIndex, "", 3, 10, 2)
	service.SetElasticClient(s.ec)
	concepts, err := service.FindAllConceptsByType(ftGenreType, false, true)

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
	service := NewEsConceptSearchService(testDefaultIndex, "", 10, 10, 2)
	service.SetElasticClient(s.ec)

	_, err := service.FindAllConceptsByType("http://www.ft.com/ontology/Foo", false, true)

	assert.EqualError(s.T(), err, fmt.Sprintf(util.ErrInvalidConceptTypeFormat, "http://www.ft.com/ontology/Foo"), "expected error")
}

func (s *EsConceptSearchServiceTestSuite) TestFindAllConceptsByTypeDeprecatedFlag() {
	service := NewEsConceptSearchService(testDefaultIndex, "", 10, 10, 2)
	service.SetElasticClient(s.ec)

	uuid := uuid.NewV4().String()
	prefLabel := "Rick and Morty"

	err := writeTestConceptModel(s.ec, esPeopleType, EsConceptModel{
		Id:           uuid,
		ApiUrl:       fmt.Sprintf("%s/%s/%s", apiBaseURL, esPeopleType, uuid),
		PrefLabel:    prefLabel,
		Types:        []string{ftPeopleType},
		DirectType:   ftPeopleType,
		Aliases:      []string{},
		IsDeprecated: true,
	})
	assert.NoError(s.T(), err, "no error expected during indexing a new person concept")
	_, err = s.ec.Refresh(testDefaultIndex).Do(context.Background())
	require.NoError(s.T(), err)

	conceptsWithoutDeprecated, err := service.FindAllConceptsByType("http://www.ft.com/ontology/person/Person", false, false)
	assert.NoError(s.T(), err, "no error expected")

	for _, concept := range conceptsWithoutDeprecated {
		assert.NotEqual(s.T(), prefLabel, concept.PrefLabel)
		assert.False(s.T(), concept.IsDeprecated)
	}

	conceptsWithDeprecated, err := service.FindAllConceptsByType("http://www.ft.com/ontology/person/Person", false, true)
	assert.NoError(s.T(), err, "no error expected")

	deprecatedConceptsFound := 0
	for _, concept := range conceptsWithDeprecated {
		if prefLabel == concept.PrefLabel {
			deprecatedConceptsFound++
			assert.True(s.T(), concept.IsDeprecated)
		} else {
			assert.False(s.T(), concept.IsDeprecated)
		}
	}
	assert.Equal(s.T(), 1, deprecatedConceptsFound, "expect found concepts")

	cleanup(s.T(), s.ec, esPeopleType, uuid)
}

func (s *EsConceptSearchServiceTestSuite) TestFindAllConceptsByDirectType() {
	service := NewEsConceptSearchService(testDefaultIndex, "", 10, 10, 2)
	service.SetElasticClient(s.ec)

	concepts, err := service.FindAllConceptsByDirectType(ftPublicCompanies, false, false)

	assert.NoError(s.T(), err, "expected no error for ES read")
	assert.Len(s.T(), concepts, 4, "there should be four public companies")

	var prev string
	for i := range concepts {
		if i > 0 {
			assert.Equal(s.T(), -1, strings.Compare(prev, concepts[i].PrefLabel), "concepts should be ordered")
		}
		assert.Equal(s.T(), ftPublicCompanies, concepts[i].ConceptType, "Results should be of type PublicCompany")
		prev = concepts[i].PrefLabel
	}
}

func (s *EsConceptSearchServiceTestSuite) TestSearchConceptByTextAndTypes() {
	service := NewEsConceptSearchService(testDefaultIndex, "", 10, 10, 2)
	service.SetElasticClient(s.ec)

	concepts, err := service.SearchConceptByTextAndTypes("test", []string{ftPeopleType}, false, true)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), concepts, 8)

	for _, concept := range concepts {
		assert.Equal(s.T(), ftPeopleType, concept.ConceptType, "expect the results to only contain people")
	}
}

func (s *EsConceptSearchServiceTestSuite) TestSearchConceptByTextAndTypesMultipleTypes() {
	service := NewEsConceptSearchService(testDefaultIndex, "", 10, 10, 2)
	service.SetElasticClient(s.ec)

	concepts, err := service.SearchConceptByTextAndTypes("test", []string{ftBrandType, ftAlphavilleSeriesType}, false, true)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), concepts, 5)

	for _, concept := range concepts {
		assert.True(s.T(), concept.ConceptType == ftBrandType || concept.ConceptType == ftAlphavilleSeriesType, "expect concept to be either brand or alphaville-series")
	}
}

func (s *EsConceptSearchServiceTestSuite) TestSearchConceptByTextAndTypesPublicCompanies() {
	service := NewEsConceptSearchService(testDefaultIndex, "", 10, 10, 2)
	service.SetElasticClient(s.ec)

	concepts, err := service.SearchConceptByTextAndTypes("test", []string{ftPublicCompanies}, false, true)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), concepts, 4)

	for _, concept := range concepts {
		assert.True(s.T(), concept.ConceptType == ftPublicCompanies, "expect concept to have type PublicCompany")
	}
}

func (s *EsConceptSearchServiceTestSuite) TestSearchConceptByTextAndTypesMultipleTypesWithPublicCompanies() {
	service := NewEsConceptSearchService(testDefaultIndex, "", 10, 10, 2)
	service.SetElasticClient(s.ec)

	concepts, err := service.SearchConceptByTextAndTypes("test", []string{ftBrandType, ftPublicCompanies}, false, true)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), concepts, 8)

	for _, concept := range concepts {
		assert.True(s.T(), concept.ConceptType == ftBrandType || concept.ConceptType == ftPublicCompanies, "expect concept to be either brand or public company")
	}
}

func (s *EsConceptSearchServiceTestSuite) TestSearchConceptByTextAndTypesNoText() {
	service := NewEsConceptSearchService(testDefaultIndex, "", 10, 10, 2)
	service.SetElasticClient(s.ec)

	_, err := service.SearchConceptByTextAndTypes("", []string{ftPeopleType}, false, true)
	assert.EqualError(s.T(), err, errEmptyTextParameter.Error())
}

func (s *EsConceptSearchServiceTestSuite) TestFindConceptsByIdsSingle() {
	uuid1 := uuid.NewV4().String()
	err := writeTestConcept(s.ec, uuid1, esPeopleType, ftPeopleType, "Eric Phillips inc", []string{}, nil)
	require.NoError(s.T(), err)
	_, err = s.ec.Refresh(testDefaultIndex).Do(context.Background())
	require.NoError(s.T(), err)

	service := NewEsConceptSearchService(testDefaultIndex, "", 10, 10, 2)
	service.SetElasticClient(s.ec)

	concepts, err := service.FindConceptsById([]string{uuid1})

	assert.NoError(s.T(), err, "expected no error for ES read")
	assert.Len(s.T(), concepts, 1, "there should be one concept")
	assert.Equal(s.T(), uuid1, concepts[0].Id, "retrieved concepts should have id %s ", uuid1)

	cleanup(s.T(), s.ec, esPeopleType, uuid1)
}

func (s *EsConceptSearchServiceTestSuite) TestFindConceptsByIdsMultiple() {
	uuid1 := uuid.NewV4().String()
	err := writeTestConcept(s.ec, uuid1, esOrganisationType, ftOrganisationType, "Matilda Phillips", []string{}, nil)
	require.NoError(s.T(), err)

	uuid2 := uuid.NewV4().String()
	err = writeTestConcept(s.ec, uuid2, esLocationType, ftLocationType, "little pond", []string{}, nil)
	require.NoError(s.T(), err)

	_, err = s.ec.Refresh(testDefaultIndex).Do(context.Background())
	require.NoError(s.T(), err)

	service := NewEsConceptSearchService(testDefaultIndex, "", 10, 10, 2)
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
	cleanup(s.T(), s.ec, esOrganisationType, uuid1)
	cleanup(s.T(), s.ec, esLocationType, uuid2)
}

func (s *EsConceptSearchServiceTestSuite) TestFindConceptsByIdsSingleInvalidUUID() {
	service := NewEsConceptSearchService(testDefaultIndex, "", 10, 10, 2)
	service.SetElasticClient(s.ec)

	concepts, err := service.FindConceptsById([]string{"uuid1"})

	assert.NoError(s.T(), err, "expected no error for ES read")
	assert.Len(s.T(), concepts, 0, "there should be no concepts")
}

func (s *EsConceptSearchServiceTestSuite) TestFindConceptsByIdsMultipleMixValidInvalid() {
	uuid1 := uuid.NewV4().String()
	err := writeTestConcept(s.ec, uuid1, esOrganisationType, ftOrganisationType, "Betty Phillips", []string{}, nil)
	require.NoError(s.T(), err)

	uuid2 := uuid.NewV4().String()
	err = writeTestConcept(s.ec, uuid2, esLocationType, ftLocationType, "big pond", []string{}, nil)
	require.NoError(s.T(), err)

	_, err = s.ec.Refresh(testDefaultIndex).Do(context.Background())
	require.NoError(s.T(), err)

	service := NewEsConceptSearchService(testDefaultIndex, "", 10, 10, 2)
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

	cleanup(s.T(), s.ec, esOrganisationType, uuid1)
	cleanup(s.T(), s.ec, esLocationType, uuid2)
}

func (s *EsConceptSearchServiceTestSuite) TestFindConceptsByIdsEmptyStringValue() {
	service := NewEsConceptSearchService(testDefaultIndex, "", 10, 10, 2)
	service.SetElasticClient(s.ec)

	_, err := service.FindConceptsById([]string{""})
	assert.EqualError(s.T(), err, errEmptyIdsParameter.Error())
}

func (s *EsConceptSearchServiceTestSuite) TestFindConceptsByIdsEmptySlice() {
	service := NewEsConceptSearchService(testDefaultIndex, "", 10, 10, 2)
	service.SetElasticClient(s.ec)

	_, err := service.FindConceptsById([]string{})
	assert.EqualError(s.T(), err, errEmptyIdsParameter.Error())
}

func (s *EsConceptSearchServiceTestSuite) TestFindConceptsByIdsNilSlice() {
	service := NewEsConceptSearchService(testDefaultIndex, "", 10, 10, 2)
	service.SetElasticClient(s.ec)

	_, err := service.FindConceptsById(nil)
	assert.EqualError(s.T(), err, errEmptyIdsParameter.Error())
}

func (s *EsConceptSearchServiceTestSuite) TestSearchConceptByTextAndTypesNoConceptTypes() {
	service := NewEsConceptSearchService(testDefaultIndex, "", 10, 10, 2)
	service.SetElasticClient(s.ec)

	_, err := service.SearchConceptByTextAndTypes("pippo", []string{}, false, true)
	assert.EqualError(s.T(), err, util.ErrNoConceptTypeParameter.Error())
}

func (s *EsConceptSearchServiceTestSuite) TestSearchConceptByTextAndTypesInvalidConceptType() {
	service := NewEsConceptSearchService(testDefaultIndex, "", 10, 10, 2)
	service.SetElasticClient(s.ec)

	_, err := service.SearchConceptByTextAndTypes("pippo", []string{"http://www.ft.com/ontology/Foo"}, false, true)
	assert.EqualError(s.T(), err, fmt.Sprintf(util.ErrInvalidConceptTypeFormat, "http://www.ft.com/ontology/Foo"))
}

func (s *EsConceptSearchServiceTestSuite) TestSearchConceptByTextAndTypesTermMatchBoosted() {
	service := NewEsConceptSearchService(testDefaultIndex, "", 10, 10, 2)
	service.SetElasticClient(s.ec)

	uuid1 := uuid.NewV4().String()
	err := writeTestConcept(s.ec, uuid1, esPeopleType, ftPeopleType, "Donaldo Trump", []string{}, nil)
	require.NoError(s.T(), err)

	uuid2 := uuid.NewV4().String()
	err = writeTestConcept(s.ec, uuid2, esPeopleType, ftPeopleType, "Donald J Trump", []string{}, nil)
	require.NoError(s.T(), err)

	_, err = s.ec.Refresh(testDefaultIndex).Do(context.Background())
	require.NoError(s.T(), err)

	concepts, err := service.SearchConceptByTextAndTypes("donald trump", []string{ftPeopleType}, false, true)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), concepts, 2)

	elPresidente := concepts[0]
	donaldo := concepts[1]

	assert.Equal(s.T(), elPresidente.PrefLabel, "Donald J Trump", "Failure could indicate that the wrong concept had the higher boost")
	assert.Equal(s.T(), donaldo.PrefLabel, "Donaldo Trump", "Failure could indicate that the wrong concept had the higher boost")
	cleanup(s.T(), s.ec, esPeopleType, uuid1, uuid2)
}

func (s *EsConceptSearchServiceTestSuite) TestSearchConceptByTextAndTypesExactMatchBoosted() {
	service := NewEsConceptSearchService(testDefaultIndex, "", 10, 10, 2)
	service.SetElasticClient(s.ec)

	uuid1 := uuid.NewV4().String()
	err := writeTestConcept(s.ec, uuid1, esLocationType, ftLocationType, "New York", []string{}, nil)
	require.NoError(s.T(), err)

	uuid2 := uuid.NewV4().String()
	err = writeTestConcept(s.ec, uuid2, esLocationType, ftLocationType, "New York City Magistrates (New York, New York)", []string{}, nil)
	require.NoError(s.T(), err)

	_, err = s.ec.Refresh(testDefaultIndex).Do(context.Background())
	require.NoError(s.T(), err)

	concepts, err := service.SearchConceptByTextAndTypes("new york", []string{ftLocationType}, false, true)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), concepts, 2)

	nyc := concepts[0]
	magistrates := concepts[1]

	assert.Equal(s.T(), "New York", nyc.PrefLabel, "Failure could indicate that the wrong concept had the higher boost")
	assert.Equal(s.T(), "New York City Magistrates (New York, New York)", magistrates.PrefLabel, "Failure could indicate that the wrong concept had the higher boost")
	cleanup(s.T(), s.ec, esLocationType, uuid1, uuid2)
}

func (s *EsConceptSearchServiceTestSuite) TestSearchConceptByTextAndTypesExactMatchBoostedWithScopeNotePresent() {
	service := NewEsConceptSearchService(testDefaultIndex, "", 10, 10, 2)
	service.SetElasticClient(s.ec)

	uuid1 := uuid.NewV4().String()
	err := writeTestConcept(s.ec, uuid1, esLocationType, ftLocationType, "New York", []string{}, nil)
	require.NoError(s.T(), err)

	uuid2 := uuid.NewV4().String()
	err = writeTestConceptWithScopeNote(s.ec, uuid2, esLocationType, ftLocationType, "New York City Magistrates (New York, New York)", []string{}, "New York City Magistrates scopeNote")
	require.NoError(s.T(), err)

	_, err = s.ec.Refresh(testDefaultIndex).Do(context.Background())
	require.NoError(s.T(), err)

	concepts, err := service.SearchConceptByTextAndTypes("new yor", []string{ftLocationType}, false, true)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), concepts, 2)

	magistrates := concepts[0]
	nyc := concepts[1]

	assert.Equal(s.T(), "New York", nyc.PrefLabel, "Failure could indicate that the wrong concept had the higher boost")
	assert.Equal(s.T(), "New York City Magistrates (New York, New York)", magistrates.PrefLabel, "Failure could indicate that the wrong concept had the higher boost")
	cleanup(s.T(), s.ec, esLocationType, uuid1, uuid2)
}

func (s *EsConceptSearchServiceTestSuite) TestSearchConceptByTextAndTypesDeprecated() {
	service := NewEsConceptSearchService(testDefaultIndex, "", 10, 10, 2)
	service.SetElasticClient(s.ec)

	uuid1 := uuid.NewV4().String()
	err := writeTestConcept(s.ec, uuid1, esLocationType, ftLocationType, "New York", []string{}, nil)
	require.NoError(s.T(), err)

	uuid2 := uuid.NewV4().String()
	err = writeTestConceptModel(s.ec, esLocationType, EsConceptModel{
		Id:           uuid2,
		ApiUrl:       fmt.Sprintf("%s/%s/%s", apiBaseURL, ftLocationType, uuid2),
		PrefLabel:    "New York Deprecated",
		Types:        []string{ftLocationType},
		DirectType:   ftLocationType,
		Aliases:      []string{},
		IsDeprecated: true,
	})
	require.NoError(s.T(), err)

	_, err = s.ec.Refresh(testDefaultIndex).Do(context.Background())
	require.NoError(s.T(), err)

	concepts, err := service.SearchConceptByTextAndTypes("new york", []string{ftLocationType}, false, true)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), concepts, 2)

	nyc := concepts[0]
	nycDeprecated := concepts[1]

	assert.Equal(s.T(), "New York", nyc.PrefLabel, "Failure could indicate that the wrong concept had the higher boost")
	assert.Equal(s.T(), "New York Deprecated", nycDeprecated.PrefLabel, "Failure could indicate that the wrong concept had the higher boost")

	concepts, err = service.SearchConceptByTextAndTypes("new york", []string{ftLocationType}, false, false)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), concepts, 1)

	nyc = concepts[0]

	assert.Equal(s.T(), "New York", nyc.PrefLabel, "Failure could indicate that the wrong concept had the higher boost")

	cleanup(s.T(), s.ec, esLocationType, uuid1, uuid2)
}

func (s *EsConceptSearchServiceTestSuite) TestSearchConceptByTextAndTypesWithAuthorsBoost() {
	service := NewEsConceptSearchService(testDefaultIndex, "", 10, 10, 2)
	service.SetElasticClient(s.ec)

	uuid1 := uuid.NewV4().String()
	err := writeTestConcept(s.ec, uuid1, esPeopleType, ftPeopleType, "Roberto Shrimpley", []string{}, nil)
	require.NoError(s.T(), err)

	uuid2 := uuid.NewV4().String()
	err = writeTestConcept(s.ec, uuid2, esPeopleType, ftPeopleType, "Robert Real Shrimpley", []string{}, nil)
	require.NoError(s.T(), err)

	uuid3 := uuid.NewV4().String()
	err = writeTestPerson(s.ec, uuid3, "Robert Author Shrimpley", "true")
	require.NoError(s.T(), err)

	_, err = s.ec.Refresh(testDefaultIndex).Do(context.Background())
	require.NoError(s.T(), err)

	concepts, err := service.SearchConceptByTextAndTypesWithBoost("robert shrimpley", []string{ftPeopleType}, "authors", false, true)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), concepts, 3)

	theAuthor := concepts[0]
	theEditor := concepts[1]
	theFraud := concepts[2]

	assert.Equal(s.T(), "Robert Author Shrimpley", theAuthor.PrefLabel)
	assert.Equal(s.T(), "Robert Real Shrimpley", theEditor.PrefLabel)
	assert.Equal(s.T(), "Roberto Shrimpley", theFraud.PrefLabel)
	cleanup(s.T(), s.ec, esPeopleType, uuid1, uuid2, uuid3)
}

// If 4 concepts are equivalent, then the type boosts should order them as expected.
func (s *EsConceptSearchServiceTestSuite) TestSearch__SpecificTypesAreBoosted() {
	service := NewEsConceptSearchService(testDefaultIndex, "", 10, 10, 2)
	service.SetElasticClient(s.ec)

	uuid1 := uuid.NewV4().String()
	err := writeTestConcept(s.ec, uuid1, esPeopleType, ftPeopleType, "Fannie Mae", []string{}, nil)
	require.NoError(s.T(), err)

	uuid2 := uuid.NewV4().String()
	err = writeTestConcept(s.ec, uuid2, esLocationType, ftLocationType, "Fannie Mae", []string{}, nil)
	require.NoError(s.T(), err)

	uuid3 := uuid.NewV4().String()
	err = writeTestConcept(s.ec, uuid3, esOrganisationType, ftOrganisationType, "Fannie Mae", []string{}, nil)
	require.NoError(s.T(), err)

	uuid4 := uuid.NewV4().String()
	err = writeTestConcept(s.ec, uuid4, esTopicType, ftTopicType, "Fannie Mae", []string{}, nil)
	require.NoError(s.T(), err)

	_, err = s.ec.Refresh(testDefaultIndex).Do(context.Background())
	require.NoError(s.T(), err)

	concepts, err := service.SearchConceptByTextAndTypes("Fannie Mae", []string{ftPeopleType, ftTopicType, ftLocationType, ftOrganisationType}, false, true)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), concepts, 4)

	topic := concepts[0]
	location := concepts[1]
	person := concepts[2]
	org := concepts[3]

	assert.Equal(s.T(), ftTopicType, topic.ConceptType)
	assert.Equal(s.T(), ftLocationType, location.ConceptType)
	assert.Equal(s.T(), ftPeopleType, person.ConceptType)
	assert.Equal(s.T(), ftOrganisationType, org.ConceptType)

	cleanup(s.T(), s.ec, esPeopleType, uuid1)
	cleanup(s.T(), s.ec, esLocationType, uuid2)
	cleanup(s.T(), s.ec, esOrganisationType, uuid3)
	cleanup(s.T(), s.ec, esTopicType, uuid4)
}

func (s *EsConceptSearchServiceTestSuite) TestSearchConceptByTextAndTypesWithAuthorsBoostAndDeprecated() {
	service := NewEsConceptSearchService(testDefaultIndex, "", 10, 10, 2)
	service.SetElasticClient(s.ec)

	uuid1 := uuid.NewV4().String()
	err := writeTestConcept(s.ec, uuid1, esPeopleType, ftPeopleType, "Roberto Shrimpley", []string{}, nil)
	require.NoError(s.T(), err)

	uuid2 := uuid.NewV4().String()
	err = writeTestConcept(s.ec, uuid2, esPeopleType, ftPeopleType, "Robert Real Shrimpley", []string{}, nil)
	require.NoError(s.T(), err)

	uuid3 := uuid.NewV4().String()
	err = writeTestPerson(s.ec, uuid3, "Robert Author Shrimpley", "true")
	require.NoError(s.T(), err)

	uuid4 := uuid.NewV4().String()
	authorFlag := "true"
	err = writeTestConceptModel(s.ec, esPeopleType, EsConceptModel{
		Id:           uuid4,
		ApiUrl:       fmt.Sprintf("%s/%s/%s", apiBaseURL, esPeopleType, uuid4),
		PrefLabel:    "Robert Shrimpley",
		Types:        []string{ftPeopleType},
		DirectType:   ftPeopleType,
		Aliases:      []string{},
		IsFTAuthor:   &authorFlag,
		IsDeprecated: true,
	})
	require.NoError(s.T(), err)

	_, err = s.ec.Refresh(testDefaultIndex).Do(context.Background())
	require.NoError(s.T(), err)

	conceptsWithDeprecated, err := service.SearchConceptByTextAndTypesWithBoost("robert shrimpley", []string{ftPeopleType}, "authors", false, true)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), conceptsWithDeprecated, 4)

	theAuthor := conceptsWithDeprecated[0]
	theDeprecatedAuthor := conceptsWithDeprecated[1]
	theRealEditor := conceptsWithDeprecated[2]
	theFake := conceptsWithDeprecated[3]

	assert.Equal(s.T(), "Robert Author Shrimpley", theAuthor.PrefLabel)
	assert.Equal(s.T(), "Robert Shrimpley", theDeprecatedAuthor.PrefLabel)
	assert.Equal(s.T(), "Robert Real Shrimpley", theRealEditor.PrefLabel)
	assert.Equal(s.T(), "Roberto Shrimpley", theFake.PrefLabel)

	conceptsWithoutDeprecated, err := service.SearchConceptByTextAndTypesWithBoost("robert shrimpley", []string{ftPeopleType}, "authors", false, false)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), conceptsWithoutDeprecated, 3)

	theAuthor = conceptsWithoutDeprecated[0]
	theRealEditor = conceptsWithoutDeprecated[1]
	theFake = conceptsWithoutDeprecated[2]

	assert.Equal(s.T(), "Robert Author Shrimpley", theAuthor.PrefLabel)
	assert.Equal(s.T(), "Robert Real Shrimpley", theRealEditor.PrefLabel)
	assert.Equal(s.T(), "Roberto Shrimpley", theFake.PrefLabel)

	cleanup(s.T(), s.ec, esPeopleType, uuid1, uuid2, uuid3, uuid4)
}

func (s *EsConceptSearchServiceTestSuite) TestSearchConceptsByExactMatchAliases() {
	service := NewEsConceptSearchService(testDefaultIndex, "", 10, 10, 2)
	service.SetElasticClient(s.ec)

	uuid1 := uuid.NewV4().String()
	err := writeTestConcept(s.ec, uuid1, esLocationType, ftLocationType, "United States of America", []string{"USA"}, nil)
	require.NoError(s.T(), err)

	uuid2 := uuid.NewV4().String()
	err = writeTestConcept(s.ec, uuid2, esLocationType, ftLocationType, "USADA", []string{}, nil)
	require.NoError(s.T(), err)

	_, err = s.ec.Refresh(testDefaultIndex).Do(context.Background())
	require.NoError(s.T(), err)

	concepts, err := service.SearchConceptByTextAndTypes("USA", []string{ftLocationType}, false, true)
	require.NoError(s.T(), err)
	require.Len(s.T(), concepts, 2)

	theCountry := concepts[0]
	theFraud := concepts[1]

	assert.Equal(s.T(), "United States of America", theCountry.PrefLabel)
	assert.Equal(s.T(), "USADA", theFraud.PrefLabel)
	cleanup(s.T(), s.ec, esLocationType, uuid1, uuid2)
}

func (s *EsConceptSearchServiceTestSuite) TestSearchConceptByTextAndTypesWithBoostRestrictedSize() {
	service := NewEsConceptSearchService(testDefaultIndex, "", 10, 1, 2)
	service.SetElasticClient(s.ec)

	concepts, err := service.SearchConceptByTextAndTypesWithBoost("test", []string{ftPeopleType}, "authors", false, true)
	assert.NoError(s.T(), err, "expected no error for ES read")
	assert.Len(s.T(), concepts, 1, "there should be one results")
}

func (s *EsConceptSearchServiceTestSuite) TestSearchConceptByTextAndTypesWithBoostNoInputText() {
	service := NewEsConceptSearchService(testDefaultIndex, "", 10, 10, 2)
	service.SetElasticClient(s.ec)

	concepts, err := service.SearchConceptByTextAndTypesWithBoost("", []string{ftPeopleType}, "authors", false, true)
	assert.EqualError(s.T(), err, errEmptyTextParameter.Error())
	assert.Nil(s.T(), concepts)
}

func (s *EsConceptSearchServiceTestSuite) TestSearchConceptByTextAndTypesWithBoostNoTypes() {
	service := NewEsConceptSearchService(testDefaultIndex, "", 10, 10, 2)
	service.SetElasticClient(s.ec)

	concepts, err := service.SearchConceptByTextAndTypesWithBoost("test", []string{}, "authors", false, true)
	assert.EqualError(s.T(), err, util.ErrNoConceptTypeParameter.Error())
	assert.Nil(s.T(), concepts)
}

func (s *EsConceptSearchServiceTestSuite) TestSearchConceptByTextAndTypesWithBoostMultipleTypes() {
	service := NewEsConceptSearchService(testDefaultIndex, "", 10, 10, 2)
	service.SetElasticClient(s.ec)

	concepts, err := service.SearchConceptByTextAndTypesWithBoost("test", []string{ftPeopleType, ftLocationType}, "authors", false, true)
	assert.EqualError(s.T(), err, util.ErrNotSupportedCombinationOfConceptTypes.Error())
	assert.Nil(s.T(), concepts)
}

func (s *EsConceptSearchServiceTestSuite) TestSearchConceptByTextAndTypesWithInvalidBoost() {
	service := NewEsConceptSearchService(testDefaultIndex, "", 10, 10, 2)
	service.SetElasticClient(s.ec)

	concepts, err := service.SearchConceptByTextAndTypesWithBoost("test", []string{ftPeopleType}, "pluto", false, true)
	assert.EqualError(s.T(), err, util.ErrInvalidBoostTypeParameter.Error())
	assert.Nil(s.T(), concepts)
}

func (s *EsConceptSearchServiceTestSuite) TestSearchConceptByTextAndTypesWithBoostNoESConnection() {
	service := NewEsConceptSearchService(testDefaultIndex, "", 10, 10, 2)

	concepts, err := service.SearchConceptByTextAndTypesWithBoost("test", []string{ftPeopleType}, "authors", false, true)
	assert.EqualError(s.T(), err, util.ErrNoElasticClient.Error())
	assert.Nil(s.T(), concepts)
}

func (s *EsConceptSearchServiceTestSuite) TestSearchConceptByTextAndTypesWithBoostInvalidConceptType() {
	service := NewEsConceptSearchService(testDefaultIndex, "", 10, 10, 2)

	concepts, err := service.SearchConceptByTextAndTypesWithBoost("test", []string{ftGenreType}, "authors", false, true)
	assert.EqualError(s.T(), err, fmt.Sprintf(util.ErrInvalidConceptTypeFormat, ftGenreType))
	assert.Nil(s.T(), concepts)
}

func (s *EsConceptSearchServiceTestSuite) TestSearchConceptsByPopularity() {
	service := NewEsConceptSearchService(testDefaultIndex, "", 10, 10, 2)
	service.SetElasticClient(s.ec)

	uuid1 := uuid.NewV4().String()
	err := writeTestConcept(s.ec, uuid1, esLocationType, ftLocationType, "United States of America", []string{"USA"}, &ConceptMetrics{AnnotationsCount: 15000})
	require.NoError(s.T(), err)

	uuid2 := uuid.NewV4().String()
	err = writeTestConcept(s.ec, uuid2, esLocationType, ftLocationType, "USADA", []string{"USA"}, &ConceptMetrics{AnnotationsCount: 4})
	require.NoError(s.T(), err)

	_, err = s.ec.Refresh(testDefaultIndex).Do(context.Background())
	require.NoError(s.T(), err)

	concepts, err := service.SearchConceptByTextAndTypes("USA", []string{ftLocationType}, false, true)
	require.NoError(s.T(), err)
	require.Len(s.T(), concepts, 2)

	theCountry := concepts[0]
	theFraud := concepts[1]

	assert.Equal(s.T(), "United States of America", theCountry.PrefLabel)
	assert.Equal(s.T(), "USADA", theFraud.PrefLabel)
	cleanup(s.T(), s.ec, esLocationType, uuid1, uuid2)
}

func (s *EsConceptSearchServiceTestSuite) TestSearchConceptsByPopularityAliasMatch() {
	service := NewEsConceptSearchService(testDefaultIndex, "", 10, 10, 2)
	service.SetElasticClient(s.ec)

	uuid1 := uuid.NewV4().String()
	err := writeTestConcept(s.ec, uuid1, esLocationType, ftLocationType, "Luca Panziera", []string{"Dr Git"}, &ConceptMetrics{AnnotationsCount: 15000})
	require.NoError(s.T(), err)

	uuid2 := uuid.NewV4().String()
	err = writeTestConcept(s.ec, uuid2, esLocationType, ftLocationType, "Luca The Fraud", []string{"Dr Git"}, &ConceptMetrics{AnnotationsCount: 4})
	require.NoError(s.T(), err)

	_, err = s.ec.Refresh(testDefaultIndex).Do(context.Background())
	require.NoError(s.T(), err)

	concepts, err := service.SearchConceptByTextAndTypes("Dr G", []string{ftLocationType}, false, true)
	require.NoError(s.T(), err)
	require.Len(s.T(), concepts, 2)

	theDoctor := concepts[0]
	theFraud := concepts[1]

	assert.Equal(s.T(), "Luca Panziera", theDoctor.PrefLabel)
	assert.Equal(s.T(), "Luca The Fraud", theFraud.PrefLabel)
	cleanup(s.T(), s.ec, esLocationType, uuid1, uuid2)
}

func (s *EsConceptSearchServiceTestSuite) TestSearchConceptsByRecentPopularitySameAnnotationsCount() {
	service := NewEsConceptSearchService(testDefaultIndex, "", 10, 10, 2)
	service.SetElasticClient(s.ec)

	uuid1 := uuid.NewV4().String()
	err := writeTestConcept(s.ec, uuid1, esLocationType, ftLocationType, "United States of America", []string{"USA"}, &ConceptMetrics{PrevWeekAnnotationsCount: 7, AnnotationsCount: 10})
	require.NoError(s.T(), err)

	uuid2 := uuid.NewV4().String()
	err = writeTestConcept(s.ec, uuid2, esLocationType, ftLocationType, "USADA", []string{"USA"}, &ConceptMetrics{PrevWeekAnnotationsCount: 2, AnnotationsCount: 10})
	require.NoError(s.T(), err)

	_, err = s.ec.Refresh(testDefaultIndex).Do(context.Background())
	require.NoError(s.T(), err)

	concepts, err := service.SearchConceptByTextAndTypes("USA", []string{ftLocationType}, false, true)
	require.NoError(s.T(), err)
	require.Len(s.T(), concepts, 2)

	theCountry := concepts[0]
	theFraud := concepts[1]

	assert.Equal(s.T(), "United States of America", theCountry.PrefLabel)
	assert.Equal(s.T(), "USADA", theFraud.PrefLabel)
	cleanup(s.T(), s.ec, esLocationType, uuid1, uuid2)
}

func (s *EsConceptSearchServiceTestSuite) TestSearchConceptsByRecentPopularityNoRecentAnnotations() {
	service := NewEsConceptSearchService(testDefaultIndex, "", 10, 10, 2)
	service.SetElasticClient(s.ec)

	uuid1 := uuid.NewV4().String()
	err := writeTestConcept(s.ec, uuid1, esLocationType, ftLocationType, "United States of America", []string{"USA"}, &ConceptMetrics{PrevWeekAnnotationsCount: 0, AnnotationsCount: 100})
	require.NoError(s.T(), err)

	uuid2 := uuid.NewV4().String()
	err = writeTestConcept(s.ec, uuid2, esLocationType, ftLocationType, "USADA", []string{"USA"}, &ConceptMetrics{PrevWeekAnnotationsCount: 0, AnnotationsCount: 10})
	require.NoError(s.T(), err)

	_, err = s.ec.Refresh(testDefaultIndex).Do(context.Background())
	require.NoError(s.T(), err)

	concepts, err := service.SearchConceptByTextAndTypes("USA", []string{ftLocationType}, false, true)
	require.NoError(s.T(), err)
	require.Len(s.T(), concepts, 2)

	theCountry := concepts[0]
	theFraud := concepts[1]

	assert.Equal(s.T(), "United States of America", theCountry.PrefLabel)
	assert.Equal(s.T(), "USADA", theFraud.PrefLabel)
	cleanup(s.T(), s.ec, esLocationType, uuid1, uuid2)
}

func (s *EsConceptSearchServiceTestSuite) TestSearchConceptsByRecentPopularity() {
	service := NewEsConceptSearchService(testDefaultIndex, "", 10, 10, 2)
	service.SetElasticClient(s.ec)

	uuid1 := uuid.NewV4().String()
	err := writeTestConcept(s.ec, uuid1, esLocationType, ftLocationType, "United States of America", []string{"USA"}, &ConceptMetrics{PrevWeekAnnotationsCount: 10, AnnotationsCount: 1000})
	require.NoError(s.T(), err)

	uuid2 := uuid.NewV4().String()
	err = writeTestConcept(s.ec, uuid2, esLocationType, ftLocationType, "USADA", []string{"USA"}, &ConceptMetrics{PrevWeekAnnotationsCount: 20, AnnotationsCount: 100})
	require.NoError(s.T(), err)

	_, err = s.ec.Refresh(testDefaultIndex).Do(context.Background())
	require.NoError(s.T(), err)

	concepts, err := service.SearchConceptByTextAndTypes("USA", []string{ftLocationType}, false, true)
	require.NoError(s.T(), err)
	require.Len(s.T(), concepts, 2)

	theCountry := concepts[0]
	theFraud := concepts[1]

	assert.Equal(s.T(), "United States of America", theCountry.PrefLabel)
	assert.Equal(s.T(), "USADA", theFraud.PrefLabel)
	cleanup(s.T(), s.ec, esLocationType, uuid1, uuid2)
}

func (s *EsConceptSearchServiceTestSuite) TestSearchConceptsByAliasPartialMatch() {
	service := NewEsConceptSearchService(testDefaultIndex, "", 10, 10, 2)
	service.SetElasticClient(s.ec)

	uuid1 := uuid.NewV4().String()
	err := writeTestConcept(s.ec, uuid1, esLocationType, ftLocationType, "United States of America", []string{"Franklin D Roosevelt"}, &ConceptMetrics{AnnotationsCount: 0})
	require.NoError(s.T(), err)

	uuid2 := uuid.NewV4().String()
	err = writeTestConcept(s.ec, uuid2, esLocationType, ftLocationType, "USADA", []string{"USA"}, &ConceptMetrics{AnnotationsCount: 0})
	require.NoError(s.T(), err)

	_, err = s.ec.Refresh(testDefaultIndex).Do(context.Background())
	require.NoError(s.T(), err)

	concepts, err := service.SearchConceptByTextAndTypes("roose", []string{ftLocationType}, false, true)
	require.NoError(s.T(), err)
	require.Len(s.T(), concepts, 1)

	theCountry := concepts[0]

	assert.Equal(s.T(), "United States of America", theCountry.PrefLabel)
	cleanup(s.T(), s.ec, esLocationType, uuid1, uuid2)
}

func (s *EsConceptSearchServiceTestSuite) TestFindOrganisationWithCountryCodeAndCountryOfIncorporation() {
	service := NewEsConceptSearchService(testDefaultIndex, "", 10, 10, 2)
	service.SetElasticClient(s.ec)

	uuid := uuid.NewV4().String()
	err := writeTestConceptWithCountryCodeAndCountryOfIncorporation(s.ec, uuid, esOrganisationType, ftOrganisationType, "MooTech Ltd.", []string{"MooTech Ltd."}, "CA", "US")
	require.NoError(s.T(), err)

	_, err = s.ec.Refresh(testDefaultIndex).Do(context.Background())
	require.NoError(s.T(), err)

	concepts, err := service.SearchConceptByTextAndTypes("Moo", []string{ftOrganisationType}, false, false)
	require.NoError(s.T(), err)
	require.Len(s.T(), concepts, 1)

	theCompany := concepts[0]

	assert.Equal(s.T(), "MooTech Ltd.", theCompany.PrefLabel)
	assert.Equal(s.T(), "CA", theCompany.CountryCode)
	assert.Equal(s.T(), "US", theCompany.CountryOfIncorporation)

	cleanup(s.T(), s.ec, esOrganisationType, uuid)
}
