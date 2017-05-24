package service

import (
	"net/http"

	log "github.com/Sirupsen/logrus"
	awsauth "github.com/smartystreets/go-aws-auth"
	"gopkg.in/olivere/elastic.v5"
)

type EsAccessConfig struct {
	accessKey  string
	secretKey  string
	esEndpoint string
}

func NewAccessConfig(accessKey string, secretKey string, endpoint string) EsAccessConfig {
	return EsAccessConfig{accessKey: accessKey, secretKey: secretKey, esEndpoint: endpoint}
}

type AWSSigningTransport struct {
	HTTPClient  *http.Client
	Credentials awsauth.Credentials
}

// RoundTrip implementation
func (a AWSSigningTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return a.HTTPClient.Do(awsauth.Sign4(cloneRequest(req), a.Credentials))
}

// cloneRequest returns a clone of the provided *http.Request.
// The clone is a shallow copy of the struct and its Header map.
// Taken from https://github.com/golang/oauth2/blob/master/transport.go
// to comply with the RoundTripper stipulation that "RoundTrip should not modify the request".
func cloneRequest(r *http.Request) *http.Request {
	// shallow copy of the struct
	r2 := new(http.Request)
	*r2 = *r
	// deep copy of the Header
	r2.Header = make(http.Header, len(r.Header))
	for k, s := range r.Header {
		r2.Header[k] = append([]string(nil), s...)
	}
	return r2
}

func newAmazonClient(config EsAccessConfig) (*elastic.Client, error) {

	signingTransport := AWSSigningTransport{
		Credentials: awsauth.Credentials{
			AccessKeyID:     config.accessKey,
			SecretAccessKey: config.secretKey,
		},
		HTTPClient: http.DefaultClient,
	}
	signingClient := &http.Client{Transport: http.RoundTripper(signingTransport)}

	log.Infof("connecting with AWSSigningTransport to %s", config.esEndpoint)
	return elastic.NewClient(
		elastic.SetURL(config.esEndpoint),
		elastic.SetScheme("https"),
		elastic.SetHttpClient(signingClient),
		elastic.SetSniff(false), //needs to be disabled due to EAS behavior. Healthcheck still operates as normal.
	)
}

func newSimpleClient(config EsAccessConfig) (*elastic.Client, error) {
	log.Infof("connecting with default transport to %s", config.esEndpoint)
	return elastic.NewClient(
		elastic.SetURL(config.esEndpoint),
		elastic.SetSniff(false),
	)
}

func NewElasticClient(region string, config EsAccessConfig) (*elastic.Client, error) {
	if region == "local" {
		return newSimpleClient(config)
	} else {
		return newAmazonClient(config)
	}
}
