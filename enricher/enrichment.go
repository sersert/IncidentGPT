package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"
)

// attachMetrics — унифицированная функция для запроса метрик Prometheus и заполнения PromSample.
// defs — список определений метрик (из metricsCfg.Cluster / Node / Workload / External).
// prefix — префикс ключа в PromSample ("cluster_", "node_", "svc_", "ext_").
func attachMetrics(
	ctx context.Context,
	e *EnrichedAlert,
	defs []MetricDef,
	tmplCtx metricTemplateContext,
	prefix string,
	from, to time.Time,
) {
	for _, m := range defs {
		expr, err := renderExpr(m.Expr, tmplCtx)
		if err != nil {
			log.Printf("WARN: render %s metric %s expr failed: %v", prefix, m.Name, err)
			continue
		}
		stats, err := queryStats(ctx, expr, from, to)
		if err != nil {
			log.Printf("WARN: query %s metric %s failed: %v", prefix, m.Name, err)
			continue
		}
		if stats.Count == 0 {
			continue
		}

		p := prefix + m.Name
		e.PromSample[p+"_min"] = fmt.Sprintf("%f", stats.Min)
		e.PromSample[p+"_max"] = fmt.Sprintf("%f", stats.Max)
		e.PromSample[p+"_avg"] = fmt.Sprintf("%f", stats.Avg)
		e.PromSample[p+"_last"] = fmt.Sprintf("%f", stats.Last)
		e.PromSample[p+"_count"] = strconv.Itoa(stats.Count)
		e.PromSample[p+"_trend"] = stats.Trend
	}
}

// attachClusterContext — метрики уровня кластера.
func attachClusterContext(ctx context.Context, e *EnrichedAlert, _ AMAlert, from, to time.Time) {
	if appCfg.ClusterName == "" || appCfg.ClusterName == "unknown" {
		return
	}
	tmplCtx := metricTemplateContext{Cluster: appCfg.ClusterName}
	attachMetrics(ctx, e, metricsCfg.Cluster, tmplCtx, "cluster_", from, to)
}

// attachNodeContext — метрики уровня ноды.
func attachNodeContext(ctx context.Context, e *EnrichedAlert, a AMAlert, from, to time.Time) {
	instance := a.Labels["instance"]
	node := a.Labels["node"]
	if instance == "" && node == "" {
		return
	}
	if node != "" {
		e.K8sContext["node"] = node
	}
	if instance != "" {
		e.K8sContext["instance"] = instance
	}
	tmplCtx := metricTemplateContext{
		Cluster:   appCfg.ClusterName,
		Namespace: a.Labels["namespace"],
		Service:   a.Labels["service"],
		Node:      node,
		Instance:  instance,
	}
	attachMetrics(ctx, e, metricsCfg.Node, tmplCtx, "node_", from, to)
}

// attachWorkloadContext — метрики уровня workload (по namespace/service).
func attachWorkloadContext(ctx context.Context, e *EnrichedAlert, a AMAlert, from, to time.Time) {
	ns := a.Labels["namespace"]
	svc := a.Labels["service"]
	if ns == "" && svc == "" {
		return
	}
	if ns != "" {
		e.K8sContext["namespace"] = ns
	}
	if svc != "" {
		e.K8sContext["service"] = svc
	}
	tmplCtx := metricTemplateContext{
		Cluster:   appCfg.ClusterName,
		Namespace: ns,
		Service:   svc,
		Node:      a.Labels["node"],
		Instance:  a.Labels["instance"],
	}
	attachMetrics(ctx, e, metricsCfg.Workload, tmplCtx, "svc_", from, to)
}

// attachExternalContext — метрики внешних зависимостей (БД, Kafka, etc.).
func attachExternalContext(ctx context.Context, e *EnrichedAlert, a AMAlert, from, to time.Time) {
	svc := a.Labels["service"]
	inst := a.Labels["instance"]
	scope := a.Labels["scope"]
	component := a.Labels["component"]
	if svc == "" && inst == "" && scope != "external" && component == "" {
		return
	}
	if scope != "" {
		e.K8sContext["scope"] = scope
	}
	if component != "" {
		e.K8sContext["component"] = component
	}
	if svc != "" {
		e.K8sContext["external_service"] = svc
	}
	if inst != "" {
		e.K8sContext["external_instance"] = inst
	}
	tmplCtx := metricTemplateContext{
		Cluster:   appCfg.ClusterName,
		Namespace: a.Labels["namespace"],
		Service:   svc,
		Node:      a.Labels["node"],
		Instance:  inst,
	}
	attachMetrics(ctx, e, metricsCfg.External, tmplCtx, "ext_", from, to)
}
