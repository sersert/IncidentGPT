package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type dependencyCheck struct {
	OK       bool   `json:"ok"`
	Optional bool   `json:"optional,omitempty"`
	Error    string `json:"error,omitempty"`
}

func readyzHandler(w http.ResponseWriter, _ *http.Request) {
	// readyz — только локальные мгновенные проверки, без сетевых вызовов.
	// Проба бежит каждые 5s; здоровье downstream'ов (Prometheus/Redis) смотрим в /status.
	// Иначе моргание Prometheus → enricher NotReady → его выкидывает из endpoints →
	// Alertmanager не доставит вебхук → потеря алертов ровно во время инцидента.
	checks := map[string]dependencyCheck{
		"config":         {OK: appCfg.PromURL != "" && appCfg.BackendURL != ""},
		"metrics_config": {OK: metricsCfgLoaded()},
		"redis":          {OK: redisClient != nil, Optional: true},
		"kubernetes":     checkK8sReady(),
	}

	ready := true
	for _, check := range checks {
		if !check.OK && !check.Optional {
			ready = false
			break
		}
	}

	status := http.StatusOK
	if !ready {
		status = http.StatusServiceUnavailable
	}
	writeStatusJSON(w, status, map[string]interface{}{
		"service": "enricher",
		"ready":   ready,
		"checks":  checks,
	})
}

func statusHandler(w http.ResponseWriter, _ *http.Request) {
	writeStatusJSON(w, http.StatusOK, map[string]interface{}{
		"service":          "enricher",
		"enricher_version": appCfg.EnricherVersion,
		"cluster":          appCfg.ClusterName,
		"environment":      appCfg.Environment,
		"prometheus": map[string]interface{}{
			"url_configured": appCfg.PromURL != "",
			"reachable":      checkPrometheusReady().OK,
			"range_before":   appCfg.RangeBefore.String(),
			"range_after":    appCfg.RangeAfter.String(),
		},
		"enrichment": map[string]interface{}{
			"cluster_context":  appCfg.EnableClusterContext,
			"node_context":     appCfg.EnableNodeContext,
			"workload_context": appCfg.EnableWorkloadContext,
			"external_context": appCfg.EnableExternalContext,
			"k8s_context":      appCfg.EnableK8sContext,
		},
		"redis": map[string]interface{}{
			"configured": appCfg.RedisAddr != "",
			"enabled":    redisClient != nil,
			"reachable":  checkRedisReady().OK,
			"window":     appCfg.CorrWindow.String(),
			"settle":     appCfg.CorrSettle.String(),
		},
		"metrics_config": map[string]int{
			"cluster":  len(metricsCfg.Cluster),
			"node":     len(metricsCfg.Node),
			"workload": len(metricsCfg.Workload),
			"external": len(metricsCfg.External),
		},
	})
}

func metricsCfgLoaded() bool {
	return metricsConfigLoaded
}

func checkPrometheusReady() dependencyCheck {
	if appCfg.PromURL == "" {
		return dependencyCheck{OK: false, Error: "PROMETHEUS_URL is empty"}
	}
	u, err := url.Parse(appCfg.PromURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return dependencyCheck{OK: false, Error: "invalid PROMETHEUS_URL"}
	}
	u.Path = "/-/ready"
	u.RawQuery = ""

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return dependencyCheck{OK: false, Error: err.Error()}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return dependencyCheck{OK: false, Error: err.Error()}
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<10))
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return dependencyCheck{OK: true}
	}
	return dependencyCheck{OK: false, Error: fmt.Sprintf("HTTP %d", resp.StatusCode)}
}

func checkRedisReady() dependencyCheck {
	if appCfg.RedisAddr == "" {
		return dependencyCheck{OK: true, Optional: true}
	}
	if redisClient == nil {
		return dependencyCheck{OK: false, Optional: true, Error: "disabled"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		return dependencyCheck{OK: false, Optional: true, Error: err.Error()}
	}
	return dependencyCheck{OK: true, Optional: true}
}

func checkK8sReady() dependencyCheck {
	if !appCfg.EnableK8sContext {
		return dependencyCheck{OK: true, Optional: true}
	}
	if k8sClient == nil {
		return dependencyCheck{OK: false, Optional: true, Error: "disabled"}
	}
	return dependencyCheck{OK: true, Optional: true}
}

func writeStatusJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
