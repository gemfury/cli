package testutil

import (
	"net/http"
	"net/http/httptest"
)

const loginResponse = `{
			"token": "token-abc-123",
			"user": {
				"id": "acct_ace",
				"email": "u@example.com",
				"username": "test-user"
			}
		}`

func APIServer(method, path, resp string, code int) *httptest.Server {
	h := http.NewServeMux()

	h.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			w.WriteHeader(http.StatusNotImplemented)
		}

		w.WriteHeader(code)
		w.Write([]byte(resp))
	})

	h.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusNotImplemented)
		}
		w.Write([]byte(loginResponse))
	})

	return httptest.NewServer(h)
}
