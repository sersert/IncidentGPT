package main

import (
	"context"
	"crypto/subtle"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

func enricherOperations(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(os.Getenv("OPERATIONS_SERVICE_TOKEN"))
	if os.Getenv("OPERATIONS_ENABLED") != "true" {
		http.Error(w, "operations disabled", 404)
		return
	}
	if len(token) < 32 || r.Header.Get("Origin") != "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+token)) != 1 {
		http.Error(w, "unauthorized", 401)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2200*time.Millisecond)
	defer cancel()
	results := make([]map[string]any, 3)
	var wg sync.WaitGroup
	for i, id := range []string{"metrics", "enricher_redis", "kubernetes"} {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			state, detail := "unknown", "Нет подтверждённой проверки"
			switch id {
			case "metrics":
				if appCfg.PromURL == "" {
					state, detail = "not_configured", "Источник метрик не настроен"
				} else if checkPrometheusReady().OK {
					state, detail = "healthy", "Prometheus readiness успешна"
				} else {
					state, detail = "unhealthy", "Источник метрик не отвечает на readiness"
				}
			case "enricher_redis":
				if appCfg.RedisAddr == "" {
					state, detail = "not_configured", "Redis enricher не настроен"
				} else if checkRedisReady().OK {
					state, detail = "healthy", "Redis enricher отвечает на PING"
				} else {
					state, detail = "unhealthy", "Нет связи enricher с Redis"
				}
			case "kubernetes":
				if !appCfg.EnableK8sContext {
					state, detail = "disabled", "Kubernetes-контекст выключен"
				} else if k8sClient == nil {
					state, detail = "not_configured", "Kubernetes client не настроен"
				} else if k8sClient.Discovery().RESTClient().Get().AbsPath("/version").Do(ctx).Error() == nil {
					state, detail = "healthy", "API /version ответил; workload RBAC отдельно не проверялся"
				} else {
					state, detail = "unhealthy", "Нет подтверждённого доступа к Kubernetes API"
				}
			}
			results[i] = map[string]any{"id": id, "status": state, "detail": detail, "checked_at": time.Now().UTC()}
		}(i, id)
	}
	wg.Wait()
	writeStatusJSON(w, 200, map[string]any{"service": "enricher", "checks": results})
}
