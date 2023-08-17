package util

import (
	"errors"
	"fmt"
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

func TestExtractUUID(t *testing.T) {
	tests := []struct {
		name         string
		id           string
		expectedUUID string
		err          error
	}{
		{
			name:         "valid id to UUID",
			id:           "http://api.ft.com/things/82cba3ce-329b-3010-b29d-4282a215889f",
			expectedUUID: "82cba3ce-329b-3010-b29d-4282a215889f",
		},
		{
			name: "empty id causes error",
			err:  errors.New("cannot extract UUID because Id doesn't contain a valid UUID substring: "),
		},
		{
			name: "invalid id causes error",
			id:   "invalid id",
			err:  errors.New("cannot extract UUID because Id doesn't contain a valid UUID substring: invalid id"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			uuid, err := ExtractUUID(test.id)
			if err != nil {
				if test.err == nil {
					t.Errorf("unexpected error occurred: %v", err)
					return
				}

				if err.Error() != test.err.Error() {
					t.Errorf("expected error: %v, but got: %v", test.err, err)
				}
				return
			}

			if test.err != nil {
				t.Errorf("expected error did not occur: %v", test.err)
				return
			}

			if uuid != test.expectedUUID {
				t.Errorf("expected UUID: %v, but got: %v", test.expectedUUID, uuid)
			}
		})
	}
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

func TestValidateEsTypesNoError(t *testing.T) {
	res, isPublicCompany, err := ValidateAndConvertToEsTypes([]string{"http://www.ft.com/ontology/person/Person"})
	assert.NoError(t, err)
	assert.Len(t, res, 2)
	assert.Equal(t, "", res[0])
	assert.Equal(t, "people", res[1])
	assert.Equal(t, false, isPublicCompany)
}

func TestValidateEsTypesReturnError(t *testing.T) {
	_, isPublicCompany, err := ValidateAndConvertToEsTypes([]string{"http://www.ft.com/ontology/Foo", "http://www.ft.com/ontology/person/Person"})
	assert.Contains(t, err.Error(), "http://www.ft.com/ontology/Foo")
	assert.Equal(t, false, isPublicCompany)
}

func TestValidateEsTypesWithPublicCompany(t *testing.T) {
	res, isPublicCompany, err := ValidateAndConvertToEsTypes([]string{"http://www.ft.com/ontology/person/Person", "http://www.ft.com/ontology/company/PublicCompany"})
	fmt.Printf("%v", res)
	assert.NoError(t, err)
	assert.Len(t, res, 3)
	assert.Equal(t, "", res[0])
	assert.Equal(t, "", res[1])
	assert.Equal(t, "people", res[2])
	assert.Equal(t, true, isPublicCompany)
}

func TestValidateConceptTypesForTextModeSearch(t *testing.T) {
	validConceptTypes1 := []string{"http://www.ft.com/ontology/organisation/Organisation", "http://www.ft.com/ontology/company/PublicCompany"}
	validConceptTypes2 := []string{"http://www.ft.com/ontology/organisation/Organisation", "http://www.ft.com/ontology/product/Brand"}
	invalidConceptTypes1 := []string{"http://www.ft.com/ontology/person/Person"}
	invalidConceptTypes2 := []string{}

	assert.Nil(t, ValidateConceptTypesForTextModeSearch(validConceptTypes1))
	assert.Nil(t, ValidateConceptTypesForTextModeSearch(validConceptTypes2))

	assert.Error(t, ValidateConceptTypesForTextModeSearch(invalidConceptTypes1))
	assert.Error(t, ValidateConceptTypesForTextModeSearch(invalidConceptTypes2))
}
