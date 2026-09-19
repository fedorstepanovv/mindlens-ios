package scan

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
)

// Exemplar is a merged file that resembles a changed one: the positive oracle the idiom
// lane judges against. "Match this" is a sharper instruction than "do not resemble
// that", it needs no second checkout, and it works on the pull requests — most of
// them — whose screens have no Dart counterpart.
type Exemplar struct {
	// For is the changed file this exemplar was chosen for.
	For  string
	Path string
	Body string
}

// Exemplars picks up to perFile files from baseRef that resemble each changed
// production Swift file, by kind, directory and name. Reading them off the base ref
// rather than the working tree is what makes them merged code and not this PR's own.
func Exemplars(repoDir, baseRef string, changed []ChangedFile, perFile, maxBytes int) []Exemplar {
	if len(changed) == 0 {
		return nil
	}
	changedPaths := map[string]bool{}
	for _, f := range changed {
		changedPaths[f.Path] = true
	}

	listing, err := git(repoDir, "ls-tree", "-r", "--name-only", baseRef, "--", "Packages/MindlensKit/Sources", "mindlens")
	if err != nil {
		return nil
	}
	var pool []candidate
	for _, p := range strings.Split(strings.TrimSpace(listing), "\n") {
		cf := ChangedFile{Path: p}
		if !cf.IsSwift() || cf.IsTest() || changedPaths[p] {
			continue
		}
		body, err := git(repoDir, "show", baseRef+":"+p)
		if err != nil {
			continue
		}
		pool = append(pool, describe(p, body))
	}

	var out []Exemplar
	for _, f := range changed {
		if !f.IsSwift() || f.IsTest() {
			continue
		}
		var added strings.Builder
		for _, l := range f.Added {
			added.WriteString(l.Text)
			added.WriteByte('\n')
		}
		for _, c := range rank(describe(f.Path, added.String()), pool, perFile) {
			out = append(out, Exemplar{For: f.Path, Path: c.path, Body: capString(c.body, maxBytes)})
		}
	}
	return out
}

// candidate is a file reduced to the features similarity is judged on.
type candidate struct {
	path, body string
	dir        string // its directory
	module     string // Sources/<Module> or Features/<Feature>, or the app target
	suffix     string // the last capitalised word of the file name: View, Model, Repository…
	kinds      map[string]bool
}

var kindMarkers = map[string]*regexp.Regexp{
	"view":     regexp.MustCompile(`:\s*View\b|some View\b`),
	"model":    regexp.MustCompile(`@Observable\b`),
	"actor":    regexp.MustCompile(`\bactor\s+\w+`),
	"protocol": regexp.MustCompile(`(?m)^\s*(?:public\s+)?protocol\s+\w+`),
	"wire":     regexp.MustCompile(`\bEndpoint\b|\bDTO\b|\bDecodable\b|\bCodable\b`),
	"keychain": regexp.MustCompile(`\bSecItem|\bKeychain\w*\b`),
	// No marker for AppError: nearly every file names it, so it would make everything
	// resemble everything.
}

var suffixShape = regexp.MustCompile(`([A-Z][a-z]+)\.swift$`)

func describe(p, body string) candidate {
	c := candidate{path: p, body: body, dir: path.Dir(p), kinds: map[string]bool{}}
	cf := ChangedFile{Path: p}
	switch {
	case cf.FeatureTarget() != "":
		c.module = "Features/" + cf.FeatureTarget()
	case strings.HasPrefix(p, "Packages/MindlensKit/Sources/"):
		c.module, _, _ = strings.Cut(strings.TrimPrefix(p, "Packages/MindlensKit/Sources/"), "/")
	default:
		c.module = "app"
	}
	if m := suffixShape.FindStringSubmatch(p); m != nil {
		c.suffix = m[1]
	}
	for kind, re := range kindMarkers {
		if re.MatchString(body) {
			c.kinds[kind] = true
		}
	}
	return c
}

// rank orders the pool by resemblance to target and keeps the best n with any
// resemblance at all. The weights say what "structurally similar" means here: the same
// kind of type first, then the same name shape, then the same neighbourhood.
func rank(target candidate, pool []candidate, n int) []candidate {
	type scored struct {
		candidate
		score int
	}
	var all []scored
	for _, c := range pool {
		s := 0
		for kind := range target.kinds {
			if c.kinds[kind] {
				s += 4
			}
		}
		if target.suffix != "" && c.suffix == target.suffix {
			s += 3
		}
		if c.dir == target.dir {
			s += 3
		} else if c.module == target.module {
			s += 2
		}
		if s > 0 {
			all = append(all, scored{c, s})
		}
	}
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].score != all[j].score {
			return all[i].score > all[j].score
		}
		return all[i].path < all[j].path
	})
	var out []candidate
	for i := 0; i < len(all) && i < n; i++ {
		out = append(out, all[i].candidate)
	}
	return out
}

// Describe renders the exemplar's provenance for the brief.
func (e Exemplar) Describe() string {
	return fmt.Sprintf("`%s`, chosen for `%s`", e.Path, e.For)
}
