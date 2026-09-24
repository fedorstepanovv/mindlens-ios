import Foundation

/// What a new account answered before signing in: one goal, how they feel, one hurdle.
///
/// Held in memory only. A relaunch before sign-in starts the survey over, which is the
/// behaviour `docs/features/auth.md` settles.
public struct SurveyAnswers: Sendable, Equatable {
    /// The slider's range. The server validates the same bounds, so a value outside them is a 400.
    public static let feelingRange = 1...10

    public let goal: SurveyGoal
    public let feeling: Int
    public let hurdle: SurveyHurdle

    /// `feeling` is clamped into `feelingRange` rather than trusted: the slider is the only
    /// producer and cannot leave it, and a clamp costs less than a 400 after sign-in.
    public init(goal: SurveyGoal, feeling: Int, hurdle: SurveyHurdle) {
        self.goal = goal
        self.feeling = min(max(feeling, Self.feelingRange.lowerBound), Self.feelingRange.upperBound)
        self.hurdle = hurdle
    }
}

/// The goals a new account chooses from. The raw value is the id the server stores as the goal's
/// title — it keeps a label map keyed on exactly these six.
public enum SurveyGoal: String, CaseIterable, Sendable {
    case recoverFromBurnout = "recover_burnout"
    case clearBrainFog = "clear_brainfog"
    case stopOverthinking = "stop_overthinking"
    case buildRoutines = "build_routines"
    case fixSleep = "fix_sleep"
    case reduceScreenTime = "reduce_screentime"
}

/// What usually gets in the way. The server stores free text, and the source app has always sent
/// the English label, so the raw value is that label — the wire value, not the copy a screen shows.
public enum SurveyHurdle: String, CaseIterable, Sendable {
    case constantExhaustion = "Constant exhaustion"
    case gettingOverwhelmed = "Getting overwhelmed"
    case losingMomentum = "Losing momentum"
    case overthinking = "Overthinking"
    case allOrNothing = "All-or-nothing mindset"
}
