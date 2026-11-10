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
                chips
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
            ProgressRing(pulse: pulse)
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
    private var chips: some View {
        let pulse = pulse ?? DailyPulse()
        let showTriage = pulse.inbox > 0
        let showNow = !pulse.doing.isEmpty && style == .today
        let showReview = !pulse.attention.isEmpty && style == .today
        if showTriage || showNow || showReview {
            ScrollView(.horizontal, showsIndicators: false) {
                HStack(spacing: 8) {
                    if showTriage {
                        RitualChip(title: "Start the Day", detail: "\(pulse.inbox)", symbol: "sunrise.fill",
                                   tint: DocketPalette.today, fill: style == .inbox ? DocketPalette.Solid.today : nil) {
                            workspace.ritual = .startDay
                        }
                        .accessibilityIdentifier("ritual.startDay")
                    }
                    if showNow, let doing = pulse.doing.first {
                        RitualChip(title: "Now", detail: doing.title, symbol: "scope",
                                   tint: DocketPalette.state("doing")) {
                            workspace.ritual = .now
                        }
                        .accessibilityIdentifier("ritual.now")
                    }
                    if showReview {
                        RitualChip(title: "Review", detail: "\(pulse.attention.count)", symbol: "checkmark.seal",
                                   tint: DocketPalette.review) {
                            workspace.ritual = .review
                        }
                        .accessibilityIdentifier("ritual.review")
                    }
                }
                .padding(.vertical, 2)
            }
            .scrollClipDisabled()
        }
    }
}

/// Today's plan as a ring: the done share in the brand's mint, the count in
/// the middle rolling as it changes, and a checkmark once the plan is done.
struct ProgressRing: View {
    let pulse: DailyPulse?
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    var body: some View {
        let progress = pulse?.progress ?? 0
        let complete = pulse?.planComplete ?? false
        ZStack {
            Circle()
                .stroke(DocketPalette.brand.opacity(0.16), lineWidth: 7)
            Circle()
                .trim(from: 0, to: complete ? 1 : progress)
                .stroke(
                    AngularGradient(colors: [DocketPalette.brand.opacity(0.7), DocketPalette.brand],
                                    center: .center, startAngle: .degrees(-90), endAngle: .degrees(270)),
                    style: StrokeStyle(lineWidth: 7, lineCap: .round)
                )
                .rotationEffect(.degrees(-90))
                .animation(reduceMotion ? nil : .spring(response: 0.6, dampingFraction: 0.8), value: progress)
            if complete {
                Image(systemName: "checkmark")
                    .font(.title2.weight(.bold))
                    .foregroundStyle(DocketPalette.brand)
                    .symbolEffect(.bounce, value: complete)
                    .transition(.scale.combined(with: .opacity))
            } else {
                VStack(spacing: -2) {
                    Text("\(pulse?.doneToday ?? 0)")
                        .font(.system(.title3, design: .rounded).weight(.bold))
                        .contentTransition(.numericText(value: Double(pulse?.doneToday ?? 0)))
                    Text("of \((pulse?.doneToday ?? 0) + (pulse?.todayOpen ?? 0))")
                        .font(.caption2.weight(.medium))
                        .foregroundStyle(.secondary)
                        .contentTransition(.numericText())
                }
                .monospacedDigit()
            }
        }
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
                Text(detail)
                    .lineLimit(1)
                    .truncationMode(.tail)
                    .frame(maxWidth: 140, alignment: .leading)
                    .fixedSize(horizontal: true, vertical: false)
                    .opacity(0.8)
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
