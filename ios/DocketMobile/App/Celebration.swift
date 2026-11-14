import SwiftUI
import DocketShared

/// The daily rituals the shell can show over everything else.
enum Ritual: String, Identifiable, Hashable {
    /// Inbox triage, one card at a time.
    case startDay
    /// The task in progress, full screen, with an optional focus session.
    case now
    /// Every task asking for review or a check-in, one after another.
    case review
    /// The evening's close: what got done, what is left, and where it goes.
    case recap

    var id: String { rawValue }
}

/// Confetti for a list the person just cleared. It plays once per change of
/// `trigger`, never blocks a touch, and under Reduce Motion becomes a single
/// checkmark that fades in place.
struct CelebrationOverlay: View {
    /// Read here rather than by the shell, so a celebration re-renders only
    /// this overlay.
    let workspace: WorkspaceModel
    private var trigger: Int { workspace.celebration }
    @State private var start: Date?
    @State private var particles: [ConfettiParticle] = []
    @State private var showingCheck = false
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    private static let duration: TimeInterval = 2.2

    var body: some View {
        GeometryReader { proxy in
            ZStack {
                if let start {
                    TimelineView(.animation) { timeline in
                        Canvas { context, size in
                            let elapsed = timeline.date.timeIntervalSince(start)
                            for particle in particles {
                                particle.draw(in: &context, size: size, elapsed: elapsed, duration: Self.duration)
                            }
                        }
                    }
                }
                if showingCheck {
                    Image(systemName: "checkmark.circle.fill")
                        .font(.system(size: 96, weight: .bold))
                        .foregroundStyle(DocketPalette.brand)
                        .transition(.opacity)
                        .frame(maxWidth: .infinity, maxHeight: .infinity)
                }
            }
            .onChange(of: trigger) { _, _ in play(in: proxy.frame(in: .global)) }
        }
        .allowsHitTesting(false)
        .accessibilityHidden(true)
        .ignoresSafeArea()
    }

    /// The ring closes and its checkmark lands first; then the burst, the
    /// chord, and the rising taps arrive together. With Today's ring on
    /// screen the confetti bursts from it, otherwise from near the top.
    private func play(in frame: CGRect) {
        Task {
            try? await Task.sleep(for: .milliseconds(reduceMotion ? 0 : 280))
            SoundBoard.shared.play(.planDone)
            HapticConductor.shared.planComplete()
            if reduceMotion {
                withAnimation(.easeIn(duration: 0.2)) { showingCheck = true }
                try? await Task.sleep(for: .seconds(1.2))
                withAnimation(.easeOut(duration: 0.4)) { showingCheck = false }
                return
            }
            let burst = workspace.delight.ringCenter.flatMap { center -> CGPoint? in
                guard frame.width > 0, frame.height > 0 else { return nil }
                return CGPoint(x: (center.x - frame.minX) / frame.width, y: (center.y - frame.minY) / frame.height)
            }
            particles = (0..<(burst == nil ? 90 : 110)).map { _ in
                burst.map(ConfettiParticle.burst(from:)) ?? ConfettiParticle.random()
            }
            let begun = Date()
            start = begun
            try? await Task.sleep(for: .seconds(Self.duration))
            if start == begun { start = nil }
        }
    }
}

/// One piece of confetti: launched up and outward from near the top centre,
/// then falling, tumbling, and fading.
struct ConfettiParticle {
    var origin: CGPoint
    var velocity: CGVector
    var spin: Double
    var phase: Double
    var extent: CGSize
    var color: Color
    var round: Bool

    static func random() -> ConfettiParticle {
        let angle = Double.random(in: (-.pi * 0.9)...(-.pi * 0.1))
        let speed = Double.random(in: 380...820)
        return ConfettiParticle(
            origin: CGPoint(x: Double.random(in: 0.35...0.65), y: Double.random(in: 0.22...0.32)),
            velocity: CGVector(dx: cos(angle) * speed, dy: sin(angle) * speed),
            spin: Double.random(in: -9...9),
            phase: Double.random(in: 0...(2 * .pi)),
            extent: CGSize(width: Double.random(in: 6...11), height: Double.random(in: 9...16)),
            color: DocketPalette.celebration.randomElement() ?? DocketPalette.brand,
            round: Bool.random() && Bool.random()
        )
    }

    /// A piece thrown outward in every direction from a point, leaning
    /// upward, as if the ring itself burst.
    static func burst(from origin: CGPoint) -> ConfettiParticle {
        let angle = Double.random(in: 0...(2 * .pi))
        let speed = Double.random(in: 260...700)
        var particle = random()
        particle.origin = origin
        particle.velocity = CGVector(dx: cos(angle) * speed, dy: sin(angle) * speed - 260)
        return particle
    }

    func draw(in context: inout GraphicsContext, size: CGSize, elapsed t: Double, duration: Double) {
        guard t < duration else { return }
        let gravity = 1100.0
        let drag = exp(-1.4 * t)
        let x = origin.x * size.width + velocity.dx * (1 - drag) / 1.4
        let y = origin.y * size.height + velocity.dy * (1 - drag) / 1.4 + 0.5 * gravity * t * t * 0.6
        let fade = t > duration * 0.6 ? max(0, 1 - (t - duration * 0.6) / (duration * 0.4)) : 1
        var piece = context
        piece.opacity = fade
        piece.translateBy(x: x, y: y)
        piece.rotate(by: .radians(phase + spin * t))
        // A flutter: the piece's width breathes as it turns over.
        let width = extent.width * abs(cos(phase + t * 6))
        let rect = CGRect(x: -width / 2, y: -extent.height / 2, width: max(1.5, width), height: extent.height)
        let path = round ? Path(ellipseIn: rect.insetBy(dx: 0, dy: extent.height * 0.2)) : Path(roundedRect: rect, cornerRadius: 1.5)
        piece.fill(path, with: .color(color))
    }
}
