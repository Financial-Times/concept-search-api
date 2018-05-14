package service

import (
	"strconv"
	"strings"

	log "github.com/Sirupsen/logrus"
)

type EsConceptModel struct {
	Id           string   `json:"id"`
	ApiUrl       string   `json:"apiUrl"`
	PrefLabel    string   `json:"prefLabel"`
	Types        []string `json:"types"`
	DirectType   string   `json:"directType"`
	Aliases      []string `json:"aliases,omitempty"`
	IsFTAuthor   *string  `json:"isFTAuthor,omitempty"`
	IsDeprecated *string  `json:"isDeprecated,omitempty"`
}

type Concept struct {
	Id           string `json:"id"`
	ApiUrl       string `json:"apiUrl"`
	PrefLabel    string `json:"prefLabel"`
	ConceptType  string `json:"type"`
	IsFTAuthor   *bool  `json:"isFTAuthor,omitempty"`
	IsDeprecated *bool  `json:"isDeprecated,omitempty"`
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

func ConvertToSimpleConcept(esConcept EsConceptModel) Concept {
	c := Concept{}
	c.Id = correctPath(esConcept.Id)
	c.ApiUrl = esConcept.ApiUrl
	c.ConceptType = esConcept.DirectType
	c.PrefLabel = esConcept.PrefLabel
	if esConcept.IsFTAuthor != nil {
		ftAuthor, err := strconv.ParseBool(*esConcept.IsFTAuthor)
		if err != nil {
			log.WithField("id", esConcept.Id).WithField("isFtAuthor", esConcept.IsFTAuthor).Warn("Failed to parse boolean field isFtAuthor - is there a data issue")
		} else {
			c.IsFTAuthor = &ftAuthor
		}
	}
	if esConcept.IsDeprecated != nil {
		deprecated, err := strconv.ParseBool(*esConcept.IsDeprecated)
		if err != nil {
			log.WithField("id", esConcept.Id).WithField("isDeprecated", esConcept.IsDeprecated).Warn("Failed to parse boolean field isDeprecated - is there a data issue")
		} else {
			c.IsDeprecated = &deprecated
		}
	}

	return c
}

func esType(ftType string) string {
	return esTypeMapping[ftType]
}

func correctPath(id string) string {
	if strings.HasPrefix(id, incorrectPath) {
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
