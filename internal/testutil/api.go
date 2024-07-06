package testutil

import (
	"github.com/gemfury/cli/api"
	"github.com/tomnomnom/linkheader"

	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const loginResponse = `{
			"token": "token-abc-123",
			"user": {
				"id": "acct_ace",
				"email": "u@example.com",
				"username": "test-user"
			}
		}`

func APIServer(t *testing.T, method, path, resp string, code int) *httptest.Server {
	return APIServerCustom(t, func(mux *http.ServeMux) {
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			t.Logf("API Request: %s %s", r.Method, r.URL.String())

			if r.Method != method {
				w.WriteHeader(http.StatusNotImplemented)
				return
			}

			w.WriteHeader(code)
			w.Write([]byte(resp))
		})
	})
}

// Allow responses to be paginated forward. Page param is just a string of "p" characters to
// simplify implementation (without parsing), and prevent parsing page number as an integer
func APIServerPaginated(t *testing.T, method, path string, resps []string, code int) *httptest.Server {
	return APIServerCustom(t, func(mux *http.ServeMux) {
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			t.Logf("API Request: %s %s", r.Method, r.URL.String())

			if r.Method != method {
				w.WriteHeader(http.StatusNotImplemented)
				return
			}

			// Page from JSON body or query
			APIPaginatedResponse(t, w, r, resps, code)
		})
	})
}

func APIPaginatedResponse(t *testing.T, w http.ResponseWriter, r *http.Request, resps []string, code int) {
	// Page from JSON body or query
	pageReq := api.PaginationRequest{}
	page := len(r.URL.Query().Get("page"))
	if err := json.NewDecoder(r.Body).Decode(&pageReq); err == nil && pageReq.Page != "" {
		page = len(pageReq.Page)
	}

	// Out of bounds empty response
	if page > len(resps) {
		w.WriteHeader(code)
		w.Write([]byte("[]"))
		return
	}

	// Populate "Link" header
	if page < len(resps)-1 {
		newURL := *r.URL // Copy incoming URL
		newURL.Scheme, newURL.Host = "", ""

		query := newURL.Query()
		query.Set("page", strings.Repeat("p", page+1))
		newURL.RawQuery = query.Encode()
		linkStr := linkheader.Links{
			{URL: newURL.String(), Rel: "next"},
		}.String()

		t.Logf("Next page Link: %s", linkStr)
		w.Header().Set("Link", linkStr)
	}

	w.WriteHeader(code)
	w.Write([]byte(resps[page]))
}

func APIServerCustom(t *testing.T, custom func(*http.ServeMux)) *httptest.Server {
	h := http.NewServeMux()

	// Add custom path handlers
	custom(h)

	// Default handler for browser auth
	h.HandleFunc("/cli/auth", func(w http.ResponseWriter, r *http.Request) {
		if m := r.Method; m == "POST" {
			w.Write([]byte(`{
				  "browser_url": "https://gemfury.com",
				  "cli_url": "/cli/auth?wait=true",
				  "token": "xyz-123"
			  }`))
		} else if m == "GET" {
			if a := r.Header.Get("Authorization"); a != "Bearer xyz-123" {
				t.Errorf("Incorrect Authorization: %q", m)
			}
			w.Write([]byte(`{
				  "user": { "email" : "u@example.com" },
				  "token": "token-abc-123"
			  }`))
		} else {
			t.Errorf("Incorrect method: %q", m)
		}
	})

	// Default handler for interactive auth
	h.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusNotImplemented)
		}
		w.Write([]byte(loginResponse))
	})

	// Check if mux has a handler for "/"
	rootRequest := httptest.NewRequest("GET", "/", nil)
	if _, pattern := h.Handler(rootRequest); pattern == "" {
		h.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			t.Errorf("Unexpected: %s %s", r.Method, r.URL.String())
			http.NotFound(w, r)
		})
	}

	return httptest.NewServer(h)
}
