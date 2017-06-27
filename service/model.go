package service

import (
	"strings"
)

type EsConceptModel struct {
	Id         string   `json:"id"`
	ApiUrl     string   `json:"apiUrl"`
	PrefLabel  string   `json:"prefLabel"`
	Types      []string `json:"types"`
	DirectType string   `json:"directType"`
	Aliases    []string `json:"aliases,omitempty"`
}

type Concept struct {
	Id          string `json:"id"`
	ApiUrl      string `json:"apiUrl"`
	PrefLabel   string `json:"prefLabel"`
	ConceptType string `json:"type"`
}

type Concepts []Concept

var (
	esTypeMapping = map[string]string{
		"http://www.ft.com/ontology/Genre":                     "genres",
		"http://www.ft.com/ontology/product/Brand":             "brands",
		"http://www.ft.com/ontology/person/Person":             "people",
		"http://www.ft.com/ontology/organisation/Organisation": "organisations",
		"http://www.ft.com/ontology/Location":                  "locations",
		"http://www.ft.com/ontology/Topic":                     "topics",
	}

	incorrectPath = "http://api.ft.com/things/"
)

func ConvertToSimpleConcept(esConcept EsConceptModel, esType string) Concept {
	c := Concept{}
	c.Id = correctPath(esConcept.Id)
	c.ApiUrl = esConcept.ApiUrl
	c.ConceptType = ftType(esType)
	c.PrefLabel = esConcept.PrefLabel

	return c
}

func esType(ftType string) string {
	return esTypeMapping[ftType]
}

func correctPath(id string) string {
	if (strings.HasPrefix(id, incorrectPath)){
		return strings.Replace(id, incorrectPath, "http://www.ft.com/thing/", 1)
	}
	return id
}

func ftType(esType string) string {
	for k, v := range esTypeMapping {
		if v == esType {
			return k
		}
	}

	return ""
}

func (slice Concepts) Len() int {
	return len(slice)
}

func (slice Concepts) Less(i, j int) bool {
	return slice[i].PrefLabel < slice[j].PrefLabel
}

func (slice Concepts) Swap(i, j int) {
	slice[i], slice[j] = slice[j], slice[i]
}
