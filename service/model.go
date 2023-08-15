package service

import (
	"strconv"
	"strings"

	"github.com/Financial-Times/concept-search-api/util"
	log "github.com/sirupsen/logrus"
)

type EsConceptModel struct {
	Id                     string          `json:"id"`
	Type                   string          `json:"type"`
	ApiUrl                 string          `json:"apiUrl"`
	PrefLabel              string          `json:"prefLabel"`
	Types                  []string        `json:"types"`
	DirectType             string          `json:"directType"`
	Aliases                []string        `json:"aliases,omitempty"`
	IsFTAuthor             *string         `json:"isFTAuthor,omitempty"`
	IsDeprecated           bool            `json:"isDeprecated,omitempty"`
	ScopeNote              string          `json:"scopeNote,omitempty"`
	Metrics                *ConceptMetrics `json:"metrics,omitempty"`
	CountryCode            string          `json:"countryCode,omitempty"`
	CountryOfIncorporation string          `json:"countryOfIncorporation,omitempty"`
}

type ConceptMetrics struct {
	AnnotationsCount         int `json:"annotationsCount"`
	PrevWeekAnnotationsCount int `json:"prevWeekAnnotationsCount"`
}

type Concept struct {
	Id                     string `json:"id"`
	UUID                   string `json:"uuid"`
	ApiUrl                 string `json:"apiUrl"`
	PrefLabel              string `json:"prefLabel"`
	ConceptType            string `json:"type"`
	IsFTAuthor             *bool  `json:"isFTAuthor,omitempty"`
	IsDeprecated           bool   `json:"isDeprecated,omitempty"`
	ScopeNote              string `json:"scopeNote,omitempty"`
	CountryCode            string `json:"countryCode,omitempty"`
	CountryOfIncorporation string `json:"countryOfIncorporation,omitempty"`
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
	c.ScopeNote = esConcept.ScopeNote
	c.CountryCode = esConcept.CountryCode
	c.CountryOfIncorporation = esConcept.CountryOfIncorporation
	if esConcept.IsFTAuthor != nil {
		ftAuthor, err := strconv.ParseBool(*esConcept.IsFTAuthor)
		if err != nil {
			log.WithField("id", esConcept.Id).WithField("isFtAuthor", esConcept.IsFTAuthor).Warn("Failed to parse boolean field isFtAuthor - is there a data issue")
		} else {
			c.IsFTAuthor = &ftAuthor
		}
	}
	c.IsDeprecated = esConcept.IsDeprecated
	uuid, err := util.ExtractUUID(esConcept.Id)
	if err != nil {
		log.WithField("id", esConcept.Id).Warn("couldn't extract concept UUID from ID")
	}
	c.UUID = uuid
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
