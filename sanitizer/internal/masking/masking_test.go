package masking

import (
	"encoding/json"
	"strings"
	"testing"

	"incidentgpt-sanitizer/internal/config"
	"incidentgpt-sanitizer/internal/detector"
	"incidentgpt-sanitizer/internal/policy"
	"incidentgpt-sanitizer/internal/pseudonymizer"
)

func testEngine(t *testing.T, cfg config.Config) *Engine {
	t.Helper()
	if cfg.MaxDepth == 0 {
		cfg.MaxDepth = 20
	}
	if cfg.IPMode == "" {
		cfg.IPMode = "none"
	}
	d, err := detector.New(cfg.CustomPatterns)
	if err != nil {
		t.Fatal(err)
	}
	return NewEngine(cfg, d, policy.NewEngine(cfg), pseudonymizer.New(cfg.HashKey))
}

func TestEvidenceHashesAreNotPhonesButCredentialsRemainMasked(t *testing.T) {
	engine := testEngine(t, config.Config{})
	// Real failing ID from the isolated diagnostics deployment: the decimal run
	// inside its SHA256 digest used to be masked, invalidating evidence citations.
	id := "evidence-00e5fbce111a80c765cc429e2cbdce93ae759179969133cdfff101d372830dff"
	job := "job-aabbccdd1234567890123456aabbccdd"
	phone := "+7 999 123-45-67"
	input, _ := json.Marshal(map[string]any{
		"evidence_id":             id,
		"operation_id":            job,
		"supporting_evidence_ids": []string{id},
		"excerpt":                 phone + " " + id + " " + phone,
		"password":                id,
		"authorization":           "Bearer " + strings.TrimPrefix(id, "evidence-"),
		"numeric":                 strings.Repeat("1", 32),
	})
	for _, destination := range []string{"llm", "webhook", "telegram"} {
		out, summary, err := engine.SanitizePayload(destination, input)
		if err != nil {
			t.Fatal(err)
		}
		data := out.(map[string]any)
		if data["evidence_id"] != id || data["operation_id"] != job || data["supporting_evidence_ids"].([]any)[0] != id {
			t.Fatalf("%s corrupted routing/citation IDs: %v", destination, data)
		}
		if strings.Contains(data["excerpt"].(string), phone) || !strings.Contains(data["excerpt"].(string), id) {
			t.Fatalf("%s lost hash or leaked adjacent phone: %v", destination, data)
		}
		if data["password"] == id || strings.Contains(data["authorization"].(string), strings.TrimPrefix(id, "evidence-")) || data["numeric"] == strings.Repeat("1", 32) {
			t.Fatalf("%s weakened secret/numeric masking: %v", destination, data)
		}
		if summary.AppliedRules["phone"] != 3 {
			t.Fatalf("%s must mask the two real phones and numeric string: %+v", destination, summary)
		}
		findings, err := engine.InspectText(destination, id)
		if err != nil || len(findings) != 0 {
			t.Fatalf("inspect falsely reports hash as phone: %v %v", findings, err)
		}
	}
	custom := testEngine(t, config.Config{CustomPatterns: []config.CustomPattern{{Name: "id", Pattern: `evidence-[0-9a-f]{64}`, Replacement: "[CUSTOM_REDACTED]"}}})
	text, _, err := custom.SanitizeText("llm", id)
	if err != nil || text != "[CUSTOM_REDACTED]" {
		t.Fatalf("hash handling bypassed configured custom rules: %s %v", text, err)
	}
}

func TestSanitizePayloadRedactsBuiltInRules(t *testing.T) {
	engine := testEngine(t, config.Config{RedactEmails: true, RedactPhones: true, IPMode: "partial"})
	payload := json.RawMessage(`{
		"password":"super-secret",
		"authorization":"Bearer secret-token-value",
		"description":"postgres://admin:secret@db.internal:5432/app https://user:password@example.org/api eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.signaturehere",
		"notes":["Cookie: session_id=abcdef1234567890","call +7 999 123-45-67","mail sergey@example.com","ip 10.24.18.91"]
	}`)
	out, summary, err := engine.SanitizePayload("telegram", payload)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(out)
	got := string(b)
	for _, leaked := range []string{"super-secret", "secret-token-value", "admin:secret", "user:password", "sergey@example.com", "10.24.18.91"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("sanitized payload leaked %q: %s", leaked, got)
		}
	}
	for _, marker := range []string{"[REDACTED_SECRET]", "[REDACTED_TOKEN]", "[REDACTED_PASSWORD]", "[REDACTED_CREDENTIALS]", "s*****@example.com", "10.24.x.x"} {
		if !strings.Contains(got, marker) {
			t.Fatalf("sanitized payload missing %q: %s", marker, got)
		}
	}
	if summary.RedactionCount < 6 {
		t.Fatalf("expected several redactions, got %#v", summary)
	}
}

func TestSanitizeKubernetesSecret(t *testing.T) {
	engine := testEngine(t, config.Config{})
	out, _, err := engine.SanitizePayload("llm", json.RawMessage(`{
		"kind":"Secret",
		"metadata":{"name":"database-credentials","namespace":"production"},
		"data":{"password":"c2VjcmV0"},
		"stringData":{"token":"plain"}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(out)
	got := string(b)
	if strings.Contains(got, "c2VjcmV0") || strings.Contains(got, "plain") {
		t.Fatalf("kubernetes secret leaked: %s", got)
	}
	if strings.Count(got, "[REDACTED_SECRET_DATA]") != 2 {
		t.Fatalf("secret data was not fully redacted: %s", got)
	}
}

func TestCustomPatternsAndIdempotency(t *testing.T) {
	engine := testEngine(t, config.Config{CustomPatterns: []config.CustomPattern{{
		Name:        "customer_id",
		Pattern:     `customer-[0-9]{8}`,
		Replacement: "[REDACTED_CUSTOMER_ID]",
	}}})
	first, _, err := engine.SanitizeText("llm", "customer-12345678 customer-12345678")
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := engine.SanitizeText("llm", first)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("sanitize should be idempotent: first=%q second=%q", first, second)
	}
	if strings.Contains(first, "customer-12345678") {
		t.Fatalf("custom pattern leaked: %s", first)
	}
}

func TestPseudonymizationIsStable(t *testing.T) {
	engine := testEngine(t, config.Config{RedactResourceNames: true, HashKey: "test-hash-key"})
	payload := json.RawMessage(`{"pod":"payment-api-7b6d9f8c9-x4k21","namespace":"production","again":{"pod":"payment-api-7b6d9f8c9-x4k21"}}`)
	out, _, err := engine.SanitizePayload("llm", payload)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(out)
	got := string(b)
	if strings.Contains(got, "payment-api") || strings.Contains(got, "production") {
		t.Fatalf("resource name leaked: %s", got)
	}
	if strings.Count(got, "pod-") != 2 {
		t.Fatalf("stable pod pseudonym missing twice: %s", got)
	}
}

func TestUnknownDestinationAndMaxDepth(t *testing.T) {
	engine := testEngine(t, config.Config{MaxDepth: 2})
	if _, _, err := engine.SanitizeText("unknown", "Authorization: Bearer secret-token"); err == nil {
		t.Fatalf("unknown destination must be rejected")
	}
	if _, _, err := engine.SanitizePayload("llm", json.RawMessage(`{"a":{"b":{"c":"too deep"}}}`)); err == nil {
		t.Fatalf("max depth must be enforced")
	}
}
