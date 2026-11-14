import SwiftUI
import DocketEngine
import DocketShared

/// Wrap Up: the evening's close to the day Start the Day opens. It names what
/// got finished, shows the week's rhythm as seven quiet bars, and asks one
/// question of what is left on Today — keep it there for tomorrow, or send it
/// back to Next — before ending on a calm card. Opening it writes nothing;
/// only Send Back to Next does, as one transaction and one undo entry.
struct RecapView: View {
    let workspace: WorkspaceModel
    @Environment(\.dismiss) private var dismiss
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    @State private var recap: DayRecap?
    @State private var failure: String?
    @State private var acting = false
    @State private var closed: Closing?
    @State private var appeared = false

    /// How the person closed the day, which the last card reflects.
    enum Closing: Equatable {
        case kept(Int)
        case sentBack(Int)
        case clear
    }

    var body: some View {
        NavigationStack {
            ZStack {
                EveningBackdrop()
                if let closed {
                    closingCard(closed)
                        .transition(.asymmetric(insertion: .scale(scale: 0.92).combined(with: .opacity), removal: .opacity))
                } else if let recap {
                    ScrollView {
                        content(recap)
                            .padding(.horizontal, 24)
                            .padding(.bottom, 32)
                    }
                    .transition(.opacity)
                } else if let failure {
                    ScreenStateView(state: .error(title: "Wrap Up could not load", message: failure))
                } else {
                    ProgressView().controlSize(.large)
                }
            }
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Close") { dismiss() }
                        .accessibilityIdentifier("recap.close")
                }
            }
            .toolbarBackground(.hidden, for: .navigationBar)
        }
        .preferredColorScheme(.dark)
        .tint(DocketPalette.brand)
        .task { await load() }
    }

    @ViewBuilder
    private func content(_ recap: DayRecap) -> some View {
        VStack(alignment: .leading, spacing: 28) {
            VStack(alignment: .leading, spacing: 6) {
                Text(Date.now.formatted(.dateTime.weekday(.wide).month(.wide).day()))
                    .font(.subheadline.weight(.semibold))
                    .foregroundStyle(.white.opacity(0.6))
                Text(Self.headline(done: recap.doneToday.count))
                    .font(.system(.largeTitle, design: .rounded).weight(.bold))
                    .foregroundStyle(.white)
                    .fixedSize(horizontal: false, vertical: true)
                    .accessibilityIdentifier("recap.headline")
            }
            .padding(.top, 12)

            WeekRhythm(days: recap.week)
                .opacity(appeared ? 1 : 0)
                .offset(y: appeared || reduceMotion ? 0 : 12)

            if !recap.doneToday.isEmpty {
                section("Finished today", count: recap.doneToday.count, tint: DocketPalette.state("done")) {
                    ForEach(recap.doneToday, id: \.locator) { task in
                        row(task, symbol: "checkmark.circle.fill", tint: DocketPalette.state("done"), projectName: recap.projectNames[task.projectID])
                    }
                }
            }

            if recap.leftovers.isEmpty {
                Button {
                    finish(.clear)
                } label: {
                    Label("Close the Day", systemImage: "moon.stars.fill")
                        .font(.headline)
                        .frame(maxWidth: .infinity)
                }
                .buttonStyle(.borderedProminent)
                .tint(DocketPalette.brand)
                .foregroundStyle(DocketPalette.onBrand)
                .controlSize(.large)
                .accessibilityIdentifier("recap.closeDay")
            } else {
                section("Still on Today", count: recap.leftovers.count, tint: DocketPalette.today) {
                    ForEach(recap.leftovers, id: \.locator) { task in
                        row(task, symbol: task.state.symbol, tint: task.state.tint, projectName: recap.projectNames[task.projectID])
                    }
                }
                VStack(spacing: 10) {
                    Button {
                        finish(.kept(recap.leftovers.count))
                    } label: {
                        Label("Keep for Tomorrow", systemImage: "sunrise.fill")
                            .font(.headline)
                            .frame(maxWidth: .infinity)
                    }
                    .buttonStyle(.borderedProminent)
                    .tint(DocketPalette.brand)
                    .foregroundStyle(DocketPalette.onBrand)
                    .controlSize(.large)
                    .accessibilityIdentifier("recap.keep")
                    Button {
                        sendBack(recap.leftovers)
                    } label: {
                        Label("Send Back to Next", systemImage: "arrow.uturn.backward.circle")
                            .font(.headline)
                            .frame(maxWidth: .infinity)
                    }
                    .buttonStyle(.bordered)
                    .tint(.white)
                    .controlSize(.large)
                    .disabled(acting)
                    .accessibilityIdentifier("recap.sendBack")
                    Text("Today keeps its plan overnight. Sending work back clears tomorrow’s slate.")
                        .font(.footnote)
                        .foregroundStyle(.white.opacity(0.55))
                        .multilineTextAlignment(.center)
                        .frame(maxWidth: .infinity)
                }
            }
            if let failure {
                Text(failure)
                    .font(.footnote)
                    .foregroundStyle(DocketPalette.overdue)
                    .accessibilityIdentifier("recap.failure")
            }
        }
        .onAppear {
            withAnimation(reduceMotion ? nil : .easeOut(duration: 0.6).delay(0.15)) { appeared = true }
        }
    }

    private func section<Rows: View>(_ title: String, count: Int, tint: Color, @ViewBuilder rows: () -> Rows) -> some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack(spacing: 6) {
                Text(title.uppercased())
                Text("\(count)").foregroundStyle(tint)
            }
            .font(.caption.weight(.bold))
            .foregroundStyle(.white.opacity(0.6))
            .accessibilityElement(children: .combine)
            VStack(alignment: .leading, spacing: 0) {
                rows()
            }
            .background(.white.opacity(0.06), in: RoundedRectangle(cornerRadius: 20, style: .continuous))
        }
    }

    private func row(_ task: TaskSnapshot, symbol: String, tint: Color, projectName: String?) -> some View {
        HStack(alignment: .firstTextBaseline, spacing: 12) {
            Image(systemName: symbol)
                .foregroundStyle(tint)
                .accessibilityHidden(true)
            VStack(alignment: .leading, spacing: 2) {
                Text(task.title)
                    .foregroundStyle(.white)
                if let projectName {
                    Text(projectName)
                        .font(.caption)
                        .foregroundStyle(DocketPalette.projectHue(task.projectID))
                }
            }
            Spacer(minLength: 0)
        }
        .padding(.horizontal, 16)
        .padding(.vertical, 12)
        .accessibilityElement(children: .combine)
    }

    @ViewBuilder
    private func closingCard(_ closing: Closing) -> some View {
        VStack(spacing: 18) {
            Image(systemName: "moon.stars.fill")
                .font(.system(size: 64, weight: .semibold))
                .symbolRenderingMode(.hierarchical)
                .foregroundStyle(DocketPalette.today)
                .symbolEffect(.bounce, options: .nonRepeating, value: closed)
            Text("See you tomorrow")
                .font(.system(.title, design: .rounded).weight(.bold))
                .foregroundStyle(.white)
                .accessibilityIdentifier("recap.goodnight")
            Text(Self.closingLine(closing))
                .font(.body)
                .foregroundStyle(.white.opacity(0.7))
                .multilineTextAlignment(.center)
                .fixedSize(horizontal: false, vertical: true)
            Button("Good Night") { dismiss() }
                .buttonStyle(.bordered)
                .tint(.white)
                .controlSize(.large)
                .padding(.top, 8)
                .accessibilityIdentifier("recap.goodnight.close")
        }
        .padding(32)
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }

    static func headline(done: Int) -> String {
        switch done {
        case 0: "A quiet day."
        case 1: "One thing done today."
        default: "\(done) things done today."
        }
    }

    static func closingLine(_ closing: Closing) -> String {
        switch closing {
        case .kept(let count): count == 1 ? "One task waits on Today for the morning." : "\(count) tasks wait on Today for the morning."
        case .sentBack(let count): count == 1 ? "One task went back to Next. Tomorrow starts clear." : "\(count) tasks went back to Next. Tomorrow starts clear."
        case .clear: "Nothing left hanging. Rest well."
        }
    }

    private func finish(_ closing: Closing) {
        workspace.signal(.planned)
        withAnimation(reduceMotion ? nil : .spring(response: 0.5, dampingFraction: 0.85)) { closed = closing }
    }

    private func sendBack(_ tasks: [TaskSnapshot]) {
        guard !acting else { return }
        acting = true
        failure = nil
        Task {
            let result = await workspace.bulk(tasks.map(\.locator), .plan(.addNext))
            acting = false
            if let refusal = result.refusal {
                failure = refusal
            } else {
                finish(.sentBack(tasks.count))
            }
        }
    }

    private func load() async {
        do {
            recap = try await DayRecap.load(workspace.engine)
        } catch {
            failure = WorkspaceModel.describe(error)
        }
    }
}

/// What Wrap Up reads: today's finished work, the open work still planned for
/// today, and how many tasks were finished on each of the last seven days.
struct DayRecap: Equatable {
    struct Day: Equatable, Identifiable {
        var date: Date
        var done: Int
        var id: Date { date }
    }

    var doneToday: [TaskSnapshot] = []
    var leftovers: [TaskSnapshot] = []
    var week: [Day] = []
    var projectNames: [String: String] = [:]

    static func load(_ engine: DocketEngine, now: Date = .now) async throws -> DayRecap {
        var recap = DayRecap()
        let completed = try await engine.tasks(mode: "recent", query: "?completed").items.map(\.task)
            .filter { $0.state == .done }
        recap.week = week(completed, now: now)
        let midnight = TaskSchedule.calendar.startOfDay(for: now)
        recap.doneToday = completed
            .filter { ($0.completedAt ?? .distantPast) >= midnight }
            .sorted { ($0.completedAt ?? .distantPast) < ($1.completedAt ?? .distantPast) }
        recap.leftovers = try await engine.tasks(mode: "today").items.map(\.task).filter { !$0.state.isTerminal }
        if let projects = try? await engine.projects() {
            for project in projects.active + projects.parked + projects.archived {
                recap.projectNames[project.id] = project.name
            }
        }
        return recap
    }

    /// The last seven days, oldest first and ending today, each with the
    /// number of tasks finished on it.
    static func week(_ completed: [TaskSnapshot], now: Date) -> [Day] {
        let calendar = TaskSchedule.calendar
        let today = calendar.startOfDay(for: now)
        return (0..<7).reversed().compactMap { offset -> Day? in
            guard let day = calendar.date(byAdding: .day, value: -offset, to: today) else { return nil }
            let count = completed.filter { task in
                task.completedAt.map { calendar.isDate($0, inSameDayAs: day) } ?? false
            }.count
            return Day(date: day, done: count)
        }
    }
}

/// The week as seven bars, today's in the brand's mint and the rest quieter,
/// each labelled with its weekday's initial. A count above a bar names it;
/// the whole reads to VoiceOver as one sentence.
struct WeekRhythm: View {
    let days: [DayRecap.Day]
    var tint: Color = DocketPalette.brand
    var onDark = true
    @ScaledMetric(relativeTo: .caption) private var height: CGFloat = 64

    var body: some View {
        let peak = max(1, days.map(\.done).max() ?? 1)
        VStack(alignment: .leading, spacing: 8) {
            Text("THIS WEEK")
                .font(.caption.weight(.bold))
                .foregroundStyle(secondary)
            HStack(alignment: .bottom, spacing: 10) {
                ForEach(days) { day in
                    let isToday = day.id == days.last?.id
                    VStack(spacing: 6) {
                        Text(day.done > 0 ? "\(day.done)" : " ")
                            .font(.caption2.weight(.semibold))
                            .foregroundStyle(isToday ? tint : secondary)
                            .monospacedDigit()
                        RoundedRectangle(cornerRadius: 5, style: .continuous)
                            .fill(isToday ? AnyShapeStyle(tint) : AnyShapeStyle(tint.opacity(0.35)))
                            .frame(height: max(6, height * CGFloat(day.done) / CGFloat(peak)))
                        Text(day.date.formatted(.dateTime.weekday(.narrow)))
                            .font(.caption2.weight(isToday ? .bold : .regular))
                            .foregroundStyle(isToday ? primary : secondary)
                    }
                    .frame(maxWidth: .infinity)
                }
            }
        }
        .accessibilityElement(children: .ignore)
        .accessibilityLabel("This week")
        .accessibilityValue(spoken)
        .accessibilityIdentifier("recap.week")
    }

    private var primary: Color { onDark ? .white : .primary }
    private var secondary: Color { onDark ? .white.opacity(0.55) : .secondary }

    private var spoken: String {
        let total = days.map(\.done).reduce(0, +)
        let today = days.last?.done ?? 0
        return "\(total) finished in seven days, \(today) today"
    }
}

/// Dusk behind Wrap Up: the icon's navy drifting toward indigo, with a low
/// warm glow, still under Reduce Motion.
struct EveningBackdrop: View {
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    var body: some View {
        TimelineView(.animation(minimumInterval: 1 / 20, paused: reduceMotion)) { timeline in
            let t = reduceMotion ? 0 : timeline.date.timeIntervalSinceReferenceDate
            let drift = Float(sin(t / 7) * 0.08)
            MeshGradient(
                width: 3, height: 3,
                points: [
                    [0, 0], [0.5, 0], [1, 0],
                    [0, 0.5], [0.5 + drift, 0.45 - drift], [1, 0.5],
                    [0, 1], [0.5 - drift, 1], [1, 1],
                ],
                colors: [
                    DocketPalette.navyDeep, DocketPalette.navy, DocketPalette.navyDeep,
                    Color(red: 0.16, green: 0.14, blue: 0.30), Color(red: 0.13, green: 0.15, blue: 0.27), Color(red: 0.18, green: 0.13, blue: 0.28),
                    Color(red: 0.30, green: 0.20, blue: 0.22), Color(red: 0.24, green: 0.16, blue: 0.26), DocketPalette.navyDeep,
                ]
            )
        }
        .ignoresSafeArea()
    }
}
