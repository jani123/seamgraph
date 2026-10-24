package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestReviewWritesRequireConfirmationAndSameOrigin(t *testing.T) {
	app := fixtureApp(t)
	for _, test := range []struct{ token, origin string }{
		{"wrong", ""}, {app.token, "http://untrusted.example"},
	} {
		form := url.Values{"token": {test.token}, "baseRef": {"HEAD"}, "headRef": {"HEAD"}, "file": {"src/npc/main.go"}}
		request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/review", strings.NewReader(form.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if test.origin != "" {
			request.Header.Set("Origin", test.origin)
		}
		response := httptest.NewRecorder()
		app.Handler().ServeHTTP(response, request)
		if response.Code != http.StatusForbidden {
			t.Fatal(response.Code, response.Body.String())
		}
	}
}
