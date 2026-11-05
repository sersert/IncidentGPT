package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBrowserSessionLifecycle(t *testing.T) {
	t.Setenv("TIMELINE_READ_USER", "operator")
	t.Setenv("TIMELINE_READ_PASSWORD", "test-password")
	t.Setenv("BROWSER_SESSION_SECURE", "true")
	old := store.disk
	store.disk = nil
	defer func() { store.disk = old }()
	handler := backendHandler()
	call := func(method, body, origin string, cookie *http.Cookie) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, "https://preview.example/api/v1/auth/session", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		if cookie != nil {
			req.AddCookie(cookie)
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		return response
	}
	if got := call("GET", "", "", nil); got.Code != 401 || got.Header().Get("WWW-Authenticate") != "" {
		t.Fatalf("anonymous session: %d", got.Code)
	}
	for _, origin := range []string{"https://evil.example", ""} {
		if got := call("POST", `{"username":"operator","password":"test-password"}`, origin, nil); got.Code != 403 {
			t.Fatalf("cross-origin login: %d", got.Code)
		}
	}
	if got := call("POST", `{"username":"operator","password":"wrong"}`, "https://preview.example", nil); got.Code != 401 {
		t.Fatalf("bad credentials: %d", got.Code)
	}
	login := call("POST", `{"username":"operator","password":"test-password"}`, "https://preview.example", nil)
	if login.Code != 200 {
		t.Fatal(login.Body.String())
	}
	cookie := login.Result().Cookies()[0]
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode || cookie.MaxAge != 28800 || strings.Contains(cookie.Value, "test-password") {
		t.Fatalf("unsafe cookie: %+v", cookie)
	}
	if got := call("GET", "", "", cookie); got.Code != 200 {
		t.Fatalf("session not restored: %d", got.Code)
	}
	// Existing handlers receive a verified Basic principal, preserving their auth and audit checks.
	probe := browserSessionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, password, ok := r.BasicAuth()
		if !ok || user != "operator" || password != "test-password" {
			t.Error("missing verified principal")
		}
		if !changeReadAuth(w, r) {
			t.Error("existing authorization rejected session")
		}
	}))
	request := httptest.NewRequest("GET", "https://preview.example/", nil)
	request.AddCookie(cookie)
	probe.ServeHTTP(httptest.NewRecorder(), request)
	forged := *cookie
	forged.Value = strings.Repeat("0", 64)
	if got := call("GET", "", "", &forged); got.Code != 401 {
		t.Fatalf("forged session: %d", got.Code)
	}
	if got := call("DELETE", "", "https://evil.example", cookie); got.Code != 403 {
		t.Fatalf("cross-origin logout: %d", got.Code)
	}
	if got := call("DELETE", "", "https://preview.example", cookie); got.Code != 204 || got.Result().Cookies()[0].MaxAge != -1 {
		t.Fatalf("logout: %d", got.Code)
	}
	if got := call("GET", "", "", cookie); got.Code != 401 {
		t.Fatalf("revoked cookie accepted: %d", got.Code)
	}
	login = call("POST", `{"username":"operator","password":"test-password"}`, "https://preview.example", nil)
	cookie = login.Result().Cookies()[0]
	browserSessions.Lock()
	browserSessions.entries[sessionKey(cookie.Value)] = browserSession{Credential: credentialFingerprint(), Expires: time.Now().Add(-time.Minute)}
	browserSessions.Unlock()
	if got := call("GET", "", "", cookie); got.Code != 401 {
		t.Fatalf("expired cookie accepted: %d", got.Code)
	}
	login = call("POST", `{"username":"operator","password":"test-password"}`, "https://preview.example", nil)
	cookie = login.Result().Cookies()[0]
	t.Setenv("TIMELINE_READ_PASSWORD", "rotated-password")
	if got := call("GET", "", "", cookie); got.Code != 401 {
		t.Fatalf("password rotation did not revoke cookie: %d", got.Code)
	}
}

func TestBasicAccessDoesNotRestoreBrowserSession(t *testing.T) {
	t.Setenv("TIMELINE_READ_USER", "operator")
	t.Setenv("TIMELINE_READ_PASSWORD", "test-password")
	for _, path := range []string{"session", "check"} {
		request := httptest.NewRequest("GET", "https://preview.example/api/v1/auth/"+path, nil)
		request.SetBasicAuth("operator", "test-password")
		response := httptest.NewRecorder()
		backendHandler().ServeHTTP(response, request)
		expected := 401
		if path == "check" {
			expected = 204
		}
		if response.Code != expected {
			t.Fatalf("%s: %d", path, response.Code)
		}
	}
}
