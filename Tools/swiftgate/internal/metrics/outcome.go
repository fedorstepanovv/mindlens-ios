package metrics

import (
	"fmt"
	"sort"
	"time"
)

// Evidence is what the classifier is told about one finding. All of it is read off
// git and the records; none of it is a judgement.
type Evidence struct {
	// Waived: a `// swiftgate:allow <rule>` covers the location at the head.
	Waived bool
	// Overridden: the pull request merged under the gate-override label.
	Overridden bool
	// Deleted: the file no longer exists at the head.
	Deleted bool
	// Touched: the lines the finding pointed at changed between its SHA and the head.
	Touched bool
	// HeadRun: a gate run at the merged head exists in the records.
	HeadRun bool
	// StillReported: that run reported the same finding.
	StillReported bool
}

// Classify applies the precedence in the Class doc comments: waived, changed,
// resolved, untouched. The second value is the one-line reason for the record.
func Classify(e Evidence) (Class, string) {
	switch {
	case e.Waived:
		return Waived, "a swiftgate:allow for the rule covers the location at the head"
	case e.Overridden:
		return Waived, "the pull request merged under the override label"
	case e.Deleted:
		return Changed, "the file is gone at the head"
	case e.Touched:
		return Changed, "the lines the finding pointed at were edited after it was raised"
	case e.HeadRun && !e.StillReported:
		return Resolved, "the run at the head no longer reported it, and the lines it pointed at did not change"
	case e.HeadRun:
		return Untouched, "the lines are unchanged at the head and the finding still stood there"
	default:
		return Untouched, "the lines are unchanged at the head, and no run at the head says otherwise"
	}
}

// Raised is one distinct finding on a pull request and the first SHA it was seen on.
type Raised struct {
	Key
	SHA      string
	Severity string
	Title    string
}

// FirstRaised lists each distinct finding on a pull request once, dated from the
// first run that reported it. A judge that repeats itself on every push is one
// finding, not one chance to act per push.
func FirstRaised(lanes []LaneRecord, pr int) []Raised {
	sorted := append([]LaneRecord(nil), lanes...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].RecordedAt.Before(sorted[j].RecordedAt) })

	seen := map[Key]bool{}
	var out []Raised
	for _, r := range sorted {
		if r.PR != pr {
			continue
		}
		for _, f := range r.Findings {
			k := f.key(r.Lane)
			if seen[k] {
				continue
			}
			seen[k] = true
			out = append(out, Raised{Key: k, SHA: r.SHA, Severity: string(f.Severity), Title: f.Title})
		}
	}
	return out
}

// HeadRun reports whether the records hold a run of this pull request at the SHA.
func HeadRun(lanes []LaneRecord, pr int, sha string) (time.Time, bool) {
	var latest time.Time
	found := false
	for _, r := range lanes {
		if r.PR == pr && r.SHA == sha {
			found = true
			if r.RecordedAt.After(latest) {
				latest = r.RecordedAt
			}
		}
	}
	return latest, found
}

// ReportedAt is the set of findings the run at one SHA reported.
func ReportedAt(lanes []LaneRecord, pr int, sha string) map[Key]bool {
	out := map[Key]bool{}
	for _, r := range lanes {
		if r.PR != pr || r.SHA != sha {
			continue
		}
		for _, f := range r.Findings {
			out[f.key(r.Lane)] = true
		}
	}
	return out
}

// String renders the class for a table.
func (c Class) String() string { return string(c) }

func percent(num, den int) string {
	if den == 0 {
		return "—"
	}
	return fmt.Sprintf("%d%%", int(float64(num)*100/float64(den)+0.5))
}
