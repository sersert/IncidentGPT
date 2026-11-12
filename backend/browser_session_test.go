package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBrowserSessionLifecycle(t *testing.T) {
	t.Setenv("BROWSER_SESSIONS_ENABLED", "true")
	t.Setenv("REDIS_ADDR", "")
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
		if r.Header.Get("Authorization") != "" || browserPrincipal(r) != "operator" || verifiedActor(r) != "operator" {
			t.Error("session leaked credentials or lost verified actor")
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
	browserSessions.entries[sessionKey(cookie.Value)] = browserSession{Credential: credentialProof(cookie.Value), Expires: time.Now().Add(-time.Minute)}
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
	t.Setenv("BROWSER_SESSIONS_ENABLED", "true")
	t.Setenv("REDIS_ADDR", "")
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

func TestBrowserSessionsAreOptIn(t *testing.T) {
	t.Setenv("BROWSER_SESSIONS_ENABLED", "false")
	for _, path := range []string{"session", "check"} {
		response := httptest.NewRecorder()
		backendHandler().ServeHTTP(response, httptest.NewRequest("GET", "https://preview.example/api/v1/auth/"+path, nil))
		if response.Code != 404 {
			t.Fatalf("%s enabled by default: %d", path, response.Code)
		}
	}
}
func TestConfiguredUnavailableRedisCannotUseMemorySessions(t *testing.T) {
	t.Setenv("BROWSER_SESSIONS_ENABLED", "true")
	t.Setenv("REDIS_ADDR", "127.0.0.1:1")
	t.Setenv("TIMELINE_READ_USER", "operator")
	t.Setenv("TIMELINE_READ_PASSWORD", "test-password")
	old := store.disk
	store.disk = nil
	defer func() { store.disk = old }()
	request := httptest.NewRequest("POST", "https://preview.example/api/v1/auth/session", strings.NewReader(`{"username":"operator","password":"test-password"}`))
	request.Header.Set("Origin", "https://preview.example")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	backendHandler().ServeHTTP(response, request)
	if response.Code != 503 || len(response.Result().Cookies()) != 0 {
		t.Fatalf("Redis failure fell back to local sessions: %d", response.Code)
	}
}
func TestViewerRoleIsReadOnly(t *testing.T) {
	t.Setenv("BROWSER_SESSIONS_ENABLED", "true")
	t.Setenv("REDIS_ADDR", "")
	t.Setenv("TIMELINE_READ_USER", "operator")
	t.Setenv("TIMELINE_READ_PASSWORD", "test-password")
	t.Setenv("VIEWER_USER", "view")
	t.Setenv("VIEWER_PASSWORD", "view-password")
	old := store.disk
	store.disk = nil
	defer func() { store.disk = old }()
	handler := backendHandler()
	call := func(method, path, body string, cookie *http.Cookie, basic [2]string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, "https://preview.example"+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "https://preview.example")
		if cookie != nil {
			req.AddCookie(cookie)
		}
		if basic[0] != "" {
			req.SetBasicAuth(basic[0], basic[1])
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		return response
	}
	none := [2]string{}
	login := call("POST", "/api/v1/auth/session", `{"username":"view","password":"view-password"}`, nil, none)
	if login.Code != 200 || !strings.Contains(login.Body.String(), `"role":"viewer"`) {
		t.Fatalf("viewer login: %d %s", login.Code, login.Body.String())
	}
	viewer := login.Result().Cookies()[0]
	if got := call("GET", "/api/v1/auth/session", "", viewer, none); got.Code != 200 || !strings.Contains(got.Body.String(), `"username":"view"`) {
		t.Fatalf("viewer session: %d %s", got.Code, got.Body.String())
	}
	if got := call("GET", "/api/v1/auth/check", "", viewer, none); got.Code != 204 {
		t.Fatalf("viewer access check: %d", got.Code)
	}
	if got := call("GET", "/api/v1/dashboard/stats", "", viewer, none); got.Code != 200 {
		t.Fatalf("viewer read: %d", got.Code)
	}
	for _, mutation := range [][2]string{{"PATCH", "/api/v1/incidents/INC-1"}, {"PUT", "/api/v1/settings/llm"}, {"POST", "/api/v1/incidents/INC-1/investigations"}, {"PUT", "/api/v1/operations/ai-control"}} {
		if got := call(mutation[0], mutation[1], `{}`, viewer, none); got.Code != 403 {
			t.Fatalf("viewer session %s %s: %d", mutation[0], mutation[1], got.Code)
		}
		if got := call(mutation[0], mutation[1], `{}`, nil, [2]string{"view", "view-password"}); got.Code != 403 {
			t.Fatalf("viewer basic %s %s: %d", mutation[0], mutation[1], got.Code)
		}
	}
	if got := call("GET", "/api/v1/auth/check", "", nil, [2]string{"view", "view-password"}); got.Code != 204 {
		t.Fatalf("viewer basic access check: %d", got.Code)
	}
	probe := browserSessionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if verifiedActor(r) != "view" || !viewerRequest(r) || !changeReadAuth(w, r) {
			t.Error("viewer principal or read access lost")
		}
	}))
	request := httptest.NewRequest("GET", "https://preview.example/api/v1/incidents/INC-1/analyses", nil)
	request.AddCookie(viewer)
	probe.ServeHTTP(httptest.NewRecorder(), request)
	// Administrator keeps mutations; viewer rotation revokes only viewer sessions.
	admin := call("POST", "/api/v1/auth/session", `{"username":"operator","password":"test-password"}`, nil, none).Result().Cookies()[0]
	if got := call("PATCH", "/api/v1/incidents/INC-1", `{}`, admin, none); got.Code == 403 {
		t.Fatalf("administrator session blocked: %d", got.Code)
	}
	if got := call("DELETE", "/api/v1/auth/session", "", viewer, none); got.Code != 204 {
		t.Fatalf("viewer logout: %d", got.Code)
	}
	viewer = call("POST", "/api/v1/auth/session", `{"username":"view","password":"view-password"}`, nil, none).Result().Cookies()[0]
	t.Setenv("VIEWER_PASSWORD", "rotated")
	if got := call("GET", "/api/v1/auth/session", "", viewer, none); got.Code != 401 {
		t.Fatalf("viewer rotation did not revoke cookie: %d", got.Code)
	}
	if got := call("GET", "/api/v1/auth/session", "", admin, none); got.Code != 200 {
		t.Fatalf("viewer rotation revoked administrator: %d", got.Code)
	}
	t.Setenv("VIEWER_PASSWORD", "")
	if got := call("POST", "/api/v1/auth/session", `{"username":"view","password":"view-password"}`, nil, none); got.Code != 401 {
		t.Fatalf("viewer without password accepted: %d", got.Code)
	}
}

func TestSessionProofDependsOnCookieAndCredentialRotation(t *testing.T) {
	t.Setenv("TIMELINE_READ_USER", "operator")
	t.Setenv("TIMELINE_READ_PASSWORD", "test-password")
	first := credentialProof("first-random-cookie")
	second := credentialProof("second-random-cookie")
	if first == second {
		t.Fatal("Redis exposes a reusable password verifier")
	}
	t.Setenv("TIMELINE_READ_PASSWORD", "rotated-password")
	if first == credentialProof("first-random-cookie") {
		t.Fatal("rotation did not invalidate proof")
	}
}
