package main

import (
	"github.com/olivere/elastic/v7"
)

type searchCriteria struct {
	Term           *string  `json:"term"`
	BestMatchTerms []string `json:"bestMatchTerms"`
	ConceptTypes   []string `json:"conceptTypes"`
	BoostType      string   `json:"boost"`
	FilterType     string   `json:"filter"`
}

type concept struct {
	ID                     string   `json:"id"`
	APIUrl                 string   `json:"apiUrl"`
	PrefLabel              string   `json:"prefLabel"`
	Types                  []string `json:"types"`
	DirectType             string   `json:"directType"`
	Aliases                []string `json:"aliases,omitempty"`
	Score                  float64  `json:"score,omitempty"`
	IsFTAuthor             string   `json:"isFTAuthor,omitempty"`
	ScopeNote              string   `json:"scopeNote,omitempty"`
	IsDeprecated           bool     `json:"isDeprecated,omitempty"`
	CountryCode            string   `json:"countryCode,omitempty"`
	CountryOfIncorporation string   `json:"countryOfIncorporation,omitempty"`
}

type searchResult struct {
	Results []concept `json:"results"`
}

type multiSearchWrapper struct {
	term          string
	searchRequest *elastic.SearchRequest
}
