package policy

import (
	"fmt"

	"incidentgpt-sanitizer/internal/config"
)

type Policy struct {
	Destination         string
	RedactEmails        bool
	RedactPhones        bool
	RedactCertificates  bool
	RedactUsernames     bool
	RedactResourceNames bool
	IPMode              string
}

type Engine struct {
	defaults config.Config
}

func NewEngine(cfg config.Config) *Engine {
	return &Engine{defaults: cfg}
}

func (e *Engine) ForDestination(destination string) (Policy, error) {
	p := Policy{
		Destination:         destination,
		RedactEmails:        e.defaults.RedactEmails,
		RedactPhones:        e.defaults.RedactPhones,
		RedactCertificates:  e.defaults.RedactCertificates,
		RedactUsernames:     e.defaults.RedactUsernames,
		RedactResourceNames: e.defaults.RedactResourceNames,
		IPMode:              e.defaults.IPMode,
	}
	switch destination {
	case "ai-worker":
		return p, nil
	case "llm":
		p.RedactEmails = true
		p.RedactPhones = true
		if p.IPMode == "none" {
			p.IPMode = "partial"
		}
		p.RedactResourceNames = e.defaults.RedactResourceNames
		return p, nil
	case "telegram":
		p.RedactEmails = true
		p.RedactPhones = true
		if p.IPMode == "none" {
			p.IPMode = "partial"
		}
		return p, nil
	case "logs", "webhook", "debug":
		p.RedactEmails = true
		p.RedactPhones = true
		p.RedactCertificates = true
		p.IPMode = "partial"
		return p, nil
	default:
		return Policy{}, fmt.Errorf("unknown destination")
	}
}
