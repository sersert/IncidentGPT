package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Обратный канал: отдаём разбор туда, где его ждёт веб-интерфейс.
//
// До этого ai-worker публиковал результат только в Telegram, и всё остальное о
// разборе не знало. Если ANALYSIS_CALLBACK_URL не задан, поведение прежнее —
// компонент остаётся самодостаточным.

var analysisCallbackURL = os.Getenv("ANALYSIS_CALLBACK_URL")

var analysisHTTP = &http.Client{Timeout: 10 * time.Second}

type analysisPayload struct {
	GroupKey       string           `json:"group_key"`
	RootCause      string           `json:"root_cause"`
	Summary        string           `json:"summary"`
	Raw            string           `json:"raw"`
	Confidence     int              `json:"confidence,omitempty"`
	DiscardedNoise []discardedAlert `json:"discarded_noise,omitempty"`
}

type discardedAlert struct {
	AlertName string `json:"alertname"`
	Reason    string `json:"reason"`
}

// postAnalysis отправляет разбор в бэкенд. Ошибки только логируем: Telegram уже
// получил результат, и ронять из-за недоступного получателя нечего.
func postAnalysis(groupKey, aiText string) {
	if analysisCallbackURL == "" {
		return
	}

	// В интерфейс отдаём разбор целиком — тот же текст, что уходит в Telegram,
	// со всеми разделами. Разбирать его на поля бессмысленно: модель пишет
	// свободным markdown, и любой парсер теряет часть содержимого.
	payload := analysisPayload{
		GroupKey:       groupKey,
		Raw:            aiText,
		RootCause:      aiText,
		DiscardedNoise: extractDiscarded(aiText),
	}
	payload.Confidence = extractConfidence(aiText)
	// Для списка инцидентов нужна одна строка — берём суть из «Первопричины».
	if head := extractSection(aiText, "Первопричина", "Корень", "Root Cause"); head != "" {
		payload.Summary = firstMeaningfulLine(head)
	} else {
		payload.Summary = firstMeaningfulLine(aiText)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("ERROR: marshal analysis callback: %v", err)
		return
	}

	url := fmt.Sprintf("%s?group_key=%s", analysisCallbackURL, groupKey)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		log.Printf("ERROR: build analysis callback: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := analysisHTTP.Do(req)
	if err != nil {
		log.Printf("WARN: analysis callback failed: %v", err)
		return
	}
	defer resp.Body.Close()
	log.Printf("INFO: analysis callback sent key=%s status=%d", groupKey, resp.StatusCode)
}

// extractSection достаёт текст раздела по любому из заголовков.
// Модель пишет разделы как `**Первопричина:**`, поэтому ищем по вхождению.
func extractSection(text string, headings ...string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		clean := strings.ToLower(strings.Trim(line, "*# :"))
		for _, h := range headings {
			if !strings.HasPrefix(clean, strings.ToLower(h)) {
				continue
			}
			// Заголовок и текст бывают на одной строке.
			if idx := strings.Index(line, ":"); idx >= 0 && len(strings.TrimSpace(line[idx+1:])) > 0 {
				rest := strings.TrimSpace(strings.Trim(line[idx+1:], "* "))
				if len(rest) > 0 {
					return collectUntilNextHeading(rest, lines[i+1:])
				}
			}
			return collectUntilNextHeading("", lines[i+1:])
		}
	}
	return ""
}

var headingRe = regexp.MustCompile(`^\s*\*{0,2}#{0,3}\s*(Исправление|Профилактика|Цепочка|Отброшенный|Recommendations|Prevention)`)

func collectUntilNextHeading(first string, rest []string) string {
	var b strings.Builder
	if first != "" {
		b.WriteString(first)
	}
	for _, line := range rest {
		if headingRe.MatchString(line) {
			break
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(line)
	}
	return strings.TrimSpace(b.String())
}

// extractDiscarded вытаскивает алерты, помеченные разбором как шум.
// Модель перечисляет их в разделе «Отброшенный шум» с именами в бэктиках.
var backtickRe = regexp.MustCompile("`([^`]+)`")

func extractDiscarded(text string) []discardedAlert {
	section := extractSection(text, "Отброшенный шум", "Отброшенные", "Discarded")
	if section == "" {
		return nil
	}
	var out []discardedAlert
	for _, name := range backtickRe.FindAllStringSubmatch(section, -1) {
		out = append(out, discardedAlert{AlertName: name[1], Reason: firstMeaningfulLine(section)})
	}
	return out
}

func firstMeaningfulLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(strings.Trim(line, "*# "))
		if len([]rune(line)) > 20 {
			r := []rune(line)
			if len(r) > 200 {
				return string(r[:200]) + "…"
			}
			return line
		}
	}
	return strings.TrimSpace(s)
}

// extractConfidence достаёт оценку уверенности, которую модель ставит в конце
// разбора. Раньше интерфейс показывал фиксированные 75%, что вводило в
// заблуждение: цифра выглядела как оценка модели, а была заглушкой.
var confidenceRe = regexp.MustCompile(`(?i)увер[а-яё]*\W{0,4}(\d{1,3})\s*%?`)

func extractConfidence(text string) int {
	m := confidenceRe.FindStringSubmatch(text)
	if len(m) < 2 {
		return 0
	}
	v, err := strconv.Atoi(m[1])
	if err != nil || v < 0 || v > 100 {
		return 0
	}
	return v
}
