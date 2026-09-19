// Package evidence asserts, before a judge lane runs, that the lane's inputs exist and
// are non-empty. Absence is read off a list, never judged: a lane whose evidence is
// missing is CANNOT_EVALUATE before a model is called, so a checkout that "succeeded"
// with an empty tree can never again be reported as a pass.
package evidence

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/scan"
)

// Lane names one judge. Each has its own rubric, its own evidence, its own verdict.
type Lane string

const (
	// Verification asks whether the suite exercises the changed production path.
	Verification Lane = "verification"
	// Idiom asks whether this is the Swift we write here, or translated Dart.
	Idiom Lane = "idiom"
	// Spec asks whether the diff does what the feature step agreed.
	Spec Lane = "spec"
)

// Lanes is every lane, in the order they are reported.
var Lanes = []Lane{Verification, Idiom, Spec}

// Parse turns a flag value into a Lane.
func Parse(s string) (Lane, bool) {
	for _, l := range Lanes {
		if string(l) == strings.ToLower(strings.TrimSpace(s)) {
			return l, true
		}
	}
	return "", false
}

// Inputs is everything a gate may read off. Nothing here is a judgement.
type Inputs struct {
	RepoDir string
	Diff    scan.Diff
	// Branch is the head branch, e.g. feature/auth. The spec lane reads the feature file
	// it names.
	Branch string
	// Exemplars is how many merged files were retrieved for the idiom lane to judge
	// against. The Dart checkout used to be this lane's evidence; it never worked, and
	// a positive oracle from this repository needs no second checkout.
	Exemplars int
}

// Result is what one lane's gate found: each required input, present or missing.
type Result struct {
	Lane Lane `json:"lane"`
	// Skipped says why the lane has nothing to judge on this pull request. Not applying
	// is not the same as failing.
	Skipped string   `json:"skipped,omitempty"`
	Present []string `json:"present,omitempty"`
	Missing []string `json:"missing,omitempty"`
}

// Applies reports whether the lane has something to judge.
func (r Result) Applies() bool { return r.Skipped == "" }

// OK reports whether every required input is there.
func (r Result) OK() bool { return len(r.Missing) == 0 }

// String renders the result the way the log and the report say it.
func (r Result) String() string {
	switch {
	case !r.Applies():
		return fmt.Sprintf("%s: skipped — %s", r.Lane, r.Skipped)
	case !r.OK():
		return fmt.Sprintf("%s: cannot evaluate — missing %s", r.Lane, strings.Join(r.Missing, "; "))
	default:
		return fmt.Sprintf("%s: evidence present — %s", r.Lane, strings.Join(r.Present, "; "))
	}
}

// Check runs one lane's gate.
func Check(lane Lane, in Inputs) Result {
	switch lane {
	case Verification:
		return verification(in)
	case Idiom:
		return idiom(in)
	case Spec:
		return spec(in)
	}
	return Result{Lane: lane, Missing: []string{fmt.Sprintf("a lane named %q", lane)}}
}

// All runs every lane's gate.
func All(in Inputs) map[Lane]Result {
	out := make(map[Lane]Result, len(Lanes))
	for _, l := range Lanes {
		out[l] = Check(l, in)
	}
	return out
}

const packageSources = "Packages/MindlensKit/Sources/"

// verification needs production Swift in the diff and a test target, non-empty, for
// each module that Swift lives in. Whether the target compiles is the build job's
// business; that it exists and has tests in it is read off disk here.
func verification(in Inputs) Result {
	r := Result{Lane: Verification}
	targets := map[string]string{} // module → test target dir, repo-relative
	for _, f := range in.Diff.SwiftFiles() {
		if f.IsTest() {
			continue
		}
		switch {
		case f.FeatureTarget() != "":
			targets[f.FeatureTarget()] = "Packages/MindlensKit/Tests/" + f.FeatureTarget() + "Tests"
		case strings.HasPrefix(f.Path, packageSources):
			module, _, _ := strings.Cut(strings.TrimPrefix(f.Path, packageSources), "/")
			targets[module] = "Packages/MindlensKit/Tests/" + module + "Tests"
		case strings.HasPrefix(f.Path, "mindlens/"):
			targets["mindlens"] = "mindlensUITests"
		}
	}
	if len(targets) == 0 {
		r.Skipped = "no production Swift changed"
		return r
	}
	for module, dir := range targets {
		if n := countFiles(filepath.Join(in.RepoDir, dir), ".swift"); n > 0 {
			r.Present = append(r.Present, fmt.Sprintf("%s (%d test file(s)) for %s", dir, n, module))
		} else {
			r.Missing = append(r.Missing, fmt.Sprintf("a test target for %s: %s has no Swift in it", module, dir))
		}
	}
	return r
}

// idiom needs the diff and at least one exemplar to hold it against. The bug this
// package exists for was a judge run without its standard: the checkout directory was
// there, the Dart was not, and the gate passed.
func idiom(in Inputs) Result {
	r := Result{Lane: Idiom}
	if len(in.Diff.SwiftFiles()) == 0 {
		r.Skipped = "no Swift changed"
		return r
	}
	if strings.TrimSpace(in.Diff.Unified) == "" {
		r.Missing = append(r.Missing, "a non-empty diff")
	} else {
		r.Present = append(r.Present, fmt.Sprintf("a diff over %d Swift file(s)", len(in.Diff.SwiftFiles())))
	}
	if in.Exemplars > 0 {
		r.Present = append(r.Present, fmt.Sprintf("%d exemplar(s) from the base branch", in.Exemplars))
	} else {
		r.Missing = append(r.Missing, "an exemplar — no merged Swift resembles the changed files, so the lane has no standard to hold them to")
	}
	return r
}

var openStep = regexp.MustCompile(`(?m)^\d+\.\s+(?:🟡|⬜)`)

// spec needs the feature file the branch names, with a step still open in it. Every
// initiative is a feature/ branch (ADR 0015), so one with no feature file is tooling,
// not a product feature: the lane skips, and the readiness comment shows that it did.
func spec(in Inputs) Result {
	r := Result{Lane: Spec}
	name, ok := strings.CutPrefix(in.Branch, "feature/")
	if !ok || name == "" {
		r.Skipped = "not a feature branch"
		return r
	}
	rel := "docs/features/" + name + ".md"
	body, err := os.ReadFile(filepath.Join(in.RepoDir, filepath.FromSlash(rel)))
	if err != nil {
		r.Skipped = "no " + rel + " — a feature branch without a feature file is tooling, not a product feature"
		return r
	}
	if !openStep.Match(body) {
		r.Missing = append(r.Missing, fmt.Sprintf("an unticked step (🟡 or ⬜) in %s", rel))
		return r
	}
	r.Present = append(r.Present, rel+" with an open step")
	return r
}

func countFiles(dir, ext string) int {
	n := 0
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(d.Name(), ext) {
			n++
		}
		return nil
	})
	return n
}
