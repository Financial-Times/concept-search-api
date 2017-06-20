package resources

import (
	"net/http"
	"strings"
)

type AcceptInterceptor struct {
}

func (i *AcceptInterceptor) Before() bool {
	return true
}

func (i *AcceptInterceptor) After() bool {
	return false
}

func (i *AcceptInterceptor) Intercept(w http.ResponseWriter, r *http.Request) bool {
	accept := r.Header.Get("Accept")

	if accept == "" || strings.Contains(accept, "application/json") || strings.Contains(accept, "*/*") {
		return true
	}

	w.WriteHeader(http.StatusNotAcceptable)
	return false
}
