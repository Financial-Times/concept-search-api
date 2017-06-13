package resources

import (
	"net/http"
	"net/http/httptest"

	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInterceptorLifecycle(t *testing.T) {
	i := AcceptInterceptor{}
	assert.True(t, i.Before(), "interceptor should run before request is handled")
	assert.False(t, i.After(), "interceptor should not run after request is handled")
}

func TestAcceptNoAcceptHeader(t *testing.T) {
	i := AcceptInterceptor{}

	req := httptest.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fontology%2FGenre&q=fast", nil)
	w := httptest.NewRecorder()

	actual := i.Intercept(w, req)

	assert.True(t, actual, "request should be permitted")
}

func TestAcceptApplicationJson(t *testing.T) {
	i := AcceptInterceptor{}

	req := httptest.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fontology%2FGenre&q=fast", nil)
	req.Header.Add("Accept", "application/json")
	w := httptest.NewRecorder()

	actual := i.Intercept(w, req)

	assert.True(t, actual, "request should be permitted")
}

func TestAcceptWilcard(t *testing.T) {
	i := AcceptInterceptor{}

	req := httptest.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fontology%2FGenre&q=fast", nil)
	req.Header.Add("Accept", "*/*")
	w := httptest.NewRecorder()

	actual := i.Intercept(w, req)

	assert.True(t, actual, "request should be permitted")
}

func TestDoNotAcceptApplicationXml(t *testing.T) {
	i := AcceptInterceptor{}

	req := httptest.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fontology%2FGenre&q=fast", nil)
	req.Header.Add("Accept", "application/xml")
	w := httptest.NewRecorder()

	actual := i.Intercept(w, req)

	assert.False(t, actual, "request should not be permitted")
	assert.Equal(t, http.StatusNotAcceptable, w.Code, "http status")
}

func TestAcceptMultipleTypesContainingApplicationJson(t *testing.T) {
	i := AcceptInterceptor{}

	req := httptest.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fontology%2FGenre&q=fast", nil)
	req.Header.Add("Accept", "application/xml, application/json")
	w := httptest.NewRecorder()

	actual := i.Intercept(w, req)

	assert.True(t, actual, "request should be permitted")
}

func TestDoNotAcceptMultipleTypesNotContainingApplicationJson(t *testing.T) {
	i := AcceptInterceptor{}

	req := httptest.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fontology%2FGenre&q=fast", nil)
	req.Header.Add("Accept", "application/xml, text/xml")
	w := httptest.NewRecorder()

	actual := i.Intercept(w, req)

	assert.False(t, actual, "request should not be permitted")
	assert.Equal(t, http.StatusNotAcceptable, w.Code, "http status")
}

func TestAcceptMultipleTypesContainingWildcard(t *testing.T) {
	i := AcceptInterceptor{}

	req := httptest.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fontology%2FGenre&q=fast", nil)
	req.Header.Add("Accept", "application/xml, */*")
	w := httptest.NewRecorder()

	actual := i.Intercept(w, req)

	assert.True(t, actual, "request should be permitted")
}
