package main

type searchCriteria struct {
	Term            *string  `json:"term"`
	ExactMatchTerms []string `json:"exactMatchTerms"`
}

type concept struct {
	ID         string   `json:"id"`
	APIUrl     string   `json:"apiUrl"`
	PrefLabel  string   `json:"prefLabel"`
	Types      []string `json:"types"`
	DirectType string   `json:"directType"`
	Aliases    []string `json:"aliases,omitempty"`
	Score      float64  `json:"score,omitempty"`
}

type searchResult struct {
	Results []concept `json:"results"`
}
