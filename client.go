package main

import (
	"context"
	"net"
	"net/http"
	"time"

	awsauth "github.com/smartystreets/go-aws-auth"
	"gopkg.in/olivere/elastic.v5"
)

type esClient interface {
	query(indexName string, query elastic.Query, resultLimit int, minScore float64) (*elastic.SearchResult, error)
	getClusterHealth() (*elastic.ClusterHealthResponse, error)
}

type esClientWrapper struct {
	elasticClient *elastic.Client
}

type awsSigningTransport struct {
	HTTPClient  *http.Client
	Credentials awsauth.Credentials
}

// RoundTrip implementation
func (a awsSigningTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return a.HTTPClient.Do(awsauth.Sign4(req, a.Credentials))
}

func newElasticClient(accessKey string, secretKey string, endpoint *string, region *string) (esClient, error) {

	signingTransport := awsSigningTransport{
		Credentials: awsauth.Credentials{
			AccessKeyID:     accessKey,
			SecretAccessKey: secretKey,
		},
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
			Transport: &http.Transport{
				MaxIdleConnsPerHost: 30,
				Dial: (&net.Dialer{
					KeepAlive: 30 * time.Second,
				}).Dial,
			},
		},
	}
	signingClient := &http.Client{Transport: http.RoundTripper(signingTransport)}

	elasticClient, err := elastic.NewClient(
		elastic.SetURL(*endpoint),
		elastic.SetScheme("https"),
		elastic.SetHttpClient(signingClient),
		elastic.SetSniff(false), //needs to be disabled due to EAS behavior.
		elastic.SetMaxRetries(3),
	)
	return &esClientWrapper{elasticClient: elasticClient}, err
}

func (ec esClientWrapper) query(indexName string, query elastic.Query, resultLimit int, minScore float64) (*elastic.SearchResult, error) {
	q := ec.elasticClient.Search().Index(indexName).Query(query)
	q.MinScore(minScore)
	return q.Size(resultLimit).Do(context.Background())
}

func (ec esClientWrapper) getClusterHealth() (*elastic.ClusterHealthResponse, error) {
	return ec.elasticClient.ClusterHealth().Do(context.Background())
}
