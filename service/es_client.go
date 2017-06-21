package service

import (
	"net/http"
	"time"

	log "github.com/Sirupsen/logrus"
	awsauth "github.com/smartystreets/go-aws-auth"
	"gopkg.in/olivere/elastic.v5"
)

type ESService interface {
	SetElasticClient(*elastic.Client)
}

type awsESAccessConfig struct {
	accessKey  string
	secretKey  string
	esEndpoint string
}

func newAWSAccessConfig(accessKey string, secretKey string, endpoint string) awsESAccessConfig {
	return awsESAccessConfig{accessKey: accessKey, secretKey: secretKey, esEndpoint: endpoint}
}

type awsSigningTransport struct {
	HTTPClient  *http.Client
	Credentials awsauth.Credentials
}

// RoundTrip implementation
func (a awsSigningTransport) RoundTrip(req *http.Request) (*http.Response, error) {
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

func NewAWSClient(config awsESAccessConfig, traceLogging bool) (*elastic.Client, error) {
	signingTransport := awsSigningTransport{
		Credentials: awsauth.Credentials{
			AccessKeyID:     config.accessKey,
			SecretAccessKey: config.secretKey,
		},
		HTTPClient: http.DefaultClient,
	}
	signingClient := &http.Client{Transport: http.RoundTripper(signingTransport)}

	log.Infof("connecting with AWSSigningTransport to %s", config.esEndpoint)
	return newClient(config.esEndpoint, traceLogging,
		elastic.SetScheme("https"),
		elastic.SetHttpClient(signingClient),
	)
}

func NewSimpleClient(endpoint string, traceLogging bool) (*elastic.Client, error) {
	log.Infof("connecting with default transport to %s", endpoint)
	return newClient(endpoint, traceLogging)
}

func newClient(endpoint string, traceLogging bool, options ...elastic.ClientOptionFunc) (*elastic.Client, error) {
	optionFuncs := []elastic.ClientOptionFunc{
		elastic.SetURL(endpoint),
		elastic.SetSniff(false), //needs to be disabled due to EAS behavior. Healthcheck still operates as normal.
	}
	optionFuncs = append(optionFuncs, options...)

	if traceLogging {
		optionFuncs = append(optionFuncs, elastic.SetTraceLog(log.New()))
	}

	return elastic.NewClient(optionFuncs...)
}

func SimpleClientSetup(endpoint string, traceLogging bool, tryEvery time.Duration, services ...ESService) {
	for {
		ec, err := NewSimpleClient(endpoint, traceLogging)
		if err != nil {
			log.WithError(err).Errorf("could not connect to ElasticSearch cluster, retring in %v...", tryEvery)
			time.Sleep(tryEvery)
		} else {
			for _, s := range services {
				s.SetElasticClient(ec)
			}
			return
		}
	}
}

func AWSClientSetup(accessKey string, secretKey string, endpoint string, traceLogging bool, tryEvery time.Duration, services ...ESService) {
	accessConfig := newAWSAccessConfig(accessKey, secretKey, endpoint)
	for {
		ec, err := NewAWSClient(accessConfig, traceLogging)
		if err != nil {
			log.WithError(err).Errorf("could not connect to AWS ElasticSearch cluster, retring in %v...", tryEvery)
			time.Sleep(tryEvery)
		} else {
			for _, s := range services {
				s.SetElasticClient(ec)
			}
			return
		}
	}
}
