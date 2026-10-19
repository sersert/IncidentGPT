package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
)

var diagnosticSlots = make(chan struct{}, 4)
var diagnosticChecks = map[string]bool{"pod_termination": true, "pod_events": true, "workload_rollout": true, "node_conditions": true, "pod_restarts": true, "logs": true}

func validateDiagnosticRequest(r DiagnosticRequest) error {
	if r.Version != 1 || r.IncidentID == "" || r.OperationID == "" || r.Scope.ClusterID != envOr("CLUSTER_ID", appCfg.ClusterName) || r.Scope.Namespace == "" || len(r.Checks) == 0 || len(r.Checks) > 6 || r.Scope.To.Before(r.Scope.From) || r.Scope.To.Sub(r.Scope.From) > time.Hour || r.Scope.To.After(time.Now().Add(time.Minute)) || r.Scope.From.Before(time.Now().Add(-7*24*time.Hour)) {
		return fmt.Errorf("invalid incident scope or budget")
	}
	for _, name := range []string{r.Scope.Namespace, r.Scope.Pod, r.Scope.Node, r.Scope.Workload} {
		if len(name) > 200 || (name != "" && len(validation.IsDNS1123Subdomain(name)) > 0) {
			return fmt.Errorf("invalid resource name")
		}
	}
	allowedNS := false
	for _, ns := range strings.Split(os.Getenv("DIAGNOSTIC_NAMESPACES"), ",") {
		if strings.TrimSpace(ns) == r.Scope.Namespace {
			allowedNS = true
		}
	}
	if !allowedNS {
		return fmt.Errorf("namespace not permitted for diagnostics")
	}
	seen := map[string]bool{}
	for _, check := range r.Checks {
		if !diagnosticChecks[check] || seen[check] {
			return fmt.Errorf("unsupported or repeated check")
		}
		seen[check] = true
	}
	return nil
}
func boundedSourceJSON(resp *http.Response, v any) error {
	defer resp.Body.Close()
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return fmt.Errorf("unauthorized")
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("source failed")
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, (256<<10)+1))
	if err != nil || len(raw) > 256<<10 {
		return fmt.Errorf("source result exceeds budget")
	}
	return json.Unmarshal(raw, v)
}
func collectTypedDiagnostic(ctx context.Context, check string, scope DiagnosticScope) (any, string, string) {
	if check == "pod_restarts" {
		if appCfg.PromURL == "" {
			return nil, "disabled", ""
		}
		if scope.Pod == "" {
			return nil, "unavailable", ""
		}
		expression := fmt.Sprintf("kube_pod_container_status_restarts_total{namespace=%s,pod=%s}", strconv.Quote(scope.Namespace), strconv.Quote(scope.Pod))
		target, err := url.Parse(appCfg.PromURL)
		if err != nil {
			return nil, "failed", ""
		}
		target.Path = "/api/v1/query_range"
		q := url.Values{"query": {expression}, "start": {strconv.FormatInt(scope.From.Unix(), 10)}, "end": {strconv.FormatInt(scope.To.Unix(), 10)}, "step": {"30"}}
		target.RawQuery = q.Encode()
		req, err := http.NewRequestWithContext(ctx, "GET", target.String(), nil)
		if err != nil {
			return nil, "failed", expression
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, diagnosticErrorStatus(ctx, err), expression
		}
		var result promRangeResponse
		if err = boundedSourceJSON(resp, &result); err != nil {
			return nil, diagnosticErrorStatus(ctx, err), expression
		}
		if result.Status != "success" {
			return nil, "failed", expression
		}
		if len(result.Data.Result) == 0 {
			return []any{}, "empty", expression
		}
		if len(result.Data.Result) > 20 {
			return nil, "failed", expression
		}
		summaries := []map[string]any{}
		for _, series := range result.Data.Result {
			values := []float64{}
			for _, pair := range series.Values {
				if len(pair) == 2 {
					if v, ok := parsePromValue(pair[1]); ok {
						values = append(values, v)
					}
				}
			}
			if len(values) > 0 {
				stats := calcStats(values)
				summaries = append(summaries, map[string]any{"container": series.Metric["container"], "samples": len(values), "stats": stats})
			}
		}
		if len(summaries) == 0 {
			return summaries, "empty", expression
		}
		return summaries, "available", expression
	}
	if check == "logs" {
		if !logsCfg.enabled {
			return nil, "disabled", ""
		}
		if scope.Pod == "" {
			return nil, "unavailable", ""
		}
		payload := map[string]any{"size": 30, "sort": []any{map[string]string{"@timestamp": "asc"}}, "_source": []string{"@timestamp", "message", "log"}, "query": map[string]any{"bool": map[string]any{"filter": []any{map[string]any{"term": map[string]string{"kubernetes.namespace_name.keyword": scope.Namespace}}, map[string]any{"term": map[string]string{"kubernetes.pod_name.keyword": scope.Pod}}, map[string]any{"range": map[string]any{"@timestamp": map[string]string{"gte": scope.From.Format(time.RFC3339), "lte": scope.To.Format(time.RFC3339)}}}}}}}
		raw, _ := json.Marshal(payload)
		target := strings.TrimRight(logsCfg.url, "/") + "/" + url.PathEscape(logsCfg.index) + "/_search"
		req, err := http.NewRequestWithContext(ctx, "POST", target, bytes.NewReader(raw))
		if err != nil {
			return nil, "failed", ""
		}
		req.Header.Set("Content-Type", "application/json")
		if logsCfg.user != "" {
			req.SetBasicAuth(logsCfg.user, logsCfg.password)
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, diagnosticErrorStatus(ctx, err), "configured logs pod/time filter"
		}
		var result struct {
			Hits struct {
				Hits []struct {
					Source map[string]any `json:"_source"`
				} `json:"hits"`
			} `json:"hits"`
		}
		if err = boundedSourceJSON(resp, &result); err != nil {
			return nil, diagnosticErrorStatus(ctx, err), "configured logs pod/time filter"
		}
		if len(result.Hits.Hits) == 0 {
			return []any{}, "empty", "configured logs pod/time filter"
		}
		if len(result.Hits.Hits) > 30 {
			return nil, "failed", ""
		}
		return result.Hits.Hits, "available", "configured logs pod/time filter"
	}
	if k8sClient == nil {
		return nil, "unavailable", ""
	}
	switch check {
	case "pod_termination":
		if scope.Pod == "" {
			return nil, "unavailable", ""
		}
		pod, err := k8sClient.CoreV1().Pods(scope.Namespace).Get(ctx, scope.Pod, metav1.GetOptions{})
		if err != nil {
			return nil, diagnosticErrorStatus(ctx, err), ""
		}
		statuses := []map[string]any{}
		for _, c := range pod.Status.ContainerStatuses {
			value := map[string]any{"container": c.Name, "restarts": c.RestartCount, "ready": c.Ready}
			if c.LastTerminationState.Terminated != nil {
				t := c.LastTerminationState.Terminated
				value["reason"] = t.Reason
				value["exit_code"] = t.ExitCode
				value["finished_at"] = t.FinishedAt.Time
			}
			statuses = append(statuses, value)
			if len(statuses) == 20 {
				break
			}
		}
		return statuses, "available", "pod container termination status"
	case "pod_events":
		if scope.Pod == "" {
			return nil, "unavailable", ""
		}
		events, err := k8sClient.CoreV1().Events(scope.Namespace).List(ctx, metav1.ListOptions{FieldSelector: "involvedObject.name=" + scope.Pod + ",involvedObject.kind=Pod", Limit: 30})
		if err != nil {
			return nil, diagnosticErrorStatus(ctx, err), ""
		}
		data := []map[string]any{}
		for _, e := range events.Items {
			at := e.LastTimestamp.Time
			if at.IsZero() {
				at = e.CreationTimestamp.Time
			}
			if at.Before(scope.From) || at.After(scope.To) {
				continue
			}
			message := e.Message
			if len(message) > 1024 {
				message = string([]rune(message)[:min(200, len([]rune(message)))]) + " [truncated]"
			}
			data = append(data, map[string]any{"reason": e.Reason, "type": e.Type, "count": e.Count, "message": message, "source_timestamp": at})
		}
		if len(data) == 0 {
			return data, "empty", "pod events in bounded window"
		}
		return data, "available", "pod events in bounded window"
	case "node_conditions":
		if scope.Node == "" {
			return nil, "unavailable", ""
		}
		node, err := k8sClient.CoreV1().Nodes().Get(ctx, scope.Node, metav1.GetOptions{})
		if err != nil {
			return nil, diagnosticErrorStatus(ctx, err), ""
		}
		data := []map[string]any{}
		for _, c := range node.Status.Conditions {
			data = append(data, map[string]any{"type": c.Type, "status": c.Status, "reason": c.Reason, "last_transition": c.LastTransitionTime.Time})
		}
		return data, "available", "node conditions"
	case "workload_rollout":
		if scope.Workload == "" {
			return nil, "unavailable", ""
		}
		d, err := k8sClient.AppsV1().Deployments(scope.Namespace).Get(ctx, scope.Workload, metav1.GetOptions{})
		if err != nil {
			return nil, diagnosticErrorStatus(ctx, err), ""
		}
		return map[string]any{"generation": d.Generation, "observed_generation": d.Status.ObservedGeneration, "replicas": d.Status.Replicas, "updated": d.Status.UpdatedReplicas, "available": d.Status.AvailableReplicas}, "available", "Deployment rollout status"
	}
	return nil, "failed", "unsupported"
}
func diagnosticErrorStatus(ctx context.Context, err error) string {
	if ctx.Err() != nil {
		return "timed_out"
	}
	if apierrors.IsForbidden(err) || apierrors.IsUnauthorized(err) || err.Error() == "unauthorized" {
		return "unauthorized"
	}
	return "failed"
}
func diagnosticsHandler(w http.ResponseWriter, r *http.Request) {
	token := os.Getenv("ANALYSIS_SERVICE_TOKEN")
	if os.Getenv("GUIDED_DIAGNOSTICS_ENABLED") != "true" {
		http.Error(w, "diagnostics disabled", 503)
		return
	}
	if token == "" || !constantToken(r.Header.Get("Authorization"), "Bearer "+token) {
		http.Error(w, "unauthorized", 401)
		return
	}
	select {
	case diagnosticSlots <- struct{}{}:
		defer func() { <-diagnosticSlots }()
	default:
		http.Error(w, "diagnostics busy", 503)
		return
	}
	defer r.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(r.Body, 16<<10))
	var request DiagnosticRequest
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err != nil || d.Decode(&request) != nil || d.Decode(&struct{}{}) != io.EOF || validateDiagnosticRequest(request) != nil {
		http.Error(w, "unsupported diagnostic scope", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	result := DiagnosticResult{Status: "complete", CollectedAt: time.Now().UTC(), Evidence: []Evidence{}}
	for _, check := range request.Checks {
		callCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		value, status, query := collectTypedDiagnostic(callCtx, check, request.Scope)
		cancel()
		at := time.Now().UTC()
		raw, _ := json.Marshal(value)
		if len(raw) > 8192 {
			status = "failed"
			raw = []byte(`"source output exceeded evidence budget"`)
		}
		if status != "available" && status != "empty" {
			result.Status = "partial"
		}
		from, to := request.Scope.From, request.Scope.To
		if check == "pod_termination" || check == "node_conditions" || check == "workload_rollout" {
			from = at
			to = at
		}
		e := Evidence{Version: 1, ID: "diagnostic-" + request.OperationID + "-" + check, IncidentID: request.IncidentID, ClusterID: request.Scope.ClusterID, Resource: request.Scope.Namespace + "/" + request.Scope.Pod, Source: check, ObservedFrom: &from, ObservedTo: &to, CollectedAt: &at, IngestedAt: at, Excerpt: string(raw), SourceQuery: query, CollectorVersion: "typed-diagnostics/v1", Availability: status}
		result.Evidence = append(result.Evidence, e)
	}
	result.CollectedAt = time.Now().UTC()
	safe, err := sanitizerClient.SanitizePayload(ctx, "enricher", "webhook", result)
	if err != nil {
		http.Error(w, "diagnostic sanitization failed", 503)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(safe)
}
