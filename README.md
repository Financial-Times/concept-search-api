# Concept Search API

[![Circle CI](https://circleci.com/gh/Financial-Times/concept-search-api/tree/master.png?style=shield)](https://circleci.com/gh/Financial-Times/concept-search-api/tree/master) [![Go Report Card](https://goreportcard.com/badge/github.com/Financial-Times/concept-search-api)](https://goreportcard.com/report/github.com/Financial-Times/concept-search-api) [![Coverage Status](https://coveralls.io/repos/github/Financial-Times/concept-search-api/badge.svg?branch=master)](https://coveralls.io/github/Financial-Times/concept-search-api?branch=master)

API for searching concepts in an Amazon Elasticsearch domain.

:warning: The AWS SDK for Go [does not currently include support for ES data plane api](https://github.com/aws/aws-sdk-go/issues/710), but the Signer is exposed since v1.2.0.

The taken approach to access AES (Amazon Elasticsearch Service):
- Create Transport based on [https://github.com/smartystreets/go-aws-auth](https://github.com/smartystreets/go-aws-auth), using v4 signer [317](
).
- Use https://github.com/olivere/elastic library to any ES request, after passing in the above created client

## How to run

```
go get -u github.com/Financial-Times/concept-search-api
go build
./concept-search-api --aws-access-key="{access key}" --aws-secret-access-key="{secret key}"
```
It is also possible to provide the Elasticsearch endpoint, region, the port you expect the app to run on, the Elasticsearch index on which the search is performed and the maximum number of returned results.

Other parameters:
- elasticsearch-endpoint
- elasticsearch-region
- port (defaults to 8080)
- index-name (defaults to concept)
- elasticsearch-index (defaults to concept)
- search-result-limit (defaults to 50)

## Available DATA endpoints:

### POST /concept/search

The endpoint is used for searching concepts. The payload is a JSON with a field called `term`. The value of this field represents the search criteria. For example searching for _FOO_ looks like this:
```
curl {concept-search-api-url}/concept/search -d '{"term":"FOO"}'
```

The matching concepts are returned ordered by the strength of their match. However the actual score is not included.

To include the score you need to add the query parameter `include_score` with the value `true`. If the parameter has a value other than `true` the score will not be included. The score is a field that appears in each concept alongside the data that represents the actual concept. For example searching for _FOO_ with scoring looks like this:
```
curl {concept-search-api-url}/concept/search?include_score=true -d '{"term":"FOO"}'
```

Exact matches are preferred over partial ones and an example of search results with scoring would look like this:
```
[
  {
    "id": "http://api.ft.com/things/d79f6383-9271-3a03-aacd-5ce8e57d6f5e",
    "apiUrl": "http://api.ft.com/organisations/d79f6383-9271-3a03-aacd-5ce8e57d6f5e",
    "prefLabel": "FOO LLC",
    "types": [
      "http://www.ft.com/ontology/core/Thing",
      "http://www.ft.com/ontology/concept/Concept",
      "http://www.ft.com/ontology/organisation/Organisation"
    ],
    "directType": "http://www.ft.com/ontology/organisation/Organisation",
    "aliases": [
      "FOO LLC",
      "FOO"
    ],
    "score": 10.117536
  },
  {
    "id": "http://api.ft.com/things/87c69c2c-ad53-3888-9958-835098db4dae",
    "apiUrl": "http://api.ft.com/organisations/87c69c2c-ad53-3888-9958-835098db4dae",
    "prefLabel": "FOO International",
    "types": [
      "http://www.ft.com/ontology/core/Thing",
      "http://www.ft.com/ontology/concept/Concept",
      "http://www.ft.com/ontology/organisation/Organisation"
    ],
    "directType": "http://www.ft.com/ontology/organisation/Organisation",
    "aliases": [
      "FOO International",
      "FOO INTERNATIONAL"
    ],
    "score": 2.8585405
  }
]
```
If no results are found a 404 - Not Found response will be returned. In case the payload of the search request does not follow the indicated structure a 400 - Bad request will be returned. If the search fails for various reasons independent from the caller a 500 - Internal Server Error is returned. 


## Available HEALTH endpoints:

### GET /__health

Provides the standard FT output indicating the connectivity and the cluster's health.

### GET /__health-details

Provides a detailed health status of the ES cluster.
It matches the response from [elasticsearch-endpoint/_cluster/health](https://www.elastic.co/guide/en/elasticsearch/reference/current/cluster-health.html)
It returns 503 is the service is currently unavailable, and cannot connect to elasticsearch.

### GET /__gtg

Return 200 if the application is healthy, 503 Service Unavailable if the app is unhealthy.
