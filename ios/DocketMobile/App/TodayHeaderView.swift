import SwiftUI
import DocketEngine
import DocketShared

/// The top of Today and the Inbox: where the day stands and the one move to
/// make next. Today draws a progress ring over the plan and a greeting that
/// names that move; the Inbox draws how much is left to sort. Both offer the
/// rituals that fit — Start the Day, Now, and Review — as chips that open them.
struct TodayHeaderView: View {
    enum Style { case today, inbox }

    let workspace: WorkspaceModel
    let style: Style
    private var pulse: DailyPulse? { workspace.pulse }
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    var body: some View {
        TimelineView(.periodic(from: .now, by: 60)) { context in
            VStack(alignment: .leading, spacing: 14) {
                switch style {
                case .today: todayHeadline(now: context.date)
                case .inbox: inboxHeadline
                }
                chips(now: context.date)
            }
            .padding(.vertical, 6)
        }
        .animation(reduceMotion ? nil : .snappy, value: pulse)
        .task {
            if workspace.pulse == nil { workspace.refreshPulse() }
        }
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier(style == .today ? "today.header" : "inbox.header")
    }

    /// Grows with text up to the largest standard size, where the ring's
    /// count still fits; past it the header turns into one sentence.
    @ScaledMetric(relativeTo: .title3) private var ringSize: CGFloat = 64
    @Environment(\.dynamicTypeSize) private var dynamicTypeSize

    @ViewBuilder
    private func todayHeadline(now: Date) -> some View {
        if dynamicTypeSize.isAccessibilitySize {
            // At the accessibility sizes the header is one sentence, so the
            // list's first rows stay on the first screen; the progress is
            // said in words rather than drawn as a ring.
            Text(pulse.map { DailyPulse.sentence($0.move(now: now)) } ?? " ")
                .font(.headline)
                .fixedSize(horizontal: false, vertical: true)
                .accessibilityIdentifier("today.greeting")
        } else {
            regularHeadline(now: now)
        }
    }

    @ViewBuilder
    private func regularHeadline(now: Date) -> some View {
        HStack(alignment: .center, spacing: 16) {
            ProgressRing(pulse: pulse, delight: workspace.delight)
                .frame(width: ringSize, height: ringSize)
            VStack(alignment: .leading, spacing: 4) {
                Text(DailyPulse.salutation(now: now))
                    .font(.title3.weight(.bold))
                    .fontDesign(.rounded)
                Text(pulse.map { DailyPulse.sentence($0.move(now: now)) } ?? " ")
                    .font(.subheadline)
                    .foregroundStyle(.secondary)
                    .fixedSize(horizontal: false, vertical: true)
                    .contentTransition(.opacity)
                    .accessibilityIdentifier("today.greeting")
            }
            Spacer(minLength: 0)
        }
    }

    @ViewBuilder
    private var inboxHeadline: some View {
        let count = pulse?.inbox ?? 0
        HStack(alignment: .firstTextBaseline, spacing: 6) {
            Text("\(count)")
                .font(.system(.largeTitle, design: .rounded).weight(.bold))
                .foregroundStyle(DocketPalette.state("inbox"))
                .contentTransition(.numericText(value: Double(count)))
            Text(count == 1 ? "capture to sort" : "captures to sort")
                .font(.headline)
                .foregroundStyle(.secondary)
        }
        .accessibilityElement(children: .combine)
        .accessibilityIdentifier("inbox.count")
    }

    @ViewBuilder
    private func chips(now: Date) -> some View {
        let pulse = pulse ?? DailyPulse()
        let showRecap = style == .today && Self.isEvening(now) && ((pulse.doneToday ?? 0) > 0 || pulse.todayOpen > 0)
        let showTriage = pulse.inbox > 0
        let showNow = !pulse.doing.isEmpty && style == .today
        let showReview = !pulse.attention.isEmpty && style == .today
        if showTriage || showNow || showReview || showRecap {
            // Chips wrap onto a second line rather than scrolling off the
            // edge, so a long Now title never ends in a clipped capsule. Now
            // leads: the work already under way is the likeliest next tap.
            // At the accessibility sizes wrapping would stack them past the
            // first screen, so they scroll sideways instead.
            if dynamicTypeSize.isAccessibilitySize {
                ScrollView(.horizontal, showsIndicators: false) {
                    HStack(spacing: 8) {
                        chipViews(pulse, now: showNow, triage: showTriage, review: showReview, recap: showRecap)
                    }
                    .padding(.vertical, 2)
                }
                .scrollClipDisabled()
            } else {
                WrapLayout(spacing: 8, lineSpacing: 8) {
                    chipViews(pulse, now: showNow, triage: showTriage, review: showReview, recap: showRecap)
                }
                .padding(.vertical, 2)
            }
        }
    }
}

extension TodayHeaderView {
    /// The ritual chips worth showing now, in the order they are offered.
    @ViewBuilder
    func chipViews(_ pulse: DailyPulse, now showNow: Bool, triage showTriage: Bool, review showReview: Bool, recap showRecap: Bool) -> some View {
        if showNow, let doing = pulse.doing.first {
            RitualChip(title: "Now", detail: doing.title, symbol: "scope",
                       tint: DocketPalette.state("doing")) {
                workspace.ritual = .now
            }
            .zoomSource(Ritual.now)
            .accessibilityIdentifier("ritual.now")
        }
        if showTriage {
            RitualChip(title: "Start the Day", detail: "\(pulse.inbox)", symbol: "sunrise.fill",
                       tint: DocketPalette.today, fill: style == .inbox ? DocketPalette.Solid.today : nil) {
                workspace.ritual = .startDay
            }
            .zoomSource(Ritual.startDay)
            .accessibilityIdentifier("ritual.startDay")
        }
        if showReview {
            RitualChip(title: "Review", detail: "\(pulse.attention.count)", symbol: "checkmark.seal",
                       tint: DocketPalette.review) {
                workspace.ritual = .review
            }
            .zoomSource(Ritual.review)
            .accessibilityIdentifier("ritual.review")
        }
        if showRecap {
            RitualChip(title: "Wrap Up", detail: "\(pulse.doneToday ?? 0) done", symbol: "moon.stars.fill",
                       tint: DocketPalette.next) {
                workspace.ritual = .recap
            }
            .zoomSource(Ritual.recap)
            .accessibilityIdentifier("ritual.recap")
        }
    }

    /// From five in the evening until the small hours, Today offers to wrap
    /// the day up.
    static func isEvening(_ now: Date, calendar: Calendar = .current) -> Bool {
        let hour = calendar.component(.hour, from: now)
        return hour >= 17 || hour < 4
    }
}

/// Today's plan as a ring: one segment per planned task, so a fresh plan
/// reads as slots waiting to fill rather than an empty gauge; the done share
/// in the brand's mint; the count in the middle rolling as it changes; and a
/// checkmark once the plan is done. It swells each time a completion flies
/// into it, and tells the delight center where it sits so one can.
struct ProgressRing: View {
    let pulse: DailyPulse?
    var delight: DelightCenter?
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    /// Past this many tasks the gaps would crowd out the arcs, so the ring
    /// draws continuously.
    static let maxSegments = 8

    var body: some View {
        let done = pulse?.doneToday ?? 0
        let open = pulse?.todayOpen ?? 0
        let total = done + open
        let progress = pulse?.progress ?? 0
        let complete = pulse?.planComplete ?? false
        let segments = (2...Self.maxSegments).contains(total) && !complete ? total : 1
        let swell = delight?.ringPulse ?? 0
        ZStack {
            SegmentedArc(segments: segments, progress: 1)
                .stroke(DocketPalette.brand.opacity(done == 0 ? 0.24 : 0.16), style: StrokeStyle(lineWidth: 7, lineCap: .round))
            SegmentedArc(segments: segments, progress: complete ? 1 : progress)
                .stroke(
                    AngularGradient(colors: [DocketPalette.brand.opacity(0.7), DocketPalette.brand],
                                    center: .center, startAngle: .degrees(-90), endAngle: .degrees(270)),
                    style: StrokeStyle(lineWidth: 7, lineCap: .round)
                )
                .animation(reduceMotion ? nil : .spring(response: 0.6, dampingFraction: 0.8), value: progress)
            if complete {
                Image(systemName: "checkmark")
                    .font(.title2.weight(.bold))
                    .foregroundStyle(DocketPalette.brand)
                    .symbolEffect(.bounce, value: complete)
                    .transition(.scale.combined(with: .opacity))
            } else if done == 0 {
                // Nothing finished yet: lead with what is planned, not a zero.
                VStack(spacing: -2) {
                    Text("\(open)")
                        .font(.system(.title3, design: .rounded).weight(.bold))
                        .contentTransition(.numericText(value: Double(open)))
                    Text("planned")
                        .font(.caption2.weight(.medium))
                        .foregroundStyle(.secondary)
                }
                .monospacedDigit()
            } else {
                VStack(spacing: -2) {
                    Text("\(done)")
                        .font(.system(.title3, design: .rounded).weight(.bold))
                        .contentTransition(.numericText(value: Double(done)))
                    Text("of \(total)")
                        .font(.caption2.weight(.medium))
                        .foregroundStyle(.secondary)
                        .contentTransition(.numericText())
                }
                .monospacedDigit()
            }
        }
        .background {
            // The glow a landing completion leaves behind.
            Circle()
                .fill(DocketPalette.brand.opacity(0.25))
                .blur(radius: 10)
                .keyframeAnimator(initialValue: 0.0, trigger: swell) { glow, value in
                    glow.opacity(value).scaleEffect(1 + value * 0.2)
                } keyframes: { _ in
                    LinearKeyframe(1, duration: 0.08)
                    LinearKeyframe(0, duration: 0.5)
                }
        }
        .keyframeAnimator(initialValue: 1.0, trigger: swell) { ring, scale in
            ring.scaleEffect(scale)
        } keyframes: { _ in
            SpringKeyframe(1.12, duration: 0.12, spring: .snappy)
            SpringKeyframe(1, duration: 0.45, spring: .bouncy)
        }
        .onGeometryChange(for: CGRect.self) { $0.frame(in: .global) } action: { frame in
            delight?.ringMoved(to: frame)
        }
        .onDisappear { delight?.ringMoved(to: nil) }
        .accessibilityElement(children: .ignore)
        .accessibilityLabel("Today’s progress")
        .accessibilityValue(accessibilityValue)
        .accessibilityIdentifier("today.progress")
    }

    private var accessibilityValue: String {
        guard let pulse else { return "Loading" }
        if pulse.planComplete { return "Plan complete" }
        return "\(pulse.doneToday ?? 0) done, \(pulse.todayOpen) to go"
    }
}

/// A ring cut into equal arcs with a small gap between each, drawn clockwise
/// from twelve o'clock up to `progress` of the way round. One segment is a
/// plain circle. `progress` animates, so a filling segment sweeps in.
struct SegmentedArc: Shape {
    var segments: Int
    var progress: Double

    var animatableData: Double {
        get { progress }
        set { progress = newValue }
    }

    func path(in rect: CGRect) -> Path {
        let radius = min(rect.width, rect.height) / 2
        let center = CGPoint(x: rect.midX, y: rect.midY)
        let clamped = min(max(progress, 0), 1)
        var path = Path()
        guard clamped > 0 else { return path }
        guard segments > 1 else {
            path.addArc(center: center, radius: radius, startAngle: .degrees(-90),
                        endAngle: .degrees(-90 + 360 * clamped), clockwise: false)
            return path
        }
        // The round caps reach past each arc's end, so the gap allows for them.
        let gap = 360 / Double(segments) * 0.1 + 19
        let span = 360 / Double(segments)
        let filled = clamped * Double(segments)
        for index in 0..<segments {
            let share = min(1, max(0, filled - Double(index)))
            guard share > 0 else { break }
            let start = -90 + span * Double(index) + gap / 2
            let end = start + (span - gap) * share
            path.move(to: CGPoint(x: center.x + radius * cos(start * .pi / 180), y: center.y + radius * sin(start * .pi / 180)))
            path.addArc(center: center, radius: radius, startAngle: .degrees(start), endAngle: .degrees(max(start + 0.1, end)), clockwise: false)
        }
        return path
    }
}

/// One ritual's entry: a capsule naming it, with its count or subject.
struct RitualChip: View {
    let title: String
    let detail: String
    let symbol: String
    let tint: Color
    /// A solid fill under white text, for the one chip a screen leads with.
    var fill: Color?
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            HStack(spacing: 6) {
                Image(systemName: symbol)
                    .symbolRenderingMode(.hierarchical)
                Text(title).fontWeight(.semibold)
                // Full strength, so the count keeps its contrast on the night
                // canvas; the lighter weight is what sets it apart.
                Text(detail)
                    .fontWeight(.regular)
                    .lineLimit(1)
                    .truncationMode(.tail)
                    .frame(maxWidth: 140, alignment: .leading)
                    .fixedSize(horizontal: true, vertical: false)
            }
            .font(.subheadline)
            .padding(.horizontal, 12)
            .padding(.vertical, 8)
            .foregroundStyle(fill == nil ? tint : Color.white)
            .background(fill.map { AnyShapeStyle($0) } ?? AnyShapeStyle(tint.opacity(0.14)), in: Capsule())
            .contentShape(Capsule())
        }
        .buttonStyle(PressableButtonStyle())
        .accessibilityLabel("\(title), \(detail)")
    }
}

/// A button that gives a little under the finger.
struct PressableButtonStyle: ButtonStyle {
    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .scaleEffect(configuration.isPressed ? 0.95 : 1)
            .animation(.spring(response: 0.25, dampingFraction: 0.6), value: configuration.isPressed)
    }
}
