// Крошечный healthcheck-пробник для distroless-образа (внутри нет shell/wget).
// Docker HEALTHCHECK запускает его: exit 0 — healthy, любой другой код — unhealthy.
// URL берётся из os.Args[1], иначе из HEALTHCHECK_URL, иначе дефолт.
package main

import (
	"net/http"
	"os"
	"time"
)

func main() {
	url := "http://127.0.0.1:9099/healthz"
	if len(os.Args) > 1 && os.Args[1] != "" {
		url = os.Args[1]
	} else if env := os.Getenv("HEALTHCHECK_URL"); env != "" {
		url = env
	}

	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		os.Exit(1)
	}
}
