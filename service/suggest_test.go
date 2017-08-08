package service

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (s *EsConceptSearchServiceTestSuite) TestSuggestConceptByTextAndTypesInvalidTextParameter() {
	service := NewEsConceptSearchService(testIndexName, 10, 10, 2)
	service.SetElasticClient(s.ec)

	_, err := service.SuggestConceptByTextAndTypes("", []string{ftBrandType})
	assert.EqualError(s.T(), err, errEmptyTextParameter.Error(), "error response")
}

func (s *EsConceptSearchServiceTestSuite) TestSuggestConceptByTextAndType() {
	service := NewEsConceptSearchService(testIndexName, 10, 10, 2)
	service.(*esConceptSearchService).mappingRefreshInterval = time.Second
	service.SetElasticClient(s.ec)

	// for short test, run once and don't sleep
	// otherwise, repeat 5 times with random sleep between 500 and 1500 ms each time
	// to prove the read and write (refresh) goroutines interact safely with each other
	iterations := 5
	if testing.Short() {
		iterations = 1
	}
	for i := 0; i < iterations; i++ {
		concepts, err := service.SuggestConceptByTextAndTypes("test", []string{ftBrandType})
		assert.NoError(s.T(), err, "expected no error for ES read")
		assert.Len(s.T(), concepts, 4, "there should be four results")
		for _, c := range concepts {
			assert.Equal(s.T(), ftBrandType, c.ConceptType, "Results should be of type FT Brand")
		}

		if iterations > 1 {
			time.Sleep(time.Duration(500+rand.Int31n(1000)) * time.Millisecond)
		}
	}
}

func (s *EsConceptSearchServiceTestSuite) TestSuggestConceptByTextAndTypeInvalidAutocompleteType() {
	service := NewEsConceptSearchService(testIndexName, 10, 10, 2)
	service.SetElasticClient(s.ec)

	_, err := service.SuggestConceptByTextAndTypes("test", []string{ftOrganisationType})
	assert.EqualError(s.T(), err, errInvalidConceptTypeForAutocompleteByType.Error(), "error response")
}

func (s *EsConceptSearchServiceTestSuite) TestSuggestConceptByTextAndTypesMissingTypes() {
	service := NewEsConceptSearchService(testIndexName, 10, 10, 2)
	service.SetElasticClient(s.ec)

	_, err := service.SuggestConceptByTextAndTypes("test", []string{})
	assert.EqualError(s.T(), err, errNoConceptTypeParameter.Error(), "expected no concept type parameter error")
}

func (s *EsConceptSearchServiceTestSuite) TestSuggestConceptByTextAndTypeInvalidType() {
	service := NewEsConceptSearchService(testIndexName, 10, 10, 2)
	service.SetElasticClient(s.ec)

	_, err := service.SuggestConceptByTextAndTypes("test", []string{"pippo"})
	assert.EqualError(s.T(), err, fmt.Sprintf(errInvalidConceptTypeFormat, "pippo"), "expected invalid type error")
}

func (s *EsConceptSearchServiceTestSuite) TestSuggestConceptByTextAndTypesNotEnoughValidTypes() {
	service := NewEsConceptSearchService(testIndexName, 10, 10, 2)
	service.SetElasticClient(s.ec)

	_, err := service.SuggestConceptByTextAndTypes("test", []string{ftOrganisationType, ftLocationType, ftPeopleType})
	assert.EqualError(s.T(), err, errNotSupportedCombinationOfConceptTypes.Error(), "expected error not supported combination of concept types")
}

func (s *EsConceptSearchServiceTestSuite) TestSuggestConceptByTextAndTypesNotValidTypeInCombination() {
	service := NewEsConceptSearchService(testIndexName, 10, 10, 2)
	service.SetElasticClient(s.ec)

	_, err := service.SuggestConceptByTextAndTypes("test", []string{ftOrganisationType, ftLocationType, ftPeopleType, ftBrandType})
	assert.EqualError(s.T(), err, errNotSupportedCombinationOfConceptTypes.Error(), "expected error not supported combination of concept types")
}

func (s *EsConceptSearchServiceTestSuite) TestSuggestConceptByTextAndTypesNotExistingTypeInCombination() {
	service := NewEsConceptSearchService(testIndexName, 10, 10, 2)
	service.SetElasticClient(s.ec)

	_, err := service.SuggestConceptByTextAndTypes("test", []string{ftOrganisationType, ftLocationType, ftPeopleType, "pippo"})
	assert.EqualError(s.T(), err, fmt.Sprintf(errInvalidConceptTypeFormat, "pippo"), "expected error invalid concept type")
}

func (s *EsConceptSearchServiceTestSuite) TestSuggestConceptByTextAndTypesWithBoost() {
	service := NewEsConceptSearchService(testIndexName, 10, 10, 2)
	service.SetElasticClient(s.ec)

	concepts, err := service.SuggestConceptByTextAndTypesWithBoost("test", []string{ftPeopleType}, "authors")
	assert.NoError(s.T(), err, "expected no error for ES read")
	assert.Len(s.T(), concepts, 8, "there should be eight results")

	for i, concept := range concepts {
		assert.Equal(s.T(), ftPeopleType, concept.ConceptType)
		if i < 4 {
			require.NotNil(s.T(), concept.IsFTAuthor)
			assert.True(s.T(), *concept.IsFTAuthor)
		} else {
			assert.Nil(s.T(), concept.IsFTAuthor)
		}
	}
}

func (s *EsConceptSearchServiceTestSuite) TestSuggestConceptByTextAndTypesWithBoostRestrictedSize() {
	service := NewEsConceptSearchService(testIndexName, 10, 1, 2)
	service.SetElasticClient(s.ec)

	concepts, err := service.SuggestConceptByTextAndTypesWithBoost("test", []string{ftPeopleType}, "authors")
	assert.NoError(s.T(), err, "expected no error for ES read")
	assert.Len(s.T(), concepts, 1, "there should be one results")
}

func (s *EsConceptSearchServiceTestSuite) TestSuggestConceptByTextAndTypesWithBoostNoInputText() {
	service := NewEsConceptSearchService(testIndexName, 10, 10, 2)
	service.SetElasticClient(s.ec)

	concepts, err := service.SuggestConceptByTextAndTypesWithBoost("", []string{ftPeopleType}, "authors")
	assert.EqualError(s.T(), err, errEmptyTextParameter.Error())
	assert.Nil(s.T(), concepts)
}

func (s *EsConceptSearchServiceTestSuite) TestSuggestConceptByTextAndTypesWithBoostNoTypes() {
	service := NewEsConceptSearchService(testIndexName, 10, 10, 2)
	service.SetElasticClient(s.ec)

	concepts, err := service.SuggestConceptByTextAndTypesWithBoost("test", []string{}, "authors")
	assert.EqualError(s.T(), err, errNoConceptTypeParameter.Error())
	assert.Nil(s.T(), concepts)
}

func (s *EsConceptSearchServiceTestSuite) TestSuggestConceptByTextAndTypesWithBoostMultipleTypes() {
	service := NewEsConceptSearchService(testIndexName, 10, 10, 2)
	service.SetElasticClient(s.ec)

	concepts, err := service.SuggestConceptByTextAndTypesWithBoost("test", []string{ftPeopleType, ftLocationType}, "authors")
	assert.EqualError(s.T(), err, errNotSupportedCombinationOfConceptTypes.Error())
	assert.Nil(s.T(), concepts)
}

func (s *EsConceptSearchServiceTestSuite) TestSuggestConceptByTextAndTypesWithInvalidBoost() {
	service := NewEsConceptSearchService(testIndexName, 10, 10, 2)
	service.SetElasticClient(s.ec)

	concepts, err := service.SuggestConceptByTextAndTypesWithBoost("test", []string{ftPeopleType}, "pluto")
	assert.EqualError(s.T(), err, errInvalidBoostTypeParameter.Error())
	assert.Nil(s.T(), concepts)
}

func (s *EsConceptSearchServiceTestSuite) TestSuggestConceptByTextAndTypesWithBoostNoESConnection() {
	service := NewEsConceptSearchService(testIndexName, 10, 10, 2)

	concepts, err := service.SuggestConceptByTextAndTypesWithBoost("test", []string{ftPeopleType}, "authors")
	assert.EqualError(s.T(), err, ErrNoElasticClient.Error())
	assert.Nil(s.T(), concepts)
}

func (s *EsConceptSearchServiceTestSuite) TestSuggestConceptByTextAndTypesWithBoostInvalidConceptType() {
	service := NewEsConceptSearchService(testIndexName, 10, 10, 2)

	concepts, err := service.SuggestConceptByTextAndTypesWithBoost("test", []string{ftGenreType}, "authors")
	assert.EqualError(s.T(), err, fmt.Sprintf(errInvalidConceptTypeFormat, ftGenreType))
	assert.Nil(s.T(), concepts)
}

func (s *EsConceptSearchServiceTestSuite) TestAutocompletionResultSize() {
	service := NewEsConceptSearchService(testIndexName, 10, 3, 2)
	service.SetElasticClient(s.ec)
	concepts, err := service.SuggestConceptByTextAndTypes("test", []string{ftBrandType})
	assert.NoError(s.T(), err, "expected no error for ES read")
	assert.Len(s.T(), concepts, 3, "there should be three results")
	for _, c := range concepts {
		assert.Equal(s.T(), ftBrandType, c.ConceptType, "Results should be of type FT Brand")
	}
}

func (s *EsConceptSearchServiceTestSuite) TestSuggestConceptByTextAndMultipleType() {
	service := NewEsConceptSearchService(testIndexName, 20, 20, 2)
	service.(*esConceptSearchService).mappingRefreshInterval = time.Second
	service.SetElasticClient(s.ec)

	// for short test, run once and don't sleep
	// otherwise, repeat 5 times with random sleep between 500 and 1500 ms each time
	// to prove the read and write (refresh) goroutines interact safely with each other
	iterations := 5
	if testing.Short() {
		iterations = 1
	}
	for i := 0; i < iterations; i++ {
		types := []string{ftLocationType, ftOrganisationType, ftPeopleType, ftTopicType}
		concepts, err := service.SuggestConceptByTextAndTypes("test", types)
		assert.NoError(s.T(), err, "expected no error for ES read")
		assert.Len(s.T(), concepts, 13, "there should be thirteen results")
		counts := map[string]int{}
		for _, c := range concepts {
			i := counts[c.ConceptType]
			counts[c.ConceptType] = i + 1
		}

		assert.Equal(s.T(), 8, counts[ftPeopleType], "people")
		assert.Equal(s.T(), 1, counts[ftOrganisationType], "organisations")
		assert.Equal(s.T(), 2, counts[ftLocationType], "locations")
		assert.Equal(s.T(), 2, counts[ftTopicType], "topics")

		if iterations > 1 {
			time.Sleep(time.Duration(500+rand.Int31n(1000)) * time.Millisecond)
		}
	}
}
