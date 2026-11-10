import SwiftUI
import DocketEngine
import DocketShared

/// Now: the task in progress, full screen and nothing else. How long it has
/// been going, where you left off, its checklist, and a focus session with a
/// countdown that also runs on the Lock Screen. Mark Done finishes it; Pause
/// asks where you left off first. With several tasks in Doing, the newest
/// leads and the others are a swipe away; with none, it offers to start the
/// first one planned for today.
struct NowView: View {
    let workspace: WorkspaceModel
    @Environment(\.dismiss) private var dismiss
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    @State private var doing: [TaskSnapshot] = []
    @State private var nextUp: TaskSnapshot?
    @State private var page: TaskLocator?
    @State private var loaded = false
    @State private var acting = false
    @State private var failure: String?
    @State private var pausing: TaskSnapshot?
    @State private var focusMinutes = 25
    @State private var opened: TaskLocator?
    @State private var finished: String?
    /// The task just pinned to the Lock Screen, which the activity list does
    /// not report to SwiftUI on its own.
    @State private var pinned: TaskLocator?

    private static let focusChoices = [15, 25, 50]

    var body: some View {
        NavigationStack {
            ZStack {
                LinearGradient(colors: [DocketPalette.navy, DocketPalette.navyDeep], startPoint: .top, endPoint: .bottom)
                    .ignoresSafeArea()
                if !loaded {
                    ProgressView().controlSize(.large)
                } else if doing.isEmpty {
                    emptyState
                } else {
                    TabView(selection: $page) {
                        ForEach(doing) { task in
                            ScrollView {
                                taskPage(task)
                                    .padding(.horizontal, 24)
                                    .padding(.bottom, 32)
                            }
                            .tag(Optional(task.locator))
                        }
                    }
                    .tabViewStyle(.page(indexDisplayMode: doing.count > 1 ? .always : .never))
                }
            }
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Close") { dismiss() }
                        .accessibilityIdentifier("now.close")
                }
                if let task = current {
                    ToolbarItem(placement: .primaryAction) {
                        Button("Details", systemImage: "info.circle") { opened = task.locator }
                            .accessibilityIdentifier("now.details")
                    }
                }
            }
            .toolbarBackground(.hidden, for: .navigationBar)
            .navigationDestination(item: $opened) { locator in
                TaskDetailView(workspace: workspace, locator: locator)
            }
        }
        .preferredColorScheme(.dark)
        .tint(DocketPalette.brand)
        .resumeNotePrompt(for: $pausing, workspace: workspace) { task, refusal in
            if let refusal { failure = refusal } else { transition(task, to: .paused) }
        }
        .task(id: workspace.generation) { await load() }
    }

    private var current: TaskSnapshot? {
        doing.first { $0.locator == page } ?? doing.first
    }

    // MARK: Page

    @ViewBuilder
    private func taskPage(_ task: TaskSnapshot) -> some View {
        VStack(alignment: .leading, spacing: 22) {
            VStack(alignment: .leading, spacing: 8) {
                Label(workspace.pulse?.projectNames[task.projectID] ?? "In progress", systemImage: "scope")
                    .font(.subheadline.weight(.semibold))
                    .foregroundStyle(DocketPalette.state("doing"))
                Text(task.title)
                    .font(.system(.largeTitle, design: .rounded).weight(.bold))
                    .foregroundStyle(.white)
                    .fixedSize(horizontal: false, vertical: true)
                    .accessibilityIdentifier("now.title")
                TimelineView(.periodic(from: .now, by: 30)) { context in
                    Text("Doing for \(Self.duration(since: task.stateChangedAt, now: context.date))")
                        .font(.subheadline)
                        .foregroundStyle(.white.opacity(0.6))
                }
            }
            .padding(.top, 12)

            if let note = task.resumeNote, !note.isEmpty {
                VStack(alignment: .leading, spacing: 6) {
                    Label("Where you left off", systemImage: "bookmark.fill")
                        .font(.caption.weight(.bold))
                        .textCase(.uppercase)
                        .foregroundStyle(DocketPalette.today)
                    Text(note)
                        .font(.body)
                        .foregroundStyle(.white)
                }
                .padding(16)
                .frame(maxWidth: .infinity, alignment: .leading)
                .background(.white.opacity(0.08), in: RoundedRectangle(cornerRadius: 18, style: .continuous))
                .accessibilityElement(children: .combine)
                .accessibilityIdentifier("now.resume")
            }

            focusPanel(task)

            if let checklist = task.checklist, !checklist.isEmpty {
                checklistPanel(task, checklist)
            }

            if let failure {
                Text(failure)
                    .font(.footnote)
                    .foregroundStyle(DocketPalette.overdue)
                    .accessibilityIdentifier("now.failure")
            }

            VStack(spacing: 12) {
                Button { complete(task) } label: {
                    Label("Mark Done", systemImage: "checkmark")
                        .font(.headline)
                        .frame(maxWidth: .infinity)
                        .padding(.vertical, 14)
                        .foregroundStyle(DocketPalette.navyDeep)
                        .background(DocketPalette.brand, in: Capsule())
                }
                .buttonStyle(PressableButtonStyle())
                .accessibilityIdentifier("now.done")
                HStack(spacing: 12) {
                    Button { pausing = task } label: {
                        Label("Pause", systemImage: "pause.fill")
                            .frame(maxWidth: .infinity)
                            .padding(.vertical, 12)
                            .background(.white.opacity(0.1), in: Capsule())
                    }
                    .accessibilityIdentifier("now.pause")
                    if workspace.activities.available, workspace.activities.shownTask != task.locator, pinned != task.locator {
                        Button {
                            workspace.pinToLockScreen(task, projectName: projectName(task))
                            pinned = task.locator
                        } label: {
                            Label("Lock Screen", systemImage: "lock.rectangle.on.rectangle")
                                .frame(maxWidth: .infinity)
                                .padding(.vertical, 12)
                                .background(.white.opacity(0.1), in: Capsule())
                        }
                        .accessibilityIdentifier("now.pin")
                    }
                }
                .font(.subheadline.weight(.semibold))
                .foregroundStyle(.white)
                .buttonStyle(PressableButtonStyle())
            }
            .disabled(acting)
        }
    }

    @ViewBuilder
    private func focusPanel(_ task: TaskSnapshot) -> some View {
        let session = workspace.focus.flatMap { $0.locator == task.locator ? $0 : nil }
        VStack(spacing: 16) {
            if let session {
                TimelineView(.periodic(from: .now, by: 1)) { context in
                    let remaining = session.remaining(at: context.date)
                    ZStack {
                        Circle().stroke(.white.opacity(0.1), lineWidth: 10)
                        Circle()
                            .trim(from: 0, to: 1 - session.progress(at: context.date))
                            .stroke(DocketPalette.state("doing"), style: StrokeStyle(lineWidth: 10, lineCap: .round))
                            .rotationEffect(.degrees(-90))
                            .animation(reduceMotion ? nil : .linear(duration: 1), value: context.date)
                        VStack(spacing: 2) {
                            Text(remaining > 0 ? Self.clock(remaining) : "Time’s up")
                                .font(.system(size: 44, weight: .bold, design: .rounded))
                                .monospacedDigit()
                                .contentTransition(.numericText(countsDown: true))
                                .foregroundStyle(.white)
                            Text(remaining > 0 ? "focus" : "Nice work")
                                .font(.subheadline)
                                .foregroundStyle(.white.opacity(0.6))
                        }
                    }
                    .frame(width: 210, height: 210)
                    .accessibilityElement(children: .ignore)
                    .accessibilityLabel("Focus session")
                    .accessibilityValue(remaining > 0 ? "\(Int(remaining / 60)) minutes left" : "Finished")
                }
                Button("End Focus") { workspace.stopFocus() }
                    .font(.subheadline.weight(.semibold))
                    .foregroundStyle(.white.opacity(0.8))
                    .accessibilityIdentifier("now.focus.stop")
            } else {
                HStack(spacing: 8) {
                    ForEach(Self.focusChoices, id: \.self) { minutes in
                        Button("\(minutes) min") { focusMinutes = minutes }
                            .font(.subheadline.weight(.semibold))
                            .padding(.horizontal, 14)
                            .padding(.vertical, 8)
                            .foregroundStyle(focusMinutes == minutes ? DocketPalette.navyDeep : .white)
                            .background(focusMinutes == minutes ? AnyShapeStyle(DocketPalette.state("doing")) : AnyShapeStyle(.white.opacity(0.1)), in: Capsule())
                            .accessibilityAddTraits(focusMinutes == minutes ? .isSelected : [])
                    }
                }
                Button {
                    workspace.startFocus(task, minutes: focusMinutes, projectName: projectName(task))
                } label: {
                    Label("Start Focus", systemImage: "timer")
                        .font(.headline)
                        .padding(.horizontal, 22)
                        .padding(.vertical, 10)
                        .foregroundStyle(DocketPalette.state("doing"))
                        .background(DocketPalette.state("doing").opacity(0.16), in: Capsule())
                }
                .buttonStyle(PressableButtonStyle())
                .accessibilityIdentifier("now.focus.start")
            }
        }
        .frame(maxWidth: .infinity)
        .padding(.vertical, 18)
        .background(.white.opacity(0.05), in: RoundedRectangle(cornerRadius: 24, style: .continuous))
        .animation(reduceMotion ? nil : .snappy, value: session)
    }

    @ViewBuilder
    private func checklistPanel(_ task: TaskSnapshot, _ checklist: [ChecklistItem]) -> some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack {
                Text("Checklist").font(.caption.weight(.bold)).textCase(.uppercase)
                Spacer()
                Text("\(checklist.filter(\.done).count) of \(checklist.count)")
                    .font(.caption.weight(.semibold))
                    .contentTransition(.numericText())
            }
            .foregroundStyle(.white.opacity(0.6))
            ForEach(Array(checklist.enumerated()), id: \.offset) { index, item in
                Button {
                    toggle(task, index: index, seen: checklist)
                } label: {
                    HStack(alignment: .firstTextBaseline, spacing: 12) {
                        Image(systemName: item.done ? "checkmark.circle.fill" : "circle")
                            .foregroundStyle(item.done ? DocketPalette.brand : .white.opacity(0.5))
                            .contentTransition(.symbolEffect(.replace))
                        Text(item.text)
                            .strikethrough(item.done)
                            .foregroundStyle(item.done ? .white.opacity(0.5) : .white)
                            .multilineTextAlignment(.leading)
                        Spacer(minLength: 0)
                    }
                }
                .accessibilityIdentifier("now.checklist.\(index)")
            }
        }
        .padding(16)
        .background(.white.opacity(0.05), in: RoundedRectangle(cornerRadius: 18, style: .continuous))
        .disabled(acting)
    }

    // MARK: Empty

    private var emptyState: some View {
        VStack(spacing: 18) {
            Image(systemName: finished == nil ? "scope" : "checkmark.circle.fill")
                .font(.system(size: 64, weight: .semibold))
                .foregroundStyle(finished == nil ? DocketPalette.state("doing") : DocketPalette.brand)
                .symbolEffect(.bounce, value: finished)
            Text(finished.map { "Finished “\($0)”" } ?? "Nothing in progress")
                .font(.title2.weight(.bold))
                .fontDesign(.rounded)
                .foregroundStyle(.white)
                .multilineTextAlignment(.center)
                .accessibilityIdentifier("now.empty")
            if let nextUp {
                Text("Up next from Today")
                    .font(.subheadline)
                    .foregroundStyle(.white.opacity(0.6))
                Button { transition(nextUp, to: .doing) } label: {
                    Label("Start “\(nextUp.title)”", systemImage: "play.fill")
                        .font(.headline)
                        .lineLimit(2)
                        .padding(.horizontal, 20)
                        .padding(.vertical, 12)
                        .foregroundStyle(DocketPalette.navyDeep)
                        .background(DocketPalette.brand, in: Capsule())
                }
                .buttonStyle(PressableButtonStyle())
                .accessibilityIdentifier("now.startNext")
            } else {
                Text("Start a task from Today and it will show here.")
                    .foregroundStyle(.white.opacity(0.6))
            }
        }
        .padding(32)
        .disabled(acting)
    }

    // MARK: Actions

    private func projectName(_ task: TaskSnapshot) -> String {
        workspace.pulse?.projectNames[task.projectID] ?? ""
    }

    private func load() async {
        do {
            let result = try await workspace.engine.tasks(mode: "all", query: "$doing")
            let loadedDoing = result.items.map(\.task).filter { $0.state == .doing }
                .sorted { $0.stateChangedAt > $1.stateChangedAt }
            if loadedDoing.isEmpty {
                let today = try? await workspace.engine.tasks(mode: "today", sort: TaskSort(field: "manual"))
                nextUp = today?.items.map(\.task).first { !$0.state.isTerminal }
            } else {
                nextUp = nil
            }
            withAnimation(reduceMotion ? nil : .snappy) { doing = loadedDoing }
            if page == nil || !loadedDoing.contains(where: { $0.locator == page }) {
                page = loadedDoing.first?.locator
            }
        } catch {
            failure = WorkspaceModel.describe(error)
        }
        loaded = true
    }

    private func complete(_ task: TaskSnapshot) {
        acting = true
        failure = nil
        Task {
            switch await workspace.listAction(task.locator, .transition(.done)) {
            case .success:
                if workspace.focus?.locator == task.locator { workspace.stopFocus() }
                finished = task.title
                if doing.count == 1 { workspace.celebrate() }
            case .failure(let refusal):
                failure = refusal.message
            }
            acting = false
        }
    }

    private func transition(_ task: TaskSnapshot, to state: WorkflowState) {
        acting = true
        failure = nil
        Task {
            if let refusal = await workspace.transition(task.locator, to: state) {
                failure = refusal
            } else if state != .doing, workspace.focus?.locator == task.locator {
                workspace.stopFocus()
            }
            if state == .doing { finished = nil }
            acting = false
        }
    }

    private func toggle(_ task: TaskSnapshot, index: Int, seen: [ChecklistItem]) {
        acting = true
        Task {
            failure = await workspace.checklist(task.locator, .toggle(index: index), seen: seen)
            acting = false
        }
    }

    static func duration(since start: Date, now: Date) -> String {
        let minutes = max(0, Int(now.timeIntervalSince(start) / 60))
        if minutes < 1 { return "less than a minute" }
        if minutes < 60 { return "\(minutes) min" }
        let hours = minutes / 60
        if hours < 24 { return minutes % 60 == 0 ? "\(hours) h" : "\(hours) h \(minutes % 60) min" }
        let days = hours / 24
        return days == 1 ? "1 day" : "\(days) days"
    }

    static func clock(_ interval: TimeInterval) -> String {
        let seconds = Int(interval.rounded(.up))
        return String(format: "%d:%02d", seconds / 60, seconds % 60)
    }
}
