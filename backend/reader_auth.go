package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Existing APIs retain their trusted UI proxy / NetworkPolicy boundary by
// default. Enabling independent backend authentication must be explicit and
// fails closed when credentials are missing. New privileged APIs always use
// changeReadAuth, regardless of this compatibility setting.
func legacyAPIAuth(w http.ResponseWriter, r *http.Request) bool {
	if env("LEGACY_API_AUTH_REQUIRED", "false") != "false" {
		return changeReadAuth(w, r)
	}
	return browserOriginAllowed(w, r)
}

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
