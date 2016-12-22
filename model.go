package main

type searchCriteria struct {
	Term *string `json:"term"`
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

type conceptQuery struct {
	Term string
}

// Source returns a JSON serializable form of the query
func (q conceptQuery) Source() (interface{}, error) {
	//`{
	//    "query": {
	//        "bool": {
	//            "should": [{
	//                "multi_match": {
	//                    "query": "{SEARCH_TERM}",
	//                    "type": "most_fields",
	//                    "fields": [
	//                        "prefLabel",
	//                        "aliases"
	//                    ]
	//                }
	//            }, {
	//                "term": {
	//                    "prefLabel.raw": "{SEARCH_TERM}",
	//                    "boost": 2
	//                }
	//            }, {
	//                "term": {
	//                    "aliases.raw": "{SEARCH_TERM}",
	//                    "boost": 2
	//                }
	//            }]
	//        }
	//    }
	//}`

	//TODO: extract common parts to func like the creation of terms for raw aliases and prefLabel.
	//TODO: maybe extract even more functions to make the query creation more readable
	source := make(map[string]interface{})

	boolQuery := make(map[string]interface{})
	source["bool"] = boolQuery

	var should []interface{}

	multimatchJson := make(map[string]interface{})
	should = append(should, multimatchJson)

	multimatch := make(map[string]interface{})
	multimatchJson["multi_match"] = multimatch

	multimatch["query"] = q.Term
	multimatch["type"] = "most_fields"

	var multimatchFileds []string
	multimatchFileds = append(multimatchFileds, "prefLabel")
	multimatchFileds = append(multimatchFileds, "aliases")

	multimatch["fields"] = multimatchFileds

	preflabelTermJson := make(map[string]interface{})
	should = append(should, preflabelTermJson)

	preflabelTerm := make(map[string]interface{})
	preflabelTermJson["term"] = preflabelTerm

	preflabelTerm["prefLabel.raw"] = q.Term
	preflabelTerm["boost"] = 2

	aliasesTermJson := make(map[string]interface{})
	should = append(should, aliasesTermJson)

	aliasesTerm := make(map[string]interface{})
	aliasesTermJson["term"] = aliasesTerm

	aliasesTerm["aliases.raw"] = q.Term
	aliasesTerm["boost"] = 2

	boolQuery["should"] = should

	return source, nil
}
