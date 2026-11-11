package main

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
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

func browserSessionsEnabled() bool { return env("BROWSER_SESSIONS_ENABLED", "false") == "true" }

// Доказательство зависит от случайного токена: дамп Redis не даёт проверить
// словарный пароль без cookie. Смена credentials отзывает старые сессии.
func credentialProof(token string) string {
	mac := hmac.New(sha256.New, []byte(env("TIMELINE_READ_USER", "")+"\x00"+env("TIMELINE_READ_PASSWORD", "")))
	_, _ = mac.Write([]byte(token))
	return hex.EncodeToString(mac.Sum(nil))
}

var sessionRedis struct {
	sync.Mutex
	config string
	client *redis.Client
}

// Сессии подключаются независимо от durable-очереди и повторяют соединение
// после позднего старта/рестарта Redis. Настроенный Redis не заменяется памятью.
func browserSessionRedis() *redis.Client {
	if store.disk != nil {
		return store.disk.rdb
	}
	addr := strings.TrimSpace(env("REDIS_ADDR", ""))
	if addr == "" {
		return nil
	}
	password := env("REDIS_PASSWORD", "")
	sessionRedis.Lock()
	defer sessionRedis.Unlock()
	config := addr + "\x00" + password
	if sessionRedis.client == nil || sessionRedis.config != config {
		if sessionRedis.client != nil {
			_ = sessionRedis.client.Close()
		}
		sessionRedis.config = config
		sessionRedis.client = redis.NewClient(&redis.Options{Addr: addr, Password: password, DialTimeout: time.Second, ReadTimeout: time.Second, WriteTimeout: time.Second, ContextTimeoutEnabled: true, MaxRetries: -1})
	}
	return sessionRedis.client
}

func browserPrincipal(r *http.Request) string {
	user, _ := r.Context().Value(verifiedPrincipalKey{}).(string)
	return user
}
func validBrowserCredentials(user, password string) bool {
	expectedUser, expectedPassword := env("TIMELINE_READ_USER", ""), env("TIMELINE_READ_PASSWORD", "")
	return expectedUser != "" && expectedPassword != "" && subtle.ConstantTimeCompare([]byte(user), []byte(expectedUser)) == 1 && subtle.ConstantTimeCompare([]byte(password), []byte(expectedPassword)) == 1
}
func sessionKey(token string) string {
	sum := sha256.Sum256([]byte(token))
	return "igpt:session:" + hex.EncodeToString(sum[:])
}
func loadBrowserSession(r *http.Request) (bool, error) {
	if !browserSessionsEnabled() {
		return false, nil
	}
	cookie, err := r.Cookie(sessionCookie)
	if err != nil || len(cookie.Value) != 64 {
		return false, nil
	}
	key := sessionKey(cookie.Value)
	var session browserSession
	if client := browserSessionRedis(); client != nil {
		data, err := client.Get(r.Context(), key).Bytes()
		if errors.Is(err, redis.Nil) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if json.Unmarshal(data, &session) != nil {
			return false, nil
		}
	} else {
		browserSessions.Lock()
		session = browserSessions.entries[key]
		if !session.Expires.After(time.Now()) {
			delete(browserSessions.entries, key)
		}
		browserSessions.Unlock()
	}
	return session.Expires.After(time.Now()) && subtle.ConstantTimeCompare([]byte(session.Credential), []byte(credentialProof(cookie.Value))) == 1 && env("TIMELINE_READ_PASSWORD", "") != "", nil
}
func setSessionCookie(w http.ResponseWriter, token string, age int) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/", HttpOnly: true, Secure: env("BROWSER_SESSION_SECURE", "true") != "false", SameSite: http.SameSiteStrictMode, MaxAge: age})
}
func handleBrowserSession(w http.ResponseWriter, r *http.Request) {
	if !browserSessionsEnabled() {
		http.NotFound(w, r)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodGet {
		valid, err := loadBrowserSession(r)
		if err != nil {
			http.Error(w, "session storage unavailable", 503)
			return
		}
		if !valid {
			http.Error(w, "unauthorized", 401)
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
			if client := browserSessionRedis(); client != nil {
				if err := client.Del(r.Context(), key).Err(); err != nil {
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
	session := browserSession{Credential: credentialProof(value), Expires: time.Now().Add(sessionTTL)}
	key := sessionKey(value)
	if client := browserSessionRedis(); client != nil {
		data, _ := json.Marshal(session)
		if err := client.Set(r.Context(), key, data, sessionTTL).Err(); err != nil {
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

// В контексте хранится только подтверждённое имя, без пароля и нового Basic
// заголовка. Сервисные Bearer/Basic запросы сохраняют прежний контракт.
func browserSessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if browserSessionsEnabled() && r.Header.Get("Authorization") == "" {
			ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
			valid, err := loadBrowserSession(r.WithContext(ctx))
			cancel()
			if err != nil {
				http.Error(w, "session storage unavailable", 503)
				return
			}
			if valid {
				if r.Method != "GET" && r.Method != "HEAD" && !trustedBrowserOrigin(r) {
					http.Error(w, "trusted Origin required", 403)
					return
				}
				r = r.WithContext(context.WithValue(r.Context(), verifiedPrincipalKey{}, env("TIMELINE_READ_USER", "")))
			}
		}
		next.ServeHTTP(w, r)
	})
}

// Этот маршрут используется внутренним auth_request nginx. Basic API-клиенты
// проверяются отдельно и не восстанавливают браузерную сессию после выхода.
func handleBrowserAccess(w http.ResponseWriter, r *http.Request) {
	if !browserSessionsEnabled() {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	user, password, ok := r.BasicAuth()
	if (ok && validBrowserCredentials(user, password)) || browserPrincipal(r) != "" {
		w.WriteHeader(204)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	valid, err := loadBrowserSession(r.WithContext(ctx))
	if err != nil {
		http.Error(w, "session storage unavailable", 503)
		return
	}
	if valid {
		w.WriteHeader(204)
		return
	}
	http.Error(w, "unauthorized", 401)
}
