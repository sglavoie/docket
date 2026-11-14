import AVFoundation
import CoreHaptics
import SwiftUI
import DocketShared

/// The moments that answer a person's own action with motion, sound, and
/// touch: a completed task flying into Today's ring, the ring answering, the
/// plan's confetti bursting from it, and a capture landing on its tab. It
/// holds positions and counters only; nothing here writes to the workspace.
@MainActor @Observable
final class DelightCenter {
    /// A mint dot travelling from a completed row to Today's ring.
    struct Flight: Identifiable, Equatable {
        let id: Int
        /// Where it starts and lands, in window coordinates.
        let from: CGPoint
        let to: CGPoint
        let start: Date
    }

    /// A captured draft shrinking onto the tab it landed in.
    struct CaptureFlight: Identifiable, Equatable {
        let id: Int
        let title: String
        let tab: ShellTab
        let start: Date
    }

    private(set) var flights: [Flight] = []
    private(set) var captureFlight: CaptureFlight?
    /// Raised when a flight lands; the ring swells once for each.
    private(set) var ringPulse = 0

    /// Today's ring, in window coordinates, while it is on screen.
    @ObservationIgnored private var ringFrame: CGRect?
    @ObservationIgnored private var sequence = 0

    static let flightDuration: TimeInterval = 0.55
    static let captureFlightDuration: TimeInterval = 0.7

    func ringMoved(to frame: CGRect?) {
        ringFrame = frame
    }

    /// Where the ring's centre is, if a person can see it now.
    var ringCenter: CGPoint? {
        guard let ringFrame, ringFrame.maxY > 0, ringFrame.width > 0 else { return nil }
        return CGPoint(x: ringFrame.midX, y: ringFrame.midY)
    }

    /// Sends a completion from a row to the ring, which answers on landing.
    /// With the ring out of sight, or under Reduce Motion, nothing flies.
    func launchCompletion(from origin: CGRect, reduceMotion: Bool) {
        guard !reduceMotion, let target = ringCenter else { return }
        sequence += 1
        let flight = Flight(id: sequence, from: CGPoint(x: origin.midX, y: origin.midY), to: target, start: .now)
        flights.append(flight)
        Task { [weak self] in
            try? await Task.sleep(for: .seconds(Self.flightDuration))
            guard let self else { return }
            self.flights.removeAll { $0.id == flight.id }
            self.ringPulse += 1
        }
    }

    /// Shows a saved capture landing on its tab. `delay` lets the capture
    /// sheet slide away first, so the card is uncovered as it starts to move.
    func launchCapture(title: String, tab: ShellTab, reduceMotion: Bool, delay: TimeInterval = 0.3) {
        guard !reduceMotion else { return }
        sequence += 1
        let flight = CaptureFlight(id: sequence, title: title, tab: tab, start: Date().addingTimeInterval(delay))
        captureFlight = flight
        Task { [weak self] in
            try? await Task.sleep(for: .seconds(delay + Self.captureFlightDuration + 0.35))
            if self?.captureFlight == flight { self?.captureFlight = nil }
        }
    }
}

// MARK: - Sound

/// Docket's three short sounds, played over whatever else is playing and
/// silenced by the ring/silent switch. A person can turn them off in
/// settings; they are off under UI tests so a suite stays quiet.
@MainActor
final class SoundBoard {
    enum Cue: String, CaseIterable {
        case complete
        case planDone = "plan-done"
        case focusEnd = "focus-end"
    }

    static let shared = SoundBoard()
    static let enabledKey = "delight.sounds"

    private var players: [Cue: AVAudioPlayer] = [:]
    private var prepared = false

    static var enabled: Bool {
        UserDefaults.standard.object(forKey: enabledKey) as? Bool ?? true
    }

    func play(_ cue: Cue) {
        guard Self.enabled, !LaunchConfiguration.current.isUITest else { return }
        prepare()
        guard let player = players[cue] else { return }
        player.currentTime = 0
        player.play()
    }

    private func prepare() {
        guard !prepared else { return }
        prepared = true
        // Ambient: mixes with music and obeys the silent switch, the way a
        // keyboard click does.
        try? AVAudioSession.sharedInstance().setCategory(.ambient, options: [.mixWithOthers])
        for cue in Cue.allCases {
            guard let url = Bundle.main.url(forResource: cue.rawValue, withExtension: "wav"),
                  let player = try? AVAudioPlayer(contentsOf: url) else { continue }
            player.volume = cue == .complete ? 0.5 : 0.65
            player.prepareToPlay()
            players[cue] = player
        }
    }
}

// MARK: - Haptics

/// Patterns the system's one-shot feedback cannot say. Without a Taptic
/// Engine, or when the engine will not start, each falls back to a system
/// notification so the moment still lands.
@MainActor
final class HapticConductor {
    static let shared = HapticConductor()
    private var engine: CHHapticEngine?

    /// A rising run of taps that resolves in a soft swell: the plan closing.
    func planComplete() {
        let taps: [(TimeInterval, Float, Float)] = [(0, 0.35, 0.3), (0.07, 0.5, 0.4), (0.14, 0.65, 0.5), (0.22, 0.9, 0.7)]
        var events = taps.map { time, intensity, sharpness in
            CHHapticEvent(eventType: .hapticTransient, parameters: [
                CHHapticEventParameter(parameterID: .hapticIntensity, value: intensity),
                CHHapticEventParameter(parameterID: .hapticSharpness, value: sharpness),
            ], relativeTime: time)
        }
        events.append(CHHapticEvent(eventType: .hapticContinuous, parameters: [
            CHHapticEventParameter(parameterID: .hapticIntensity, value: 0.45),
            CHHapticEventParameter(parameterID: .hapticSharpness, value: 0.15),
        ], relativeTime: 0.3, duration: 0.35))
        play(events, fallback: .success)
    }

    /// Two gentle taps: a focus session's time is up.
    func focusEnd() {
        let events = [0, 0.42].map { time in
            CHHapticEvent(eventType: .hapticTransient, parameters: [
                CHHapticEventParameter(parameterID: .hapticIntensity, value: 0.6),
                CHHapticEventParameter(parameterID: .hapticSharpness, value: 0.2),
            ], relativeTime: time)
        }
        play(events, fallback: .success)
    }

    private func play(_ events: [CHHapticEvent], fallback: UINotificationFeedbackGenerator.FeedbackType) {
        guard CHHapticEngine.capabilitiesForHardware().supportsHaptics else {
            UINotificationFeedbackGenerator().notificationOccurred(fallback)
            return
        }
        do {
            if engine == nil {
                let engine = try CHHapticEngine()
                engine.isAutoShutdownEnabled = true
                engine.resetHandler = { [weak self] in
                    Task { @MainActor in self?.engine = nil }
                }
                self.engine = engine
            }
            try engine?.start()
            let player = try engine?.makePlayer(with: CHHapticPattern(events: events, parameters: []))
            try player?.start(atTime: CHHapticTimeImmediate)
        } catch {
            UINotificationFeedbackGenerator().notificationOccurred(fallback)
        }
    }
}

// MARK: - Overlays

/// Draws every completion in flight: a mint dot that arcs from the row up to
/// the ring, shrinking and trailing a short tail as it goes. Never takes a
/// touch and says nothing to VoiceOver; the row and the ring already speak.
struct CompletionFlightOverlay: View {
    let delight: DelightCenter

    var body: some View {
        GeometryReader { proxy in
            let offset = proxy.frame(in: .global).origin
            if !delight.flights.isEmpty {
                TimelineView(.animation) { timeline in
                    Canvas { context, _ in
                        for flight in delight.flights {
                            draw(flight, in: &context, offset: offset, now: timeline.date)
                        }
                    }
                }
            }
        }
        .allowsHitTesting(false)
        .accessibilityHidden(true)
        .ignoresSafeArea()
    }

    private func draw(_ flight: DelightCenter.Flight, in context: inout GraphicsContext, offset: CGPoint, now: Date) {
        let raw = now.timeIntervalSince(flight.start) / DelightCenter.flightDuration
        guard raw >= 0, raw <= 1 else { return }
        for step in 0..<5 {
            // The head and four fading echoes a few frames behind it.
            let t = max(0, raw - Double(step) * 0.035)
            let eased = 1 - pow(1 - t, 3)
            let point = Self.point(on: flight, at: eased, offset: offset)
            let radius = (7 - 4 * eased) * (1 - Double(step) * 0.17)
            var dot = context
            dot.opacity = (1 - Double(step) * 0.22) * (raw > 0.9 ? (1 - raw) * 10 : 1)
            dot.fill(Path(ellipseIn: CGRect(x: point.x - radius, y: point.y - radius, width: radius * 2, height: radius * 2)),
                     with: .color(DocketPalette.brand))
        }
    }

    /// A quadratic arc whose control point lifts above both ends, so the dot
    /// rises and drops into the ring instead of sliding in a straight line.
    static func point(on flight: DelightCenter.Flight, at t: Double, offset: CGPoint) -> CGPoint {
        let from = CGPoint(x: flight.from.x - offset.x, y: flight.from.y - offset.y)
        let to = CGPoint(x: flight.to.x - offset.x, y: flight.to.y - offset.y)
        let control = CGPoint(x: (from.x + to.x) / 2 + (from.x - to.x) * 0.25, y: min(from.y, to.y) - 60)
        let u = 1 - t
        return CGPoint(x: u * u * from.x + 2 * u * t * control.x + t * t * to.x,
                       y: u * u * from.y + 2 * u * t * control.y + t * t * to.y)
    }
}

/// A saved capture as a small card that shrinks from the bottom of the screen
/// onto the tab it landed in, where a ring ripples once.
struct CaptureFlightOverlay: View {
    let delight: DelightCenter

    var body: some View {
        GeometryReader { proxy in
            if let flight = delight.captureFlight {
                TimelineView(.animation) { timeline in
                    let elapsed = timeline.date.timeIntervalSince(flight.start)
                    let t = min(1, max(0, elapsed / DelightCenter.captureFlightDuration))
                    let eased = t < 0.5 ? 4 * t * t * t : 1 - pow(-2 * t + 2, 3) / 2
                    let size = proxy.size
                    let start = CGPoint(x: size.width / 2, y: size.height * 0.62)
                    let end = Self.tabCenter(flight.tab, in: proxy)
                    let x = start.x + (end.x - start.x) * eased
                    let y = start.y + (end.y - start.y) * eased - sin(eased * .pi) * 40
                    ZStack {
                        if t < 1 {
                            Label(flight.title, systemImage: "tray.and.arrow.down.fill")
                                .font(.subheadline.weight(.semibold))
                                .lineLimit(1)
                                .padding(.horizontal, 14)
                                .padding(.vertical, 10)
                                .frame(maxWidth: 260)
                                .foregroundStyle(DocketPalette.onBrand)
                                .background(DocketPalette.brand, in: Capsule())
                                .shadow(color: .black.opacity(0.18), radius: 10, y: 4)
                                .scaleEffect(1 - 0.85 * eased)
                                .opacity(t > 0.85 ? (1 - t) / 0.15 : 1)
                                .position(x: x, y: y)
                        }
                        let ripple = max(0, (elapsed - DelightCenter.captureFlightDuration) / 0.35)
                        if ripple > 0, ripple < 1 {
                            Circle()
                                .stroke(DocketPalette.brand, lineWidth: 3 * (1 - ripple))
                                .frame(width: 30 + 40 * ripple, height: 30 + 40 * ripple)
                                .opacity(1 - ripple)
                                .position(end)
                        }
                    }
                }
            }
        }
        .allowsHitTesting(false)
        .accessibilityHidden(true)
    }

    /// Where a tab's icon sits in the floating tab bar: four equal slots
    /// across the bar, inset from the screen's edges, just above the home
    /// indicator.
    static func tabCenter(_ tab: ShellTab, in proxy: GeometryProxy) -> CGPoint {
        let index: CGFloat = switch tab {
        case .today: 0
        case .inbox: 1
        case .notes: 2
        case .browse: 3
        }
        let inset: CGFloat = 25
        let slot = (proxy.size.width - inset * 2) / 4
        return CGPoint(x: inset + slot * (index + 0.5), y: proxy.size.height - 34)
    }
}
