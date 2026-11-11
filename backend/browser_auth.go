package main

import (
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"strings"
)

func normalizedOrigin(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.Contains(u.Host, "*") || u.Opaque != "" {
		return "", fmt.Errorf("invalid browser origin")
	}
	host := strings.ToLower(u.Host)
	if (u.Scheme == "http" && u.Port() == "80") || (u.Scheme == "https" && u.Port() == "443") {
		host = strings.TrimSuffix(host, ":"+u.Port())
	}
	return u.Scheme + "://" + host, nil
}

func allowedBrowserOrigins() (map[string]bool, error) {
	out := map[string]bool{}
	setting := strings.TrimSpace(env("ALLOWED_ORIGINS", ""))
	if setting == "" {
		return out, nil
	}
	for _, raw := range strings.Split(setting, ",") {
		origin, err := normalizedOrigin(strings.TrimSpace(raw))
		if err != nil {
			return nil, fmt.Errorf("invalid ALLOWED_ORIGINS")
		}
		out[origin] = true
	}
	return out, nil
}

func browserOriginAllowed(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == "GET" || r.Method == "HEAD" || r.Header.Get("Origin") == "" {
		return true
	}
	allowed, configErr := allowedBrowserOrigins()
	origin, err := normalizedOrigin(r.Header.Get("Origin"))
	sameHost := false
	if err == nil {
		u, _ := url.Parse(origin)
		expected, hostErr := normalizedOrigin(u.Scheme + "://" + r.Host)
		sameHost = hostErr == nil && expected == origin
	}
	if configErr != nil || err != nil || (!sameHost && !allowed[origin]) {
		http.Error(w, "browser origin denied", http.StatusForbidden)
		return false
	}
	return true
}

// The existing credential pair has one shared administrator scope. Actor
// identity is the verified Basic principal; untrusted X-Auth-User cannot replace it.
func sharedAdminAuth(w http.ResponseWriter, r *http.Request) bool { return changeReadAuth(w, r) }
func sharedAdminMutationAuth(w http.ResponseWriter, r *http.Request) bool {
	if !sharedAdminAuth(w, r) {
		return false
	}
	contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || contentType != "application/json" {
		http.Error(w, "application/json required", 415)
		return false
	}
	return true
}
func trustedBrowserOrigin(r *http.Request) bool {
	origin, err := normalizedOrigin(r.Header.Get("Origin"))
	if err != nil {
		return false
	}
	allowed, err := allowedBrowserOrigins()
	if err != nil {
		return false
	}
	u, _ := url.Parse(origin)
	expected, err := normalizedOrigin(u.Scheme + "://" + r.Host)
	return (err == nil && expected == origin) || allowed[origin]
}

// Имя используется только после проверки доступа; заголовок X-Auth-User
// не может подменить Basic principal или подтверждённую серверную сессию.
type verifiedPrincipalKey struct{}

func verifiedActor(r *http.Request) string {
	if user, _ := r.Context().Value(verifiedPrincipalKey{}).(string); user != "" {
		return user
	}
	user, _, _ := r.BasicAuth()
	return user
}
