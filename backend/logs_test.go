package main

import "testing"

// Уровень в строке лога не размечен — его угадывают по тексту. Порядок правил
// важен: «fatal error» обязан стать FATAL, а не ERROR.
func TestDetectLevel(t *testing.T) {
	tests := []struct {
		line string
		want string
	}{
		{"panic: runtime error: invalid memory address", "FATAL"},
		{"fatal error: concurrent map writes", "FATAL"},
		{"ERROR database connection error", "ERROR"},
		{"connection refused", "ERROR"},
		{"context deadline exceeded: timeout", "ERROR"},
		{"WARN Read-only file system", "WARN"},
		{"listening on :8080", "INFO"},
		{"", "INFO"},
	}
	for _, tt := range tests {
		if got := detectLevel(tt.line); got != tt.want {
			t.Errorf("detectLevel(%q) = %q, ожидали %q", tt.line, got, tt.want)
		}
	}
}

// Текст строки лежит то в log, то в message — зависит от сборщика. Берём
// первое непустое, иначе строка молча теряется.
func TestFirstString(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]any
		want string
	}{
		{"первый ключ", map[string]any{"log": "из log", "message": "из message"}, "из log"},
		{"фолбэк на второй", map[string]any{"message": "из message"}, "из message"},
		{"пустой первый не в счёт", map[string]any{"log": "", "message": "из message"}, "из message"},
		{"не строка не в счёт", map[string]any{"log": 42, "message": "из message"}, "из message"},
		{"ничего нет", map[string]any{"other": "x"}, ""},
		{"пустая карта", map[string]any{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := firstString(tt.m, "log", "message"); got != tt.want {
				t.Errorf("firstString() = %q, ожидали %q", got, tt.want)
			}
		})
	}
}
