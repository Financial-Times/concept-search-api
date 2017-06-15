package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
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

func TestEsType(t *testing.T) {
	assert.Equal(t, "genres", esType("http://www.ft.com/ontology/Genre"), "known type conversion")
	assert.Equal(t, "", esType("http://www.ft.com/ontology/Foo"), "unknown type conversion")
}

func TestFtType(t *testing.T) {
	assert.Equal(t, "http://www.ft.com/ontology/Genre", ftType("genres"), "known type conversion")
	assert.Equal(t, "", ftType("tardigrades"), "unknown type conversion")
}

func TestConvertToSimpleConcept(t *testing.T) {
	id := "http://www.ft.com/thing/id"
	apiUrl := "http://www.example.com/1"
	label := "Test Concept"

	esConcept := EsConceptModel{
		Id:         id,
		ApiUrl:     apiUrl,
		PrefLabel:  label,
		Types:      []string{"any"},
		DirectType: "any",
		Aliases:    []string{},
	}

	actual := ConvertToSimpleConcept(esConcept, "genres")

	assert.Equal(t, id, actual.Id, "http://www.ft.com/thing/id")
	assert.Equal(t, apiUrl, actual.ApiUrl, "apiUrl")
	assert.Equal(t, "http://www.ft.com/ontology/Genre", actual.ConceptType, "type")
	assert.Equal(t, label, actual.PrefLabel, "prefLabel")
}

func TestConvertToSimpleConceptWithIdCorrect(t *testing.T) {
	id := "http://api.ft.com/things/id"
	apiUrl := "http://www.example.com/1"
	label := "Another Test Concept"

	esConcept := EsConceptModel{
		Id:         id,
		ApiUrl:     apiUrl,
		PrefLabel:  label,
		Types:      []string{"any"},
		DirectType: "any",
		Aliases:    []string{},
	}

	actual := ConvertToSimpleConcept(esConcept, "genres")
	assert.Equal(t, id, actual.Id, "http://www.ft.com/thing/id")
}

