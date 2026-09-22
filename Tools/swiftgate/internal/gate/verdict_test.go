package gate

import (
	"strings"
	"testing"
)

// lane is a lane the config has granted: its verdict can stop a merge.
func lane(name string, v Verdict) Lane {
	return Lane{Name: name, Verdict: v, Reason: "r", Blocks: true}
}

// advisory is a lane as every lane starts (ADR 0021): judged, reported and recorded,
// and unable to stop anything.
func advisory(name string, v Verdict) Lane { return Lane{Name: name, Verdict: v, Reason: "r"} }

// The blocking table, row by row. Every combination that stops a merge is here, and
// every one that must not.
func TestScoreTruthTable(t *testing.T) {
	staticBlocker := []Finding{{Rule: "arch/x", Severity: Blocker}}
	staticWarning := []Finding{{Rule: "arch/y", Severity: Warning}}

	cases := []struct {
		name    string
		static  []Finding
		lanes   []Lane
		blocked bool
		reason  string
	}{
		{"nothing at all", nil, nil, false, ""},
		{"static warning only", staticWarning, nil, false, ""},
		{"static blocker", staticBlocker, nil, true, "1 static blocker(s)"},
		{"lane PASS", nil, []Lane{lane("idiom", Pass)}, false, ""},
		{"lane CONCERNS", nil, []Lane{lane("idiom", Concerns)}, false, ""},
		{"lane BLOCK", nil, []Lane{lane("idiom", Block)}, true, "lane idiom: BLOCK"},
		{"lane CANNOT_EVALUATE", nil, []Lane{lane("idiom", CannotEvaluate)}, true, "lane idiom: CANNOT_EVALUATE — r"},
		{"one lane passes, another cannot evaluate", nil, []Lane{lane("idiom", Pass), lane("spec", CannotEvaluate)}, true, "lane spec"},
		{"skipped lane never scores", nil, []Lane{{Name: "spec", Skipped: "not a feature branch"}}, false, ""},
		{"skipped lane with a stale verdict still never scores", nil, []Lane{{Name: "spec", Verdict: Block, Skipped: "n/a"}}, false, ""},
		{"static warning beside a lane pass", staticWarning, []Lane{lane("idiom", Pass)}, false, ""},
		{"every reason is listed", staticBlocker, []Lane{lane("idiom", Block), lane("spec", CannotEvaluate)}, true, "lane spec"},

		// ADR 0021. A lane starts advisory and earns blocking on its record; until it
		// does, the worst it can say is reported and recorded, and the merge does not
		// wait on it. The static rules are unaffected — they are the deterministic
		// blocking layer, and no grant is involved.
		{"an advisory lane's BLOCK stops nothing", nil, []Lane{advisory("idiom", Block)}, false, ""},
		{"an advisory lane that cannot evaluate stops nothing", nil, []Lane{advisory("idiom", CannotEvaluate)}, false, ""},
		{"a static blocker still blocks beside an advisory lane", staticBlocker, []Lane{advisory("idiom", Block)}, true, "1 static blocker(s)"},
		{"a granted lane blocks beside an advisory one", nil, []Lane{advisory("idiom", Block), lane("spec", Block)}, true, "lane spec: BLOCK"},
	}
	for _, c := range cases {
		d := Score(c.static, c.lanes)
		if d.Blocked != c.blocked {
			t.Errorf("%s: blocked = %v, want %v (%v)", c.name, d.Blocked, c.blocked, d.Reasons)
		}
		if c.reason != "" && !strings.Contains(strings.Join(d.Reasons, "\n"), c.reason) {
			t.Errorf("%s: reasons %v should name %q", c.name, d.Reasons, c.reason)
		}
		if c.name == "every reason is listed" && len(d.Reasons) != 3 {
			t.Errorf("all three conditions fired; want three reasons, got %v", d.Reasons)
		}
	}
}

// A judge returns one of three verdicts. It cannot return the harness's fourth, and a
// verdict it misspells is not a verdict.
func TestParseVerdictAcceptsOnlyJudgements(t *testing.T) {
	for in, want := range map[string]Verdict{"PASS": Pass, " pass ": Pass, "Concerns": Concerns, "BLOCK": Block} {
		if got, ok := ParseVerdict(in); !ok || got != want {
			t.Errorf("ParseVerdict(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"CANNOT_EVALUATE", "", "ok", "blocked", "PASS."} {
		if got, ok := ParseVerdict(in); ok {
			t.Errorf("ParseVerdict(%q) accepted as %q; a judge may not return that", in, got)
		}
	}
}

func TestParseSeverityAcceptsOnlyTheVocabulary(t *testing.T) {
	for in, want := range map[string]Severity{"blocker": Blocker, " Warning ": Warning, "nit": Nit, "off": Off, "error": Blocker} {
		if got, ok := ParseSeverity(in); !ok || got != want {
			t.Errorf("ParseSeverity(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	// It used to default these to Warning, which silently downgraded a blocker.
	for _, in := range []string{"blokcer", "", "high", "blocker!"} {
		if got, ok := ParseSeverity(in); ok {
			t.Errorf("ParseSeverity(%q) accepted as %q; a severity outside the vocabulary must be refused", in, got)
		}
	}
}

func TestVerdictFromFindings(t *testing.T) {
	if got := VerdictFromFindings(nil); got != Pass {
		t.Errorf("no findings is PASS, got %q", got)
	}
	if got := VerdictFromFindings([]Finding{{Severity: Nit}, {Severity: Warning}}); got != Concerns {
		t.Errorf("a warning is CONCERNS, got %q", got)
	}
	if got := VerdictFromFindings([]Finding{{Severity: Warning}, {Severity: Blocker}}); got != Block {
		t.Errorf("a blocker is BLOCK wherever it sits in the list, got %q", got)
	}
}

func TestNormaliseKeepsStaticOverLaneOnTheSameLine(t *testing.T) {
	r := Result{
		Findings: []Finding{{Rule: "a/b", File: "A.swift", Line: 3, Severity: Warning, Source: FromStatic}, {Rule: "z/off", Severity: Off}},
		// Granted, so "nothing blocks" below is about the dropped duplicate and not
		// about the lane being advisory.
		Lanes: []Lane{{Name: "idiom", Blocks: true, Findings: []Finding{
			{Rule: "a/b", File: "A.swift", Line: 3, Severity: Blocker, Source: FromAgent},
			{Rule: "c/d", File: "A.swift", Line: 1, Severity: Nit},
			{Rule: "c/d", File: "A.swift", Line: 1, Severity: Nit},
		}}},
	}
	r.Normalise()
	if len(r.Findings) != 1 || len(r.Lanes[0].Findings) != 1 || r.Lanes[0].Findings[0].Rule != "c/d" {
		t.Errorf("static wins the duplicate, Off is dropped, lane duplicates collapse: %+v", r)
	}
	if r.Blocked() {
		t.Error("the lane's duplicate blocker was dropped in favour of the static warning; nothing blocks")
	}
}

func TestReportStampsTheHeadSHAOnEveryComment(t *testing.T) {
	ctx := ReportContext{SHA: "0123456789abcdef"}
	r := Result{Lanes: []Lane{lane("idiom", CannotEvaluate)}}
	if body := r.Markdown(ctx); !strings.Contains(body, "[READINESS] 0123456") || !strings.Contains(body, "CANNOT_EVALUATE") {
		t.Errorf("readiness comment must be stamped and say why it blocked:\n%s", body)
	}
	if body := r.Lanes[0].Markdown(ctx, "x"); !strings.Contains(body, "[IDIOM] 0123456") || !strings.Contains(body, LaneMarker("idiom")) {
		t.Errorf("lane comment must be stamped and carry its own marker:\n%s", body)
	}
}
