package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"incidentgpt-sanitizer/internal/audit"
	"incidentgpt-sanitizer/internal/auth"
	"incidentgpt-sanitizer/internal/config"
	"incidentgpt-sanitizer/internal/domain"
	"incidentgpt-sanitizer/internal/masking"
	"incidentgpt-sanitizer/internal/metrics"
)

type Server struct {
	cfg      config.Config
	engine   *masking.Engine
	auth     *auth.Verifier
	metrics  *metrics.Metrics
	readyErr error
}

func New(cfg config.Config, engine *masking.Engine, verifier *auth.Verifier, m *metrics.Metrics, readyErr error) *Server {
	return &Server{cfg: cfg, engine: engine, auth: verifier, metrics: m, readyErr: readyErr}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.healthz)
	mux.HandleFunc("/readyz", s.readyz)
	mux.HandleFunc("/version", s.version)
	mux.HandleFunc("/metrics", s.metrics.Handler)
	mux.HandleFunc("/v1/sanitize", s.sanitize)
	mux.HandleFunc("/v1/sanitize/text", s.sanitizeText)
	mux.HandleFunc("/v1/inspect", s.inspect)
	return mux
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) readyz(w http.ResponseWriter, _ *http.Request) {
	if s.readyErr != nil {
		writeError(w, "", http.StatusServiceUnavailable, domain.ErrConfiguration)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ready"))
}

func (s *Server) version(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"service":    "incidentgpt-sanitizer",
		"version":    s.cfg.Version,
		"commit":     s.cfg.Commit,
		"build_time": s.cfg.BuildTime,
	})
}

func (s *Server) sanitize(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	var req domain.SanitizeRequest
	body, ok := s.readSignedRequest(w, r, s.cfg.MaxInputBytes, &req)
	if !ok {
		return
	}
	labels := map[string]string{"source": req.Source, "destination": req.Destination, "status": "error"}
	defer func() { s.metrics.ObserveDuration(req.Source, req.Destination, labels["status"], start) }()
	s.metrics.Add("incidentgpt_sanitizer_input_bytes_total", map[string]string{"source": req.Source, "destination": req.Destination}, float64(len(body)))
	if req.ContentType != "" && req.ContentType != "application/json" {
		s.fail(w, req.RequestID, req.Source, req.Destination, http.StatusUnsupportedMediaType, domain.ErrUnsupportedContentType)
		return
	}
	payload, summary, err := s.engine.SanitizePayload(req.Destination, req.Payload)
	if err != nil {
		code := domain.ErrSanitizationFailed
		status := http.StatusUnprocessableEntity
		if errors.Is(err, io.ErrUnexpectedEOF) {
			code = domain.ErrInvalidRequest
			status = http.StatusBadRequest
		}
		if err.Error() == "unknown destination" {
			code = domain.ErrPolicyRejected
			status = http.StatusUnprocessableEntity
		}
		if err.Error() == "max depth exceeded" {
			code = domain.ErrMaxDepthExceeded
			status = http.StatusRequestEntityTooLarge
		}
		s.fail(w, req.RequestID, req.Source, req.Destination, status, code)
		return
	}
	labels["status"] = "sanitized"
	s.metrics.Inc("incidentgpt_sanitizer_requests_total", labels)
	s.metrics.Add("incidentgpt_sanitizer_redactions_total", map[string]string{"source": req.Source, "destination": req.Destination}, float64(summary.RedactionCount))
	audit.Log(req.RequestID, req.Source, req.Destination, decision(summary), summary)
	resp := domain.SanitizeResponse{RequestID: req.RequestID, Status: "sanitized", Payload: payload, Summary: summary}
	if out, err := json.Marshal(resp); err == nil {
		s.metrics.Add("incidentgpt_sanitizer_output_bytes_total", map[string]string{"source": req.Source, "destination": req.Destination}, float64(len(out)))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) sanitizeText(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	var req domain.TextRequest
	body, ok := s.readSignedRequest(w, r, s.cfg.MaxInputBytes, &req)
	if !ok {
		return
	}
	labels := map[string]string{"source": req.Source, "destination": req.Destination, "status": "error"}
	defer func() { s.metrics.ObserveDuration(req.Source, req.Destination, labels["status"], start) }()
	s.metrics.Add("incidentgpt_sanitizer_input_bytes_total", map[string]string{"source": req.Source, "destination": req.Destination}, float64(len(body)))
	text, summary, err := s.engine.SanitizeText(req.Destination, req.Text)
	if err != nil {
		code := domain.ErrSanitizationFailed
		if err.Error() == "unknown destination" {
			code = domain.ErrPolicyRejected
		}
		s.fail(w, req.RequestID, req.Source, req.Destination, http.StatusUnprocessableEntity, code)
		return
	}
	labels["status"] = "sanitized"
	s.metrics.Inc("incidentgpt_sanitizer_requests_total", labels)
	s.metrics.Add("incidentgpt_sanitizer_redactions_total", map[string]string{"source": req.Source, "destination": req.Destination}, float64(summary.RedactionCount))
	audit.Log(req.RequestID, req.Source, req.Destination, decision(summary), summary)
	resp := domain.TextResponse{RequestID: req.RequestID, Status: "sanitized", Text: text, Summary: summary}
	if out, err := json.Marshal(resp); err == nil {
		s.metrics.Add("incidentgpt_sanitizer_output_bytes_total", map[string]string{"source": req.Source, "destination": req.Destination}, float64(len(out)))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) inspect(w http.ResponseWriter, r *http.Request) {
	var req domain.InspectRequest
	if _, ok := s.readSignedRequest(w, r, s.cfg.MaxInputBytes, &req); !ok {
		return
	}
	findings, err := s.engine.InspectPayload(req.Destination, req.Payload)
	if err != nil {
		s.fail(w, req.RequestID, req.Source, req.Destination, http.StatusUnprocessableEntity, domain.ErrSanitizationFailed)
		return
	}
	if req.Text != "" {
		textFindings, err := s.engine.InspectText(req.Destination, req.Text)
		if err != nil {
			s.fail(w, req.RequestID, req.Source, req.Destination, http.StatusUnprocessableEntity, domain.ErrPolicyRejected)
			return
		}
		findings = append(findings, textFindings...)
	}
	writeJSON(w, http.StatusOK, domain.InspectResponse{RequestID: req.RequestID, ContainsSensitiveData: len(findings) > 0, Findings: findings})
}

func (s *Server) readSignedRequest(w http.ResponseWriter, r *http.Request, limit int64, dst any) ([]byte, bool) {
	if r.Method != http.MethodPost {
		writeError(w, "", http.StatusMethodNotAllowed, domain.ErrInvalidRequest)
		return nil, false
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, limit+1))
	if err != nil {
		writeError(w, "", http.StatusBadRequest, domain.ErrInvalidRequest)
		return nil, false
	}
	defer r.Body.Close()
	if int64(len(body)) > limit {
		writeError(w, "", http.StatusRequestEntityTooLarge, domain.ErrPayloadTooLarge)
		return nil, false
	}
	if err := s.auth.Verify(r, body); err != nil {
		s.metrics.Inc("incidentgpt_sanitizer_auth_failures_total", nil)
		log.Printf("WARN: sanitizer auth failed path=%s: %v", r.URL.Path, err)
		writeError(w, "", http.StatusUnauthorized, domain.ErrInvalidRequest)
		return nil, false
	}
	if err := json.Unmarshal(body, dst); err != nil {
		writeError(w, "", http.StatusBadRequest, domain.ErrInvalidRequest)
		return nil, false
	}
	return body, true
}

func (s *Server) fail(w http.ResponseWriter, requestID, source, destination string, status int, code string) {
	s.metrics.Inc("incidentgpt_sanitizer_failures_total", map[string]string{"source": source, "destination": destination, "error_code": code})
	if code == domain.ErrPolicyRejected {
		s.metrics.Inc("incidentgpt_sanitizer_denied_total", map[string]string{"source": source, "destination": destination, "error_code": code})
	}
	audit.Log(requestID, source, destination, "error", domain.Summary{})
	writeError(w, requestID, status, code)
}

func decision(summary domain.Summary) string {
	if summary.RedactionCount > 0 {
		return "allow_after_sanitization"
	}
	return "allow"
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, requestID string, status int, code string) {
	msg := "Payload could not be safely processed"
	if code == domain.ErrConfiguration {
		msg = "Service is not ready"
	}
	writeJSON(w, status, domain.ErrorResponse{RequestID: requestID, Error: domain.ErrorDetails{Code: code, Message: msg}})
}

func ContextWithTimeout(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, timeout)
}
