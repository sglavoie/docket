import SwiftUI
import DocketEngine
import DocketShared

/// Start the Day: the Inbox as a stack of cards, one capture at a time.
/// Swipe right to plan it for Today, up for Next, left for the Backlog; or
/// tap Done, Skip, or open it. Each decision commits at once through the same
/// actions the list's swipes use, so undo and sync see nothing new, and the
/// last decision of an Inbox it emptied ends on its own card.
struct TriageView: View {
    let workspace: WorkspaceModel
    @Environment(\.dismiss) private var dismiss
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    @State private var deck: [TaskSnapshot] = []
    @State private var total = 0
    @State private var projectNames: [String: String] = [:]
    @State private var loading = true
    @State private var drag: CGSize = .zero
    @State private var leaving: (task: TaskSnapshot, offset: CGSize)?
    @State private var committing = false
    @State private var failure: String?
    @State private var tally = Tally()
    @State private var history: [Decision] = []
    @State private var opened: TaskLocator?

    private static let threshold: CGFloat = 110

    struct Tally: Equatable {
        var today = 0, next = 0, backlog = 0, done = 0
        var total: Int { today + next + backlog + done }
    }

    enum Choice: Equatable {
        case today, next, backlog, done

        var action: TaskListAction {
            switch self {
            case .today: .plan(.addToday)
            case .next: .plan(.addNext)
            case .backlog: .plan(.backlog)
            case .done: .transition(.done)
            }
        }

        var title: String {
            switch self {
            case .today: "Today"
            case .next: "Next"
            case .backlog: "Backlog"
            case .done: "Done"
            }
        }

        var symbol: String {
            switch self {
            case .today: "sun.max.fill"
            case .next: "arrow.up.circle.fill"
            case .backlog: "archivebox.fill"
            case .done: "checkmark.circle.fill"
            }
        }

        var tint: Color {
            switch self {
            case .today: DocketPalette.today
            case .next: DocketPalette.next
            case .backlog: DocketPalette.state("backlog")
            case .done: DocketPalette.state("done")
            }
        }

        /// Where the card flies when this choice is made.
        var exit: CGSize {
            switch self {
            case .today: CGSize(width: 700, height: 40)
            case .next: CGSize(width: 0, height: -1000)
            case .backlog: CGSize(width: -700, height: 40)
            case .done: CGSize(width: 0, height: 1000)
            }
        }
    }

    struct Decision {
        let task: TaskSnapshot
        let choice: Choice
        let receipt: TaskActionReceipt
    }

    var body: some View {
        NavigationStack {
            ZStack {
                background
                VStack(spacing: 20) {
                    progressHeader
                    ZStack {
                        if loading {
                            ProgressView().controlSize(.large)
                        } else if deck.isEmpty {
                            finishedCard
                                .transition(.scale(scale: 0.9).combined(with: .opacity))
                        } else {
                            cards
                        }
                    }
                    .frame(maxWidth: .infinity, maxHeight: .infinity)
                    if !deck.isEmpty { controls }
                }
                .padding(.horizontal, 20)
                .padding(.bottom, 12)
            }
            .navigationTitle("Start the Day")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Close") { dismiss() }
                        .accessibilityIdentifier("triage.close")
                }
                ToolbarItem(placement: .primaryAction) {
                    Button("Undo", systemImage: "arrow.uturn.backward") { undo() }
                        .disabled(history.isEmpty || committing)
                        .accessibilityIdentifier("triage.undo")
                }
            }
            .navigationDestination(item: $opened) { locator in
                TaskDetailView(workspace: workspace, locator: locator)
            }
        }
        .sensoryFeedback(.selection, trigger: crossedChoice)
        .task { await load() }
    }

    // MARK: Pieces

    private var background: some View {
        LinearGradient(
            colors: [DocketPalette.today.opacity(0.18), Color(uiColor: .systemBackground)],
            startPoint: .top, endPoint: .center
        )
        .ignoresSafeArea()
    }

    private var progressHeader: some View {
        VStack(spacing: 8) {
            let done = total - deck.count
            ProgressView(value: Double(done), total: Double(max(total, 1)))
                .tint(DocketPalette.today)
                .animation(reduceMotion ? nil : .snappy, value: done)
            HStack {
                Text(deck.isEmpty ? "All sorted" : "\(done + 1) of \(total)")
                    .contentTransition(.numericText())
                Spacer()
                if let failure {
                    Text(failure)
                        .foregroundStyle(DocketPalette.overdue)
                        .lineLimit(2)
                        .accessibilityIdentifier("triage.failure")
                }
            }
            .font(.footnote.weight(.medium))
            .foregroundStyle(.secondary)
        }
        .padding(.top, 8)
    }

    private var cards: some View {
        ZStack {
            // The next two cards wait underneath, a little smaller.
            ForEach(Array(deck.prefix(3).enumerated().reversed()), id: \.element.id) { index, task in
                card(task, depth: index)
            }
            if let leaving {
                TriageCard(task: leaving.task, projectName: projectNames[leaving.task.projectID], choice: nil)
                    .offset(leaving.offset)
                    .rotationEffect(.degrees(Double(leaving.offset.width) / 25))
                    .allowsHitTesting(false)
            }
        }
    }

    @ViewBuilder
    private func card(_ task: TaskSnapshot, depth: Int) -> some View {
        let top = depth == 0
        TriageCard(task: task, projectName: projectNames[task.projectID], choice: top ? pendingChoice : nil)
            .scaleEffect(1 - CGFloat(depth) * 0.05)
            .offset(y: CGFloat(depth) * 14)
            .offset(top ? drag : .zero)
            .rotationEffect(.degrees(top ? Double(drag.width) / 22 : 0))
            .opacity(depth > 1 ? 0.6 : 1)
            .allowsHitTesting(top && !committing)
            .gesture(dragGesture(task), including: top ? .all : .none)
            .transition(.identity)
            .onTapGesture { if top { opened = task.locator } }
            .accessibilityElement(children: .combine)
            .accessibilityHint(top ? "Swipe right for Today, up for Next, left for Backlog." : "")
            .accessibilityActions {
                if top {
                    Button("Today") { decide(.today) }
                    Button("Next") { decide(.next) }
                    Button("Backlog") { decide(.backlog) }
                    Button("Done") { decide(.done) }
                    Button("Skip") { skip() }
                }
            }
            .accessibilityIdentifier(top ? "triage.card" : "triage.card.waiting")
    }

    private var controls: some View {
        HStack(spacing: 14) {
            choiceButton(.backlog, identifier: "triage.backlog")
            choiceButton(.next, identifier: "triage.next")
            choiceButton(.done, identifier: "triage.done")
            choiceButton(.today, identifier: "triage.today")
        }
        .overlay(alignment: .bottom) {
            Button("Skip for now") { skip() }
                .font(.footnote.weight(.medium))
                .foregroundStyle(.secondary)
                .offset(y: 30)
                .accessibilityIdentifier("triage.skip")
        }
        .padding(.bottom, 30)
        .disabled(committing)
    }

    private func choiceButton(_ choice: Choice, identifier: String) -> some View {
        Button { decide(choice) } label: {
            VStack(spacing: 6) {
                Image(systemName: choice.symbol)
                    .font(.title2)
                    .frame(width: 60, height: 60)
                    .background(choice.tint.opacity(pendingChoice == choice ? 0.3 : 0.14), in: Circle())
                    .scaleEffect(pendingChoice == choice ? 1.12 : 1)
                Text(choice.title)
                    .font(.caption.weight(.semibold))
            }
            .foregroundStyle(choice.tint)
            .frame(maxWidth: .infinity)
        }
        .buttonStyle(PressableButtonStyle())
        .animation(reduceMotion ? nil : .spring(response: 0.3, dampingFraction: 0.6), value: pendingChoice)
        .accessibilityIdentifier(identifier)
    }

    private var finishedCard: some View {
        VStack(spacing: 18) {
            Image(systemName: "checkmark.seal.fill")
                .font(.system(size: 72, weight: .semibold))
                .foregroundStyle(DocketPalette.brand)
                .symbolEffect(.bounce, options: .nonRepeating, value: deck.isEmpty)
            Text(tally.total == 0 ? "Your Inbox is clear" : "Inbox sorted")
                .font(.title.weight(.bold))
                .fontDesign(.rounded)
                .accessibilityIdentifier("triage.finished")
            if tally.total > 0 {
                HStack(spacing: 18) {
                    tallyItem(tally.today, .today)
                    tallyItem(tally.next, .next)
                    tallyItem(tally.backlog, .backlog)
                    tallyItem(tally.done, .done)
                }
            }
            Text(tally.today > 0 ? "Your day is planned. Go make it happen." : "Nothing waiting. Enjoy the quiet.")
                .foregroundStyle(.secondary)
            Button {
                dismiss()
                if tally.today > 0 { workspace.linkRequest = .view(.today) }
            } label: {
                Text(tally.today > 0 ? "See Today" : "Done")
                    .font(.headline)
                    .frame(maxWidth: 240)
                    .padding(.vertical, 6)
            }
            .buttonStyle(.borderedProminent)
            .accessibilityIdentifier("triage.finish")
        }
        .padding(32)
    }

    private func tallyItem(_ count: Int, _ choice: Choice) -> some View {
        VStack(spacing: 4) {
            Text("\(count)")
                .font(.system(.title2, design: .rounded).weight(.bold))
                .foregroundStyle(count > 0 ? choice.tint : .secondary)
            Text(choice.title)
                .font(.caption)
                .foregroundStyle(.secondary)
        }
        .accessibilityElement(children: .combine)
    }

    // MARK: Gesture

    /// The choice the drag currently points at, past the threshold.
    private var pendingChoice: Choice? { Self.choice(for: drag) }

    /// Changes only when the drag crosses into or out of a choice, so the
    /// selection tick plays once per crossing.
    private var crossedChoice: Choice? { pendingChoice }

    static func choice(for drag: CGSize) -> Choice? {
        if -drag.height > threshold, abs(drag.height) > abs(drag.width) { return .next }
        if drag.width > threshold { return .today }
        if drag.width < -threshold { return .backlog }
        return nil
    }

    private func dragGesture(_ task: TaskSnapshot) -> some Gesture {
        DragGesture()
            .onChanged { value in drag = value.translation }
            .onEnded { value in
                let predicted = value.predictedEndTranslation
                if let choice = Self.choice(for: value.translation) ?? Self.choice(for: CGSize(width: predicted.width * 0.5, height: predicted.height * 0.5)) {
                    decide(choice)
                } else {
                    withAnimation(.spring(response: 0.35, dampingFraction: 0.7)) { drag = .zero }
                }
            }
    }

    // MARK: Actions

    private func load() async {
        do {
            let result = try await workspace.engine.tasks(mode: "inbox", sort: TaskSort(field: "manual"))
            let projects = try await workspace.engine.projects()
            var names: [String: String] = [:]
            for project in projects.active + projects.parked + projects.archived { names[project.id] = project.name }
            projectNames = names
            deck = result.items.map(\.task).filter { !$0.state.isTerminal }
            total = deck.count
        } catch {
            failure = WorkspaceModel.describe(error)
        }
        loading = false
    }

    private func decide(_ choice: Choice) {
        guard let task = deck.first, !committing else { return }
        committing = true
        failure = nil
        let start = drag
        drag = .zero
        leaving = (task, start)
        withAnimation(reduceMotion ? .easeOut(duration: 0.15) : .easeIn(duration: 0.28)) {
            leaving = (task, choice.exit)
            deck.removeFirst()
        }
        Task {
            let result = await workspace.listAction(task.locator, choice.action)
            leaving = nil
            switch result {
            case .success(let receipt):
                history.append(Decision(task: task, choice: choice, receipt: receipt))
                switch choice {
                case .today: tally.today += 1
                case .next: tally.next += 1
                case .backlog: tally.backlog += 1
                case .done: tally.done += 1
                }
                if deck.isEmpty { workspace.celebrate() }
            case .failure(let refusal):
                failure = refusal.message
                withAnimation(.spring(response: 0.4, dampingFraction: 0.75)) { deck.insert(task, at: 0) }
            }
            committing = false
        }
    }

    private func skip() {
        guard deck.count > 1, !committing else { return }
        withAnimation(reduceMotion ? nil : .spring(response: 0.4, dampingFraction: 0.8)) {
            deck.append(deck.removeFirst())
        }
    }

    private func undo() {
        guard let last = history.last, !committing else { return }
        committing = true
        Task {
            if let refusal = await workspace.undoReceipt(last.receipt) {
                failure = refusal
            } else {
                history.removeLast()
                switch last.choice {
                case .today: tally.today -= 1
                case .next: tally.next -= 1
                case .backlog: tally.backlog -= 1
                case .done: tally.done -= 1
                }
                withAnimation(.spring(response: 0.4, dampingFraction: 0.75)) { deck.insert(last.task, at: 0) }
            }
            committing = false
        }
    }
}

/// One capture as a card: its title large, where it came from, how long it
/// has waited, and what it already carries. While a drag points at a choice,
/// the card wears that choice's stamp.
struct TriageCard: View {
    let task: TaskSnapshot
    let projectName: String?
    let choice: TriageView.Choice?

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            HStack {
                Label(projectName ?? "Inbox", systemImage: "folder")
                    .font(.footnote.weight(.semibold))
                    .foregroundStyle(.secondary)
                Spacer()
                Text("Captured \(task.createdAt, format: .relative(presentation: .named))")
                    .font(.footnote)
                    .foregroundStyle(.tertiary)
            }
            Text(task.title)
                .font(.title2.weight(.semibold))
                .fontDesign(.rounded)
                .fixedSize(horizontal: false, vertical: true)
                .lineLimit(6)
            if let description = task.description, !description.isEmpty {
                Text(description)
                    .font(.body)
                    .foregroundStyle(.secondary)
                    .lineLimit(5)
            }
            Spacer(minLength: 0)
            FlowChips(task: task)
        }
        .padding(24)
        .frame(maxWidth: .infinity, maxHeight: 420, alignment: .topLeading)
        .background {
            RoundedRectangle(cornerRadius: 28, style: .continuous)
                .fill(Color(uiColor: .secondarySystemGroupedBackground))
                .shadow(color: .black.opacity(0.12), radius: 18, y: 8)
        }
        .overlay {
            RoundedRectangle(cornerRadius: 28, style: .continuous)
                .strokeBorder(choice?.tint ?? .clear, lineWidth: 3)
        }
        .overlay(alignment: .center) {
            if let choice {
                Label(choice.title, systemImage: choice.symbol)
                    .font(.title.weight(.heavy))
                    .fontDesign(.rounded)
                    .foregroundStyle(choice.tint)
                    .padding(.horizontal, 18)
                    .padding(.vertical, 10)
                    .background(.thinMaterial, in: Capsule())
                    .rotationEffect(.degrees(-8))
                    .transition(.scale.combined(with: .opacity))
            }
        }
        .animation(.spring(response: 0.25, dampingFraction: 0.6), value: choice)
    }
}

/// What a task already carries, as small chips: priority, deadline, tags,
/// context, and counts of its checklist, comments, and attachments.
struct FlowChips: View {
    let task: TaskSnapshot

    var body: some View {
        let chips = items
        if !chips.isEmpty {
            ViewThatFits(in: .horizontal) {
                HStack(spacing: 6) { ForEach(chips, id: \.self) { chip($0) } }
                VStack(alignment: .leading, spacing: 6) {
                    HStack(spacing: 6) { ForEach(chips.prefix(3), id: \.self) { chip($0) } }
                    HStack(spacing: 6) { ForEach(chips.dropFirst(3), id: \.self) { chip($0) } }
                }
            }
        }
    }

    private var items: [String] {
        var out: [String] = []
        if task.priority > 0 { out.append("!\(task.priority)") }
        if let due = task.dueDate { out.append("@\(TaskSchedule.relativeDeadline(due, now: .now))") }
        out += task.tags.prefix(4).map { "#\($0)" }
        if let context = task.context, !context.isEmpty { out.append("##\(context)") }
        if let checklist = task.checklist, !checklist.isEmpty {
            out.append("☑︎ \(checklist.filter(\.done).count)/\(checklist.count)")
        }
        return out
    }

    private func chip(_ text: String) -> some View {
        Text(text)
            .font(.caption.weight(.semibold))
            .lineLimit(1)
            .padding(.horizontal, 9)
            .padding(.vertical, 5)
            .background(CaptureHighlighter.color(forToken: text).opacity(0.14), in: Capsule())
            .foregroundStyle(CaptureHighlighter.color(forToken: text))
    }
}
