package service

import (
	"net/http"

	log "github.com/Sirupsen/logrus"
	awsauth "github.com/smartystreets/go-aws-auth"
	"gopkg.in/olivere/elastic.v3"
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
	return a.HTTPClient.Do(awsauth.Sign4(req, a.Credentials))
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
