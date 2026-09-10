import SwiftUI

/// A small token set on top of the system scale.
///
/// Tokens exist to keep spacing consistent — not to build a parallel design language.
/// Type comes from semantic styles (`.body`, `.headline`) so Dynamic Type works; there
/// are deliberately no font-size tokens here.
public enum Spacing {
    public static let tight: CGFloat = 4
    public static let snug: CGFloat = 8
    public static let regular: CGFloat = 16
    public static let loose: CGFloat = 24
    public static let section: CGFloat = 32
}

public enum Radius {
    public static let control: CGFloat = 10
    public static let card: CGFloat = 16
}
