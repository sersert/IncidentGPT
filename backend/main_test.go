package main

import (
	"fmt"
	"sync"
	"testing"
)

// Enricher шлёт группу как {group_key, window, alerts[]}, но тот же эндпоинт
// принимает и массив, и одиночный алерт. Один раз объект уже разобрали как
// одиночный алерт: ошибки не было, а в ленту уехал пустой «Unknown Alert».
// Поэтому проверяем и распознавание форм, и отказ на неузнанной.
func TestParseIngestPayload(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantCount int
		wantGroup string
		wantErr   bool
	}{
		{
			name:      "группа от enricher",
			body:      `{"group_key":"sock-shop/2026-08-09T10:00:00Z","window":"40s","alerts":[{"fingerprint":"a1","labels":{"alertname":"PodDown"}},{"fingerprint":"a2","labels":{"alertname":"High5xx"}}]}`,
			wantCount: 2,
			wantGroup: "sock-shop/2026-08-09T10:00:00Z",
		},
		{
			name:      "группа без group_key",
			body:      `{"alerts":[{"fingerprint":"a1","labels":{"alertname":"PodDown"}}]}`,
			wantCount: 1,
		},
		{
			name:      "пустая группа — валидна, но алертов нет",
			body:      `{"group_key":"ns/t","alerts":[]}`,
			wantCount: 0,
			wantGroup: "ns/t",
		},
		{
			name:      "массив алертов",
			body:      `[{"fingerprint":"a1","labels":{"alertname":"PodDown"}}]`,
			wantCount: 1,
		},
		{
			name:      "одиночный алерт",
			body:      `{"fingerprint":"a1","labels":{"alertname":"PodDown"}}`,
			wantCount: 1,
		},
		// Объект без fingerprint и без labels — это не алерт. Раньше такой
		// проходил молча и превращался в инцидент-пустышку.
		{name: "неузнанная форма", body: `{"foo":"bar"}`, wantErr: true},
		{name: "пустое тело", body: ``, wantErr: true},
		{name: "пробелы вместо тела", body: "  \n\t ", wantErr: true},
		{name: "битый JSON", body: `{"alerts":[`, wantErr: true},
		{name: "alerts не массив", body: `{"alerts":"нет"}`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alerts, groupKey, err := parseIngestPayload([]byte(tt.body))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ожидали ошибку, получили %d алертов", len(alerts))
				}
				return
			}
			if err != nil {
				t.Fatalf("неожиданная ошибка: %v", err)
			}
			if len(alerts) != tt.wantCount {
				t.Errorf("алертов = %d, ожидали %d", len(alerts), tt.wantCount)
			}
			if groupKey != tt.wantGroup {
				t.Errorf("group_key = %q, ожидали %q", groupKey, tt.wantGroup)
			}
		})
	}
}

// Счётчик инкрементировался без блокировки — при одновременных запросах два
// инцидента получали один id и второй затирал первый.
func TestNextIDUnique(t *testing.T) {
	s := NewStore()

	const n = 200
	ids := make(chan string, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ids <- s.NextID()
		}()
	}
	wg.Wait()
	close(ids)

	seen := make(map[string]bool, n)
	for id := range ids {
		if seen[id] {
			t.Fatalf("id выдан дважды: %s", id)
		}
		seen[id] = true
	}
	if len(seen) != n {
		t.Errorf("уникальных id = %d, ожидали %d", len(seen), n)
	}
}

func TestEnv(t *testing.T) {
	if got := env("ЗАВЕДОМО_НЕТ_ТАКОЙ_ПЕРЕМЕННОЙ", "по умолчанию"); got != "по умолчанию" {
		t.Errorf("env() = %q, ожидали фолбэк", got)
	}
	t.Setenv("IGPT_TEST_KEY", "значение")
	if got := env("IGPT_TEST_KEY", "по умолчанию"); got != "значение" {
		t.Errorf("env() = %q, ожидали значение из окружения", got)
	}
	// Пустая переменная считается незаданной. Следствие: настройку с непустым
	// дефолтом нельзя отключить, выставив её в пустое значение — нужен сам
	// дефолт пустой. Проверяем осознанно, чтобы поведение не поменялось молча.
	t.Setenv("IGPT_TEST_EMPTY", "")
	if got := env("IGPT_TEST_EMPTY", "по умолчанию"); got != "по умолчанию" {
		t.Errorf("env() = %q, ожидали фолбэк при пустой переменной", got)
	}
}

func TestNextIDFormat(t *testing.T) {
	s := NewStore()
	id := s.NextID()
	var year, seq int
	if _, err := fmt.Sscanf(id, "INC-%d-%d", &year, &seq); err != nil {
		t.Fatalf("id %q не разбирается как INC-<год>-<номер>: %v", id, err)
	}
	if year < 2020 {
		t.Errorf("год в id = %d", year)
	}
}
