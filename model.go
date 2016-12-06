package main

type searchCriteria struct {
	Term string `json:"term"`
}

type concept struct {
	Id         string   `json:"id"`
	ApiUrl     string   `json:"apiUrl"`
	PrefLabel  string   `json:"prefLabel"`
	Types      []string `json:"types"`
	DirectType string   `json:"directType"`
	Aliases    []string `json:"aliases,omitempty"`
	Score      float64  `json:"score,omitempty"`
}
