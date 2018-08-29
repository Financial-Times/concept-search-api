package resources

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/husobee/vestigo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestAcceptNoAcceptHeader(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fontology%2FGenre&q=fast", nil)
	w := httptest.NewRecorder()

	h := new(mockHttpHandler)
	h.On("ServeHTTP", w, req).Return()
	r := vestigo.NewRouter()
	r.Get("/concepts", h.ServeHTTP, AcceptInterceptor)

	r.ServeHTTP(w, req)

	h.AssertExpectations(t)
}

func TestAcceptApplicationJson(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fontology%2FGenre&q=fast", nil)
	req.Header.Add("Accept", "application/json")
	w := httptest.NewRecorder()

	h := new(mockHttpHandler)
	h.On("ServeHTTP", w, req).Return()
	r := vestigo.NewRouter()
	r.Get("/concepts", h.ServeHTTP, AcceptInterceptor)

	r.ServeHTTP(w, req)

	h.AssertExpectations(t)
}

func TestAcceptWildcard(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fontology%2FGenre&q=fast", nil)
	req.Header.Add("Accept", "*/*")
	w := httptest.NewRecorder()

	h := new(mockHttpHandler)
	h.On("ServeHTTP", w, req).Return()
	r := vestigo.NewRouter()
	r.Get("/concepts", h.ServeHTTP, AcceptInterceptor)

	r.ServeHTTP(w, req)

	h.AssertExpectations(t)
}

func TestDoNotAcceptApplicationXml(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fontology%2FGenre&q=fast", nil)
	req.Header.Add("Accept", "application/xml")
	w := httptest.NewRecorder()

	h := new(mockHttpHandler)
	r := vestigo.NewRouter()
	r.Get("/concepts", h.ServeHTTP, AcceptInterceptor)

	r.ServeHTTP(w, req)

	h.AssertNotCalled(t, "ServeHTTP")
	assert.Equal(t, http.StatusNotAcceptable, w.Code, "http status")
}

func TestAcceptMultipleTypesContainingApplicationJson(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fontology%2FGenre&q=fast", nil)
	req.Header.Add("Accept", "application/xml, application/json")
	w := httptest.NewRecorder()

	h := new(mockHttpHandler)
	h.On("ServeHTTP", w, req).Return()
	r := vestigo.NewRouter()
	r.Get("/concepts", h.ServeHTTP, AcceptInterceptor)

	r.ServeHTTP(w, req)

	h.AssertExpectations(t)
}

func TestDoNotAcceptMultipleTypesNotContainingApplicationJson(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fontology%2FGenre&q=fast", nil)
	req.Header.Add("Accept", "application/xml, text/xml")
	w := httptest.NewRecorder()

	h := new(mockHttpHandler)
	r := vestigo.NewRouter()
	r.Get("/concepts", h.ServeHTTP, AcceptInterceptor)

	r.ServeHTTP(w, req)

	h.AssertNotCalled(t, "ServeHTTP")
	assert.Equal(t, http.StatusNotAcceptable, w.Code, "http status")
}

func TestAcceptMultipleTypesContainingWildcard(t *testing.T) {
	req := httptest.NewRequest("GET", "/concepts?type=http%3A%2F%2Fwww.ft.com%2Fontology%2FGenre&q=fast", nil)
	req.Header.Add("Accept", "application/xml, */*")
	w := httptest.NewRecorder()

	h := new(mockHttpHandler)
	h.On("ServeHTTP", w, req).Return()
	r := vestigo.NewRouter()
	r.Get("/concepts", h.ServeHTTP, AcceptInterceptor)

	h.On("ServeHTTP", w, req).Return()
	r.ServeHTTP(w, req)

	h.AssertExpectations(t)
}

type mockHttpHandler struct {
	mock.Mock
}

func (m *mockHttpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.Called(w, r)
}
