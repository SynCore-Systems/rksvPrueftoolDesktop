package server

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

func newTestServer(t *testing.T) (*Server, http.Handler) {
	t.Helper()
	ui := fstest.MapFS{"upload.html": {Data: []byte("<html>ok</html>")}}
	s, err := New(context.Background(), t.TempDir(), ui)
	if err != nil {
		t.Fatal(err)
	}
	return s, s.Handler()
}

func do(h http.Handler, req *http.Request, token string) *httptest.ResponseRecorder {
	if token != "" {
		req.AddCookie(&http.Cookie{Name: cookieName, Value: token})
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestAuthentication(t *testing.T) {
	s, h := newTestServer(t)

	if rec := do(h, httptest.NewRequest("GET", "/health", nil), ""); rec.Code != 200 {
		t.Errorf("/health without token: %d", rec.Code)
	}
	if rec := do(h, httptest.NewRequest("GET", "/api/status", nil), ""); rec.Code != 403 {
		t.Errorf("/api/status without token: %d, want 403", rec.Code)
	}
	if rec := do(h, httptest.NewRequest("GET", "/api/status", nil), "wrong"); rec.Code != 403 {
		t.Errorf("/api/status with wrong token: %d, want 403", rec.Code)
	}
	if rec := do(h, httptest.NewRequest("GET", "/?t=wrong", nil), ""); rec.Code != 403 {
		t.Errorf("/?t=wrong: %d, want 403", rec.Code)
	}

	rec := do(h, httptest.NewRequest("GET", "/?t="+s.Token(), nil), "")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("/?t=token: %d, want 303", rec.Code)
	}
	cookie := rec.Result().Cookies()[0]
	if cookie.Value != s.Token() || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode {
		t.Errorf("unexpected cookie: %+v", cookie)
	}
	if rec := do(h, httptest.NewRequest("GET", "/", nil), s.Token()); rec.Code != 200 || !strings.Contains(rec.Body.String(), "ok") {
		t.Errorf("/ with cookie: %d %q", rec.Code, rec.Body.String())
	}
}

func TestVerifyRequiresReadyEngine(t *testing.T) {
	s, h := newTestServer(t)
	rec := do(h, httptest.NewRequest("POST", "/api/verify/dep", nil), s.Token())
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("verify before setup: %d, want 503", rec.Code)
	}
}

func TestCleanupRequiresHeader(t *testing.T) {
	s, h := newTestServer(t)
	if rec := do(h, httptest.NewRequest("POST", "/api/cleanup", nil), s.Token()); rec.Code != 403 {
		t.Errorf("cleanup without header: %d, want 403", rec.Code)
	}
	req := httptest.NewRequest("POST", "/api/cleanup", nil)
	req.Header.Set(csrfHeader, csrfValue)
	if rec := do(h, req, s.Token()); rec.Code != 200 {
		t.Errorf("cleanup with header: %d, want 200", rec.Code)
	}
}

func TestEngineImportRejectsWrongChecksum(t *testing.T) {
	s, h := newTestServer(t)
	s.setEngine(stateMissing, "")

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("engineZip", "regkassen-verification-1.1.1.zip")
	fw.Write([]byte("not the official zip"))
	mw.Close()
	req := httptest.NewRequest("POST", "/api/engine/import", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	rec := do(h, req, s.Token())
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "Prüfsumme stimmt nicht") {
		t.Errorf("import of wrong file: %d %s", rec.Code, rec.Body.String())
	}
	if s.engineState != stateError {
		t.Errorf("engine state %q, want %q", s.engineState, stateError)
	}
}

func TestOpenExternalRejectsNonHTTP(t *testing.T) {
	s, h := newTestServer(t)
	for _, body := range []string{`{"url":"file:///C:/Windows/System32/calc.exe"}`, `{"url":"javascript:alert(1)"}`, `nope`} {
		rec := do(h, httptest.NewRequest("POST", "/api/open-external", strings.NewReader(body)), s.Token())
		if rec.Code != http.StatusBadRequest {
			t.Errorf("open-external %s: %d, want 400", body, rec.Code)
		}
	}
}

func TestSplitLines(t *testing.T) {
	cases := map[string][]string{
		"":       nil,
		"a":      {"a"},
		"a\n":    {"a"},
		"a\nb":   {"a", "b"},
		"a\n\nb": {"a", "", "b"},
	}
	for in, want := range cases {
		if got := splitLines(in); !reflect.DeepEqual(got, want) {
			t.Errorf("splitLines(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeTimezone(t *testing.T) {
	if normalizeTimezone("UTC") != "UTC" || normalizeTimezone("Mars/Base") != defaultTimezone || normalizeTimezone("") != defaultTimezone {
		t.Error("normalizeTimezone")
	}
}
