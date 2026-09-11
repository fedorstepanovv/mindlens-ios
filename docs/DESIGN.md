# Design

The app is native. Not native-*looking* — native.

The Flutter app leans on `flutter_platform_widgets`, `wolt_modal_sheet`,
`table_calendar`, `skeletonizer` and `flutter_spinkit` to *approximate* iOS, because it
has to. That entire category is deleted here. Use the real component.

## Use the system component

| Need | Use |
|---|---|
| Modal / bottom sheet | `.sheet` + `.presentationDetents` |
| Lists, settings | `List` / `Form`, `.swipeActions`, `.refreshable`, `.searchable` |
| Empty state | `ContentUnavailableView` |
| Loading placeholder | `.redacted(reason: .placeholder)` |
| Icons | SF Symbols, with `.symbolEffect` for state changes |
| Charts | Swift Charts |
| Haptics | `.sensoryFeedback` |
| Surfaces | System materials, semantic colors |

Reach for a custom component only when nothing system-provided fits — and note why.

## Liquid Glass

We build against the iOS 26 SDK with an iOS 18 floor. Standard components adopt Liquid
Glass automatically on iOS 26 while remaining correct on 18 — the strongest argument for
using system components: the design language updates for free. Custom-drawn UI does not
get that.

Three things this does **not** mean:

- **It is not free of work.** Toolbar layout, `.searchable` placement, tab bar behaviour
  and sheet chrome all changed. Supporting both means checking every screen on both.
- **There is an opt-out, and it is temporary.** `UIDesignRequiresCompatibility` in
  Info.plist reverts to the pre-26 look. Apple has said it will not last. We do not use it.
- **Corner radii are concentric now.** Nesting a fixed radius inside a container produces
  visibly wrong corners. Use `ConcentricRectangle` / `.rect(corners: .concentric)` so a
  radius derives from its container. The fixed values in `Radius` are for standalone
  controls only, and are the first thing to revisit when adopting iOS 26 visuals.

Direct API worth knowing: `.glassEffect(_:in:)`, `GlassEffectContainer`,
`.buttonStyle(.glass)`, `.backgroundExtensionEffect`.

## iPad

The app builds for iPhone and iPad (`TARGETED_DEVICE_FAMILY = "1,2"`). Until a screen is
actually designed for the regular size class, that is a liability rather than a feature —
a stretched iPhone layout reads worse than no iPad support.

**Decision:** iPhone-first. Screens must not break in a regular size class, but
`NavigationSplitView` and multitasking layouts are out of scope until iPhone is done.
Revisit before any App Store submission.

## Non-negotiable

These are build rules, not aspirations:

- **Dynamic Type everywhere.** Semantic text styles only. No fixed point sizes. Every
  screen is checked at accessibility sizes — layouts wrap and scroll, they don't clip.
- **VoiceOver labels on every interactive element.** Icon-only buttons need explicit
  labels; decorative images are hidden from accessibility.
- **Light and dark both correct**, from semantic colors and asset-catalog appearances.
  No hardcoded hex in view code.
- **Tap targets at least 44×44pt.**
- **Respect Reduce Motion.** Animations are enhancements, never the only signal.

## Tokens

Spacing, radii and colour live in `DesignSystem` as a small token set. Tokens exist to keep
spacing consistent — not to build a parallel design language that fights the platform.

**Typography is not a token set here.** Semantic text styles *are* the type system on iOS, and a
parallel scale is the habit this rewrite exists to drop; reach for `.font(.title3)` and a weight
or design, never a point size. `Spacing.swift` says the same thing at the top of the file.

## Content

Copy comes from the Flutter app, which is production-tested. Two locked brand rules
carried over from the existing product:

- Never call the app a "journal".
- Never foreground "AI" in user-facing copy.

Strings live in a String Catalog (`.xcstrings`) from day one, even while English-only —
retrofitting localization is far more expensive than starting with it.
