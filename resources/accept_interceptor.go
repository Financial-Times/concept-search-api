package resources

import (
	"net/http"
	"strings"
)

func AcceptInterceptor(f http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accept := r.Header.Get("Accept")
		if accept == "" || strings.Contains(accept, "application/json") || strings.Contains(accept, "*/*") {
			f(w, r)
			return
		}
		w.WriteHeader(http.StatusNotAcceptable)
	}
}
