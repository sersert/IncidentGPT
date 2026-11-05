package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func TestOperationsRequiresAuthAndActuallyChecksKubernetes(t *testing.T) {
	oldCfg, oldKube := appCfg, k8sClient
	defer func() { appCfg = oldCfg; k8sClient = oldKube }()
	t.Setenv("OPERATIONS_ENABLED", "true")
	t.Setenv("OPERATIONS_SERVICE_TOKEN", strings.Repeat("t", 32))
	var calls atomic.Int32
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path == "/version" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"gitVersion":"v1.30.0"}`))
			return
		}
		http.Error(w, "metrics unavailable", 503)
	}))
	defer api.Close()
	appCfg.EnableK8sContext = true
	appCfg.PromURL = api.URL
	appCfg.RedisAddr = ""
	var err error
	k8sClient, err = kubernetes.NewForConfig(&rest.Config{Host: api.URL, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	request := func(auth bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/internal/operations", nil)
		if auth {
			r.Header.Set("Authorization", "Bearer "+strings.Repeat("t", 32))
		}
		w := httptest.NewRecorder()
		enricherOperations(w, r)
		return w
	}
	if w := request(false); w.Code != 401 || calls.Load() != 0 {
		t.Fatal("unauthorized request performed network probes")
	}
	w := request(true)
	var data struct {
		Checks []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"checks"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &data) != nil {
		t.Fatal("invalid operations response")
	}
	states := map[string]string{}
	for _, check := range data.Checks {
		states[check.ID] = check.Status
	}
	if states["kubernetes"] != "healthy" || states["metrics"] != "unhealthy" || states["enricher_redis"] != "not_configured" || calls.Load() != 2 {
		t.Fatalf("real API checks missing: %v calls=%d", states, calls.Load())
	}
}
