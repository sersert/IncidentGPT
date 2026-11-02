package main

import (
	"context"
	"io"
	"net"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDurableStartupRecoversWhenRedisStartsLate(t *testing.T) {
	addr := os.Getenv("INCIDENTGPT_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("requires dedicated Redis")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	var available atomic.Bool
	rejected := make(chan struct{})
	var once sync.Once
	go func() {
		for {
			client, err := listener.Accept()
			if err != nil {
				return
			}
			if !available.Load() {
				client.Close()
				once.Do(func() { close(rejected) })
				continue
			}
			go func() {
				defer client.Close()
				upstream, err := net.DialTimeout("tcp", addr, time.Second)
				if err != nil {
					return
				}
				defer upstream.Close()
				go func() { _, _ = io.Copy(upstream, client) }()
				_, _ = io.Copy(client, upstream)
			}()
		}
	}()
	old := store
	store = &Store{}
	defer func() { store = old }()
	t.Setenv("DURABLE_ANALYSIS_ENABLED", "true")
	t.Setenv("REDIS_ADDR", listener.Addr().String())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- awaitDurableStorage(ctx) }()
	select {
	case <-rejected:
	case <-ctx.Done():
		t.Fatal("startup did not try Redis")
	}
	w := httptest.NewRecorder()
	handleReady(w, httptest.NewRequest("GET", "/readyz", nil))
	if w.Code != 503 {
		t.Fatal("durable backend reported ready without storage")
	}
	available.Store(true)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	defer store.disk.rdb.Close()
	if err := store.disk.rdb.Ping(context.Background()).Err(); err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	handleReady(w, httptest.NewRequest("GET", "/readyz", nil))
	if w.Code != 200 {
		t.Fatal("backend did not become ready after Redis recovery")
	}
}

func TestDurableStartupRequiresConfigurationAndHonorsDeadline(t *testing.T) {
	old := store
	store = &Store{}
	defer func() { store = old }()
	t.Setenv("DURABLE_ANALYSIS_ENABLED", "true")
	t.Setenv("REDIS_ADDR", "")
	if awaitDurableStorage(context.Background()) == nil {
		t.Fatal("durable mode accepted missing Redis configuration")
	}
	t.Setenv("REDIS_ADDR", "127.0.0.1:1")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if awaitDurableStorage(ctx) != context.Canceled {
		t.Fatal("startup ignored cancellation")
	}
	t.Setenv("DURABLE_ANALYSIS_ENABLED", "false")
	if err := awaitDurableStorage(ctx); err != nil {
		t.Fatal("legacy mode no longer permits memory storage")
	}
	w := httptest.NewRecorder()
	handleReady(w, httptest.NewRequest("GET", "/readyz", nil))
	if w.Code != 200 {
		t.Fatal("legacy memory mode is not ready")
	}
}
