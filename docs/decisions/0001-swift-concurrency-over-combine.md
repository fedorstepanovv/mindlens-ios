# ADR 0001 — Swift Concurrency and Observation as the primary stack

Date: 2026-09-10 · Status: Accepted

## Context
The target role's stack lists Combine explicitly. The existing Flutter app uses
Bloc/Cubit with sealed-union state, which translates most directly into
`ObservableObject` + `@Published` + publisher chains.

But SwiftUI's own direction since iOS 17 is the Observation framework, and Apple's
current guidance moves data flow to `async/await`. A codebase written entirely in
Combine in 2026 reads as dated to a senior iOS reviewer.

## Decision
`async/await` and `@Observable` are the default. Combine is used where it is genuinely
the best tool: debounced user input, `NotificationCenter` streams, and multi-source
merges. It never carries data between layers.

## Consequences
The code reads as current, and `@Observable` removes a class of SwiftUI invalidation
bugs. The cost is that the codebase does not literally match the phrase "Combine" in a
job description — mitigated by Combine appearing where it is *correct*, which is a
stronger signal than blanket adoption. Being able to explain the boundary is the point.

## What would change this
A team standard requiring Combine throughout, or a dependency that only exposes
publishers.
