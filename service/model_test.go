package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConceptsSortInterface(t *testing.T) {
	concept1 := Concept{
		Id:          "id1",
		ApiUrl:      "http://www.example.com/thing/1",
		PrefLabel:   "Concept 1",
		ConceptType: "http://www.ft.com/ontology/Genre",
	}

	concept2 := Concept{
		Id:          "id2",
		ApiUrl:      "http://www.example.com/thing/2",
		PrefLabel:   "Concept 2",
		ConceptType: "http://www.ft.com/ontology/Genre",
	}

	concepts := Concepts{concept1, concept2}

	assert.Equal(t, 2, concepts.Len(), "slice length")
	assert.True(t, concepts.Less(0, 1), "concept ordering by prefLabel")
	assert.False(t, concepts.Less(1, 0), "concept ordering by prefLabel")

	concepts.Swap(0, 1)

	assert.Equal(t, concept2, concepts[0], "swapped concepts")
	assert.Equal(t, concept1, concepts[1], "swapped concepts")

	assert.Equal(t, 2, concepts.Len(), "slice length")
	assert.True(t, concepts.Less(1, 0), "concept ordering by prefLabel")
	assert.False(t, concepts.Less(0, 1), "concept ordering by prefLabel")
}

func TestConvertToSimpleConcept(t *testing.T) {
	id := "http://www.ft.com/thing/82cba3ce-329b-3010-b29d-4282a215889f"
	apiUrl := "http://www.example.com/1"
	label := "Test Concept"
	directType := "http://www.ft.com/ontology/GenreSubClass"

	countryCode := "CA"
	countryOfIncorporation := "US"
	esConcept := EsConceptModel{
		Id:                     id,
		Type:                   "people",
		ApiUrl:                 apiUrl,
		PrefLabel:              label,
		Types:                  []string{"any"},
		DirectType:             directType,
		Aliases:                []string{},
		IsDeprecated:           true,
		CountryCode:            countryCode,
		CountryOfIncorporation: countryOfIncorporation,
	}

	actual := ConvertToSimpleConcept(esConcept)

	assert.Equal(t, id, actual.Id, "http://www.ft.com/thing/id")
	assert.Equal(t, "82cba3ce-329b-3010-b29d-4282a215889f", actual.UUID, "UUID")
	assert.Equal(t, apiUrl, actual.ApiUrl, "apiUrl")
	assert.Equal(t, directType, actual.ConceptType, "the type is not correct")
	assert.Equal(t, label, actual.PrefLabel, "prefLabel")
	assert.Equal(t, true, actual.IsDeprecated, "isDeprecated")
	assert.Equal(t, countryCode, actual.CountryCode, "countryCode")
	assert.Equal(t, countryOfIncorporation, actual.CountryOfIncorporation, "countryOfIncorporation")
}

func TestConvertToSimpleConceptWithIdCorrect(t *testing.T) {
	id := "http://api.ft.com/things/82cba3ce-329b-3010-b29d-4282a215889f"
	apiUrl := "http://www.example.com/1"
	label := "Another Test Concept"

	esConcept := EsConceptModel{
		Id:         id,
		Type:       "people",
		ApiUrl:     apiUrl,
		PrefLabel:  label,
		Types:      []string{"any"},
		DirectType: "any",
		Aliases:    []string{},
	}

	actual := ConvertToSimpleConcept(esConcept)
	assert.Equal(t, "http://www.ft.com/thing/82cba3ce-329b-3010-b29d-4282a215889f", actual.Id, "The id did not get converted properly")
}

func TestConvertToSimpleConceptWithFTAuthor(t *testing.T) {
	id := "http://api.ft.com/things/82cba3ce-329b-3010-b29d-4282a215889a"
	apiUrl := "http://www.example.com/1"
	label := "Another Test Concept"
	isFtAuthor := "true"

	esConcept := EsConceptModel{
		Id:         id,
		Type:       "people",
		ApiUrl:     apiUrl,
		PrefLabel:  label,
		Types:      []string{"any"},
		DirectType: "any",
		Aliases:    []string{},
		IsFTAuthor: &isFtAuthor,
	}

	actual := ConvertToSimpleConcept(esConcept)
	require.NotNil(t, actual.IsFTAuthor)
	assert.True(t, *actual.IsFTAuthor)
	assert.Equal(t, false, actual.IsDeprecated, "isDeprecated")
}

func TestConvertToSimpleConceptWithNonBoolFTAuthor(t *testing.T) {
	id := "http://api.ft.com/things/82cba3ce-329b-3010-b29d-4282a215889a"
	apiUrl := "http://www.example.com/1"
	label := "Another Test Concept"
	isFtAuthor := "ahhh i'm not a bool!"

	esConcept := EsConceptModel{
		Id:         id,
		Type:       "people",
		ApiUrl:     apiUrl,
		PrefLabel:  label,
		Types:      []string{"any"},
		DirectType: "any",
		Aliases:    []string{},
		IsFTAuthor: &isFtAuthor,
	}

	actual := ConvertToSimpleConcept(esConcept)
	assert.Nil(t, actual.IsFTAuthor)
}
