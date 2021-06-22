package testutil

import (
	"net/http"
	"net/http/httptest"
)

func APIServer(method, path, resp string, code int) *httptest.Server {
	h := http.NewServeMux()
	h.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			w.WriteHeader(http.StatusNotImplemented)
		}

		w.WriteHeader(code)
		w.Write([]byte(resp))
	})

	return httptest.NewServer(h)
}
