package scan

import (
	"regexp"

	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/gate"
)

// scope narrows where a rule applies. Most idiom rules would fire noisily on test
// code, where a force-unwrap or a literal date is often the clearest thing to write.
type scope int

const (
	scopeAll scope = iota
	scopeNonTest
	scopeFeatures // Packages/MindlensKit/Sources/Features/**
)

func (s scope) matches(f ChangedFile) bool {
	switch s {
	case scopeNonTest:
		return !f.IsTest()
	case scopeFeatures:
		return f.FeatureTarget() != "" && !f.IsTest()
	default:
		return true
	}
}

// LineRule is a rule that can be decided by looking at a single added line.
//
// Every rule cites the document that gives it authority. A gate that says "don't do
// that" without saying who decided is a gate people learn to argue with.
type LineRule struct {
	ID      string
	Default gate.Severity
	Scope   scope
	Pattern *regexp.Regexp
	// Except, when it matches, cancels the finding — for the one place a pattern is
	// legitimate, such as Date() inside the DateProvider that exists to contain it.
	Except *regexp.Regexp
	Title  string
	Detail string
	Fix    string
	Doc    string
}

// lineRules is the deterministic half of the gate. These are the Flutter habits and
// non-idiomatic shapes that can be caught without a model reading anything.
var lineRules = []LineRule{
	{
		ID:      "flutter/observable-object",
		Default: gate.Blocker,
		Scope:   scopeAll,
		Pattern: regexp.MustCompile(`:\s*ObservableObject\b|@Published\b|@StateObject\b|@ObservedObject\b|@EnvironmentObject\b`),
		Title:   "Pre-Observation state management",
		Detail:  "`ObservableObject`/`@Published` is the Combine-era API. This project uses the Observation framework, where a plain property on an `@Observable` class is already observed and the view updates without a publisher.",
		Fix:     "Mark the class `@Observable @MainActor` and declare plain `private(set) var` properties. In the view, hold it with `@State`.",
		Doc:     "docs/PATTERNS.md § ViewModels",
	},
	{
		ID:      "flutter/impl-suffix",
		Default: gate.Blocker,
		Scope:   scopeAll,
		Pattern: regexp.MustCompile(`\b(?:final\s+)?(?:class|struct|enum|actor)\s+\w*Impl\b`),
		Title:   "`*Impl` type name",
		Detail:  "`FooRepositoryImpl` is a Dart/Java convention for \"the one implementation of the interface\". Swift names a type for what it is, and the protocol for the capability.",
		Fix:     "Name the concrete type for its behaviour — `LocalFirstMoodRepository`, `KeychainTokenStorage` — and let the protocol carry the generic name.",
		Doc:     "docs/PATTERNS.md § Repositories",
	},
	{
		ID:      "flutter/service-locator",
		Default: gate.Blocker,
		Scope:   scopeAll,
		Pattern: regexp.MustCompile(`\b(?:GetIt|ServiceLocator|DIContainer|Resolver|Injector)\b|@Injected\b|\bsl<|\blocator\.`),
		Title:   "Service locator",
		Detail:  "This is `get_it` in Swift clothing. A type that reaches into a global registry hides its dependencies from its signature, and a dependency you cannot see is one you cannot fake in a test.",
		Fix:     "Take the dependency through `init`. The composition root in the app target is the only place that knows concrete types.",
		Doc:     "docs/ARCHITECTURE.md § Dependency injection",
	},
	{
		ID:      "arch/our-singleton",
		Default: gate.Blocker,
		Scope:   scopeAll,
		Pattern: regexp.MustCompile(`static\s+(?:let|var)\s+shared\b`),
		Title:   "New `.shared` singleton",
		Detail:  "Declaring `static let shared` creates a dependency nothing can substitute. Apple's own `.shared` types are fine to *use*; ours are not fine to *declare*.",
		Fix:     "Make it an ordinary type and inject an instance. If exactly one should exist at runtime, that is the composition root's job to guarantee, not the type's.",
		Doc:     "docs/ARCHITECTURE.md § Dependency injection",
	},
	{
		ID:      "swift/dispatch-semaphore",
		Default: gate.Blocker,
		Scope:   scopeNonTest,
		Pattern: regexp.MustCompile(`\bDispatchSemaphore\b|\bDispatchGroup\b`),
		Title:   "Blocking primitive in an async codebase",
		Detail:  "A semaphore blocks a cooperative-pool thread. Under Swift Concurrency that risks starving the pool and, in the worst case, deadlocking — the runtime assumes threads are never blocked.",
		Fix:     "Use `await`, `async let`, or `withTaskGroup`. For shared mutable state, an `actor`.",
		Doc:     "CLAUDE.md § Concurrency",
	},
	{
		ID:      "swift/gcd-queue",
		Default: gate.Warning,
		Scope:   scopeNonTest,
		Pattern: regexp.MustCompile(`DispatchQueue\.(?:main|global)\s*(?:\(|\.)`),
		Except:  regexp.MustCompile(`\.(?:debounce|throttle|receive|subscribe)\(`),
		Title:   "GCD where structured concurrency belongs",
		Detail:  "Hopping queues by hand is the pre-async/await idiom. `@MainActor` expresses the same intent to the compiler, which can then check it.",
		Fix:     "Annotate the type or method `@MainActor`, or `await MainActor.run { }` at a genuine boundary. Keep `DispatchQueue.main` only as a Combine scheduler.",
		Doc:     "CLAUDE.md § Concurrency",
	},
	{
		ID:      "swift/combine-as-transport",
		Default: gate.Warning,
		Scope:   scopeNonTest,
		Pattern: regexp.MustCompile(`\bFuture<|\bPassthroughSubject<|\bCurrentValueSubject<|eraseToAnyPublisher\(\)`),
		Title:   "Combine carrying data between layers",
		Detail:  "Combine earns its place where a stream needs operators — debounce, dedupe, merge. Wrapping a one-shot call in a publisher chain is the Bloc-stream habit rewritten in Swift.",
		Fix:     "Return the value with `async throws`. Keep Combine for the streams that actually need operators.",
		Doc:     "docs/PATTERNS.md § Combine, used deliberately",
	},
	{
		ID:      "core/calendar-current",
		Default: gate.Blocker,
		Scope:   scopeNonTest,
		Pattern: regexp.MustCompile(`\bCalendar\.current\b|\bTimeZone\.current\b`),
		Title:   "Ambient calendar or timezone",
		Detail:  "\"Today\" in this product is a local day in the *user's* timezone, which is not necessarily the device's. Reading the ambient calendar scatters that decision and makes the DST cases untestable.",
		Fix:     "Go through the date helper in `Core` and take the timezone from the user's profile.",
		Doc:     "docs/PATTERNS.md § Dates and timezones",
	},
	{
		ID:      "core/raw-date-now",
		Default: gate.Blocker,
		Scope:   scopeFeatures,
		Pattern: regexp.MustCompile(`\bDate\(\)|\bDate\.now\b`),
		Title:   "Raw `Date()` in feature code",
		Detail:  "A type that reads the clock directly cannot be tested without waiting for real time to pass.",
		Fix:     "Inject `any DateProvider` through `init` and read `dates.now`. `SystemDateProvider` is the live binding.",
		Doc:     "docs/PATTERNS.md § Dates and timezones",
	},
	{
		ID:      "design/fixed-font-size",
		Default: gate.Blocker,
		Scope:   scopeNonTest,
		Pattern: regexp.MustCompile(`\.system\(size:|Font\.custom\([^)]*size:|systemFont\(ofSize:|\.font\(\.init\(`),
		Title:   "Fixed point size defeats Dynamic Type",
		Detail:  "A hardcoded size does not scale with the user's text setting, so the screen clips at accessibility sizes. Dynamic Type is listed as a build rule, not an aspiration.",
		Fix:     "Use a semantic style — `.body`, `.headline`, `.caption`. For a custom face, `.custom(_:relativeTo:)` so it still scales.",
		Doc:     "docs/DESIGN.md § Non-negotiable",
	},
	{
		ID:      "design/hardcoded-color",
		Default: gate.Blocker,
		Scope:   scopeNonTest,
		Pattern: regexp.MustCompile(`Color\(\s*red:|UIColor\(\s*red:|Color\(hex:|#colorLiteral|Color\(\s*"#`),
		Title:   "Hardcoded colour in view code",
		Detail:  "A literal colour has one appearance. It will be wrong in dark mode, wrong at increased contrast, and it will not pick up Liquid Glass on iOS 26.",
		Fix:     "Use a semantic colour (`.primary`, `.secondary`, `Color.accentColor`) or an asset-catalog colour set with both appearances defined.",
		Doc:     "docs/DESIGN.md § Non-negotiable",
	},
	{
		ID:      "design/print-logging",
		Default: gate.Warning,
		Scope:   scopeNonTest,
		Pattern: regexp.MustCompile(`^\s*print\(|\bNSLog\(`),
		Title:   "`print` instead of the logger",
		Detail:  "`print` is invisible in a release build and carries no subsystem, category, or privacy annotation — so it is exactly the thing that is missing when you need it.",
		Fix:     "Use the `Logger` in `Core`, and mark anything user-derived `privacy: .private`.",
		Doc:     "docs/ARCHITECTURE.md § Error handling",
	},
	{
		ID:      "swift/force-try-cast",
		Default: gate.Warning,
		Scope:   scopeNonTest,
		Pattern: regexp.MustCompile(`\btry!\s|\bas!\s|\.unsafelyUnwrapped\b`),
		Title:   "Force try or force cast",
		Detail:  "This crashes the app on the exact input you did not anticipate — usually a server response that changed shape.",
		Fix:     "`try?` with a real fallback, or propagate with `throws` and map to `AppError` at the boundary.",
		Doc:     "docs/ARCHITECTURE.md § Error handling",
	},
	{
		ID:      "arch/dto-escapes-networking",
		Default: gate.Warning,
		Scope:   scopeFeatures,
		Pattern: regexp.MustCompile(`\b\w+DTO\b`),
		Title:   "DTO used in feature code",
		Detail:  "A DTO is the wire format. Letting it reach a view or ViewModel means a backend field rename becomes a UI change, which is the coupling the repository layer exists to prevent.",
		Fix:     "Map to a domain model in the repository and return that. DTOs stay inside `Networking`.",
		Doc:     "docs/PATTERNS.md § Repositories",
	},
	{
		ID:      "flutter/widget-suffix",
		Default: gate.Warning,
		Scope:   scopeAll,
		Pattern: regexp.MustCompile(`\bstruct\s+\w+Widget\b|\bclass\s+\w+(?:Cubit|Bloc|Notifier|StateNotifier)\b`),
		Title:   "Flutter type vocabulary",
		Detail:  "`Widget`, `Cubit`, `Bloc` and `StateNotifier` are Flutter's words for things SwiftUI already names. A type carrying one of them was almost certainly translated rather than designed.",
		Fix:     "A SwiftUI `View` is a view — `MoodBadge`, not `MoodBadgeWidget`. State coordination lives in an `@Observable` model named for the screen.",
		Doc:     "CLAUDE.md § The one rule that matters most",
	},
	{
		ID:      "swift/todo-in-gate",
		Default: gate.Nit,
		Scope:   scopeNonTest,
		Pattern: regexp.MustCompile(`//\s*(?:TODO|FIXME|HACK)\b`),
		Title:   "TODO left in the diff",
		Detail:  "Worth a decision before merge: either it is deferred deliberately, in which case docs/STATE.md should say so, or it is unfinished work.",
		Fix:     "Record it under \"Deliberately not doing\" in docs/STATE.md, or finish it.",
		Doc:     "docs/STATE.md",
	},
}
