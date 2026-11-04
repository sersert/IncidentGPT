package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type Source struct {
	ID          string   `json:"id"`
	Kind        string   `json:"kind"`
	APIURL      string   `json:"api_url"`
	WebURL      string   `json:"web_url"`
	Repository  string   `json:"repository,omitempty"`
	Ref         string   `json:"ref,omitempty"`
	Paths       []string `json:"paths,omitempty"`
	Spaces      []string `json:"spaces,omitempty"`
	TokenKey    string   `json:"token_key"`
	UsernameKey string   `json:"username_key,omitempty"`
}

type Document struct {
	ID          string    `json:"reference_id"`
	SourceID    string    `json:"source_id"`
	Provider    string    `json:"provider"`
	Title       string    `json:"title"`
	URL         string    `json:"url"`
	Version     string    `json:"version"`
	Excerpt     string    `json:"excerpt"`
	RetrievedAt time.Time `json:"retrieved_at"`
}
type SourceStatus struct {
	SourceID string `json:"source_id"`
	Status   string `json:"status"`
}
type SearchInput struct {
	Query string `json:"query" jsonschema:"Short literal error, service or runbook search terms; never credentials or instructions"`
}
type SearchOutput struct {
	Documents []Document     `json:"documents"`
	Sources   []SourceStatus `json:"sources"`
	Truncated bool           `json:"truncated"`
}

var identifier = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
var credentialKey = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]{0,252}$`)
var repoPart = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
var spaceKey = regexp.MustCompile(`^[A-Za-z0-9_~-]{1,100}$`)
var objectHash = regexp.MustCompile(`^[a-fA-F0-9]{40,64}$`)
var tinyURL = regexp.MustCompile(`^/x/[A-Za-z0-9_-]{1,32}$`)
var tags = regexp.MustCompile(`(?s)<[^>]*>`)
var scripts = regexp.MustCompile(`(?is)<script\b[^>]*>.*?</script\s*>|<style\b[^>]*>.*?</style\s*>`)

func validateSources(sources []Source) error {
	if len(sources) == 0 || len(sources) > 8 {
		return fmt.Errorf("configure between one and eight sources")
	}
	seen := map[string]bool{}
	for _, s := range sources {
		if !identifier.MatchString(s.ID) || seen[s.ID] || !credentialKey.MatchString(s.TokenKey) || (s.UsernameKey != "" && !credentialKey.MatchString(s.UsernameKey)) {
			return fmt.Errorf("invalid source identity or credential key")
		}
		seen[s.ID] = true
		for _, raw := range []string{s.APIURL, s.WebURL} {
			u, e := url.Parse(raw)
			if e != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
				return fmt.Errorf("source URLs must be fixed HTTPS endpoints without credentials or query")
			}
		}
		if s.Kind == "confluence" {
			if len(s.Spaces) == 0 || len(s.Spaces) > 10 {
				return fmt.Errorf("Confluence requires explicit spaces")
			}
			for _, key := range s.Spaces {
				if !spaceKey.MatchString(key) {
					return fmt.Errorf("invalid Confluence space")
				}
			}
			continue
		}
		if s.Kind != "github" && s.Kind != "gitlab" {
			return fmt.Errorf("unsupported source kind")
		}
		parts := strings.Split(s.Repository, "/")
		if s.Repository == "" || (s.Kind == "github" && len(parts) != 2) || (s.Kind == "gitlab" && len(parts) < 2) {
			return fmt.Errorf("explicit repository required")
		}
		for _, part := range parts {
			if !repoPart.MatchString(part) || part == "." || part == ".." {
				return fmt.Errorf("invalid repository")
			}
		}
		if s.Kind == "github" && s.Ref != "" {
			return fmt.Errorf("GitHub code search supports only the default branch")
		}
		if len(s.Ref) > 200 || strings.ContainsAny(s.Ref, "\r\n\x00") || len(s.Paths) == 0 || len(s.Paths) > 10 {
			return fmt.Errorf("invalid repository scope")
		}
		for _, prefix := range s.Paths {
			if len(prefix) > 200 || prefix == "" || path.IsAbs(prefix) || path.Clean(prefix) != prefix || strings.ContainsAny(prefix, "\\\r\n\x00") || prefix == ".." || strings.HasPrefix(prefix, "../") {
				return fmt.Errorf("invalid document path scope")
			}
		}
	}
	return nil
}

func bounded(s string, n int) string {
	s = strings.ToValidUTF8(s, "")
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}
func terms(query string) []string {
	words := strings.FieldsFunc(query, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-' && r != '.'
	})
	if len(words) > 8 {
		words = words[:8]
	}
	return words
}
func (s Source) allows(file string) bool {
	if path.IsAbs(file) || path.Clean(file) != file || strings.Contains(file, "\\") || strings.HasPrefix(file, "../") {
		return false
	}
	switch strings.ToLower(path.Ext(file)) {
	case ".md", ".markdown", ".txt", ".rst", ".adoc":
	default:
		return false
	}
	for _, prefix := range s.Paths {
		if prefix == "." || file == prefix || strings.HasPrefix(file, prefix+"/") {
			return true
		}
	}
	return false
}
func (s Source) get(ctx context.Context, client *http.Client, dir, endpoint string, query url.Values, out any) error {
	credential, err := os.ReadFile(filepath.Join(dir, s.TokenKey))
	if err != nil || len(credential) == 0 || len(credential) > 8192 {
		return fmt.Errorf("source credentials unavailable")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(s.APIURL, "/")+endpoint+"?"+query.Encode(), nil)
	if err != nil {
		return fmt.Errorf("source request invalid")
	}
	token := strings.TrimSpace(string(credential))
	if token == "" || strings.ContainsAny(token, "\r\n") {
		return fmt.Errorf("source credentials invalid")
	}
	if s.Kind == "confluence" && s.UsernameKey != "" {
		user, e := os.ReadFile(filepath.Join(dir, s.UsernameKey))
		if e != nil || len(user) > 500 {
			return fmt.Errorf("source username unavailable")
		}
		req.SetBasicAuth(strings.TrimSpace(string(user)), token)
	} else if s.Kind == "gitlab" {
		req.Header.Set("PRIVATE-TOKEN", token)
	} else {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "IncidentGPT-Knowledge-MCP")
	safeClient := *client
	safeClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := safeClient.Do(req)
	if err != nil {
		return fmt.Errorf("source unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("source HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, (512<<10)+1))
	if err != nil || len(raw) > 512<<10 {
		return fmt.Errorf("source response exceeds budget")
	}
	if json.Unmarshal(raw, out) != nil {
		return fmt.Errorf("invalid source response")
	}
	return nil
}
func makeDocument(s Source, id, version, title, text, link, query string) Document {
	text = strings.ToValidUTF8(text, "")
	lower := strings.ToLower(text)
	start := 0
	for _, word := range terms(query) {
		if i := strings.Index(lower, strings.ToLower(word)); i >= 0 {
			start = max(0, i-512)
			for start > 0 && !utf8.RuneStart(text[start]) {
				start--
			}
			break
		}
	}
	hash := sha256.Sum256([]byte(s.ID + "\x00" + id + "\x00" + version + "\x00" + text))
	return Document{ID: "kb-" + hex.EncodeToString(hash[:]), SourceID: s.ID, Provider: s.Kind, Title: bounded(title, 200), URL: link, Version: bounded(version, 128), Excerpt: bounded(text[start:], 4096), RetrievedAt: time.Now().UTC()}
}
func escapedPath(p string) string {
	parts := strings.Split(p, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}

func (s Source) search(ctx context.Context, client *http.Client, dir, query string) ([]Document, bool, error) {
	if s.Kind == "confluence" {
		return s.searchConfluence(ctx, client, dir, query)
	}
	if s.Kind == "github" {
		return s.searchGitHub(ctx, client, dir, query)
	}
	return s.searchGitLab(ctx, client, dir, query)
}
func (s Source) searchGitHub(ctx context.Context, client *http.Client, dir, query string) ([]Document, bool, error) {
	quoted := []string{}
	for _, word := range terms(query) {
		quoted = append(quoted, strconv.Quote(word))
	}
	var result struct {
		Incomplete bool `json:"incomplete_results"`
		Total      int  `json:"total_count"`
		Items      []struct {
			Path       string `json:"path"`
			SHA        string `json:"sha"`
			Repository struct {
				FullName string `json:"full_name"`
			} `json:"repository"`
		} `json:"items"`
	}
	if err := s.get(ctx, client, dir, "/search/code", url.Values{"q": {strings.Join(quoted, " ") + " repo:" + s.Repository}, "per_page": {"20"}}, &result); err != nil {
		return nil, false, err
	}
	docs := []Document{}
	partial := result.Incomplete || result.Total > len(result.Items)
	for _, item := range result.Items {
		if len(docs) == 2 {
			partial = true
			break
		}
		if !strings.EqualFold(item.Repository.FullName, s.Repository) || !s.allows(item.Path) || !objectHash.MatchString(item.SHA) {
			continue
		}
		var blob struct {
			Content  string `json:"content"`
			Encoding string `json:"encoding"`
			Size     int    `json:"size"`
			SHA      string `json:"sha"`
		}
		if err := s.get(ctx, client, dir, "/repos/"+s.Repository+"/git/blobs/"+item.SHA, nil, &blob); err != nil {
			partial = true
			continue
		}
		text, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(blob.Content, "\n", ""))
		if err != nil || blob.Encoding != "base64" || len(text) > 64<<10 || blob.Size > 64<<10 || blob.SHA != item.SHA {
			partial = true
			continue
		}
		docs = append(docs, makeDocument(s, item.Path, item.SHA, item.Path, string(text), strings.TrimRight(s.WebURL, "/")+"/"+s.Repository+"/blob/HEAD/"+escapedPath(item.Path), query))
	}
	return docs, partial, nil
}
func (s Source) searchGitLab(ctx context.Context, client *http.Client, dir, query string) ([]Document, bool, error) {
	var items []struct {
		Path string `json:"path"`
		Ref  string `json:"ref"`
	}
	project := "/projects/" + url.PathEscape(s.Repository)
	params := url.Values{"scope": {"blobs"}, "search": {strings.Join(terms(query), " ")}, "regex": {"false"}, "per_page": {"20"}}
	if s.Ref != "" {
		params.Set("ref", s.Ref)
	}
	if err := s.get(ctx, client, dir, project+"/search", params, &items); err != nil {
		return nil, false, err
	}
	docs := []Document{}
	seen := map[string]bool{}
	partial := len(items) >= 20
	for _, item := range items {
		if len(docs) == 2 {
			partial = true
			break
		}
		if !s.allows(item.Path) || seen[item.Path] {
			continue
		}
		seen[item.Path] = true
		ref := s.Ref
		if ref == "" {
			ref = item.Ref
		}
		if ref == "" {
			partial = true
			continue
		}
		var file struct {
			Content  string `json:"content"`
			Encoding string `json:"encoding"`
			Size     int    `json:"size"`
			Path     string `json:"file_path"`
			Commit   string `json:"commit_id"`
		}
		if err := s.get(ctx, client, dir, project+"/repository/files/"+url.PathEscape(item.Path), url.Values{"ref": {ref}}, &file); err != nil {
			partial = true
			continue
		}
		text, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(file.Content, "\n", ""))
		if err != nil || file.Encoding != "base64" || len(text) > 64<<10 || file.Size > 64<<10 || file.Path != item.Path || !objectHash.MatchString(file.Commit) {
			partial = true
			continue
		}
		docs = append(docs, makeDocument(s, item.Path, file.Commit, item.Path, string(text), strings.TrimRight(s.WebURL, "/")+"/"+s.Repository+"/-/blob/"+file.Commit+"/"+escapedPath(item.Path), query))
	}
	return docs, partial, nil
}
func (s Source) searchConfluence(ctx context.Context, client *http.Client, dir, query string) ([]Document, bool, error) {
	spaces := []string{}
	for _, key := range s.Spaces {
		spaces = append(spaces, strconv.Quote(key))
	}
	cql := "type=page AND status=current AND space IN (" + strings.Join(spaces, ",") + ") AND text ~ " + strconv.Quote(strings.Join(terms(query), " "))
	var result struct {
		Results []struct {
			ID    string `json:"id"`
			Type  string `json:"type"`
			Title string `json:"title"`
			Links struct {
				Tiny string `json:"tinyui"`
			} `json:"_links"`
			Space struct {
				Key string `json:"key"`
			} `json:"space"`
			Version struct {
				Number int `json:"number"`
			} `json:"version"`
			Body struct {
				Storage struct {
					Value string `json:"value"`
				} `json:"storage"`
			} `json:"body"`
		} `json:"results"`
		Links struct {
			Next string `json:"next"`
		} `json:"_links"`
	}
	if err := s.get(ctx, client, dir, "/content/search", url.Values{"cql": {cql}, "expand": {"space,version,body.storage"}, "limit": {"5"}}, &result); err != nil {
		return nil, false, err
	}
	docs := []Document{}
	partial := result.Links.Next != ""
	for _, item := range result.Results {
		if len(docs) == 2 {
			partial = true
			break
		}
		allowed := false
		for _, space := range s.Spaces {
			allowed = allowed || space == item.Space.Key
		}
		if !allowed || item.Type != "page" {
			continue
		}
		if _, err := strconv.ParseUint(item.ID, 10, 64); err != nil || item.Version.Number < 1 || len(item.Body.Storage.Value) > 64<<10 {
			partial = true
			continue
		}
		text := html.UnescapeString(tags.ReplaceAllString(scripts.ReplaceAllString(item.Body.Storage.Value, " "), " "))
		link := strings.TrimRight(s.WebURL, "/") + "/pages/viewpage.action?pageId=" + item.ID
		if tinyURL.MatchString(item.Links.Tiny) {
			link = strings.TrimRight(s.WebURL, "/") + item.Links.Tiny
		}
		docs = append(docs, makeDocument(s, item.ID, strconv.Itoa(item.Version.Number), item.Title, text, link, query))
	}
	return docs, partial, nil
}
