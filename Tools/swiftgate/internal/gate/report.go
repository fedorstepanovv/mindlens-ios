package gate

import (
	"fmt"
	"strings"
)

// Marker identifies the gate's own PR comment so successive runs edit one comment
// instead of burying the conversation under a new one every push.
const Marker = "<!-- swiftgate:report -->"

// ReportContext is what the renderer needs to turn findings into links.
type ReportContext struct {
	Repo     string // "owner/name"
	SHA      string // head commit, for permalinks
	RunURL   string // the Actions run, so a reader can see the raw log
	Passed   bool
	Override string
}

// Markdown renders the sticky PR comment.
func (r *Result) Markdown(ctx ReportContext) string {
	var b strings.Builder
	b.WriteString(Marker)
	b.WriteString("\n## Swift idiom gate\n\n")

	if r.Skipped != "" {
		fmt.Fprintf(&b, "Skipped — %s\n", r.Skipped)
		return b.String()
	}

	blockers, warnings, nits := r.Counts()

	switch {
	case ctx.Override != "":
		fmt.Fprintf(&b, "**Overridden.** %d blocker(s) stand, and a human took responsibility:\n\n> %s\n\n",
			blockers, ctx.Override)
	case blockers > 0:
		fmt.Fprintf(&b, "**Blocked.** %d thing(s) must change before this can merge.\n\n", blockers)
	case warnings+nits > 0:
		fmt.Fprintf(&b, "**Passed**, with %d note(s) worth reading.\n\n", warnings+nits)
	default:
		b.WriteString("**Passed.** Nothing to flag — this reads as native Swift.\n\n")
	}

	if len(r.Findings) > 0 {
		fmt.Fprintf(&b, "🛑 %d blocking · ⚠️ %d warning · 💬 %d nit\n\n", blockers, warnings, nits)
	}

	for _, f := range r.Findings {
		fmt.Fprintf(&b, "### %s %s\n\n", f.Severity.Emoji(), f.Title)
		fmt.Fprintf(&b, "**%s** · `%s`", link(ctx, f), f.Rule)
		if f.Doc != "" {
			fmt.Fprintf(&b, " · %s", f.Doc)
		}
		b.WriteString("\n\n")
		if f.Detail != "" {
			fmt.Fprintf(&b, "%s\n\n", f.Detail)
		}
		if f.Fix != "" {
			fmt.Fprintf(&b, "**Instead:** %s\n\n", f.Fix)
		}
	}

	b.WriteString("---\n\n")
	if r.Usage.Model != "" {
		fmt.Fprintf(&b, "<sub>Reviewed by `%s` over %d turn(s) · %s in / %s out",
			r.Usage.Model, r.Usage.Turns,
			humanTokens(r.Usage.InputTokens), humanTokens(r.Usage.OutputTokens))
		if r.Usage.CacheReadTokens > 0 {
			fmt.Fprintf(&b, " · %s cached", humanTokens(r.Usage.CacheReadTokens))
		}
		fmt.Fprintf(&b, " · ~$%.2f", r.Usage.EstimatedUSDCents/100)
		b.WriteString("</sub>\n")
	}
	if ctx.RunURL != "" {
		fmt.Fprintf(&b, "<sub>[Full log](%s) · re-run to re-review · a static rule can be waived in place with `// swiftgate:allow <rule> — reason`</sub>\n", ctx.RunURL)
	}

	return b.String()
}

func link(ctx ReportContext, f Finding) string {
	if ctx.Repo == "" || ctx.SHA == "" {
		return f.Location()
	}
	url := fmt.Sprintf("https://github.com/%s/blob/%s/%s", ctx.Repo, ctx.SHA, f.File)
	if f.Line > 0 {
		url += fmt.Sprintf("#L%d", f.Line)
	}
	return fmt.Sprintf("[%s](%s)", f.Location(), url)
}

// Summary renders the shorter form written to the Actions step summary, where the
// reader already has the log and wants the verdict.
func (r *Result) Summary() string {
	var b strings.Builder
	b.WriteString("## Swift idiom gate\n\n")
	if r.Skipped != "" {
		fmt.Fprintf(&b, "Skipped — %s\n", r.Skipped)
		return b.String()
	}

	blockers, warnings, nits := r.Counts()
	fmt.Fprintf(&b, "🛑 %d blocking · ⚠️ %d warning · 💬 %d nit\n\n", blockers, warnings, nits)

	if len(r.Findings) == 0 {
		return b.String()
	}

	b.WriteString("| | Where | Rule | Problem |\n|---|---|---|---|\n")
	for _, f := range r.Findings {
		fmt.Fprintf(&b, "| %s | `%s` | `%s` | %s |\n",
			f.Severity.Emoji(), f.Location(), f.Rule, escapePipes(f.Title))
	}
	return b.String()
}

func escapePipes(s string) string { return strings.ReplaceAll(s, "|", "\\|") }

func humanTokens(n int64) string {
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}
