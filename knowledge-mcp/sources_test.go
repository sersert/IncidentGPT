package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func credentials(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "provider-token"), []byte("provider-credential"), 0600); err != nil {
		t.Fatal(err)
	}
	return dir
}
func TestProviderScopesProvenanceAndReadOnlyRequests(t *testing.T) {
	for _, kind := range []string{"github", "gitlab", "confluence"} {
		t.Run(kind, func(t *testing.T) {
			sha := strings.Repeat("a", 40)
			calls := 0
			api := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "GET" {
					t.Error("source was modified")
				}
				if kind == "gitlab" {
					if r.Header.Get("PRIVATE-TOKEN") != "provider-credential" {
						t.Error("missing GitLab credentials")
					}
				} else if r.Header.Get("Authorization") != "Bearer provider-credential" {
					t.Error("missing source credentials")
				}
				var result any
				switch kind {
				case "github":
					if r.URL.Path == "/search/code" {
						q := r.URL.Query().Get("q")
						if strings.Count(q, "repo:") != 1 || !strings.HasSuffix(q, "repo:org/docs") {
							t.Errorf("unscoped query: %s", q)
						}
						result = map[string]any{"items": []any{map[string]any{"path": "docs/oom.md", "sha": sha, "repository": map[string]string{"full_name": "org/docs"}}, map[string]any{"path": "secrets/password.md", "sha": sha, "repository": map[string]string{"full_name": "org/docs"}}, map[string]any{"path": "docs/foreign.md", "sha": sha, "repository": map[string]string{"full_name": "other/repo"}}}}
					} else {
						if r.URL.Path != "/repos/org/docs/git/blobs/"+sha {
							t.Errorf("unexpected fetch %s", r.URL.Path)
						}
						result = map[string]any{"content": base64.StdEncoding.EncodeToString([]byte("# OOM\nConsult this runbook.")), "encoding": "base64", "size": 32, "sha": sha}
					}
				case "gitlab":
					if strings.HasSuffix(r.URL.Path, "/search") {
						if r.URL.EscapedPath() != "/projects/org%2Fdocs/search" || r.URL.Query().Get("ref") != "release" || r.URL.Query().Get("regex") != "false" {
							t.Error("GitLab scope/ref lost")
						}
						result = []any{map[string]string{"path": "docs/oom.md", "ref": "release"}, map[string]string{"path": "docs/../secrets.md", "ref": "release"}}
					} else {
						if r.URL.Query().Get("ref") != "release" {
							t.Error("GitLab ref lost on fetch")
						}
						result = map[string]any{"content": base64.StdEncoding.EncodeToString([]byte("# OOM runbook")), "encoding": "base64", "size": 13, "file_path": "docs/oom.md", "commit_id": sha}
					}
				case "confluence":
					q := r.URL.Query().Get("cql")
					if !strings.Contains(q, `space IN ("OPS")`) || strings.Contains(q, "repo:evil") {
						t.Error("Confluence scope was injected")
					}
					result = map[string]any{"results": []any{map[string]any{"id": "42", "type": "page", "title": "OOM runbook", "space": map[string]string{"key": "OPS"}, "version": map[string]int{"number": 3}, "body": map[string]any{"storage": map[string]string{"value": "<p>OOM runbook</p><script>never execute this</script>"}}}, map[string]any{"id": "43", "type": "page", "space": map[string]string{"key": "PRIVATE"}}}}
				}
				_ = json.NewEncoder(w).Encode(result)
			}))
			defer api.Close()
			source := Source{ID: "fixture", Kind: kind, APIURL: api.URL, WebURL: "https://docs.example.com", Repository: "org/docs", Paths: []string{"docs"}, Spaces: []string{"OPS"}, TokenKey: "provider-token"}
			if kind == "gitlab" {
				source.Ref = "release"
			}
			if err := validateSources([]Source{source}); err != nil {
				t.Fatal(err)
			}
			docs, partial, err := source.search(context.Background(), api.Client(), credentials(t), "OOM repo:evil OR")
			if err != nil || partial || len(docs) != 1 {
				t.Fatalf("documents=%v partial=%v err=%v", docs, partial, err)
			}
			if docs[0].ID == "" || docs[0].Version == "" || docs[0].RetrievedAt.IsZero() || !strings.HasPrefix(docs[0].URL, "https://docs.example.com/") || strings.Contains(docs[0].Excerpt, "execute") {
				t.Fatal("provenance or HTML text boundary lost")
			}
			wantCalls := 2
			if kind == "confluence" {
				wantCalls = 1
			}
			if calls != wantCalls {
				t.Fatalf("out-of-scope files fetched: %d", calls)
			}
		})
	}
}
func TestSourceRedirectDoesNotForwardCredentials(t *testing.T) {
	var leaked atomic.Int32
	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked.Add(1) }))
	defer target.Close()
	api := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) }))
	defer api.Close()
	s := Source{Kind: "github", APIURL: api.URL, TokenKey: "provider-token", Repository: "org/docs"}
	_, _, err := s.search(context.Background(), api.Client(), credentials(t), "OOM")
	if err == nil || leaked.Load() != 0 {
		t.Fatal("redirect forwarded source credentials")
	}
}
func TestSourceConfigurationRequiresExplicitScopes(t *testing.T) {
	valid := Source{ID: "docs", Kind: "github", APIURL: "https://api.github.com", WebURL: "https://github.com", Repository: "org/docs", Paths: []string{"docs"}, TokenKey: "github-token"}
	for _, change := range []func(*Source){func(s *Source) { s.Paths = nil }, func(s *Source) { s.Paths = []string{"../secrets"} }, func(s *Source) { s.TokenKey = "../key" }, func(s *Source) { s.APIURL = "https://token@api.github.com" }, func(s *Source) { s.Ref = "other-branch" }, func(s *Source) { s.Kind = "confluence"; s.Spaces = nil }} {
		s := valid
		change(&s)
		if validateSources([]Source{s}) == nil {
			t.Fatalf("accepted unsafe config: %#v", s)
		}
	}
	if valid.allows("docs/../secret.md") || valid.allows("docs-extra/secret.md") || valid.allows("docs/.env") || !valid.allows("docs/runbooks/oom.md") {
		t.Fatal("document path scoping failed")
	}
}

type authRoundTripper struct {
	base  http.RoundTripper
	token string
}

func (t authRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header = r.Header.Clone()
	r.Header.Set("Authorization", "Bearer "+t.token)
	return t.base.RoundTrip(r)
}
func TestMCPWireAuthenticationSanitizationAndOutage(t *testing.T) {
	var sourceCalls atomic.Int32
	api := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sourceCalls.Add(1)
		if strings.Contains(r.URL.RawQuery, "sensitive") {
			t.Error("unsanitized query left server")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{map[string]any{"id": "42", "type": "page", "title": "Runbook", "space": map[string]string{"key": "OPS"}, "version": map[string]int{"number": 1}, "body": map[string]any{"storage": map[string]string{"value": "sensitive documentation"}}}}})
	}))
	defer api.Close()
	sanitizer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-IncidentGPT-Signature") == "" {
			t.Error("unsigned sanitizer request")
		}
		var req map[string]json.RawMessage
		_ = json.NewDecoder(r.Body).Decode(&req)
		if r.URL.Path == "/v1/sanitize/text" {
			var text string
			_ = json.Unmarshal(req["text"], &text)
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "sanitized", "text": strings.ReplaceAll(text, "sensitive", "redacted")})
		} else {
			raw := strings.ReplaceAll(string(req["payload"]), "sensitive", "redacted")
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "sanitized", "payload": json.RawMessage(raw)})
		}
	}))
	defer sanitizer.Close()
	svc := &Service{sources: []Source{{ID: "ops", Kind: "confluence", APIURL: api.URL, WebURL: "https://wiki.example.com", Spaces: []string{"OPS"}, TokenKey: "provider-token"}}, credentialsDir: credentials(t), client: api.Client(), sanitizer: newSanitizerClient(sanitizer.URL, "hmac", time.Second), slots: make(chan struct{}, 8)}
	token := strings.Repeat("t", 32)
	server := httptest.NewServer(svc.handler(token + "\n"))
	defer server.Close()
	for _, scenario := range []struct {
		auth, origin string
		want         int
	}{{"", "", 401}, {"Bearer " + token, "https://untrusted.example", 403}} {
		r, _ := http.NewRequest("POST", server.URL+"/mcp", strings.NewReader(`{}`))
		r.Header.Set("Authorization", scenario.auth)
		r.Header.Set("Origin", scenario.origin)
		resp, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != scenario.want {
			t.Fatalf("guard=%d", resp.StatusCode)
		}
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: server.URL + "/mcp", HTTPClient: &http.Client{Transport: authRoundTripper{base: http.DefaultTransport, token: token}}, DisableStandaloneSSE: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	tools, err := session.ListTools(ctx, nil)
	if err != nil || len(tools.Tools) != 1 || tools.Tools[0].Name != "search_knowledge" || !tools.Tools[0].Annotations.ReadOnlyHint {
		t.Fatal("read-only MCP tool not advertised")
	}
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "search_knowledge", Arguments: map[string]string{"query": "sensitive OOM"}})
	if err != nil || result.IsError {
		t.Fatalf("MCP search failed: %v %#v", err, result)
	}
	raw, _ := json.Marshal(result.StructuredContent)
	if !strings.Contains(string(raw), "redacted documentation") || strings.Contains(string(raw), "sensitive") || sourceCalls.Load() != 1 {
		t.Fatal("sanitized document was not returned over MCP")
	}
	sanitizer.Close()
	before := sourceCalls.Load()
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "search_knowledge", Arguments: map[string]string{"query": "sensitive OOM"}})
	if err == nil && !result.IsError {
		t.Fatal("sanitizer outage returned success")
	}
	if sourceCalls.Load() != before {
		t.Fatal("sanitizer outage leaked query to source")
	}
}
