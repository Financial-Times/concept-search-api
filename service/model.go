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
	IsDeprecated bool     `json:"isDeprecated,omitempty"`
}

type Concept struct {
	Id          string `json:"id"`
	ApiUrl      string `json:"apiUrl"`
	PrefLabel   string `json:"prefLabel"`
	ConceptType string `json:"type"`
	IsFTAuthor  *bool  `json:"isFTAuthor,omitempty"`
}

type Concepts []Concept

var (
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

	return c
}

func correctPath(id string) string {
	if strings.HasPrefix(id, incorrectPath) {
		return strings.Replace(id, incorrectPath, "http://www.ft.com/thing/", 1)
	}
	return id
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
