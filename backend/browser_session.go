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

// roleViewer — вторая, необязательная учётная запись (VIEWER_USER/VIEWER_PASSWORD):
// читает всё, что видит administrator, но любые мутации /api/ получают 403.
// Пустая роль — прежний общий administrator.
const roleViewer = "viewer"

type browserSession struct {
	Credential string    `json:"credential"`
	Expires    time.Time `json:"expires"`
	Role       string    `json:"role,omitempty"`
}

var browserSessions = struct {
	sync.Mutex
	entries map[string]browserSession
}{entries: make(map[string]browserSession)}

func browserSessionsEnabled() bool { return env("BROWSER_SESSIONS_ENABLED", "false") == "true" }

// Доказательство зависит от случайного токена: дамп Redis не даёт проверить
// словарный пароль без cookie. Смена credentials отзывает старые сессии.
func credentialProof(token string) string { return credentialProofFor(token, "") }

// Для viewer доказательство строится на его паре: смена пароля viewer отзывает
// только его сессии, administrator-сессии не трогает.
func credentialProofFor(token, role string) string {
	user, password := roleCredentials(role)
	mac := hmac.New(sha256.New, []byte(user+"\x00"+password))
	_, _ = mac.Write([]byte(token))
	return hex.EncodeToString(mac.Sum(nil))
}

func roleCredentials(role string) (string, string) {
	if role == roleViewer {
		user := env("VIEWER_USER", "")
		// Одинаковые имена сделали бы audit неразличимым, а при равных паролях
		// viewer молча стал бы administrator — такой viewer считается выключенным.
		if user == "" || user == env("TIMELINE_READ_USER", "") {
			return "", ""
		}
		return user, env("VIEWER_PASSWORD", "")
	}
	return env("TIMELINE_READ_USER", ""), env("TIMELINE_READ_PASSWORD", "")
}

func viewerMisconfigured() bool {
	user := env("VIEWER_USER", "")
	return user != "" && user == env("TIMELINE_READ_USER", "")
}

// browserCredentialRole сверяет пару с administrator, затем с viewer.
// Viewer включается только когда заданы оба VIEWER_* значения.
func browserCredentialRole(user, password string) (string, bool) {
	for _, role := range []string{"", roleViewer} {
		expectedUser, expectedPassword := roleCredentials(role)
		if expectedUser != "" && expectedPassword != "" && subtle.ConstantTimeCompare([]byte(user), []byte(expectedUser)) == 1 && subtle.ConstantTimeCompare([]byte(password), []byte(expectedPassword)) == 1 {
			return role, true
		}
	}
	return "", false
}

type verifiedRoleKey struct{}

func viewerRequest(r *http.Request) bool {
	role, _ := r.Context().Value(verifiedRoleKey{}).(string)
	return role == roleViewer
}

// Viewer не может ничего менять: любой не-GET запрос к /api/, кроме входа/выхода,
// отклоняется до обработчика. Старые API не проверяют логин сами, поэтому
// граница именно здесь, а не в отдельных handler'ах.
func viewerMutation(r *http.Request) bool {
	if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
		return false
	}
	return strings.HasPrefix(r.URL.Path, "/api/") && r.URL.Path != "/api/v1/auth/session"
}

func withVerifiedRole(r *http.Request, role string) *http.Request {
	user, _ := roleCredentials(role)
	ctx := context.WithValue(r.Context(), verifiedPrincipalKey{}, user)
	return r.WithContext(context.WithValue(ctx, verifiedRoleKey{}, role))
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
func sessionKey(token string) string {
	sum := sha256.Sum256([]byte(token))
	return "igpt:session:" + hex.EncodeToString(sum[:])
}

// loadBrowserSession возвращает роль подтверждённой сессии ("" — administrator).
func loadBrowserSession(r *http.Request) (bool, string, error) {
	if !browserSessionsEnabled() {
		return false, "", nil
	}
	cookie, err := r.Cookie(sessionCookie)
	if err != nil || len(cookie.Value) != 64 {
		return false, "", nil
	}
	key := sessionKey(cookie.Value)
	var session browserSession
	if client := browserSessionRedis(); client != nil {
		data, err := client.Get(r.Context(), key).Bytes()
		if errors.Is(err, redis.Nil) {
			return false, "", nil
		}
		if err != nil {
			return false, "", err
		}
		if json.Unmarshal(data, &session) != nil {
			return false, "", nil
		}
	} else {
		browserSessions.Lock()
		session = browserSessions.entries[key]
		if !session.Expires.After(time.Now()) {
			delete(browserSessions.entries, key)
		}
		browserSessions.Unlock()
	}
	if session.Role != "" && session.Role != roleViewer {
		return false, "", nil
	}
	_, password := roleCredentials(session.Role)
	valid := session.Expires.After(time.Now()) && subtle.ConstantTimeCompare([]byte(session.Credential), []byte(credentialProofFor(cookie.Value, session.Role))) == 1 && password != ""
	if !valid {
		return false, "", nil
	}
	return true, session.Role, nil
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
		valid, role, err := loadBrowserSession(r)
		if err != nil {
			http.Error(w, "session storage unavailable", 503)
			return
		}
		if !valid {
			http.Error(w, "unauthorized", 401)
			return
		}
		user, _ := roleCredentials(role)
		writeJSON(w, 200, map[string]string{"username": user, "role": role})
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
	role, ok := browserCredentialRole(credentials.Username, credentials.Password)
	if !ok {
		http.Error(w, "invalid credentials", 401)
		return
	}
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		http.Error(w, "session unavailable", 503)
		return
	}
	value := hex.EncodeToString(token)
	session := browserSession{Credential: credentialProofFor(value, role), Expires: time.Now().Add(sessionTTL), Role: role}
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
	writeJSON(w, 200, map[string]string{"username": credentials.Username, "role": role})
}

// В контексте хранится только подтверждённое имя и роль, без пароля и нового
// Basic заголовка. Сервисные Bearer/Basic запросы сохраняют прежний контракт;
// Basic с парой viewer получает ту же read-only границу, что и его сессия.
// Cookie viewer понижает роль при любом постороннем Authorization: nginx
// пропускает такой запрос по cookie, и без этого заголовок Bearer обошёл бы 403
// на маршрутах, которые сами логин не проверяют.
func browserSessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, password, hasBasic := r.BasicAuth()
		basicRole, basicValid := browserCredentialRole(user, password)
		if hasBasic && basicValid {
			if basicRole == roleViewer {
				r = withVerifiedRole(r, roleViewer)
			}
		} else if browserSessionsEnabled() {
			ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
			valid, role, err := loadBrowserSession(r.WithContext(ctx))
			cancel()
			if err != nil {
				http.Error(w, "session storage unavailable", 503)
				return
			}
			if valid && role == roleViewer {
				r = withVerifiedRole(r, role)
			} else if valid && r.Header.Get("Authorization") == "" {
				if r.Method != "GET" && r.Method != "HEAD" && !trustedBrowserOrigin(r) {
					http.Error(w, "trusted Origin required", 403)
					return
				}
				r = withVerifiedRole(r, role)
			}
		}
		if viewerRequest(r) && viewerMutation(r) {
			http.Error(w, "viewer role is read-only", 403)
			return
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
	if _, valid := browserCredentialRole(user, password); (ok && valid) || browserPrincipal(r) != "" {
		w.WriteHeader(204)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	valid, _, err := loadBrowserSession(r.WithContext(ctx))
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
