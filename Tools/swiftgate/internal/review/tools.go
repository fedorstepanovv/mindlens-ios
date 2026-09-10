// Package review is the agentic half of the gate: a model reads the diff against the
// project's own written rules and reports what a careful reviewer would report.
package review

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/toolrunner"

	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/gate"
)

const (
	// flutterPrefix is how the model addresses the Flutter spec checkout. Everything
	// without it resolves inside the Swift repo.
	flutterPrefix = "flutter/"
	maxFileBytes  = 120 * 1024
	maxHits       = 60
)

// workspace resolves the model's paths onto disk, and refuses anything outside the
// two roots it is allowed to read.
type workspace struct {
	repo    string // absolute
	flutter string // absolute; "" when the spec repo was not checked out
}

func newWorkspace(repo, flutter string) (*workspace, error) {
	abs, err := filepath.Abs(repo)
	if err != nil {
		return nil, err
	}
	w := &workspace{repo: abs}
	if flutter != "" {
		if fa, err := filepath.Abs(flutter); err == nil {
			if _, err := os.Stat(fa); err == nil {
				w.flutter = fa
			}
		}
	}
	return w, nil
}

// resolve maps a model-supplied path to a real one, rejecting traversal.
func (w *workspace) resolve(p string) (string, error) {
	p = strings.TrimPrefix(strings.TrimSpace(p), "./")
	if filepath.IsAbs(p) {
		// An absolute path would otherwise be silently reinterpreted as relative to
		// the checkout, which reads as success and returns the wrong file.
		return "", fmt.Errorf("path %q must be relative to the checkout", p)
	}

	root := w.repo
	rel := p
	if strings.HasPrefix(p, flutterPrefix) {
		if w.flutter == "" {
			return "", fmt.Errorf("the Flutter spec repo is not available in this run; review the Swift on its own merits")
		}
		root, rel = w.flutter, strings.TrimPrefix(p, flutterPrefix)
	}

	full := filepath.Clean(filepath.Join(root, rel))
	if full != root && !strings.HasPrefix(full, root+string(os.PathSeparator)) {
		return "", fmt.Errorf("path %q is outside the reviewable checkout", p)
	}
	return full, nil
}

// display turns an absolute path back into one the model handed us, so findings come
// back with paths that match the repository.
func (w *workspace) display(full string) string {
	if w.flutter != "" && strings.HasPrefix(full, w.flutter) {
		return flutterPrefix + strings.TrimPrefix(strings.TrimPrefix(full, w.flutter), string(os.PathSeparator))
	}
	return strings.TrimPrefix(strings.TrimPrefix(full, w.repo), string(os.PathSeparator))
}

func text(s string) anthropic.BetaToolResultBlockParamContentUnion {
	return anthropic.BetaToolResultBlockParamContentUnion{
		OfText: &anthropic.BetaTextBlockParam{Text: s},
	}
}

// --- read_file ---------------------------------------------------------------

type readFileInput struct {
	Path      string `json:"path" jsonschema:"required,description=Repository-relative path. Prefix with flutter/ to read the Flutter spec repo."`
	StartLine int    `json:"start_line,omitempty" jsonschema:"description=First line to return (1-indexed). Omit to start at the top."`
	EndLine   int    `json:"end_line,omitempty" jsonschema:"description=Last line to return. Omit to read to the end."`
}

func (w *workspace) readFile(_ context.Context, in readFileInput) (anthropic.BetaToolResultBlockParamContentUnion, error) {
	full, err := w.resolve(in.Path)
	if err != nil {
		return text(err.Error()), nil
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return text(fmt.Sprintf("cannot read %s: %v", in.Path, err)), nil
	}
	if len(data) > maxFileBytes {
		data = append(data[:maxFileBytes], []byte("\n… file truncated\n")...)
	}

	lines := strings.Split(string(data), "\n")
	start, end := 1, len(lines)
	if in.StartLine > 0 {
		start = in.StartLine
	}
	if in.EndLine > 0 && in.EndLine < end {
		end = in.EndLine
	}
	if start > len(lines) {
		return text(fmt.Sprintf("%s has only %d lines", in.Path, len(lines))), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s (lines %d-%d of %d)\n", in.Path, start, end, len(lines))
	for i := start; i <= end; i++ {
		fmt.Fprintf(&b, "%d\t%s\n", i, lines[i-1])
	}
	return text(b.String()), nil
}

// --- list_directory ----------------------------------------------------------

type listDirInput struct {
	Path string `json:"path" jsonschema:"required,description=Directory to list. Prefix with flutter/ for the Flutter spec repo. Use \".\" for the repository root."`
}

func (w *workspace) listDir(_ context.Context, in listDirInput) (anthropic.BetaToolResultBlockParamContentUnion, error) {
	full, err := w.resolve(in.Path)
	if err != nil {
		return text(err.Error()), nil
	}
	entries, err := os.ReadDir(full)
	if err != nil {
		return text(fmt.Sprintf("cannot list %s: %v", in.Path, err)), nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", in.Path)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if e.IsDir() {
			fmt.Fprintf(&b, "  %s/\n", e.Name())
		} else {
			fmt.Fprintf(&b, "  %s\n", e.Name())
		}
	}
	return text(b.String()), nil
}

// --- search_code -------------------------------------------------------------

type searchInput struct {
	Pattern string `json:"pattern" jsonschema:"required,description=Go/RE2 regular expression to search for."`
	Path    string `json:"path,omitempty" jsonschema:"description=Directory to search under. Prefix with flutter/ for the Flutter spec repo. Defaults to the repository root."`
	Ext     string `json:"ext,omitempty" jsonschema:"description=Restrict to one file extension such as swift or dart."`
}

func (w *workspace) search(_ context.Context, in searchInput) (anthropic.BetaToolResultBlockParamContentUnion, error) {
	re, err := regexp.Compile(in.Pattern)
	if err != nil {
		return text(fmt.Sprintf("bad pattern: %v", err)), nil
	}
	root := in.Path
	if root == "" {
		root = "."
	}
	full, err := w.resolve(root)
	if err != nil {
		return text(err.Error()), nil
	}

	ext := strings.TrimPrefix(in.Ext, ".")
	var hits []string

	_ = filepath.WalkDir(full, func(p string, d fs.DirEntry, err error) error {
		if err != nil || len(hits) >= maxHits {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".build", "build", "DerivedData", "node_modules", "Pods", ".dart_tool", ".swiftgate":
				return fs.SkipDir
			}
			return nil
		}
		if ext != "" && !strings.HasSuffix(p, "."+ext) {
			return nil
		}
		f, err := os.Open(p)
		if err != nil {
			return nil
		}
		defer f.Close()

		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for n := 1; sc.Scan(); n++ {
			if re.MatchString(sc.Text()) {
				hits = append(hits, fmt.Sprintf("%s:%d: %s", w.display(p), n, strings.TrimSpace(sc.Text())))
				if len(hits) >= maxHits {
					return fs.SkipAll
				}
			}
		}
		return nil
	})

	if len(hits) == 0 {
		return text(fmt.Sprintf("no matches for %q under %s", in.Pattern, root)), nil
	}
	sort.Strings(hits)
	out := strings.Join(hits, "\n")
	if len(hits) >= maxHits {
		out += fmt.Sprintf("\n… stopped at %d matches; narrow the pattern.", maxHits)
	}
	return text(out), nil
}

// --- report_findings ---------------------------------------------------------

type agentFinding struct {
	Rule     string `json:"rule" jsonschema:"required,description=Stable kebab-case id in category/name form such as flutter/translated-layering or design/custom-component."`
	Severity string `json:"severity" jsonschema:"required,description=One of blocker warning nit. Use blocker only for a rule the project states absolutely."`
	File     string `json:"file" jsonschema:"required,description=Repository-relative path of the Swift file at fault."`
	Line     int    `json:"line,omitempty" jsonschema:"description=1-indexed line this is about. Omit for a whole-file judgement."`
	Title    string `json:"title" jsonschema:"required,description=One sentence naming the problem. No hedging."`
	Detail   string `json:"detail" jsonschema:"required,description=Why this is wrong here. Quote the actual code. Name the Flutter construct it mirrors when that is the issue."`
	Fix      string `json:"fix" jsonschema:"required,description=The concrete Swift to write instead. Show the shape."`
	Doc      string `json:"doc,omitempty" jsonschema:"description=Which project document says so such as docs/PATTERNS.md § ViewModels."`
}

type reportInput struct {
	Verdict  string         `json:"verdict" jsonschema:"required,description=One paragraph on whether this diff reads as native Swift or as translated Flutter. Say so plainly."`
	Findings []agentFinding `json:"findings" jsonschema:"description=Every problem worth a reviewer's attention. Empty when the diff is clean."`
}

// collector receives the model's verdict. It is a terminal tool: once called the
// review is over, which keeps a wandering model from spending the whole budget.
type collector struct {
	mu       sync.Mutex
	verdict  string
	findings []gate.Finding
	called   bool
}

func (c *collector) tool(w *workspace) func(context.Context, reportInput) (anthropic.BetaToolResultBlockParamContentUnion, error) {
	return func(_ context.Context, in reportInput) (anthropic.BetaToolResultBlockParamContentUnion, error) {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.called = true
		c.verdict = strings.TrimSpace(in.Verdict)

		for _, f := range in.Findings {
			path := strings.TrimPrefix(strings.TrimSpace(f.File), "./")
			if path == "" || strings.HasPrefix(path, flutterPrefix) {
				// The gate reviews Swift. A finding pinned to the Dart spec has no
				// line for a reviewer to act on.
				continue
			}
			c.findings = append(c.findings, gate.Finding{
				Rule:     normaliseRule(f.Rule),
				Severity: gate.ParseSeverity(f.Severity),
				File:     path,
				Line:     f.Line,
				Title:    strings.TrimSpace(f.Title),
				Detail:   strings.TrimSpace(f.Detail),
				Fix:      strings.TrimSpace(f.Fix),
				Doc:      strings.TrimSpace(f.Doc),
				Source:   gate.FromAgent,
			})
		}
		return text(fmt.Sprintf("Recorded %d finding(s). The review is complete — stop here.", len(in.Findings))), nil
	}
}

var ruleShape = regexp.MustCompile(`[^a-z0-9/\-]+`)

func normaliseRule(s string) string {
	s = ruleShape.ReplaceAllString(strings.ToLower(strings.TrimSpace(s)), "-")
	s = strings.Trim(s, "-/")
	if s == "" {
		return "review/unspecified"
	}
	if !strings.Contains(s, "/") {
		s = "review/" + s
	}
	return s
}

// buildTools wires the four tools the reviewer gets. Read, list, search, report —
// deliberately no write and no shell: the gate has an opinion, not a hand.
func buildTools(w *workspace, c *collector) ([]anthropic.BetaTool, error) {
	read, err := toolrunner.NewBetaToolFromJSONSchema(
		"read_file",
		"Read a file from the Swift repository, or from the Flutter app that serves as the product spec (prefix the path with flutter/). Use this to see the full context around a changed line rather than judging from the diff alone.",
		w.readFile,
	)
	if err != nil {
		return nil, err
	}
	list, err := toolrunner.NewBetaToolFromJSONSchema(
		"list_directory",
		"List a directory in the Swift repository, or in the Flutter spec repo (prefix with flutter/). Use it to find the Dart screen a Swift file corresponds to.",
		w.listDir,
	)
	if err != nil {
		return nil, err
	}
	search, err := toolrunner.NewBetaToolFromJSONSchema(
		"search_code",
		"Search files by regular expression. Use it to check whether a pattern already exists elsewhere in the Swift codebase, or to locate the Dart source a Swift file was derived from.",
		w.search,
	)
	if err != nil {
		return nil, err
	}
	report, err := toolrunner.NewBetaToolFromJSONSchema(
		"report_findings",
		"Report the review. Call this exactly once, as the last thing you do. Calling it ends the review.",
		c.tool(w),
	)
	if err != nil {
		return nil, err
	}
	return []anthropic.BetaTool{read, list, search, report}, nil
}
