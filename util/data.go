package util

import (
	"errors"
	"fmt"
)

var (
	esTypeMapping = map[string]string{
		"http://www.ft.com/ontology/Genre":                     "genres",
		"http://www.ft.com/ontology/product/Brand":             "brands",
		"http://www.ft.com/ontology/person/Person":             "people",
		"http://www.ft.com/ontology/organisation/Organisation": "organisations",
		"http://www.ft.com/ontology/Location":                  "locations",
		"http://www.ft.com/ontology/Topic":                     "topics",
		"http://www.ft.com/ontology/AlphavilleSeries":          "alphaville-series",
		"http://www.ft.com/ontology/company/PublicCompany":     "public-company",
	}

	ErrInvalidConceptTypeFormat              = "invalid concept type %v"
	ErrNoElasticClient                       = errors.New("no ElasticSearch client available")
	ErrNoConceptTypeParameter                = NewInputError("no concept type specified")
	ErrNotSupportedCombinationOfConceptTypes = NewInputError("the combination of concept types is not supported")
	ErrInvalidBoostTypeParameter             = NewInputError("invalid boost type")
)

func FirstError(errors ...error) error {
	for _, err := range errors {
		if err != nil {
			return err
		}
	}

	return nil
}

func ToTerms(types []string) []interface{} {
	i := make([]interface{}, 0)
	for _, v := range types {
		i = append(i, v)
	}
	return i
}

func EsType(ftType string) string {
	return esTypeMapping[ftType]
}

func FtType(esType string) string {
	for k, v := range esTypeMapping {
		if v == esType {
			return k
		}
	}

	return ""
}

func ValidateForAuthorsSearch(conceptTypes []string, boostType string) error {
	if len(conceptTypes) == 0 {
		return ErrNoConceptTypeParameter
	}
	if len(conceptTypes) > 1 {
		return ErrNotSupportedCombinationOfConceptTypes
	}
	if EsType(conceptTypes[0]) != "people" {
		return NewInputErrorf(ErrInvalidConceptTypeFormat, conceptTypes[0])
	}
	if boostType != "authors" {
		return ErrInvalidBoostTypeParameter
	}
	return nil
}

func ValidateAndConvertToEsTypes(conceptTypes []string) ([]string, error) {
	esTypes := make([]string, len(conceptTypes))
	for _, t := range conceptTypes {
		esT := EsType(t)
		if esT == "" {
			return esTypes, NewInputErrorf(ErrInvalidConceptTypeFormat, t)
		}
		esTypes = append(esTypes, esT)
	}
	return esTypes, nil
}

type InputError struct {
	msg string
}

func (e InputError) Error() string {
	return e.msg
}

func NewInputError(msg string) InputError {
	return InputError{msg}
}

func NewInputErrorf(format string, args ...interface{}) InputError {
	return InputError{fmt.Sprintf(format, args...)}
}
