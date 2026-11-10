import SwiftUI
import DocketEngine
import DocketShared

/// A review pass: every task asking for attention — flagged for review, or
/// with a check-in whose day has come — one at a time, as the terminal app's
/// `R` walks them. Reviewed acknowledges it, which clears the flag, records
/// the review, and moves a repeating check-in on; Done finishes it; Skip
/// leaves it for later. The pass never wraps and ends on its own card.
struct ReviewPassView: View {
    let workspace: WorkspaceModel
    @Environment(\.dismiss) private var dismiss
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    @State private var queue: [TaskSnapshot] = []
    @State private var total = 0
    @State private var reviewed = 0
    @State private var loading = true
    @State private var acting = false
    @State private var failure: String?
    @State private var opened: TaskLocator?

    var body: some View {
        NavigationStack {
            VStack(spacing: 18) {
                if loading {
                    Spacer()
                    ProgressView().controlSize(.large)
                    Spacer()
                } else if let task = queue.first {
                    ProgressView(value: Double(total - queue.count), total: Double(max(total, 1)))
                        .tint(DocketPalette.review)
                    Text("\(total - queue.count + 1) of \(total)")
                        .font(.footnote.weight(.medium))
                        .foregroundStyle(.secondary)
                        .frame(maxWidth: .infinity, alignment: .leading)
                    ReviewCard(task: task, projectName: workspace.pulse?.projectNames[task.projectID])
                        .id(task.id)
                        .transition(.asymmetric(insertion: .move(edge: .trailing).combined(with: .opacity),
                                                removal: .move(edge: .leading).combined(with: .opacity)))
                        .onTapGesture { opened = task.locator }
                    if let failure {
                        Text(failure).font(.footnote).foregroundStyle(DocketPalette.overdue)
                            .accessibilityIdentifier("review.failure")
                    }
                    Spacer(minLength: 0)
                    actions(task)
                } else {
                    Spacer()
                    finished
                    Spacer()
                }
            }
            .padding(20)
            .background(
                LinearGradient(colors: [DocketPalette.review.opacity(0.14), Color(uiColor: .systemBackground)],
                               startPoint: .top, endPoint: .center)
                .ignoresSafeArea()
            )
            .navigationTitle("Review")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Close") { dismiss() }
                        .accessibilityIdentifier("review.close")
                }
            }
            .navigationDestination(item: $opened) { locator in
                TaskDetailView(workspace: workspace, locator: locator)
            }
        }
        .task { await load() }
    }

    private func actions(_ task: TaskSnapshot) -> some View {
        VStack(spacing: 12) {
            Button { acknowledge(task) } label: {
                Label("Reviewed", systemImage: "checkmark.seal.fill")
                    .font(.headline)
                    .frame(maxWidth: .infinity)
                    .padding(.vertical, 14)
                    .foregroundStyle(.white)
                    .background(DocketPalette.Solid.paused, in: Capsule())
            }
            .buttonStyle(PressableButtonStyle())
            .accessibilityIdentifier("review.acknowledge")
            HStack(spacing: 12) {
                Button { complete(task) } label: {
                    Label("Done", systemImage: "checkmark")
                        .frame(maxWidth: .infinity)
                        .padding(.vertical, 12)
                        .background(DocketPalette.state("done").opacity(0.16), in: Capsule())
                        .foregroundStyle(DocketPalette.state("done"))
                }
                .accessibilityIdentifier("review.done")
                Button { advance() } label: {
                    Label("Skip", systemImage: "arrow.forward")
                        .frame(maxWidth: .infinity)
                        .padding(.vertical, 12)
                        .background(Color.secondary.opacity(0.14), in: Capsule())
                        .foregroundStyle(.secondary)
                }
                .accessibilityIdentifier("review.skip")
            }
            .font(.subheadline.weight(.semibold))
            .buttonStyle(PressableButtonStyle())
        }
        .disabled(acting)
    }

    private var finished: some View {
        VStack(spacing: 16) {
            Image(systemName: "checkmark.seal.fill")
                .font(.system(size: 72, weight: .semibold))
                .foregroundStyle(DocketPalette.review)
                .symbolEffect(.bounce, options: .nonRepeating, value: queue.isEmpty)
            Text(total == 0 ? "Nothing to review" : "All caught up")
                .font(.title.weight(.bold))
                .fontDesign(.rounded)
                .accessibilityIdentifier("review.finished")
            Text(reviewed == 0 ? "No task is asking for a check-in." : reviewed == 1 ? "You reviewed one task." : "You reviewed \(reviewed) tasks.")
                .foregroundStyle(.secondary)
            Button("Done") { dismiss() }
                .buttonStyle(.borderedProminent)
                .accessibilityIdentifier("review.finish")
        }
    }

    // MARK: Actions

    private func load() async {
        let now = Date()
        do {
            let result = try await workspace.engine.tasks(mode: "all")
            queue = result.items.map(\.task).filter { ReviewPass.needsAttention($0, now: now) }
            total = queue.count
        } catch {
            failure = WorkspaceModel.describe(error)
        }
        loading = false
    }

    private func advance() {
        guard !queue.isEmpty else { return }
        withAnimation(reduceMotion ? nil : .spring(response: 0.4, dampingFraction: 0.85)) {
            _ = queue.removeFirst()
        }
        if queue.isEmpty, reviewed > 0 { workspace.celebrate() }
    }

    private func acknowledge(_ task: TaskSnapshot) {
        acting = true
        failure = nil
        Task {
            if let refusal = await workspace.checkIn([task.locator], .acknowledge) {
                failure = refusal
            } else {
                reviewed += 1
                workspace.signal(.success)
                advance()
            }
            acting = false
        }
    }

    private func complete(_ task: TaskSnapshot) {
        acting = true
        failure = nil
        Task {
            switch await workspace.listAction(task.locator, .transition(.done)) {
            case .success:
                reviewed += 1
                advance()
            case .failure(let refusal):
                failure = refusal.message
            }
            acting = false
        }
    }
}

/// One task under review: why it is asking, its title, where it was left,
/// and its check-in rhythm.
struct ReviewCard: View {
    let task: TaskSnapshot
    let projectName: String?

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            Label(reason, systemImage: task.review ? "flag.fill" : "calendar.badge.clock")
                .font(.footnote.weight(.bold))
                .foregroundStyle(DocketPalette.review)
            Text(task.title)
                .font(.title2.weight(.semibold))
                .fontDesign(.rounded)
                .fixedSize(horizontal: false, vertical: true)
                .accessibilityIdentifier("review.title")
            HStack(spacing: 6) {
                Image(systemName: task.state.symbol).foregroundStyle(task.state.tint)
                Text(task.state.rawValue.capitalized)
                if let projectName, !projectName.isEmpty {
                    Text("·")
                    Text(projectName)
                }
            }
            .font(.subheadline)
            .foregroundStyle(.secondary)
            if let note = task.resumeNote, !note.isEmpty {
                VStack(alignment: .leading, spacing: 4) {
                    Text("Where you left off").font(.caption.weight(.bold)).foregroundStyle(DocketPalette.today)
                    Text(note)
                }
                .padding(12)
                .frame(maxWidth: .infinity, alignment: .leading)
                .background(DocketPalette.today.opacity(0.1), in: RoundedRectangle(cornerRadius: 14, style: .continuous))
            }
            if let description = task.description, !description.isEmpty {
                Text(description).foregroundStyle(.secondary).lineLimit(4)
            }
            if let rhythm {
                Label(rhythm, systemImage: "repeat")
                    .font(.footnote)
                    .foregroundStyle(.secondary)
            }
        }
        .padding(22)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background {
            RoundedRectangle(cornerRadius: 26, style: .continuous)
                .fill(Color(uiColor: .secondarySystemGroupedBackground))
                .shadow(color: .black.opacity(0.1), radius: 14, y: 6)
        }
        .accessibilityElement(children: .combine)
        .accessibilityIdentifier("review.card")
    }

    private var reason: String {
        if task.review { return "Flagged for review" }
        if let checkIn = task.nextCheckIn {
            return Calendar.current.isDateInToday(checkIn)
                ? "Check-in due today"
                : "Check-in due since \(checkIn.formatted(.dateTime.month(.abbreviated).day()))"
        }
        return "Needs attention"
    }

    private var rhythm: String? {
        guard let days = task.checkInRepeatDays, days > 0 else { return nil }
        let every = days == 1 ? "every day" : days == 7 ? "every week" : "every \(days) days"
        return "Checks in \(every); reviewing moves it on"
    }
}
