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
Glass automatically on iOS 26 while remaining correct on 18. This is the strongest
argument for using system components: the design language updates for free.

Custom-drawn UI does not get that, and every hand-rolled control is a thing that will
look dated the moment the platform moves.

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

Spacing, radii, and typography live in `DesignSystem` as a small token set on top of the
system scale. Tokens exist to keep spacing consistent — not to build a parallel design
language that fights the platform.

## Content

Copy comes from the Flutter app, which is production-tested. Two locked brand rules
carried over from the existing product:

- Never call the app a "journal".
- Never foreground "AI" in user-facing copy.

Strings live in a String Catalog (`.xcstrings`) from day one, even while English-only —
retrofitting localization is far more expensive than starting with it.
