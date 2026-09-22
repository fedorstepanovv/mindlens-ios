package scan

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/gate"
)

// Severities lets the project retune a rule without a code change. Keys are rule IDs;
// "off" disables one outright.
type Severities map[string]gate.Severity

func (s Severities) for_(id string, fallback gate.Severity) gate.Severity {
	if v, ok := s[id]; ok {
		return v
	}
	return fallback
}

// waiver matches an in-code exemption: `// swiftgate:allow rule/id — because reasons`.
// It has to name the rule and give a reason, so a waiver is reviewable in the diff
// rather than a blanket silence.
var waiver = regexp.MustCompile(`//\s*swiftgate:allow\s+([\w/\-]+)\s*[—:-]+\s*(\S.*)$`)

// Static runs the deterministic pass over the lines this PR added.
type Static struct {
	RepoDir    string
	Severities Severities
}

// Run evaluates every rule. It deliberately looks only at added lines: a gate that
// reports pre-existing debt on an unrelated PR is a gate people route around.
func (s Static) Run(d Diff) []gate.Finding {
	var out []gate.Finding
	files := d.SwiftFiles()

	for _, f := range files {
		out = append(out, s.lineFindings(f)...)
	}
	out = append(out, s.moduleGraphFindings(files)...)
	out = append(out, s.namingFindings(files)...)

	for i := range out {
		out[i].Source = gate.FromStatic
	}
	return out
}

func (s Static) lineFindings(f ChangedFile) []gate.Finding {
	var out []gate.Finding
	waived := waivedRules(f)

	for _, rule := range lineRules {
		sev := s.Severities.for_(rule.ID, rule.Default)
		if sev == gate.Off || !rule.Scope.matches(f) {
			continue
		}
		for _, line := range f.Added {
			text := stripComment(line.Text)
			if !rule.Pattern.MatchString(text) {
				continue
			}
			if rule.Except != nil && rule.Except.MatchString(line.Text) {
				continue
			}
			if waived[rule.ID] || waiverOn(f, line.Number, rule.ID) {
				continue
			}
			out = append(out, gate.Finding{
				Rule:     rule.ID,
				Severity: sev,
				File:     f.Path,
				Line:     line.Number,
				Title:    rule.Title,
				Detail:   rule.Detail + "\n\n```swift\n" + strings.TrimSpace(line.Text) + "\n```",
				Fix:      rule.Fix,
				Doc:      rule.Doc,
			})
		}
	}
	return out
}

// waivedRules collects file-wide waivers written on their own line.
func waivedRules(f ChangedFile) map[string]bool {
	out := map[string]bool{}
	for _, l := range f.Added {
		if m := waiver.FindStringSubmatch(l.Text); m != nil && strings.HasPrefix(strings.TrimSpace(l.Text), "//") {
			out[m[1]] = true
		}
	}
	return out
}

// waiverOn reports whether the offending line carries a trailing waiver for this rule.
func waiverOn(f ChangedFile, line int, rule string) bool {
	for _, l := range f.Added {
		if l.Number != line {
			continue
		}
		if m := waiver.FindStringSubmatch(l.Text); m != nil && m[1] == rule {
			return true
		}
	}
	return false
}

// stripComment removes a trailing line comment so a rule does not fire on prose that
// merely mentions the thing it forbids — including this project's own doc comments,
// which discuss Cubits and singletons at length.
func stripComment(line string) string {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "*") || strings.HasPrefix(trimmed, "/*") {
		return ""
	}
	if i := strings.Index(line, "//"); i >= 0 && !inStringLiteral(line, i) {
		return line[:i]
	}
	return line
}

// inStringLiteral reports whether position i sits inside a double-quoted string, so a
// URL like "https://..." is not mistaken for a comment.
func inStringLiteral(line string, i int) bool {
	quotes := 0
	for j := 0; j < i; j++ {
		if line[j] == '"' && (j == 0 || line[j-1] != '\\') {
			quotes++
		}
	}
	return quotes%2 == 1
}

var importLine = regexp.MustCompile(`^\s*(?:@testable\s+|@_exported\s+)?import\s+([A-Za-z_][\w.]*)`)

// knownSDKs are third-party modules that must not appear outside the app target.
// The list is the SDKs this product actually pulls in; anything else unfamiliar is
// left to the model, which can tell a vendor SDK from an Apple framework in context.
var knownSDKs = map[string]string{
	"FirebaseAuth": "Firebase", "FirebaseCore": "Firebase", "FirebaseAnalytics": "Firebase",
	"FirebaseMessaging": "Firebase", "FirebaseCrashlytics": "Firebase", "FirebaseRemoteConfig": "Firebase",
	"RevenueCat": "RevenueCat", "RevenueCatUI": "RevenueCat",
	"GoogleSignIn": "GoogleSignIn", "AppsFlyerLib": "AppsFlyer", "Mixpanel": "Mixpanel",
	"Amplitude": "Amplitude", "Sentry": "Sentry", "Alamofire": "Alamofire",
	"Kingfisher": "Kingfisher", "SnapKit": "SnapKit", "Lottie": "Lottie",
	"SwiftyJSON": "SwiftyJSON", "RealmSwift": "Realm", "Moya": "Moya", "PostHog": "PostHog",
}

// moduleGraphFindings enforces the dependency rules SPM cannot express as an error
// message a reviewer will understand — and catches them before a slow build does.
func (s Static) moduleGraphFindings(files []ChangedFile) []gate.Finding {
	var out []gate.Finding
	features := s.featureTargets()

	for _, f := range files {
		owner := f.FeatureTarget()
		waived := waivedRules(f)

		for _, line := range f.Added {
			m := importLine.FindStringSubmatch(line.Text)
			if m == nil {
				continue
			}
			imported := m[1]

			if owner != "" && imported != owner && features[imported] {
				if sev := s.Severities.for_("arch/feature-imports-feature", gate.Blocker); sev != gate.Off && !waived["arch/feature-imports-feature"] {
					out = append(out, gate.Finding{
						Rule:     "arch/feature-imports-feature",
						Severity: sev,
						File:     f.Path,
						Line:     line.Number,
						Title:    "Feature `" + owner + "` imports feature `" + imported + "`",
						Detail:   "Features are siblings, not a hierarchy. Once one imports another they cannot be built, tested, or reasoned about separately, and the graph quietly becomes a ball of mud.",
						Fix:      "Move the shared thing down into `Core`/`Models`/`DesignSystem`, or — if this is navigation — emit a typed destination the app target's router resolves.",
						Doc:      "docs/ARCHITECTURE.md § Dependency rules (1)",
					})
				}
			}

			if vendor, third := knownSDKs[imported]; third && strings.Contains(f.Path, "Packages/MindlensKit") && !f.IsTest() {
				if sev := s.Severities.for_("arch/third-party-import", gate.Blocker); sev != gate.Off && !waived["arch/third-party-import"] {
					out = append(out, gate.Finding{
						Rule:     "arch/third-party-import",
						Severity: sev,
						File:     f.Path,
						Line:     line.Number,
						Title:    vendor + " SDK imported inside the package",
						Detail:   "No target but the app imports a third-party SDK. Otherwise the SDK's types leak into feature code, the feature stops being testable without it, and replacing the vendor becomes a rewrite.",
						Fix:      "Define a protocol here for what you need, and bind it to " + vendor + " at the app target's composition root. A stub implementation is the default.",
						Doc:      "docs/decisions/0005-third-party-sdks-behind-protocols.md",
					})
				}
			}
		}
	}
	return out
}

// featureTargets reads the feature target names out of Package.swift, so adding a
// feature does not mean editing this tool.
func (s Static) featureTargets() map[string]bool {
	out := map[string]bool{}
	data, err := os.ReadFile(filepath.Join(s.RepoDir, "Packages", "MindlensKit", "Package.swift"))
	if err != nil {
		return out
	}
	re := regexp.MustCompile(`\.target\(\s*name:\s*"(\w+)"[^)]*path:\s*"Sources/Features/`)
	for _, m := range re.FindAllStringSubmatch(string(data), -1) {
		out[m[1]] = true
	}
	// Fall back to the directory listing when the manifest is formatted unusually.
	if len(out) == 0 {
		entries, _ := os.ReadDir(filepath.Join(s.RepoDir, "Packages", "MindlensKit", "Sources", "Features"))
		for _, e := range entries {
			if e.IsDir() {
				out[e.Name()] = true
			}
		}
	}
	return out
}

var flutterFileName = regexp.MustCompile(`_(?:cubit|bloc|state|event|screen|page|widget|model|repository|service)\.swift$`)

// namingFindings catches files that were carried over rather than written.
func (s Static) namingFindings(files []ChangedFile) []gate.Finding {
	sev := s.Severities.for_("flutter/dart-file-naming", gate.Blocker)
	if sev == gate.Off {
		return nil
	}
	var out []gate.Finding
	for _, f := range files {
		if f.Status != "A" {
			continue
		}
		name := Base(f.Path)
		if !flutterFileName.MatchString(strings.ToLower(name)) {
			continue
		}
		out = append(out, gate.Finding{
			Rule:     "flutter/dart-file-naming",
			Severity: sev,
			File:     f.Path,
			Title:    "Dart file naming: `" + name + "`",
			Detail:   "snake_case names ending in `_cubit`, `_state`, `_screen` or `_widget` are Dart conventions. A file named this way is usually a translated file, not a designed one.",
			Fix:      "Name the file after the Swift type it declares, in UpperCamelCase — `DashboardModel.swift`, `MoodBadge.swift`.",
			Doc:      "AGENTS.md § The one rule that matters most",
		})
	}
	return out
}
