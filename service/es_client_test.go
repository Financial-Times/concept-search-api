package service

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	elastic "gopkg.in/olivere/elastic.v5"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type ESServiceMock struct {
	mock.Mock
}

func (m *ESServiceMock) SetElasticClient(client *elastic.Client) {
	m.Called(client)
}

func TestHappyAWSClientSetup(t *testing.T) {
	esInternalServices := newESServiceMock(3)
	es := newHappyAWSESMock(t)
	defer es.Close()
	go AWSClientSetup("a-key", "a-secret", es.URL, false, time.Second, esInternalServices[0], esInternalServices[1], esInternalServices[2])
	time.Sleep(100 * time.Millisecond)
	for _, s := range esInternalServices {
		s.AssertExpectations(t)
	}
}

func TestUnhappyAWSClientSetup(t *testing.T) {
	if testing.Short() {
		t.Skip("This test takes several seconds")
	}
	esInternalServices := newESServiceMock(3)
	es := newUnhappyAWSESMockForNAttempts(t, 10)
	defer es.Close()
	go AWSClientSetup("a-key", "a-secret", es.URL, true, time.Second, esInternalServices[0], esInternalServices[1], esInternalServices[2])
	for i := 0; i < 12; i++ { // NB elastic.Client retries by default 5 times every second by itself.
		for _, s := range esInternalServices {
			s.AssertNotCalled(t, "SetElasticClient", mock.AnythingOfType("*elastic.Client"))
		}
		time.Sleep(time.Second)
	}
	time.Sleep(100 * time.Millisecond)
	for _, s := range esInternalServices {
		s.AssertExpectations(t)
	}
}

func TestHappySimpleClientSetup(t *testing.T) {
	esInternalServices := newESServiceMock(3)
	es := newHappySimpleESMock(t)
	defer es.Close()
	go SimpleClientSetup(es.URL, true, time.Second, esInternalServices[0], esInternalServices[1], esInternalServices[2])
	time.Sleep(100 * time.Millisecond)
	for _, s := range esInternalServices {
		s.AssertExpectations(t)
	}
}

func TestUnhappySimpleClientSetup(t *testing.T) {
	if testing.Short() {
		t.Skip("This test takes several seconds")
	}
	esInternalServices := newESServiceMock(3)
	es := newUnhappySimpleESMockForNAttempts(t, 10)
	defer es.Close()
	go SimpleClientSetup(es.URL, false, time.Second, esInternalServices[0], esInternalServices[1], esInternalServices[2])
	for i := 0; i < 12; i++ { // NB elastic.Client retries by default 5 times every second by itself.
		for _, s := range esInternalServices {
			s.AssertNotCalled(t, "SetElasticClient", mock.AnythingOfType("*elastic.Client"))
		}
		time.Sleep(time.Second)
	}
	time.Sleep(100 * time.Millisecond)
	for _, s := range esInternalServices {
		s.AssertExpectations(t)
	}
}

func newESServiceMock(n int) []*ESServiceMock {
	services := make([]*ESServiceMock, n)
	for i := 0; i < n; i++ {
		s := new(ESServiceMock)
		s.On("SetElasticClient", mock.AnythingOfType("*elastic.Client"))
		services[i] = s
	}
	return services
}

func newHappyAWSESMock(t *testing.T) *httptest.Server {
	handlerFunc := func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "I am working!")
		assert.Contains(t, r.Header.Get("authorization"), "AWS4-HMAC-SHA256 Credential", "It is using AWS authentication")
	}
	h := &TestHTTPHandler{0, handlerFunc}

	ts := httptest.NewServer(h)
	return ts
}

func newUnhappyAWSESMockForNAttempts(t *testing.T, attempts int) *httptest.Server {
	handlerFunc := func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "I am working!")
		assert.Contains(t, r.Header.Get("authorization"), "AWS4-HMAC-SHA256 Credential", "It is using AWS authentication")
	}
	h := &TestHTTPHandler{attempts, handlerFunc}

	ts := httptest.NewServer(h)
	return ts
}

func newHappySimpleESMock(t *testing.T) *httptest.Server {
	handlerFunc := func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "I am working!")
		assert.Empty(t, r.Header.Get("authorization"), "It is not using authentication")
	}
	h := &TestHTTPHandler{0, handlerFunc}

	ts := httptest.NewServer(h)
	return ts
}

func newUnhappySimpleESMockForNAttempts(t *testing.T, attempts int) *httptest.Server {
	handlerFunc := func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "I am working!")
		assert.Empty(t, r.Header.Get("authorization"), "It is not using authentication")
	}
	h := &TestHTTPHandler{attempts, handlerFunc}

	ts := httptest.NewServer(h)
	return ts
}

type TestHTTPHandler struct {
	attempts    int
	handlerFunc func(w http.ResponseWriter, r *http.Request)
}

func (h *TestHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.attempts == 0 {
		h.handlerFunc(w, r)
	} else {
		h.attempts--
		http.Error(w, "I don't want to work!", http.StatusInternalServerError)
	}
}
