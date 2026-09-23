# ADR 0025 — The pre-sign-in survey belongs to auth

Date: 2026-09-22 · Status: Accepted · Reverses two settled lines in `docs/features/auth.md`

## Context

When auth was built, two lines of its settled decisions put the survey in Stage 4 (Onboarding +
Paywall):
- The sign-in copy was the product's opening line. Flutter's "One last step" was rejected because
  it "presumes the Stage 4 survey, which ships later".
- "Leading-aligned headline, no carousel, no glowing background: the survey and its choreography
  are Stage 4's."

That split treated the survey as part of onboarding. The source app puts it somewhere else. Its
login screen is a five-page flow: intro, goal, feeling, hurdle, then sign-in. The same screen
serves sign-up and sign-in. A new account's answers are posted right after the token exchange,
and a returning account's are discarded. The survey is how an account is *created*, so it cannot
ship after sign-in without the first sign-in changing shape twice. Onboarding proper is the
status polling, the insight reveal and the paywall that come after.

Fedir made the call during intake on 2026-09-22, when he approved auth's `## Behaviour` (ADR 0020).

## Decision

**The pre-sign-in survey and the posting of a new account's answers are part of auth.** They are
`docs/features/auth.md` steps 7 and on, and the sign-in screen is the survey's last page. "One
last step" / "Sign in to turn your answers into your first lens." is now the right copy, because
the survey it presumes is now in front of it.

Stage 4 keeps what comes after sign-in: onboarding-status polling, the insight reveal, the paywall.

Unchanged by this: no carousel on any page, no swiping between pages, a leading-aligned headline,
and no custom-drawn background.

## Consequences

- Auth grows by two steps, and Stage 1 is not done until a new account's answers reach the server.
- Stage 4 starts from a signed-in account that has already completed onboarding.
- The sign-in screen's copy depends on the survey. Until step 7 lands, the app shows the old
  opening line, which is still true without the survey in front of it.

## What would change this

- A product decision to let people sign in before answering, for example a "Log in" affordance for
  returning users on the intro. Returning accounts would then skip the survey, which would split
  back out into onboarding.
- The backend taking survey answers at account creation, in the `/auth/*` body. The posting would
  then disappear, though the pages would stay here.
