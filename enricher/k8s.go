package main

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var k8sClient *kubernetes.Clientset

// nodeCache — кеш node conditions (обновляется не чаще чем раз в 60с).
// Node conditions меняются редко, а Get Node — дорогой вызов при массовых алертах.
var nodeCache = struct {
	mu      sync.RWMutex
	entries map[string]nodeCacheEntry
}{entries: make(map[string]nodeCacheEntry)}

type nodeCacheEntry struct {
	data      map[string]string
	fetchedAt time.Time
}

const nodeCacheTTL = 60 * time.Second

// initK8sClient создаёт клиент Kubernetes из in-cluster ServiceAccount.
// Использует QPS=50, Burst=100 чтобы не упереться в rate-limit API-сервера
// при массовых алертах (дефолт client-go: QPS=5, Burst=10).
func initK8sClient() error {
	config, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("k8s in-cluster config: %w", err)
	}
	config.Timeout = 5 * time.Second
	config.QPS = 50
	config.Burst = 100

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("k8s client: %w", err)
	}
	k8sClient = client
	return nil
}

// runK8sEnrichment запускает все K8s-вызовы параллельно и мержит результаты в e.K8sContext.
// Каждый вызов пишет в свой локальный map, потом результаты мержатся — без race conditions.
// Общий таймаут 3с — если API-сервер не ответил, enricher продолжает без K8s данных.
func runK8sEnrichment(ctx context.Context, e *EnrichedAlert, a AMAlert) {
	if k8sClient == nil {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	type result struct {
		data  map[string]string
		notes []string
	}

	// Запускаем все 4 вызова параллельно
	var wg sync.WaitGroup
	results := make([]result, 4)

	wg.Add(4)

	go func() {
		defer wg.Done()
		data, notes := collectPodContext(ctx, a)
		results[0] = result{data: data, notes: notes}
	}()

	go func() {
		defer wg.Done()
		data, notes := collectEvents(ctx, a)
		results[1] = result{data: data, notes: notes}
	}()

	go func() {
		defer wg.Done()
		data, _ := collectNodeContext(ctx, a)
		results[2] = result{data: data}
	}()

	go func() {
		defer wg.Done()
		data, _ := collectNamespaceHealth(ctx, a)
		results[3] = result{data: data}
	}()

	wg.Wait()

	// Мержим все результаты в e.K8sContext (single-threaded, без мьютекса)
	for _, r := range results {
		for k, v := range r.data {
			e.K8sContext[k] = v
		}
		for _, n := range r.notes {
			addNote(e, n)
		}
	}
}

// collectPodContext — pod phase, restarts, OOMKilled, resource requests/limits.
func collectPodContext(ctx context.Context, a AMAlert) (map[string]string, []string) {
	ns := a.Labels["namespace"]
	podName := a.Labels["pod"]
	if ns == "" || podName == "" {
		return nil, nil
	}

	pod, err := k8sClient.CoreV1().Pods(ns).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		log.Printf("WARN: k8s get pod %s/%s failed: %v", ns, podName, err)
		return nil, nil
	}

	data := make(map[string]string)
	data["pod_phase"] = string(pod.Status.Phase)
	data["pod_age"] = time.Since(pod.CreationTimestamp.Time).Truncate(time.Minute).String()

	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodReady {
			data["pod_ready"] = string(c.Status)
		}
		if c.Type == corev1.PodScheduled {
			data["pod_scheduled"] = string(c.Status)
		}
	}

	totalRestarts := 0
	var waitingReasons, terminationReasons []string

	for _, cs := range pod.Status.ContainerStatuses {
		totalRestarts += int(cs.RestartCount)

		if cs.State.Waiting != nil && cs.State.Waiting.Reason != "" {
			waitingReasons = append(waitingReasons, cs.State.Waiting.Reason)
		}
		if cs.LastTerminationState.Terminated != nil {
			t := cs.LastTerminationState.Terminated
			terminationReasons = append(terminationReasons,
				fmt.Sprintf("%s (exit %d)", t.Reason, t.ExitCode))
			data["pod_terminated_at"] = t.FinishedAt.UTC().Format(time.RFC3339)
		}
	}

	data["pod_restarts"] = strconv.Itoa(totalRestarts)
	if len(waitingReasons) > 0 {
		data["pod_waiting_reason"] = strings.Join(waitingReasons, ", ")
	}
	if len(terminationReasons) > 0 {
		data["pod_last_termination"] = strings.Join(terminationReasons, ", ")
	}

	// Init containers
	for _, cs := range pod.Status.InitContainerStatuses {
		if cs.State.Waiting != nil && cs.State.Waiting.Reason != "" {
			data["init_container_waiting"] = cs.Name + ": " + cs.State.Waiting.Reason
		}
	}

	// Resources — ищем контейнер по label "container", или берём первый
	targetContainer := a.Labels["container"]
	for _, c := range pod.Spec.Containers {
		if targetContainer != "" && c.Name != targetContainer {
			continue
		}
		if cpu := c.Resources.Requests.Cpu(); cpu != nil && !cpu.IsZero() {
			data["pod_cpu_request"] = cpu.String()
		}
		if mem := c.Resources.Requests.Memory(); mem != nil && !mem.IsZero() {
			data["pod_mem_request"] = mem.String()
		}
		if cpu := c.Resources.Limits.Cpu(); cpu != nil && !cpu.IsZero() {
			data["pod_cpu_limit"] = cpu.String()
		}
		if mem := c.Resources.Limits.Memory(); mem != nil && !mem.IsZero() {
			data["pod_mem_limit"] = mem.String()
		}
		data["pod_image"] = c.Image
		break
	}

	note := fmt.Sprintf("K8s pod %s/%s: phase=%s restarts=%d", ns, podName, pod.Status.Phase, totalRestarts)
	return data, []string{note}
}

// collectEvents — последние K8s events, детекция деплоймента.
func collectEvents(ctx context.Context, a AMAlert) (map[string]string, []string) {
	ns := a.Labels["namespace"]
	if ns == "" {
		return nil, nil
	}

	listOpts := metav1.ListOptions{}
	if podName := a.Labels["pod"]; podName != "" {
		listOpts.FieldSelector = fmt.Sprintf("involvedObject.name=%s", podName)
	}

	events, err := k8sClient.CoreV1().Events(ns).List(ctx, listOpts)
	if err != nil {
		log.Printf("WARN: k8s list events ns=%s failed: %v", ns, err)
		return nil, nil
	}

	if len(events.Items) == 0 {
		return nil, nil
	}

	// Сортируем: последние первые
	sort.Slice(events.Items, func(i, j int) bool {
		ti := eventTime(events.Items[i])
		tj := eventTime(events.Items[j])
		return ti.After(tj)
	})

	data := make(map[string]string)
	var notes []string
	var recentEvents []string
	warningCount := 0
	recentDeployment := false

	for i, ev := range events.Items {
		if ev.Type == "Warning" {
			warningCount++
		}

		evTime := eventTime(ev)
		if ev.Reason == "ScalingReplicaSet" && time.Since(evTime) < 30*time.Minute {
			recentDeployment = true
			data["deployment_time"] = evTime.UTC().Format(time.RFC3339)
			data["deployment_message"] = truncate(ev.Message, 120)
		}

		if i < 10 {
			recentEvents = append(recentEvents,
				fmt.Sprintf("%s: %s", ev.Reason, truncate(ev.Message, 120)))
		}
	}

	if len(recentEvents) > 0 {
		data["recent_events"] = strings.Join(recentEvents, "; ")
	}
	data["warning_events_count"] = strconv.Itoa(warningCount)

	if recentDeployment {
		data["recent_deployment"] = "true"
		notes = append(notes, fmt.Sprintf("Recent deployment detected in namespace %s", ns))
	}

	return data, notes
}

// collectNodeContext — node conditions, allocatable resources.
// Результаты кешируются на 60с — node conditions меняются редко.
func collectNodeContext(ctx context.Context, a AMAlert) (map[string]string, []string) {
	nodeName := a.Labels["node"]
	if nodeName == "" {
		return nil, nil
	}

	// Проверяем кеш
	nodeCache.mu.RLock()
	if entry, ok := nodeCache.entries[nodeName]; ok && time.Since(entry.fetchedAt) < nodeCacheTTL {
		nodeCache.mu.RUnlock()
		return entry.data, nil
	}
	nodeCache.mu.RUnlock()

	node, err := k8sClient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		log.Printf("WARN: k8s get node %s failed: %v", nodeName, err)
		return nil, nil
	}

	data := make(map[string]string)

	for _, c := range node.Status.Conditions {
		switch c.Type {
		case corev1.NodeReady:
			data["node_ready"] = string(c.Status)
			if c.Status != corev1.ConditionTrue {
				data["node_ready_reason"] = c.Reason
			}
		case corev1.NodeMemoryPressure:
			if c.Status == corev1.ConditionTrue {
				data["node_memory_pressure"] = "True"
			}
		case corev1.NodeDiskPressure:
			if c.Status == corev1.ConditionTrue {
				data["node_disk_pressure"] = "True"
			}
		case corev1.NodePIDPressure:
			if c.Status == corev1.ConditionTrue {
				data["node_pid_pressure"] = "True"
			}
		}
	}

	if cpu := node.Status.Allocatable.Cpu(); cpu != nil {
		data["node_cpu_allocatable"] = cpu.String()
	}
	if mem := node.Status.Allocatable.Memory(); mem != nil {
		data["node_mem_allocatable"] = mem.String()
	}

	if node.Spec.Unschedulable {
		data["node_unschedulable"] = "true"
	}

	if len(node.Spec.Taints) > 0 {
		var taints []string
		for _, t := range node.Spec.Taints {
			taints = append(taints, fmt.Sprintf("%s=%s:%s", t.Key, t.Value, t.Effect))
		}
		data["node_taints"] = strings.Join(taints, ", ")
	}

	// Записываем в кеш
	nodeCache.mu.Lock()
	nodeCache.entries[nodeName] = nodeCacheEntry{data: data, fetchedAt: time.Now()}
	nodeCache.mu.Unlock()

	return data, nil
}

// collectNamespaceHealth — масштаб проблемы: сколько подов not ready / crashloop.
func collectNamespaceHealth(ctx context.Context, a AMAlert) (map[string]string, []string) {
	ns := a.Labels["namespace"]
	if ns == "" {
		return nil, nil
	}

	pods, err := k8sClient.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("WARN: k8s list pods ns=%s failed: %v", ns, err)
		return nil, nil
	}

	total, notReady, crashLoop, pending := 0, 0, 0, 0
	for _, p := range pods.Items {
		if p.Status.Phase == corev1.PodSucceeded || p.Status.Phase == corev1.PodFailed {
			continue
		}
		total++

		if p.Status.Phase == corev1.PodPending {
			pending++
		}

		podNotReady := false
		podCrashLoop := false
		for _, cs := range p.Status.ContainerStatuses {
			if !cs.Ready {
				podNotReady = true
			}
			if cs.State.Waiting != nil && cs.State.Waiting.Reason == "CrashLoopBackOff" {
				podCrashLoop = true
			}
		}
		if podNotReady {
			notReady++
		}
		if podCrashLoop {
			crashLoop++
		}
	}

	data := map[string]string{
		"ns_pods_total":     strconv.Itoa(total),
		"ns_pods_not_ready": strconv.Itoa(notReady),
		"ns_pods_crashloop": strconv.Itoa(crashLoop),
		"ns_pods_pending":   strconv.Itoa(pending),
	}
	return data, nil
}

// eventTime возвращает время события (LastTimestamp или CreationTimestamp).
func eventTime(ev corev1.Event) time.Time {
	if !ev.LastTimestamp.IsZero() {
		return ev.LastTimestamp.Time
	}
	return ev.CreationTimestamp.Time
}

// truncate обрезает строку до maxLen символов.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
