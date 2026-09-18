package gate

import (
	"fmt"
	"strings"
)

// Marker identifies one of the gate's own PR comments so successive runs edit it
// instead of burying the conversation under a new one every push. The scorer and each
// lane have their own, so each is a sticky comment in its own right.
func Marker(kind string) string { return "<!-- swiftgate:" + kind + " -->" }

// ReportMarker is the scorer's. The /pr skill greps for it, so it keeps its old name.
const ReportMarker = "<!-- swiftgate:report -->"

// LaneMarker is the sticky comment for one lane.
func LaneMarker(lane string) string { return Marker("lane:" + lane) }

// ReportContext is what the renderer needs to turn findings into links.
type ReportContext struct {
	Repo     string // "owner/name"
	SHA      string // head commit, for permalinks and the [LANE] <sha> stamp
	RunURL   string // the Actions run, so a reader can see the raw log
	Override string
}

// stamp is the `[NAME] <sha>` heading suffix that makes a stale verdict visibly stale:
// a comment stamped with an older SHA than the PR's head is about code that is gone.
func (ctx ReportContext) stamp(name string) string {
	sha := ctx.SHA
	if len(sha) > 7 {
		sha = sha[:7]
	}
	if sha == "" {
		return fmt.Sprintf("[%s]", strings.ToUpper(name))
	}
	return fmt.Sprintf("[%s] %s", strings.ToUpper(name), sha)
}

// Markdown renders the scorer's sticky comment: the decision, every reason for it,
// the static findings, and one line per lane pointing at that lane's own comment.
func (r *Result) Markdown(ctx ReportContext) string {
	var b strings.Builder
	b.WriteString(ReportMarker)
	fmt.Fprintf(&b, "\n## PR readiness — %s\n\n", ctx.stamp("readiness"))

	if r.Skipped != "" {
		fmt.Fprintf(&b, "Skipped — %s\n", r.Skipped)
		return b.String()
	}

	d := r.Decision()
	switch {
	case ctx.Override != "" && d.Blocked:
		fmt.Fprintf(&b, "**Overridden.** The gate would block, and a human took responsibility:\n\n> %s\n\n", ctx.Override)
	case d.Blocked:
		b.WriteString("**Blocked.** The scorer stops on enumerable conditions, and these fired:\n\n")
	default:
		b.WriteString("**Ready.** No static blocker, no lane blocked, every lane could evaluate.\n\n")
	}
	for _, reason := range d.Reasons {
		fmt.Fprintf(&b, "- %s\n", reason)
	}
	if len(d.Reasons) > 0 {
		b.WriteString("\n")
	}

	if len(r.Lanes) > 0 {
		b.WriteString("| Lane | Verdict | |\n|---|---|---|\n")
		for _, l := range r.Lanes {
			if l.Skipped != "" {
				fmt.Fprintf(&b, "| %s | skipped | %s |\n", l.Name, l.Skipped)
				continue
			}
			fmt.Fprintf(&b, "| %s | **%s** | %d finding(s) — see the `%s` comment |\n",
				l.Name, l.Verdict, len(l.Findings), ctx.stamp(l.Name))
		}
		b.WriteString("\n")
	}

	if len(r.Findings) > 0 {
		blockers, warnings, nits := Counts(r.Findings)
		fmt.Fprintf(&b, "### Deterministic rules\n\n🛑 %d blocking · ⚠️ %d warning · 💬 %d nit\n\n", blockers, warnings, nits)
		writeFindings(&b, ctx, r.Findings)
	}

	b.WriteString("---\n\n")
	if ctx.RunURL != "" {
		fmt.Fprintf(&b, "<sub>[Full log](%s) · re-run to re-judge · a static rule can be waived in place with `// swiftgate:allow <rule> — reason`</sub>\n", ctx.RunURL)
	}
	return b.String()
}

// Markdown renders one lane's sticky comment.
func (l Lane) Markdown(ctx ReportContext, reviewer string) string {
	var b strings.Builder
	b.WriteString(LaneMarker(l.Name))
	fmt.Fprintf(&b, "\n## %s lane — %s\n\n", strings.ToUpper(l.Name[:1])+l.Name[1:], ctx.stamp(l.Name))

	switch {
	case l.Skipped != "":
		fmt.Fprintf(&b, "Skipped — %s\n", l.Skipped)
		return b.String()
	case l.Verdict == CannotEvaluate:
		fmt.Fprintf(&b, "**CANNOT_EVALUATE.** The lane did not judge this change, and that blocks: "+
			"an absent judgement is not a clean one.\n\n> %s\n\n", l.Reason)
	default:
		blockers, warnings, nits := Counts(l.Findings)
		fmt.Fprintf(&b, "**%s** · 🛑 %d blocking · ⚠️ %d warning · 💬 %d nit\n\n", l.Verdict, blockers, warnings, nits)
		writeFindings(&b, ctx, l.Findings)
		if l.Reason != "" {
			fmt.Fprintf(&b, "> %s\n\n", strings.ReplaceAll(l.Reason, "\n", "\n> "))
		}
	}

	b.WriteString("---\n\n")
	if reviewer != "" {
		fmt.Fprintf(&b, "<sub>Judged by %s. Advisory: the readiness comment decides.</sub>\n", reviewer)
	}
	return b.String()
}

func writeFindings(b *strings.Builder, ctx ReportContext, fs []Finding) {
	for _, f := range fs {
		fmt.Fprintf(b, "### %s %s\n\n", f.Severity.Emoji(), f.Title)
		fmt.Fprintf(b, "**%s** · `%s`", link(ctx, f), f.Rule)
		if f.Doc != "" {
			fmt.Fprintf(b, " · %s", f.Doc)
		}
		b.WriteString("\n\n")
		if f.Detail != "" {
			fmt.Fprintf(b, "%s\n\n", f.Detail)
		}
		if f.Fix != "" {
			fmt.Fprintf(b, "**Instead:** %s\n\n", f.Fix)
		}
	}
}

func link(ctx ReportContext, f Finding) string {
	if ctx.Repo == "" || ctx.SHA == "" || f.File == "" {
		return f.Location()
	}
	url := fmt.Sprintf("https://github.com/%s/blob/%s/%s", ctx.Repo, ctx.SHA, f.File)
	if f.Line > 0 {
		url += fmt.Sprintf("#L%d", f.Line)
	}
	return fmt.Sprintf("[%s](%s)", f.Location(), url)
}

// Summary renders the shorter form written to the Actions step summary, where the
// reader already has the log and wants the decision.
func (r *Result) Summary() string {
	var b strings.Builder
	b.WriteString("## PR readiness\n\n")
	if r.Skipped != "" {
		fmt.Fprintf(&b, "Skipped — %s\n", r.Skipped)
		return b.String()
	}

	d := r.Decision()
	if d.Blocked {
		fmt.Fprintf(&b, "**Blocked:** %s\n\n", strings.Join(d.Reasons, " · "))
	} else {
		b.WriteString("**Ready.**\n\n")
	}
	for _, l := range r.Lanes {
		if l.Skipped != "" {
			fmt.Fprintf(&b, "- %s: skipped — %s\n", l.Name, l.Skipped)
		} else {
			fmt.Fprintf(&b, "- %s: **%s**, %d finding(s)\n", l.Name, l.Verdict, len(l.Findings))
		}
	}

	all := r.All()
	if len(all) == 0 {
		return b.String()
	}
	b.WriteString("\n| | Where | Rule | Problem |\n|---|---|---|---|\n")
	for _, f := range all {
		fmt.Fprintf(&b, "| %s | `%s` | `%s` | %s |\n",
			f.Severity.Emoji(), f.Location(), f.Rule, escapePipes(f.Title))
	}
	return b.String()
}

func escapePipes(s string) string { return strings.ReplaceAll(s, "|", "\\|") }
