package domain

import "encoding/json"

type Summary struct {
	RedactionCount int            `json:"redaction_count"`
	AppliedRules   map[string]int `json:"applied_rules"`
}

func (s *Summary) Add(rule string) {
	if s.AppliedRules == nil {
		s.AppliedRules = make(map[string]int)
	}
	s.RedactionCount++
	s.AppliedRules[rule]++
}

type SanitizeRequest struct {
	RequestID   string          `json:"request_id"`
	Source      string          `json:"source"`
	Destination string          `json:"destination"`
	ContentType string          `json:"content_type"`
	Payload     json.RawMessage `json:"payload"`
}

type SanitizeResponse struct {
	RequestID string  `json:"request_id"`
	Status    string  `json:"status"`
	Payload   any     `json:"payload"`
	Summary   Summary `json:"summary"`
}

type TextRequest struct {
	RequestID   string `json:"request_id"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Text        string `json:"text"`
}

type TextResponse struct {
	RequestID string  `json:"request_id"`
	Status    string  `json:"status"`
	Text      string  `json:"text"`
	Summary   Summary `json:"summary"`
}

type InspectRequest struct {
	RequestID   string          `json:"request_id"`
	Source      string          `json:"source"`
	Destination string          `json:"destination"`
	ContentType string          `json:"content_type"`
	Payload     json.RawMessage `json:"payload,omitempty"`
	Text        string          `json:"text,omitempty"`
}

type Finding struct {
	Rule   string `json:"rule"`
	Path   string `json:"path"`
	Action string `json:"action"`
}

type InspectResponse struct {
	RequestID             string    `json:"request_id"`
	ContainsSensitiveData bool      `json:"contains_sensitive_data"`
	Findings              []Finding `json:"findings"`
}

type ErrorResponse struct {
	RequestID string       `json:"request_id,omitempty"`
	Error     ErrorDetails `json:"error"`
}

type ErrorDetails struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

const (
	ErrInvalidRequest         = "INVALID_REQUEST"
	ErrUnsupportedContentType = "UNSUPPORTED_CONTENT_TYPE"
	ErrPayloadTooLarge        = "PAYLOAD_TOO_LARGE"
	ErrMaxDepthExceeded       = "MAX_DEPTH_EXCEEDED"
	ErrSanitizationFailed     = "SANITIZATION_FAILED"
	ErrPolicyRejected         = "POLICY_REJECTED"
	ErrConfiguration          = "CONFIGURATION_ERROR"
	ErrInternal               = "INTERNAL_ERROR"
)
