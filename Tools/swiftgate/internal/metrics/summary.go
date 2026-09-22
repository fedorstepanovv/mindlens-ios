package metrics

import (
	"fmt"
	"sort"
	"strings"

	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/gate"
)

// Tally sums a quantity over the runs that reported it. Known is how many did; a
// mean over the others would be a number about nothing.
type Tally struct {
	Known int
	Sum   float64
}

func (t *Tally) add(v float64) { t.Known++; t.Sum += v }

// Mean is the average over the runs that reported, or 0 when none did.
func (t Tally) Mean() float64 {
	if t.Known == 0 {
		return 0
	}
	return t.Sum / float64(t.Known)
}

// LaneStats is one lane summed over every run in the records.
type LaneStats struct {
	Name string
	// Runs is every record for the lane, including skipped ones; Judged is those on
	// which a judge was called and so cost something.
	Runs, Judged, Skipped int
	Verdicts              map[gate.Verdict]int
	Causes                map[gate.Cause]int
	Findings              int
	// Unproven is how many findings this lane wrote without a proof and so never
	// reported. High beside a low Findings count is a lane writing things it cannot
	// support — which is what the kill rule is for.
	Unproven int
	// Models counts judged runs by model. A noise rate pooled across two models is a
	// number about neither, so the report shows which produced these records.
	Models     map[string]int
	Severities map[gate.Severity]int
	Cost       Tally // USD, over the runs that reported it
	Duration   Tally // milliseconds, likewise
	Outcomes   map[Class]int
}

// Classified is how many of the lane's findings have an outcome record.
func (l LaneStats) Classified() int {
	n := 0
	for _, c := range l.Outcomes {
		n += c
	}
	return n
}

// Noise is untouched ÷ classified: the share of findings nobody acted on. False when
// nothing has been classified, so an unmeasured lane is not shown as a clean one.
func (l LaneStats) Noise() (float64, bool) {
	return noise(l.Outcomes)
}

// RuleStats is one rule id summed over every finding that named it.
type RuleStats struct {
	Rule     string
	Fires    int
	ByLane   map[string]int
	Outcomes map[Class]int
}

// Noise is untouched ÷ classified for this rule; see LaneStats.Noise.
func (r RuleStats) Noise() (float64, bool) { return noise(r.Outcomes) }

func noise(outcomes map[Class]int) (float64, bool) {
	n := 0
	for _, c := range outcomes {
		n += c
	}
	if n == 0 {
		return 0, false
	}
	return float64(outcomes[Untouched]) / float64(n), true
}

// PRStats is one pull request: how many gate runs it took and what they cost.
type PRStats struct {
	PR   int
	Runs int
	Cost Tally
	// CostComplete is true when every judged run reported a cost, so Cost.Sum is the
	// bill and not a floor.
	CostComplete bool
}

// Summary is the aggregate: the answers to "which lane earns its keep, which rules
// never fire, what does a pull request cost, and what fraction of findings changed
// the code".
type Summary struct {
	Runs, PRs    int
	Findings     int         // fired, across every lane record
	Classified   int         // of those, with an outcome record
	Lanes        []LaneStats // gate order, static last
	Rules        []RuleStats // most fired first
	NeverFired   []string    // static rules in the catalog with no finding, sorted
	PullRequests []PRStats   // by number
	lanes        map[string]*LaneStats
	rules        map[string]*RuleStats
	prs          map[int]*PRStats
}

// Lane, Rule and PR look one row up.
func (s Summary) Lane(name string) (LaneStats, bool) {
	l, ok := s.lanes[name]
	if !ok {
		return LaneStats{}, false
	}
	return *l, true
}

func (s Summary) Rule(id string) (RuleStats, bool) {
	r, ok := s.rules[id]
	if !ok {
		return RuleStats{}, false
	}
	return *r, true
}

func (s Summary) PR(n int) (PRStats, bool) {
	p, ok := s.prs[n]
	if !ok {
		return PRStats{}, false
	}
	return *p, true
}

// laneOrder is the gate's reporting order, with the static pass last.
var laneOrder = []string{"verification", "idiom", "spec", StaticLane}

// Summarise sums the records. catalog is every static rule id, so the rules that
// never fired can be listed; pass scan.RuleIDs().
func Summarise(recs Records, catalog []string) Summary {
	s := Summary{lanes: map[string]*LaneStats{}, rules: map[string]*RuleStats{}, prs: map[int]*PRStats{}}
	runs := map[string]bool{}
	prs := map[int]bool{}

	for _, r := range recs.Lanes {
		runs[fmt.Sprintf("%d@%s", r.PR, r.SHA)] = true
		prs[r.PR] = true
		l := s.lane(r.Lane)
		l.Runs++
		if r.Skipped != "" {
			l.Skipped++
		} else {
			l.Verdicts[r.Verdict]++
		}
		if r.Cause != "" {
			l.Causes[r.Cause]++
		}
		if r.Judged() {
			l.Judged++
			if r.Model != "" {
				l.Models[r.Model]++
			}
		}
		l.Unproven += r.Unproven
		if r.CostUSD != nil {
			l.Cost.add(*r.CostUSD)
		}
		if r.DurationMS != nil {
			l.Duration.add(float64(*r.DurationMS))
		}
		for _, f := range r.Findings {
			l.Findings++
			l.Severities[f.Severity]++
			s.Findings++
			rule := s.rule(f.Rule)
			rule.Fires++
			rule.ByLane[r.Lane]++
		}

		p := s.pr(r.PR)
		if r.CostUSD != nil {
			p.Cost.add(*r.CostUSD)
		} else if r.Judged() {
			p.CostComplete = false
		}
	}

	for _, o := range recs.Outcomes {
		s.Classified++
		s.lane(o.Lane).Outcomes[o.Outcome]++
		s.rule(o.Rule).Outcomes[o.Outcome]++
	}

	// Runs per PR are distinct SHAs, not records.
	perPR := map[int]map[string]bool{}
	for _, r := range recs.Lanes {
		if perPR[r.PR] == nil {
			perPR[r.PR] = map[string]bool{}
		}
		perPR[r.PR][r.SHA] = true
	}
	for n, shas := range perPR {
		s.prs[n].Runs = len(shas)
	}

	s.Runs, s.PRs = len(runs), len(prs)
	for _, name := range laneOrder {
		if l, ok := s.lanes[name]; ok {
			s.Lanes = append(s.Lanes, *l)
		}
	}
	for name, l := range s.lanes {
		if !contains(laneOrder, name) {
			s.Lanes = append(s.Lanes, *l)
		}
	}
	for _, r := range s.rules {
		s.Rules = append(s.Rules, *r)
	}
	sort.Slice(s.Rules, func(i, j int) bool {
		if s.Rules[i].Fires != s.Rules[j].Fires {
			return s.Rules[i].Fires > s.Rules[j].Fires
		}
		return s.Rules[i].Rule < s.Rules[j].Rule
	})
	for _, id := range catalog {
		if _, fired := s.rules[id]; !fired {
			s.NeverFired = append(s.NeverFired, id)
		}
	}
	sort.Strings(s.NeverFired)
	for _, p := range s.prs {
		s.PullRequests = append(s.PullRequests, *p)
	}
	sort.Slice(s.PullRequests, func(i, j int) bool { return s.PullRequests[i].PR < s.PullRequests[j].PR })
	return s
}

func (s *Summary) lane(name string) *LaneStats {
	l, ok := s.lanes[name]
	if !ok {
		l = &LaneStats{Name: name, Verdicts: map[gate.Verdict]int{}, Causes: map[gate.Cause]int{},
			Models: map[string]int{}, Severities: map[gate.Severity]int{}, Outcomes: map[Class]int{}}
		s.lanes[name] = l
	}
	return l
}

func (s *Summary) rule(id string) *RuleStats {
	r, ok := s.rules[id]
	if !ok {
		r = &RuleStats{Rule: id, ByLane: map[string]int{}, Outcomes: map[Class]int{}}
		s.rules[id] = r
	}
	return r
}

func (s *Summary) pr(n int) *PRStats {
	p, ok := s.prs[n]
	if !ok {
		p = &PRStats{PR: n, CostComplete: true}
		s.prs[n] = p
	}
	return p
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// Markdown renders the report a human reads: one table per question.
func (s Summary) Markdown() string {
	var b strings.Builder
	b.WriteString("# Judge lane metrics\n\n")
	if s.Runs == 0 {
		b.WriteString("No records. Nothing has run through the gate with metrics on, or the artifacts were not downloaded.\n")
		return b.String()
	}
	fmt.Fprintf(&b, "%d run(s) over %d pull request(s); %d finding(s) fired, %d classified at merge.\n\n", s.Runs, s.PRs, s.Findings, s.Classified)

	b.WriteString("## Which lane earns its keep\n\n")
	b.WriteString("| Lane | Model | Runs | Judged | Skipped | PASS | CONCERNS | BLOCK | CANNOT_EVALUATE | Findings | Unproven | Mean cost | Mean duration | Noise |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|---|---|---|---|---|---|\n")
	for _, l := range s.Lanes {
		cannot := fmt.Sprint(l.Verdicts[gate.CannotEvaluate])
		if n := l.Verdicts[gate.CannotEvaluate]; n > 0 {
			var causes []string
			for _, c := range []gate.Cause{gate.CauseEvidence, gate.CauseJudge, gate.CauseOutput} {
				if l.Causes[c] > 0 {
					causes = append(causes, fmt.Sprintf("%d %s", l.Causes[c], c))
				}
			}
			cannot = fmt.Sprintf("%d (%s)", n, strings.Join(causes, ", "))
		}
		fmt.Fprintf(&b, "| %s | %s | %d | %d | %d | %d | %d | %d | %s | %d | %d | %s | %s | %s |\n",
			l.Name, models(l.Models), l.Runs, l.Judged, l.Skipped,
			l.Verdicts[gate.Pass], l.Verdicts[gate.Concerns], l.Verdicts[gate.Block], cannot,
			l.Findings, l.Unproven, money(l.Cost, l.Judged), duration(l.Duration, l.Judged), noiseCell(l.Outcomes))
	}
	b.WriteString("\nNoise is untouched ÷ classified: the share of a lane's findings that the merged code did not answer. " +
		"A lane whose CANNOT_EVALUATE count is mostly `judge` or `output` is failing on infrastructure, not on evidence (ADR 0014). " +
		"Two models in one row means the noise rate pools two judges and is a number about neither — split it before citing it.\n\n" +
		"**Grant** (ADR 0021): `blocks: true` on ≥10 judged pull requests, noise ≤30%, ≥1 finding classified `changed`. " +
		"**Kill**: ≥10 judged runs, noise >50%, zero `changed` — the lane is deleted, with an ADR. Both are reviewed pull " +
		"requests citing this table; neither is automatic.\n\n")

	b.WriteString("## What each pull request cost\n\n| PR | Runs | Cost |\n|---|---|---|\n")
	for _, p := range s.PullRequests {
		cost := "not reported"
		if p.Cost.Known > 0 {
			cost = fmt.Sprintf("$%.2f", p.Cost.Sum)
			if !p.CostComplete {
				cost += " (a floor: not every judged run reported)"
			}
		}
		fmt.Fprintf(&b, "| #%d | %d | %s |\n", p.PR, p.Runs, cost)
	}

	b.WriteString("\n## Rules, most fired first\n\n| Rule | Fires | From | Changed | Resolved | Waived | Untouched | Noise |\n|---|---|---|---|---|---|---|---|\n")
	for _, r := range s.Rules {
		var from []string
		for lane, n := range r.ByLane {
			from = append(from, fmt.Sprintf("%s ×%d", lane, n))
		}
		sort.Strings(from)
		fmt.Fprintf(&b, "| `%s` | %d | %s | %d | %d | %d | %d | %s |\n", r.Rule, r.Fires, strings.Join(from, ", "),
			r.Outcomes[Changed], r.Outcomes[Resolved], r.Outcomes[Waived], r.Outcomes[Untouched], noiseCell(r.Outcomes))
	}

	b.WriteString("\n## Static rules that never fired\n\n")
	if len(s.NeverFired) == 0 {
		b.WriteString("Every rule in the catalog has fired at least once.\n")
	} else {
		fmt.Fprintf(&b, "%d of the catalog's rules have never fired in these records. A rule that never fires is either "+
			"holding the line or costing nothing; a rule that fires and is always untouched is noise.\n\n", len(s.NeverFired))
		for _, id := range s.NeverFired {
			fmt.Fprintf(&b, "- `%s`\n", id)
		}
	}
	return b.String()
}

// models renders which judge produced a lane's records, with the count when more than
// one did — a rate over two models is a rate about neither.
func models(counts map[string]int) string {
	if len(counts) == 0 {
		return "—"
	}
	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) == 1 {
		return "`" + names[0] + "`"
	}
	for i, name := range names {
		names[i] = fmt.Sprintf("`%s` ×%d", name, counts[name])
	}
	return strings.Join(names, ", ")
}

func money(t Tally, judged int) string {
	if t.Known == 0 {
		return "not reported"
	}
	cell := fmt.Sprintf("$%.2f", t.Mean())
	if t.Known < judged {
		cell += fmt.Sprintf(" (%d of %d reported)", t.Known, judged)
	}
	return cell
}

func duration(t Tally, judged int) string {
	if t.Known == 0 {
		return "not reported"
	}
	cell := fmt.Sprintf("%.0f s", t.Mean()/1000)
	if t.Known < judged {
		cell += fmt.Sprintf(" (%d of %d reported)", t.Known, judged)
	}
	return cell
}

func noiseCell(outcomes map[Class]int) string {
	n := 0
	for _, c := range outcomes {
		n += c
	}
	if n == 0 {
		return "unmeasured"
	}
	return fmt.Sprintf("%s (%d of %d untouched)", percent(outcomes[Untouched], n), outcomes[Untouched], n)
}
