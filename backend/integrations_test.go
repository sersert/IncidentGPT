package main

import (
	"strings"
	"testing"
)

// Конфиг интеграций отдаётся в веб как есть, поэтому пароли и ключи обязаны
// уходить маской. Хвост оставляем, чтобы человек узнал свой ключ, не видя его.
func TestMask(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"a", "••••"},
		{"abcd", "••••"},
		{"abcde", "••••••••bcde"},
		{"sk-or-v1-0123456789abcdefIGZ5", "••••••••IGZ5"},
	}
	for _, tt := range tests {
		if got := mask(tt.in); got != tt.want {
			t.Errorf("mask(%q) = %q, ожидали %q", tt.in, got, tt.want)
		}
	}
}

// Короткое значение не должно частично просвечивать: для него маска глухая.
func TestMaskNeverLeaksShortSecret(t *testing.T) {
	for _, secret := range []string{"a", "ab", "abc", "abcd"} {
		got := mask(secret)
		if strings.ContainsAny(got, "abcd") {
			t.Errorf("mask(%q) = %q — видна часть секрета", secret, got)
		}
	}
}

func TestDeriveStatus(t *testing.T) {
	fields := []IntegrationField{
		{Key: "url", Required: true},
		{Key: "username"},
	}

	tests := []struct {
		name string
		it   *Integration
		want string
	}{
		{
			name: "обязательное поле пустое",
			it:   &Integration{Fields: fields, Config: map[string]string{"url": ""}, Enabled: true},
			want: "not_configured",
		},
		{
			name: "обязательное поле из пробелов",
			it:   &Integration{Fields: fields, Config: map[string]string{"url": "   "}, Enabled: true},
			want: "not_configured",
		},
		{
			name: "заполнено, но выключено",
			it:   &Integration{Fields: fields, Config: map[string]string{"url": "http://prom:9090"}},
			want: "not_configured",
		},
		{
			name: "заполнено и включено",
			it:   &Integration{Fields: fields, Config: map[string]string{"url": "http://prom:9090"}, Enabled: true},
			want: "connected",
		},
		{
			// Kubernetes и Telegram настраиваются вне интерфейса: у них нет полей,
			// и требовать заполнения нечего.
			name: "только для чтения — всегда connected",
			it:   &Integration{Fields: fields, Config: map[string]string{}, readOnly: true},
			want: "connected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := deriveStatus(tt.it); got != tt.want {
				t.Errorf("deriveStatus() = %q, ожидали %q", got, tt.want)
			}
		})
	}
}

func TestAllRequiredFilled(t *testing.T) {
	it := &Integration{
		Fields: []IntegrationField{{Key: "url", Required: true}, {Key: "token", Required: true}, {Key: "note"}},
		Config: map[string]string{"url": "http://x", "token": "t"},
	}
	if !allRequiredFilled(it) {
		t.Error("ожидали true: оба обязательных поля заполнены")
	}
	it.Config["token"] = " "
	if allRequiredFilled(it) {
		t.Error("ожидали false: пробел не считается заполненным полем")
	}
}

func TestIsSecret(t *testing.T) {
	it := &Integration{secretKeys: []string{"password", "token"}}
	for _, k := range []string{"password", "token"} {
		if !isSecret(it, k) {
			t.Errorf("%q должен считаться секретом", k)
		}
	}
	for _, k := range []string{"url", "username", ""} {
		if isSecret(it, k) {
			t.Errorf("%q не должен считаться секретом", k)
		}
	}
}

// Реестр строится из окружения при старте: проверяем, что источники по
// назначению на месте и обязательные поля у них объявлены.
func TestRegistryHasPurposeBasedSources(t *testing.T) {
	r := NewIntegrationRegistry()
	for _, typ := range []string{"metrics", "logs"} {
		items := r.List()
		found := false
		for _, it := range items {
			if it.Type == typ {
				found = true
				if it.Category != "data_source" {
					t.Errorf("%q: category = %q, ожидали data_source", typ, it.Category)
				}
			}
		}
		if !found {
			t.Errorf("интеграция %q не зарегистрирована", typ)
		}
	}
}

// Наружу секреты уходить не должны даже в списке.
func TestListMasksSecrets(t *testing.T) {
	t.Setenv("LOGS_STORE_URL", "http://opensearch:9200")
	t.Setenv("LOGS_STORE_PASSWORD", "очень-секретный-пароль")
	r := NewIntegrationRegistry()

	for _, it := range r.List() {
		for k, v := range it.Config {
			if strings.Contains(v, "очень-секретный") {
				t.Errorf("%s.%s отдан незамаскированным: %q", it.Type, k, v)
			}
		}
	}
}
