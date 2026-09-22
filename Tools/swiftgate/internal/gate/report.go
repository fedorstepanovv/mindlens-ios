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

	// The level comes first: it is the one line the reader acts on, and it says how
	// much of the rest of this they need.
	if r.Review.Level != "" {
		fmt.Fprintf(&b, "%s\n\n", r.Review.Sentence())
	}

	d := r.Decision()
	switch {
	case ctx.Override != "" && d.Blocked:
		fmt.Fprintf(&b, "**Overridden.** The gate would block, and a human took responsibility:\n\n> %s\n\n", ctx.Override)
	case d.Blocked:
		b.WriteString("**Blocked.** The scorer stops on enumerable conditions, and these fired:\n\n")
	default:
		b.WriteString("**Ready.** No static blocker, and no blocking lane stopped this.\n\n")
	}
	for _, reason := range d.Reasons {
		fmt.Fprintf(&b, "- %s\n", reason)
	}
	if len(d.Reasons) > 0 {
		b.WriteString("\n")
	}

	if len(r.Lanes) > 0 {
		b.WriteString("| Lane | Verdict | Blocks | |\n|---|---|---|---|\n")
		advisory := false
		for _, l := range r.Lanes {
			if l.Skipped != "" {
				fmt.Fprintf(&b, "| %s | skipped | — | %s |\n", l.Name, l.Skipped)
				continue
			}
			blocks := "no — advisory"
			if l.Blocks {
				blocks = "yes"
			}
			advisory = advisory || l.Advisory()
			note := fmt.Sprintf("%d finding(s) — see the `%s` comment", len(l.Findings), ctx.stamp(l.Name))
			if l.Unproven > 0 {
				note += fmt.Sprintf("; %d dropped for want of a proof", l.Unproven)
			}
			fmt.Fprintf(&b, "| %s | **%s** | %s | %s |\n", l.Name, l.Verdict, blocks, note)
		}
		b.WriteString("\n")
		if advisory {
			b.WriteString("An advisory lane said **BLOCK** or could not evaluate, and the merge did not wait on it. " +
				"A lane blocks once its record earns it: ≥10 judged pull requests, noise ≤30%, ≥1 finding the code " +
				"answered — then `blocks: true` in `.github/swiftgate.yml`, by a reviewed pull request citing " +
				"`swiftgate metrics` (ADR 0021). Read the lane's comment before merging anyway.\n\n")
		}
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
		stops := "and that blocks"
		if !l.Blocks {
			stops = "and this lane is advisory, so it stopped nothing"
		}
		fmt.Fprintf(&b, "**CANNOT_EVALUATE.** The lane did not judge this change, %s: "+
			"an absent judgement is not a clean one.\n\n> %s\n\n", stops, l.Reason)
	default:
		blockers, warnings, nits := Counts(l.Findings)
		fmt.Fprintf(&b, "**%s** · 🛑 %d blocking · ⚠️ %d warning · 💬 %d nit\n\n", l.Verdict, blockers, warnings, nits)
		writeFindings(&b, ctx, l.Findings)
		if l.Reason != "" {
			fmt.Fprintf(&b, "> %s\n\n", strings.ReplaceAll(l.Reason, "\n", "\n> "))
		}
	}

	if l.Unproven > 0 {
		fmt.Fprintf(&b, "%d finding(s) were dropped before this comment was written: the judge gave no proof — "+
			"the input or state and the line where it fails, or the exemplar contradicted — and a finding without "+
			"one is not reported (ADR 0021). They are counted in this lane's metrics record.\n\n", l.Unproven)
	}

	b.WriteString("---\n\n")
	if reviewer != "" {
		standing := "Advisory: this lane stops nothing until its record earns `blocks: true`"
		if l.Blocks {
			standing = "Blocking: this lane's BLOCK stops the merge"
		}
		fmt.Fprintf(&b, "<sub>Judged by %s. %s. The readiness comment decides.</sub>\n", reviewer, standing)
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
		if f.Proof != "" {
			fmt.Fprintf(b, "**Proof:** %s\n\n", f.Proof)
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
	if r.Review.Level != "" {
		fmt.Fprintf(&b, "Read: **%s** — %s\n\n", r.Review.Level, strings.Join(r.Review.Reasons, "; "))
	}
	for _, l := range r.Lanes {
		switch {
		case l.Skipped != "":
			fmt.Fprintf(&b, "- %s: skipped — %s\n", l.Name, l.Skipped)
		case l.Advisory():
			fmt.Fprintf(&b, "- %s: **%s** (advisory — stopped nothing), %d finding(s)\n", l.Name, l.Verdict, len(l.Findings))
		default:
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
