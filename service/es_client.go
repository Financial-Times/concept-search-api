package service

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws/credentials"
	awsSigner "github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/olivere/elastic/v7"
	log "github.com/sirupsen/logrus"
)

type ESService interface {
	SetElasticClient(*elastic.Client)
}

type awsESAccessConfig struct {
	awsCreds   *credentials.Credentials
	region     string
	esEndpoint string
}

func newAWSAccessConfig(awsCreds *credentials.Credentials, endpoint string, region string) awsESAccessConfig {
	return awsESAccessConfig{awsCreds: awsCreds, esEndpoint: endpoint, region: region}
}

type awsSigningTransport struct {
	HTTPClient  *http.Client
	Credentials *credentials.Credentials
	Region      string
}

// RoundTrip implementation
func (a awsSigningTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clonedRequest := cloneRequest(req)
	signer := awsSigner.NewSigner(a.Credentials)
	if clonedRequest.Body != nil {
		b, err := io.ReadAll(clonedRequest.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read request body with error: %w", err)
		}
		body := strings.NewReader(string(b))
		defer clonedRequest.Body.Close()
		_, err = signer.Sign(clonedRequest, body, "es", a.Region, time.Now())
		if err != nil {
			return nil, fmt.Errorf("failed to sign request: %w", err)
		}
	} else {
		_, err := signer.Sign(clonedRequest, nil, "es", a.Region, time.Now())
		if err != nil {
			return nil, fmt.Errorf("failed to sign request: %w", err)
		}
	}
	return a.HTTPClient.Do(clonedRequest)
}

// cloneRequest returns a clone of the provided *http.Request.
// The clone is a shallow copy of the struct and its Header map.
// Taken from https://github.com/golang/oauth2/blob/master/transport.go
// to comply with the RoundTripper stipulation that "RoundTrip should not modify the request".
func cloneRequest(r *http.Request) *http.Request {
	// shallow copy of the struct
	r2 := new(http.Request)
	*r2 = *r
	r2.Host = r.Host
	// deep copy of the Header
	r2.Header = make(http.Header, len(r.Header))
	for k, s := range r.Header {
		r2.Header[k] = append([]string(nil), s...)
	}
	return r2
}

func NewAWSClient(config awsESAccessConfig, traceLogging bool) (*elastic.Client, error) {
	signingTransport := awsSigningTransport{
		Credentials: config.awsCreds,
		HTTPClient:  http.DefaultClient,
		Region:      config.region,
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

func AWSClientSetup(awsCreds *credentials.Credentials, endpoint string, region string, traceLogging bool, tryEvery time.Duration, services ...ESService) {
	accessConfig := newAWSAccessConfig(awsCreds, endpoint, region)
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
