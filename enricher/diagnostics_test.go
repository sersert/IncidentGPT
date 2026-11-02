package main

import (
	"context"
	"encoding/json"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDiagnosticAllowlistAndPartialSources(t *testing.T) {
	old := appCfg
	defer func() { appCfg = old }()
	appCfg.ClusterName = "prod"
	t.Setenv("DIAGNOSTIC_NAMESPACES", "payments")
	now := time.Now().UTC()
	input := DiagnosticRequest{Version: 1, IncidentID: "INC-1", OperationID: "job-1", Scope: DiagnosticScope{ClusterID: "prod", Namespace: "payments", Pod: "api-1", From: now.Add(-time.Minute), To: now}, Checks: []string{"pod_termination", "node_conditions"}}
	if err := validateDiagnosticRequest(input); err != nil {
		t.Fatal(err)
	}
	input.Checks = []string{"kubectl_exec"}
	if validateDiagnosticRequest(input) == nil {
		t.Fatal("terminal operation accepted")
	}
	input.Checks = []string{"pod_termination"}
	input.Scope.ClusterID = "other"
	if validateDiagnosticRequest(input) == nil {
		t.Fatal("foreign cluster accepted")
	}
	input.Scope.ClusterID = "prod"
	input.Scope.Namespace = "unrelated"
	if validateDiagnosticRequest(input) == nil {
		t.Fatal("unpermitted namespace accepted")
	}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("query") != "kube_pod_container_status_restarts_total{namespace=\"payments\",pod=\"api-1\"}" {
			t.Error("unrestricted query emitted")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": map[string]any{"result": []any{}}})
	}))
	defer provider.Close()
	appCfg.PromURL = provider.URL
	scope := DiagnosticScope{Namespace: "payments", Pod: "api-1", From: now.Add(-time.Minute), To: now}
	_, status, _ := collectTypedDiagnostic(context.Background(), "pod_restarts", scope)
	if status != "empty" {
		t.Fatalf("successful empty query: %s", status)
	}
	provider.Close()
	_, status, _ = collectTypedDiagnostic(context.Background(), "pod_restarts", scope)
	if status != "failed" {
		t.Fatal("outage represented as empty")
	}
	oldLogs := logsCfg
	defer func() { logsCfg = oldLogs }()
	logsCfg.enabled = false
	_, status, _ = collectTypedDiagnostic(context.Background(), "logs", scope)
	if status != "disabled" {
		t.Fatal("disabled source represented as empty")
	}
}

func TestDiagnosticPodEvidenceIsReadOnly(t *testing.T) {
	oldCfg, oldKube := appCfg, k8sClient
	defer func() { appCfg = oldCfg; k8sClient = oldKube }()
	kube := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/api/v1/namespaces/payments/pods/api-1" {
			t.Errorf("unallowed Kubernetes operation %s %s", r.Method, r.URL.Path)
			http.Error(w, "unsupported", 400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"api-1","namespace":"payments"},"status":{"containerStatuses":[{"name":"api","ready":false,"restartCount":4,"lastState":{"terminated":{"reason":"OOMKilled","exitCode":137,"finishedAt":"2026-10-08T09:00:00Z"}}}]}}`))
	}))
	defer kube.Close()
	var err error
	k8sClient, err = kubernetes.NewForConfig(&rest.Config{Host: kube.URL})
	if err != nil {
		t.Fatal(err)
	}
	data, status, _ := collectTypedDiagnostic(context.Background(), "pod_termination", DiagnosticScope{Namespace: "payments", Pod: "api-1"})
	raw, _ := json.Marshal(data)
	if status != "available" || !strings.Contains(string(raw), "OOMKilled") || strings.Contains(string(raw), "env") {
		t.Fatalf("termination evidence: %s %s", status, raw)
	}
}
