package main

import (
	"strings"
	"testing"
	"time"
)

func TestKnowledgeReferencesRemainSeparateFromIncidentEvidence(t *testing.T) {
	now := time.Now().UTC()
	ref := KnowledgeReference{ID: "kb-" + strings.Repeat("a", 64), SourceID: "ops", Provider: "confluence", Title: "Runbook", URL: "https://wiki.example.com/pages/viewpage.action?pageId=42", Version: "3", Excerpt: "Hypothesis guidance", RetrievedAt: now}
	input := AnalysisInput{IncidentID: "INC-1", AnalysisID: "A-1", Evidence: []Evidence{{ID: "E-1", IncidentID: "INC-1", Excerpt: "OOMKilled"}}}
	a := AnalysisVersion{Version: 1, ID: "A-1", IncidentID: "INC-1", EvidenceIDs: []string{"E-1"}, StartedAt: now, CompletedAt: now, Status: "complete", Summary: "OOM hypothesis", Hypotheses: []Hypothesis{{Text: "OOM", Supporting: []string{"E-1"}, KnowledgeIDs: []string{ref.ID}}}, Knowledge: &KnowledgeTrace{Status: "available", Calls: 1, References: []KnowledgeReference{ref}}}
	if err := validateAnalysis(a, input); err != nil {
		t.Fatal(err)
	}
	a.Hypotheses[0].Supporting = []string{ref.ID}
	if validateAnalysis(a, input) == nil {
		t.Fatal("documentation was accepted as incident evidence")
	}
	a.Hypotheses[0].Supporting = []string{"E-1"}
	a.Hypotheses[0].KnowledgeIDs = []string{"kb-" + strings.Repeat("b", 64)}
	if validateAnalysis(a, input) == nil {
		t.Fatal("fabricated document citation accepted")
	}
	for _, bad := range []string{"http://wiki.example.com/page", "https://token@wiki.example.com/page", "https://wiki.example.com/page?token=secret", "https://wiki.example.com/pages/viewpage.action?pageId=42&redirect=https://other.example"} {
		copy := ref
		copy.URL = bad
		if validateKnowledge(&KnowledgeTrace{Status: "available", Calls: 1, References: []KnowledgeReference{copy}}) == nil {
			t.Fatalf("unsafe reference URL accepted: %s", bad)
		}
	}
	a.Knowledge = nil
	a.Hypotheses[0].KnowledgeIDs = nil
	if err := validateAnalysis(a, input); err != nil {
		t.Fatalf("legacy analysis no longer readable: %v", err)
	}
}
