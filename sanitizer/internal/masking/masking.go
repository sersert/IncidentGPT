package masking

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"incidentgpt-sanitizer/internal/config"
	"incidentgpt-sanitizer/internal/detector"
	"incidentgpt-sanitizer/internal/domain"
	"incidentgpt-sanitizer/internal/policy"
	"incidentgpt-sanitizer/internal/pseudonymizer"
)

type Engine struct {
	cfg      config.Config
	detector *detector.Detector
	policies *policy.Engine
	pseudo   *pseudonymizer.Pseudonymizer
}

func NewEngine(cfg config.Config, d *detector.Detector, pe *policy.Engine, p *pseudonymizer.Pseudonymizer) *Engine {
	return &Engine{cfg: cfg, detector: d, policies: pe, pseudo: p}
}

func (e *Engine) SanitizePayload(destination string, payload json.RawMessage) (any, domain.Summary, error) {
	pol, err := e.policies.ForDestination(destination)
	if err != nil {
		return nil, domain.Summary{}, err
	}
	var v any
	if err := json.Unmarshal(payload, &v); err != nil {
		return nil, domain.Summary{}, err
	}
	summary := domain.Summary{AppliedRules: map[string]int{}}
	out, err := e.sanitizeValue(v, "", "$", 0, pol, &summary)
	if err != nil {
		return nil, summary, err
	}
	return out, summary, nil
}

func (e *Engine) SanitizeText(destination, text string) (string, domain.Summary, error) {
	pol, err := e.policies.ForDestination(destination)
	if err != nil {
		return "", domain.Summary{}, err
	}
	summary := domain.Summary{AppliedRules: map[string]int{}}
	return e.detector.SanitizeString(text, pol, &summary), summary, nil
}

func (e *Engine) InspectPayload(destination string, payload json.RawMessage) ([]domain.Finding, error) {
	pol, err := e.policies.ForDestination(destination)
	if err != nil {
		return nil, err
	}
	var v any
	if len(payload) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal(payload, &v); err != nil {
		return nil, err
	}
	return e.inspectValue(v, "", "$", 0, pol)
}

func (e *Engine) InspectText(destination, text string) ([]domain.Finding, error) {
	pol, err := e.policies.ForDestination(destination)
	if err != nil {
		return nil, err
	}
	return e.detector.InspectString(text, "$.text", pol), nil
}

func (e *Engine) sanitizeValue(v any, key, path string, depth int, pol policy.Policy, summary *domain.Summary) (any, error) {
	if depth > e.cfg.MaxDepth {
		return nil, fmt.Errorf("max depth exceeded")
	}
	if isSensitiveKey(key, pol.RedactUsernames) {
		summary.Add("sensitive_key")
		return placeholderForKey(key), nil
	}
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		if isKubernetesSecret(x) {
			for k, val := range x {
				if k == "data" || k == "stringData" || k == "binaryData" {
					out[k] = "[REDACTED_SECRET_DATA]"
					summary.Add("kubernetes_secret")
					continue
				}
				clean, err := e.sanitizeValue(val, k, path+"."+k, depth+1, pol, summary)
				if err != nil {
					return nil, err
				}
				out[k] = clean
			}
			return out, nil
		}
		for k, val := range x {
			clean, err := e.sanitizeValue(val, k, path+"."+k, depth+1, pol, summary)
			if err != nil {
				return nil, err
			}
			out[k] = clean
		}
		return out, nil
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			clean, err := e.sanitizeValue(val, key, fmt.Sprintf("%s[%d]", path, i), depth+1, pol, summary)
			if err != nil {
				return nil, err
			}
			out[i] = clean
		}
		return out, nil
	case string:
		if pol.RedactResourceNames && isResourceKey(key) {
			summary.Add("resource_name")
			return e.pseudo.Resource(resourceType(key), x), nil
		}
		return e.detector.SanitizeString(x, pol, summary), nil
	default:
		return v, nil
	}
}

func (e *Engine) inspectValue(v any, key, path string, depth int, pol policy.Policy) ([]domain.Finding, error) {
	if depth > e.cfg.MaxDepth {
		return nil, fmt.Errorf("max depth exceeded")
	}
	if isSensitiveKey(key, pol.RedactUsernames) {
		return []domain.Finding{{Rule: "sensitive_key", Path: path, Action: "redact"}}, nil
	}
	var findings []domain.Finding
	switch x := v.(type) {
	case map[string]any:
		if isKubernetesSecret(x) {
			for _, k := range []string{"data", "stringData", "binaryData"} {
				if _, ok := x[k]; ok {
					findings = append(findings, domain.Finding{Rule: "kubernetes_secret", Path: path + "." + k, Action: "redact"})
				}
			}
		}
		for k, val := range x {
			sub, err := e.inspectValue(val, k, path+"."+k, depth+1, pol)
			if err != nil {
				return nil, err
			}
			findings = append(findings, sub...)
		}
	case []any:
		for i, val := range x {
			sub, err := e.inspectValue(val, key, fmt.Sprintf("%s[%d]", path, i), depth+1, pol)
			if err != nil {
				return nil, err
			}
			findings = append(findings, sub...)
		}
	case string:
		findings = append(findings, e.detector.InspectString(x, path, pol)...)
		if pol.RedactResourceNames && isResourceKey(key) {
			findings = append(findings, domain.Finding{Rule: "resource_name", Path: path, Action: "pseudonymize"})
		}
	}
	return findings, nil
}

var sensitiveKeyRE = regexp.MustCompile(`(?i)(password|passwd|pwd|secret|token|api[_-]?key|access[_-]?key|private[_-]?key|client[_-]?secret|authorization|auth|credential|cookie|session)`)

func isSensitiveKey(key string, redactUsernames bool) bool {
	normalized := strings.ReplaceAll(key, ".", "_")
	if sensitiveKeyRE.MatchString(normalized) {
		return true
	}
	if redactUsernames {
		lower := strings.ToLower(key)
		return lower == "user" || strings.Contains(lower, "username") || strings.Contains(lower, "user_name")
	}
	return false
}

func placeholderForKey(key string) string {
	lower := strings.ToLower(key)
	switch {
	case strings.Contains(lower, "authorization"), strings.Contains(lower, "token"):
		return "[REDACTED_TOKEN]"
	case strings.Contains(lower, "cookie"), strings.Contains(lower, "session"):
		return "[REDACTED_SESSION]"
	case strings.Contains(lower, "key"):
		return "[REDACTED_KEY]"
	case strings.Contains(lower, "user"):
		return "[REDACTED_USERNAME]"
	default:
		return "[REDACTED_SECRET]"
	}
}

func isKubernetesSecret(m map[string]any) bool {
	kind, _ := m["kind"].(string)
	return strings.EqualFold(kind, "Secret")
}

func isResourceKey(key string) bool {
	switch strings.ToLower(key) {
	case "namespace", "pod", "pod_name", "deployment", "deployment_name", "statefulset", "daemonset", "service", "service_name", "node", "node_name", "container", "cluster", "cluster_name":
		return true
	default:
		return false
	}
}

func resourceType(key string) string {
	k := strings.ToLower(strings.TrimSuffix(key, "_name"))
	if k == "cluster_name" {
		return "cluster"
	}
	return k
}
