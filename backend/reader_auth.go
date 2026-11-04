package main

import (
	"net/http"
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
