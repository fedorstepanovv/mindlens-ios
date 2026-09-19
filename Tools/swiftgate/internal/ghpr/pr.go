// Package ghpr posts the gate's verdict back to the pull request.
//
// It talks to the REST API directly rather than shelling out to `gh`, so the same
// binary behaves the same on a runner and on a laptop.
package ghpr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/gate"
	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/scan"
)

// Client is a minimal GitHub API client scoped to what the gate needs.
type Client struct {
	Token   string
	Repo    string // "owner/name"
	BaseURL string // defaults to https://api.github.com
	HTTP    *http.Client
}

func New(token, repo string) *Client {
	return &Client{
		Token:   token,
		Repo:    repo,
		BaseURL: "https://api.github.com",
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}
}

type comment struct {
	ID   int64  `json:"id"`
	Body string `json:"body"`
}

// UpsertComment edits the gate's existing comment carrying marker on the PR, or posts
// a new one. Editing matters: a gate that appends a comment on every push buries the
// discussion it is supposed to support. Each lane has its own marker, so each is its
// own sticky comment.
func (c *Client) UpsertComment(ctx context.Context, pr int, marker, body string) (string, error) {
	existing, err := c.findMarked(ctx, pr, marker)
	if err != nil {
		return "", err
	}

	payload, _ := json.Marshal(map[string]string{"body": body})

	if existing != 0 {
		var out struct {
			HTMLURL string `json:"html_url"`
		}
		err := c.do(ctx, http.MethodPatch,
			fmt.Sprintf("/repos/%s/issues/comments/%d", c.Repo, existing), payload, &out)
		return out.HTMLURL, err
	}

	var out struct {
		HTMLURL string `json:"html_url"`
	}
	err = c.do(ctx, http.MethodPost,
		fmt.Sprintf("/repos/%s/issues/%d/comments", c.Repo, pr), payload, &out)
	return out.HTMLURL, err
}

func (c *Client) findMarked(ctx context.Context, pr int, marker string) (int64, error) {
	var comments []comment
	err := c.do(ctx, http.MethodGet,
		fmt.Sprintf("/repos/%s/issues/%d/comments?per_page=100", c.Repo, pr), nil, &comments)
	if err != nil {
		return 0, err
	}
	for _, cm := range comments {
		if strings.Contains(cm.Body, marker) {
			return cm.ID, nil
		}
	}
	return 0, nil
}

type inlineComment struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Side string `json:"side"`
	Body string `json:"body"`
}

// PostInline attaches findings to the lines they are about.
//
// Best-effort by design: GitHub rejects the whole review if any comment points at a
// line outside the diff, so findings are filtered to lines this PR actually added and
// a failure here never fails the gate — the sticky comment already carries everything.
func (c *Client) PostInline(ctx context.Context, pr int, sha string, d scan.Diff, findings []gate.Finding) error {
	added := map[string]map[int]bool{}
	for _, f := range d.Files {
		lines := map[int]bool{}
		for _, l := range f.Added {
			lines[l.Number] = true
		}
		added[f.Path] = lines
	}

	var comments []inlineComment
	for _, f := range findings {
		if f.Line <= 0 || !added[f.File][f.Line] {
			continue
		}
		body := fmt.Sprintf("**%s %s**\n\n%s\n\n**Instead:** %s\n\n<sub>`%s`", f.Severity.Emoji(), f.Title, f.Detail, f.Fix, f.Rule)
		if f.Doc != "" {
			body += " · " + f.Doc
		}
		comments = append(comments, inlineComment{Path: f.File, Line: f.Line, Side: "RIGHT", Body: body + "</sub>"})
	}
	if len(comments) == 0 {
		return nil
	}

	payload, _ := json.Marshal(map[string]any{
		"commit_id": sha,
		"event":     "COMMENT",
		"comments":  comments,
	})
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/repos/%s/pulls/%d/reviews", c.Repo, pr), payload, nil)
}

// PullRequest is the subset of PR metadata the review reads.
type PullRequest struct {
	Title  string `json:"title"`
	Body   string `json:"body"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

// HasLabel reports whether the PR carries a label.
func (p PullRequest) HasLabel(name string) bool {
	for _, l := range p.Labels {
		if strings.EqualFold(l.Name, name) {
			return true
		}
	}
	return false
}

func (c *Client) PullRequest(ctx context.Context, pr int) (PullRequest, error) {
	var out PullRequest
	err := c.do(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/pulls/%d", c.Repo, pr), nil, &out)
	return out, err
}

func (c *Client) do(ctx context.Context, method, path string, body []byte, out any) error {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	payload, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("github %s %s: %s: %s", method, path, resp.Status, strings.TrimSpace(string(payload)))
	}
	if out != nil {
		return json.Unmarshal(payload, out)
	}
	return nil
}
