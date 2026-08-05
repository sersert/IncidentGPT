package detector

import (
	"net"
	"regexp"
	"strings"

	"incidentgpt-sanitizer/internal/config"
	"incidentgpt-sanitizer/internal/domain"
	"incidentgpt-sanitizer/internal/policy"
)

type Rule struct {
	Name        string
	Pattern     *regexp.Regexp
	Replacement string
}

type Detector struct {
	rules  []Rule
	custom []Rule
}

func New(customPatterns []config.CustomPattern) (*Detector, error) {
	d := &Detector{
		rules: []Rule{
			{Name: "private_key", Pattern: regexp.MustCompile(`(?s)-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----.*?-----END (?:RSA |EC |OPENSSH )?PRIVATE KEY-----`), Replacement: "[REDACTED_PRIVATE_KEY]"},
			{Name: "bearer_token", Pattern: regexp.MustCompile(`(?i)\b(Bearer)\s+[A-Za-z0-9._~+/=-]{12,}`), Replacement: "$1 [REDACTED_TOKEN]"},
			{Name: "basic_auth", Pattern: regexp.MustCompile(`(?i)\b(Basic)\s+[A-Za-z0-9+/=]{12,}`), Replacement: "$1 [REDACTED_BASIC_AUTH]"},
			{Name: "jwt", Pattern: regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\b`), Replacement: "[REDACTED_JWT]"},
			{Name: "connection_string", Pattern: regexp.MustCompile(`(?i)\b((?:postgres(?:ql)?|mysql|mongodb(?:\+srv)?|redis|rediss|amqp|amqps|kafka)://[^:\s/@]+):([^@\s]+)@`), Replacement: "$1:[REDACTED_PASSWORD]@"},
			{Name: "url_credentials", Pattern: regexp.MustCompile(`(?i)\b(https?://)[^/\s:@]+:[^@\s/]+@`), Replacement: "$1[REDACTED_CREDENTIALS]@"},
			{Name: "cookie", Pattern: regexp.MustCompile(`(?i)\b((?:Set-Cookie|Cookie|JSESSIONID|PHPSESSID|session_id|auth_session)\s*[:=]\s*)[^\s;]+`), Replacement: "$1[REDACTED_SESSION]"},
		},
	}
	for _, p := range customPatterns {
		re, err := regexp.Compile(p.Pattern)
		if err != nil {
			return nil, err
		}
		d.custom = append(d.custom, Rule{Name: "custom_" + p.Name, Pattern: re, Replacement: p.Replacement})
	}
	return d, nil
}

func (d *Detector) SanitizeString(s string, pol policy.Policy, summary *domain.Summary) string {
	out := s
	for _, rule := range d.rules {
		out = replace(rule, out, summary)
	}
	if pol.RedactEmails {
		out = redactEmails(out, summary)
	}
	if pol.RedactPhones {
		out = redactPhones(out, summary)
	}
	if pol.IPMode != "none" {
		out = redactIPs(out, pol.IPMode, summary)
	}
	for _, rule := range d.custom {
		out = replace(rule, out, summary)
	}
	return out
}

func (d *Detector) InspectString(s, path string, pol policy.Policy) []domain.Finding {
	var findings []domain.Finding
	for _, rule := range append(append([]Rule{}, d.rules...), d.custom...) {
		if rule.Pattern.MatchString(s) {
			findings = append(findings, domain.Finding{Rule: rule.Name, Path: path, Action: "redact"})
		}
	}
	if pol.RedactEmails && emailRE.MatchString(s) {
		findings = append(findings, domain.Finding{Rule: "email", Path: path, Action: "mask"})
	}
	if pol.RedactPhones && phoneRE.MatchString(s) {
		findings = append(findings, domain.Finding{Rule: "phone", Path: path, Action: "mask"})
	}
	if pol.IPMode != "none" && ipCandidateRE.MatchString(s) {
		findings = append(findings, domain.Finding{Rule: "ip_address", Path: path, Action: "mask"})
	}
	return findings
}

func replace(rule Rule, input string, summary *domain.Summary) string {
	matches := rule.Pattern.FindAllStringIndex(input, -1)
	if len(matches) == 0 {
		return input
	}
	for range matches {
		summary.Add(rule.Name)
	}
	return rule.Pattern.ReplaceAllString(input, rule.Replacement)
}

var emailRE = regexp.MustCompile(`\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b`)

func redactEmails(s string, summary *domain.Summary) string {
	return emailRE.ReplaceAllStringFunc(s, func(email string) string {
		summary.Add("email")
		parts := strings.SplitN(email, "@", 2)
		if len(parts[0]) == 0 {
			return "*****@" + parts[1]
		}
		return parts[0][:1] + "*****@" + parts[1]
	})
}

var phoneRE = regexp.MustCompile(`(?:\+?\d[\d ()-]{8,}\d)`)

func redactPhones(s string, summary *domain.Summary) string {
	return phoneRE.ReplaceAllStringFunc(s, func(phone string) string {
		digits := onlyDigits(phone)
		if len(digits) < 10 {
			return phone
		}
		summary.Add("phone")
		return "+*** *** ***-**-" + digits[len(digits)-2:]
	})
}

func onlyDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

var ipCandidateRE = regexp.MustCompile(`\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b|(?:[0-9a-fA-F]{0,4}:){2,}[0-9a-fA-F]{0,4}`)

func redactIPs(s, mode string, summary *domain.Summary) string {
	return ipCandidateRE.ReplaceAllStringFunc(s, func(candidate string) string {
		ip := net.ParseIP(candidate)
		if ip == nil {
			return candidate
		}
		summary.Add("ip_address")
		if mode == "full" {
			return "[REDACTED_IP]"
		}
		if v4 := ip.To4(); v4 != nil {
			return regexp.MustCompile(`^(\d+\.\d+)\.\d+\.\d+$`).ReplaceAllString(candidate, "$1.x.x")
		}
		parts := strings.Split(candidate, ":")
		if len(parts) > 2 {
			return parts[0] + ":" + parts[1] + ":x:x"
		}
		return "[REDACTED_IP]"
	})
}
