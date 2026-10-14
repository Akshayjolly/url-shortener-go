package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Akshayjolly/url-shortener-go/internal/shortener"
)

// newTestServer runs the real handlers on top of the in-memory store.
func newTestServer() http.Handler {
	return NewServer(shortener.NewService(shortener.NewMemoryStore()), "http://short.test").Routes()
}

func do(h http.Handler, method, target, contentType, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestAPICreateLookupAndRedirect(t *testing.T) {
	h := newTestServer()

	rec := do(h, "POST", "/api/links", "application/json", `{"url":"https://golang.org","code":"golang"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body %s", rec.Code, rec.Body)
	}
	var created map[string]any
	json.Unmarshal(rec.Body.Bytes(), &created)
	if created["shortUrl"] != "http://short.test/golang" {
		t.Errorf("shortUrl = %v", created["shortUrl"])
	}

	rec = do(h, "GET", "/golang", "", "")
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "https://golang.org" {
		t.Fatalf("redirect = %d %q", rec.Code, rec.Header().Get("Location"))
	}

	rec = do(h, "GET", "/api/links/golang", "", "")
	var got map[string]any
	json.Unmarshal(rec.Body.Bytes(), &got)
	if got["clicks"] != float64(1) {
		t.Errorf("clicks = %v, want 1", got["clicks"])
	}
}

func TestAPIErrors(t *testing.T) {
	h := newTestServer()
	if rec := do(h, "POST", "/api/links", "application/json", `not json`); rec.Code != http.StatusBadRequest {
		t.Errorf("bad json status = %d", rec.Code)
	}
	if rec := do(h, "POST", "/api/links", "application/json", `{"url":"ftp://x"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("bad url status = %d", rec.Code)
	}
	do(h, "POST", "/api/links", "application/json", `{"url":"https://a.com","code":"dup"}`)
	if rec := do(h, "POST", "/api/links", "application/json", `{"url":"https://b.com","code":"dup"}`); rec.Code != http.StatusConflict {
		t.Errorf("duplicate status = %d", rec.Code)
	}
	if rec := do(h, "GET", "/nothing-here", "", ""); rec.Code != http.StatusNotFound {
		t.Errorf("missing code status = %d", rec.Code)
	}
}

func TestHTMLFormFlow(t *testing.T) {
	h := newTestServer()
	form := url.Values{"url": {"https://www.griffith.ie"}, "code": {"gcc"}}.Encode()
	rec := do(h, "POST", "/shorten", "application/x-www-form-urlencoded", form)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), "http://short.test/gcc") {
		t.Fatalf("form status = %d", rec.Code)
	}

	home := do(h, "GET", "/", "", "")
	if !strings.Contains(home.Body.String(), "https://www.griffith.ie") {
		t.Error("home page should list the recent link")
	}

	stats := do(h, "GET", "/stats/gcc", "", "")
	if stats.Code != http.StatusOK || !strings.Contains(stats.Body.String(), "Clicks") {
		t.Errorf("stats status = %d", stats.Code)
	}
}

func TestFormShowsValidationError(t *testing.T) {
	h := newTestServer()
	rec := do(h, "POST", "/shorten", "application/x-www-form-urlencoded", "url=")
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "URL must start") {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestTemplatesEscapeHTML(t *testing.T) {
	h := newTestServer()
	do(h, "POST", "/api/links", "application/json", `{"url":"https://evil.com/<script>alert(1)</script>","code":"xss"}`)
	body := do(h, "GET", "/", "", "").Body.String()
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Error("user input must be HTML-escaped")
	}
}

func TestStaticCSS(t *testing.T) {
	if rec := do(newTestServer(), "GET", "/static/style.css", "", ""); rec.Code != http.StatusOK {
		t.Errorf("css status = %d", rec.Code)
	}
}
