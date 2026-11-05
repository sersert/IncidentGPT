package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"
)

const sessionCookie = "incidentgpt_session"
const sessionTTL = 8 * time.Hour

type browserSession struct {
	Credential string    `json:"credential"`
	Expires    time.Time `json:"expires"`
}

var browserSessions = struct {
	sync.Mutex
	entries map[string]browserSession
}{entries: make(map[string]browserSession)}

func credentialFingerprint() string {
	sum := sha256.Sum256([]byte(env("TIMELINE_READ_USER", "") + "\x00" + env("TIMELINE_READ_PASSWORD", "")))
	return hex.EncodeToString(sum[:])
}
func validBrowserCredentials(user, password string) bool {
	expectedUser, expectedPassword := env("TIMELINE_READ_USER", ""), env("TIMELINE_READ_PASSWORD", "")
	return expectedUser != "" && expectedPassword != "" && subtle.ConstantTimeCompare([]byte(user), []byte(expectedUser)) == 1 && subtle.ConstantTimeCompare([]byte(password), []byte(expectedPassword)) == 1
}
func sessionKey(token string) string {
	sum := sha256.Sum256([]byte(token))
	return "browser-session:" + hex.EncodeToString(sum[:])
}
func loadBrowserSession(r *http.Request) bool {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil || len(cookie.Value) != 64 {
		return false
	}
	key := sessionKey(cookie.Value)
	var session browserSession
	if store.disk != nil {
		data, err := store.disk.rdb.Get(r.Context(), key).Bytes()
		if err != nil || json.Unmarshal(data, &session) != nil {
			return false
		}
	} else {
		browserSessions.Lock()
		session = browserSessions.entries[key]
		if !session.Expires.After(time.Now()) {
			delete(browserSessions.entries, key)
		}
		browserSessions.Unlock()
	}
	return session.Expires.After(time.Now()) && session.Credential == credentialFingerprint() && env("TIMELINE_READ_PASSWORD", "") != ""
}
func setSessionCookie(w http.ResponseWriter, token string, age int) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/", HttpOnly: true, Secure: env("BROWSER_SESSION_SECURE", "true") != "false", SameSite: http.SameSiteStrictMode, MaxAge: age})
}
func handleBrowserSession(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodGet {
		if !loadBrowserSession(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		writeJSON(w, 200, map[string]string{"username": env("TIMELINE_READ_USER", "")})
		return
	}
	if !trustedBrowserOrigin(r) {
		http.Error(w, "browser origin denied", 403)
		return
	}
	if r.Method == http.MethodDelete {
		if cookie, err := r.Cookie(sessionCookie); err == nil {
			key := sessionKey(cookie.Value)
			if store.disk != nil {
				if err := store.disk.rdb.Del(r.Context(), key).Err(); err != nil {
					http.Error(w, "session storage unavailable", 503)
					return
				}
			} else {
				browserSessions.Lock()
				delete(browserSessions.entries, key)
				browserSessions.Unlock()
			}
		}
		setSessionCookie(w, "", -1)
		w.WriteHeader(204)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST, DELETE")
		w.WriteHeader(405)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var credentials struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") || json.NewDecoder(r.Body).Decode(&credentials) != nil {
		http.Error(w, "invalid credentials payload", 400)
		return
	}
	if !validBrowserCredentials(credentials.Username, credentials.Password) {
		http.Error(w, "invalid credentials", 401)
		return
	}
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		http.Error(w, "session unavailable", 503)
		return
	}
	value := hex.EncodeToString(token)
	session := browserSession{Credential: credentialFingerprint(), Expires: time.Now().Add(sessionTTL)}
	key := sessionKey(value)
	if store.disk != nil {
		data, _ := json.Marshal(session)
		if err := store.disk.rdb.Set(r.Context(), key, data, sessionTTL).Err(); err != nil {
			http.Error(w, "session storage unavailable", 503)
			return
		}
	} else {
		browserSessions.Lock()
		for k, v := range browserSessions.entries {
			if !v.Expires.After(time.Now()) {
				delete(browserSessions.entries, k)
			}
		}
		if len(browserSessions.entries) >= 4096 {
			browserSessions.Unlock()
			http.Error(w, "session capacity reached", 503)
			return
		}
		browserSessions.entries[key] = session
		browserSessions.Unlock()
	}
	setSessionCookie(w, value, int(sessionTTL.Seconds()))
	writeJSON(w, 200, map[string]string{"username": credentials.Username})
}

// Restore the verified principal for existing independently authenticated handlers.
// Service Bearer/Basic credentials keep their original behavior.
func browserSessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" && loadBrowserSession(r) {
			r.SetBasicAuth(env("TIMELINE_READ_USER", ""), env("TIMELINE_READ_PASSWORD", ""))
		}
		next.ServeHTTP(w, r)
	})
}

// Used only by nginx's internal auth_request. Basic clients remain supported
// without making the browser session endpoint revive cached Basic credentials.
func handleBrowserAccess(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	user, password, ok := r.BasicAuth()
	if loadBrowserSession(r) || (ok && validBrowserCredentials(user, password)) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Error(w, "unauthorized", http.StatusUnauthorized)
}
