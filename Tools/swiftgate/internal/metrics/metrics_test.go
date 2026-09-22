package metrics

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/evidence"
	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/gate"
)

// The fixtures under testdata/ are records with known answers, because no lane has
// judged a real pull request yet and there are no real records to aggregate. Every
// number asserted here was counted by hand from lanes.jsonl and outcomes.jsonl.
func fixtures(t *testing.T) Records {
	t.Helper()
	recs, err := Read("testdata/lanes.jsonl", "testdata/outcomes.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	return recs
}

var catalog = []string{"design/print-logging", "flutter/service-locator", "arch/our-singleton"}

func TestReadSeparatesTheTwoKindsOfRecord(t *testing.T) {
	recs := fixtures(t)
	if len(recs.Lanes) != 16 || len(recs.Outcomes) != 5 {
		t.Fatalf("want 16 lane records and 5 outcome records, got %d and %d", len(recs.Lanes), len(recs.Outcomes))
	}
	// A directory is read recursively, and only .jsonl files count.
	dir, err := Read("testdata")
	if err == nil {
		t.Fatal("testdata holds a broken file; reading the directory must fail on it")
	}
	if !strings.Contains(err.Error(), "broken.jsonl:2") {
		t.Errorf("the error must name the file and line: %v", err)
	}
	_ = dir
}

func TestReadRefusesALineItCannotTrust(t *testing.T) {
	if _, err := Read("testdata/broken.jsonl"); err == nil || !strings.Contains(err.Error(), "broken.jsonl:2") {
		t.Errorf("a malformed line is named by file and line number, got %v", err)
	}
	if _, err := Read("testdata/future-schema.jsonl"); err == nil || !strings.Contains(err.Error(), "schema 2") {
		t.Errorf("a record from a newer schema is refused, not silently misread, got %v", err)
	}
	if _, err := Read("testdata/does-not-exist.jsonl"); err == nil {
		t.Error("a missing path is an error")
	}
}

func TestSummaryCountsRunsAndPullRequests(t *testing.T) {
	s := Summarise(fixtures(t), catalog)
	if s.Runs != 4 || s.PRs != 3 {
		t.Errorf("4 runs over 3 pull requests, got %d and %d", s.Runs, s.PRs)
	}
}

func TestSummaryPerLane(t *testing.T) {
	s := Summarise(fixtures(t), catalog)

	idiom, ok := s.Lane("idiom")
	if !ok {
		t.Fatal("no idiom lane in the summary")
	}
	if idiom.Runs != 4 {
		t.Errorf("idiom ran 4 times, got %d", idiom.Runs)
	}
	for verdict, want := range map[gate.Verdict]int{gate.Pass: 1, gate.Concerns: 1, gate.Block: 1, gate.CannotEvaluate: 1} {
		if got := idiom.Verdicts[verdict]; got != want {
			t.Errorf("idiom %s: want %d, got %d", verdict, want, got)
		}
	}
	if idiom.Causes[gate.CauseJudge] != 1 || idiom.Causes[gate.CauseEvidence] != 0 {
		t.Errorf("idiom's one CANNOT_EVALUATE was the judge not running, got %v", idiom.Causes)
	}
	if idiom.Findings != 3 || idiom.Severities[gate.Blocker] != 1 || idiom.Severities[gate.Warning] != 1 || idiom.Severities[gate.Nit] != 1 {
		t.Errorf("idiom: 3 findings, one of each severity, got %d %v", idiom.Findings, idiom.Severities)
	}
	if idiom.Cost.Known != 3 || math.Abs(idiom.Cost.Sum-3.0) > 1e-9 {
		t.Errorf("idiom's cost is known on 3 runs and sums to 3.0, got %+v", idiom.Cost)
	}
	if idiom.Duration.Known != 3 || math.Abs(idiom.Duration.Mean()-93333.33) > 0.01 {
		t.Errorf("idiom's duration is known on 3 runs, mean 93333 ms, got %+v", idiom.Duration)
	}

	verification, _ := s.Lane("verification")
	if verification.Verdicts[gate.Pass] != 3 || verification.Causes[gate.CauseEvidence] != 1 {
		t.Errorf("verification: 3 PASS and one missing-evidence, got %v %v", verification.Verdicts, verification.Causes)
	}
	if verification.Judged != 3 {
		t.Errorf("a lane whose evidence was missing never ran its judge: 3 judged runs, got %d", verification.Judged)
	}
	if verification.Cost.Known != 2 || math.Abs(verification.Cost.Sum-0.9) > 1e-9 {
		t.Errorf("verification cost known on 2 runs summing to 0.9, got %+v", verification.Cost)
	}

	spec, _ := s.Lane("spec")
	if spec.Skipped != 3 || spec.Verdicts[gate.Concerns] != 1 || spec.Findings != 1 {
		t.Errorf("spec skipped 3 times, CONCERNS once with one finding, got %d %v %d", spec.Skipped, spec.Verdicts, spec.Findings)
	}

	static, _ := s.Lane("static")
	if static.Runs != 4 || static.Findings != 1 || static.Cost.Known != 0 {
		t.Errorf("the static pass is a lane for counting: 4 runs, 1 finding, no cost, got %+v", static)
	}

	// The order is the one the gate reports in, static last.
	var names []string
	for _, l := range s.Lanes {
		names = append(names, l.Name)
	}
	if strings.Join(names, ",") != "verification,idiom,spec,static" {
		t.Errorf("lane order: %v", names)
	}
}

func TestSummaryPerRuleAndRulesThatNeverFired(t *testing.T) {
	s := Summarise(fixtures(t), catalog)
	for rule, want := range map[string]int{
		"design/print-logging":   1,
		"review/naming":          1,
		"review/mvvm":            1,
		"review/service-locator": 1,
		"spec/step-incomplete":   1,
	} {
		r, ok := s.Rule(rule)
		if !ok || r.Fires != want {
			t.Errorf("%s: want %d fire(s), got %+v", rule, want, r)
		}
	}
	if r, _ := s.Rule("review/naming"); r.ByLane["idiom"] != 1 {
		t.Errorf("review/naming fired from the idiom lane, got %v", r.ByLane)
	}
	if got := strings.Join(s.NeverFired, ","); got != "arch/our-singleton,flutter/service-locator" {
		t.Errorf("the static rules in the catalog that never fired, sorted: got %q", got)
	}
}

func TestSummaryPerPullRequestCostSaysWhenItIsPartial(t *testing.T) {
	s := Summarise(fixtures(t), catalog)
	one, _ := s.PR(1)
	if math.Abs(one.Cost.Sum-2.7) > 1e-9 || !one.CostComplete {
		t.Errorf("PR 1: every judged run reported its cost, 2.7 in all, got %+v", one)
	}
	two, _ := s.PR(2)
	if math.Abs(two.Cost.Sum-1.2) > 1e-9 || two.CostComplete {
		t.Errorf("PR 2: the spec lane judged without reporting cost, so 1.2 is a floor, got %+v", two)
	}
	three, _ := s.PR(3)
	if three.Cost.Known != 0 || three.CostComplete {
		t.Errorf("PR 3: nothing reported cost, got %+v", three)
	}
	if two.Runs != 1 || one.Runs != 2 {
		t.Errorf("runs per PR: %d and %d", one.Runs, two.Runs)
	}
}

func TestOutcomesGiveTheNoiseRatePerLaneAndPerRule(t *testing.T) {
	s := Summarise(fixtures(t), catalog)

	idiom, _ := s.Lane("idiom")
	if idiom.Outcomes[Changed] != 2 || idiom.Outcomes[Untouched] != 1 || idiom.Classified() != 3 {
		t.Errorf("idiom: 2 changed, 1 untouched, got %v", idiom.Outcomes)
	}
	if noise, ok := idiom.Noise(); !ok || math.Abs(noise-1.0/3) > 1e-9 {
		t.Errorf("idiom noise = untouched ÷ classified = 1/3, got %v %v", noise, ok)
	}
	static, _ := s.Lane("static")
	if static.Outcomes[Waived] != 1 {
		t.Errorf("the static finding was waived, got %v", static.Outcomes)
	}
	if noise, ok := static.Noise(); !ok || noise != 0 {
		t.Errorf("a waived finding is not noise: it was read and answered, got %v %v", noise, ok)
	}
	spec, _ := s.Lane("spec")
	if noise, ok := spec.Noise(); !ok || noise != 1 {
		t.Errorf("spec's one finding was untouched: noise 1.0, got %v %v", noise, ok)
	}
	verification, _ := s.Lane("verification")
	if _, ok := verification.Noise(); ok {
		t.Error("a lane with no classified finding has no noise rate, not a zero one")
	}

	mvvm, _ := s.Rule("review/mvvm")
	if noise, ok := mvvm.Noise(); !ok || noise != 1 {
		t.Errorf("review/mvvm fired once and was ignored: noise 1.0, got %v %v", noise, ok)
	}
	if s.Classified != 5 || s.Findings != 5 {
		t.Errorf("5 findings fired, 5 classified, got %d and %d", s.Findings, s.Classified)
	}
}

func TestMarkdownSaysWhatTheReaderNeeds(t *testing.T) {
	md := Summarise(fixtures(t), catalog).Markdown()
	for _, want := range []string{
		"4 run(s)", "3 pull request(s)",
		"| verification |", "| idiom |", "| spec |", "| static |",
		"CANNOT_EVALUATE", "evidence", "judge",
		"1.20", "2.70", "floor",
		"never fired", "arch/our-singleton",
		"review/mvvm", "100%", "33%",
		"noise",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("the report is missing %q:\n%s", want, md)
		}
	}
}

func TestMarkdownOnNoRecordsSaysSo(t *testing.T) {
	md := Summarise(Records{}, catalog).Markdown()
	if !strings.Contains(md, "No records") {
		t.Errorf("an empty aggregate must say it is empty, not print zeros:\n%s", md)
	}
}

// --- writing records ---------------------------------------------------------

func TestFromLaneCarriesWhatTheScorerKnew(t *testing.T) {
	cost := 0.42
	lane := gate.Lane{
		Name: "idiom", Verdict: gate.Concerns, Reason: "one nit",
		Findings: []gate.Finding{{Rule: "review/naming", Severity: gate.Nit, File: "A.swift", Line: 3, Title: "t", Detail: "long", Fix: "long"}},
	}
	ev := evidence.Result{Lane: evidence.Idiom, Present: []string{"a diff"}}
	r := FromLane(Run{PR: 7, SHA: "abc", Branch: "feature/x", URL: "https://run"}, lane, "claude-sonnet-5", ev, Execution{CostUSD: &cost})

	if r.Kind != KindLane || r.Schema != SchemaVersion || r.PR != 7 || r.SHA != "abc" || r.Lane != "idiom" || r.RunURL != "https://run" {
		t.Errorf("identity fields: %+v", r)
	}
	if r.Verdict != gate.Concerns || r.Reason != "one nit" || r.Cause != "" {
		t.Errorf("verdict fields: %+v", r)
	}
	if r.Evidence == nil || !r.Evidence.OK || len(r.Evidence.Present) != 1 {
		t.Errorf("evidence: %+v", r.Evidence)
	}
	if len(r.Findings) != 1 || r.Findings[0].Rule != "review/naming" || r.Findings[0].Line != 3 || r.Findings[0].Title != "t" {
		t.Errorf("findings keep rule, severity, file, line and title — enough to match an outcome to: %+v", r.Findings)
	}
	if r.CostUSD == nil || *r.CostUSD != 0.42 || r.DurationMS != nil || r.Turns != nil {
		t.Errorf("cost is what the execution file said; duration and turns stay nil when it did not say: %+v", r)
	}
	if r.RecordedAt.IsZero() {
		t.Error("recorded_at is set")
	}

	// The harness's own state travels: CANNOT_EVALUATE keeps its cause, a skipped lane
	// its reason, and neither has a verdict pretending otherwise.
	cannot := FromLane(Run{}, gate.Lane{Name: "spec", Verdict: gate.CannotEvaluate, Cause: gate.CauseEvidence, Reason: "missing x"}, "claude-opus-5", evidence.Result{Lane: evidence.Spec, Missing: []string{"x"}}, Execution{})
	if cannot.Cause != gate.CauseEvidence || cannot.Evidence.OK || cannot.Evidence.Missing[0] != "x" {
		t.Errorf("cannot-evaluate record: %+v", cannot)
	}
	skipped := FromLane(Run{}, gate.Lane{Name: "spec", Skipped: "not a feature branch"}, "claude-opus-5", evidence.Result{Lane: evidence.Spec, Skipped: "not a feature branch"}, Execution{})
	if skipped.Verdict != "" || skipped.Skipped != "not a feature branch" {
		t.Errorf("skipped record: %+v", skipped)
	}
}

func TestFromStaticIsALaneForCounting(t *testing.T) {
	r := FromStatic(Run{PR: 1}, []gate.Finding{{Rule: "design/print-logging", Severity: gate.Warning, File: "A.swift", Line: 1}})
	if r.Lane != StaticLane || r.Verdict != gate.Concerns || len(r.Findings) != 1 || r.Evidence != nil || r.CostUSD != nil {
		t.Errorf("static record: %+v", r)
	}
	if none := FromStatic(Run{}, nil); none.Verdict != gate.Pass {
		t.Errorf("no static findings is PASS, got %s", none.Verdict)
	}
}

func TestAppendThenReadRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.jsonl")
	ms := int64(1234)
	lane := FromLane(Run{PR: 3, SHA: "s"}, gate.Lane{Name: "idiom", Verdict: gate.Pass}, "claude-sonnet-5", evidence.Result{Lane: evidence.Idiom}, Execution{DurationMS: &ms})
	lane.RecordedAt = time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	out := OutcomeRecord{Kind: KindOutcome, Schema: SchemaVersion, PR: 3, SHA: "s", Head: "h", Lane: "idiom", Rule: "r", File: "A.swift", Line: 2, Outcome: Resolved, Reason: "gone at the head"}
	if err := Append(path, lane, out); err != nil {
		t.Fatal(err)
	}
	if err := Append(path, lane); err != nil { // a second call appends, it does not truncate
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if n := strings.Count(strings.TrimSpace(string(data)), "\n") + 1; n != 3 {
		t.Fatalf("three lines, one record each, got %d:\n%s", n, data)
	}
	recs, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs.Lanes) != 2 || len(recs.Outcomes) != 1 {
		t.Fatalf("got %d lane and %d outcome records", len(recs.Lanes), len(recs.Outcomes))
	}
	if got := recs.Lanes[0]; got.DurationMS == nil || *got.DurationMS != 1234 || !got.RecordedAt.Equal(lane.RecordedAt) {
		t.Errorf("round trip lost a field: %+v", got)
	}
	if got := recs.Outcomes[0]; got.Outcome != Resolved || got.Head != "h" {
		t.Errorf("round trip lost a field: %+v", got)
	}
}

// --- the execution file ------------------------------------------------------

// The claude-code-action exposes an execution_file output. Its real shape has not been
// seen by anyone here, so the reader accepts every shape it might plausibly be and
// yields nothing — never a wrong number — when it is none of them.
func TestReadExecutionAcceptsEveryPlausibleShape(t *testing.T) {
	cases := map[string]struct {
		ms    int64
		cost  float64
		turns int
	}{
		"array.json":  {83210, 1.2345, 17},
		"object.json": {41000, 0.25, 4},
		"ndjson.json": {9000, 0.05, 2},
	}
	for file, want := range cases {
		got, err := ReadExecution(filepath.Join("testdata/execution", file))
		if err != nil {
			t.Errorf("%s: %v", file, err)
			continue
		}
		if got.DurationMS == nil || *got.DurationMS != want.ms || got.CostUSD == nil || *got.CostUSD != want.cost || got.Turns == nil || *got.Turns != want.turns {
			t.Errorf("%s: want %+v, got duration=%v cost=%v turns=%v", file, want, deref(got.DurationMS), deref(got.CostUSD), deref(got.Turns))
		}
	}
}

func TestReadExecutionYieldsNothingRatherThanAGuess(t *testing.T) {
	got, err := ReadExecution("testdata/execution/no-result.json")
	if err != nil || got.DurationMS != nil || got.CostUSD != nil || got.Turns != nil {
		t.Errorf("a log with no result message has no numbers to offer: %+v, %v", got, err)
	}
	if _, err := ReadExecution("testdata/execution/garbage.json"); err == nil {
		t.Error("a file that is not JSON in any shape is an error the caller logs")
	}
	if _, err := ReadExecution("testdata/execution/missing.json"); err == nil {
		t.Error("a missing file is an error the caller logs")
	}
	if got, err := ReadExecution(""); err != nil || got != (Execution{}) {
		t.Errorf("no path means the workflow had no file to offer — empty, not an error: %+v %v", got, err)
	}
}

func deref[T any](p *T) any {
	if p == nil {
		return nil
	}
	return *p
}

// --- classifying an outcome --------------------------------------------------

// Precedence, from the record's doc comment: waived, then changed, then resolved,
// then untouched. Only untouched is noise.
func TestClassifyPrecedence(t *testing.T) {
	cases := []struct {
		name string
		in   Evidence
		want Class
	}{
		{"a waiver wins even when the code also changed", Evidence{Waived: true, Touched: true}, Waived},
		{"the merge-past-a-blocker label is a waiver too", Evidence{Overridden: true, StillReported: true, HeadRun: true}, Waived},
		{"the flagged lines were edited", Evidence{Touched: true, StillReported: true, HeadRun: true}, Changed},
		{"the file was deleted", Evidence{Deleted: true}, Changed},
		{"gone from the last run without the lines changing", Evidence{HeadRun: true, StillReported: false}, Resolved},
		{"still there at the head", Evidence{HeadRun: true, StillReported: true}, Untouched},
		{"no run at the head to compare with, lines unchanged", Evidence{HeadRun: false}, Untouched},
	}
	for _, c := range cases {
		got, reason := Classify(c.in)
		if got != c.want {
			t.Errorf("%s: want %s, got %s (%s)", c.name, c.want, got, reason)
		}
		if reason == "" {
			t.Errorf("%s: every class comes with its reason", c.name)
		}
	}
}

// The same finding raised on two pushes is one finding, dated from the first push: a
// judge that repeats itself is not two chances to act.
func TestFirstRaisedDedupesAcrossRuns(t *testing.T) {
	recs := fixtures(t)
	later := recs.Lanes[2] // idiom on a1a1a1a, with two findings
	later.SHA, later.RecordedAt = "a2a2a2a", later.RecordedAt.Add(time.Hour)
	recs.Lanes = append(recs.Lanes, later)
	first := FirstRaised(recs.Lanes, 1)
	if len(first) != 3 { // print-logging, naming, mvvm — once each
		t.Fatalf("want 3 distinct findings on PR 1, got %d: %+v", len(first), first)
	}
	for _, f := range first {
		if f.SHA != "a1a1a1a" {
			t.Errorf("%s dates from the first push it was raised on, got %s", f.Rule, f.SHA)
		}
	}
	// And the head run is the set of keys reported at one SHA.
	head := ReportedAt(recs.Lanes, 1, "a2a2a2a")
	if !head[Key{Lane: "idiom", Rule: "review/naming", File: "Packages/MindlensKit/Sources/Features/Dashboard/MoodBadge.swift", Line: 3}] {
		t.Errorf("the repeated finding is reported at the head: %v", head)
	}
	if _, ok := HeadRun(recs.Lanes, 1, "zzz"); ok {
		t.Error("no run at an unknown SHA")
	}
}
