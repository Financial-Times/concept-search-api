package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEsType(t *testing.T) {
	assert.Equal(t, "genres", EsType("http://www.ft.com/ontology/Genre"), "known type conversion")
	assert.Equal(t, "", EsType("http://www.ft.com/ontology/Foo"), "unknown type conversion")
}

func TestFtType(t *testing.T) {
	assert.Equal(t, "http://www.ft.com/ontology/Genre", FtType("genres"), "known type conversion")
	assert.Equal(t, "", FtType("tardigrades"), "unknown type conversion")
}

func TestValidateAuthors(t *testing.T) {
	// validate no types given
	assert.Equal(t, ValidateForAuthorsSearch([]string{}, "authors").Error(), ErrNoConceptTypeParameter.Error())

	// validate too many types given
	assert.Equal(t, ValidateForAuthorsSearch([]string{"http://www.ft.com/ontology/person/Person", "http://www.ft.com/ontology/Genre"}, "authors").Error(), ErrNotSupportedCombinationOfConceptTypes.Error())

	// wrong type
	assert.Contains(t, ValidateForAuthorsSearch([]string{"http://www.ft.com/ontology/Genre"}, "authors").Error(), "http://www.ft.com/ontology/Genre")

	// good type and wrong boost name
	assert.Equal(t, ValidateForAuthorsSearch([]string{"http://www.ft.com/ontology/person/Person"}, "wrong_boost").Error(), ErrInvalidBoostTypeParameter.Error())

	// happy path
	assert.Nil(t, ValidateForAuthorsSearch([]string{"http://www.ft.com/ontology/person/Person"}, "authors"))
}

func TestValidateEsTypes(t *testing.T) {
	res, err := ValidateAndConvertToEsTypes([]string{"http://www.ft.com/ontology/Foo", "http://www.ft.com/ontology/person/Person"})
	assert.Contains(t, err.Error(), "http://www.ft.com/ontology/Foo")

	res, err = ValidateAndConvertToEsTypes([]string{"http://www.ft.com/ontology/person/Person"})
	assert.NoError(t, err)
	assert.Len(t, res, 2)
	assert.Equal(t, "", res[0])
	assert.Equal(t, "people", res[1])
}
