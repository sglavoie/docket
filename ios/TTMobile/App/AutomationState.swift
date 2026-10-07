import Foundation
import Observation
import SwiftUI
import TTEngine

/// What the shell is showing, as the views report it: which tab, which sheet,
/// and the appearance and text size SwiftUI actually laid them out with. Only
/// the semantic state export reads it.
@MainActor
@Observable
final class ShellScreen {
    var tab = "today"
    /// `capture`, `sync`, `task`, or nil while no sheet is up.
    var sheet: String?
    var colorScheme = "light"
    var dynamicType = "large"
    var voiceOver = false
}

/// The app's semantic state for automation: a JSON document an agent or a
/// script reads instead of a screenshot. It is written only when the launch
/// names `TT_UITEST_STATE_EXPORT`, rewritten whole and atomically a moment
/// after anything it reports changes, and never read back by the app. Drafts
/// and the lists are not observable, so they are also checked once a second;
/// a document that has not changed is not rewritten.
///
/// Schema `tt-mobile-state/v1`:
///   phase, workspace {name, identity, catalog_schema}, generation,
///   screen {tab, sheet}, environment {color_scheme, dynamic_type,
///   voice_over}, failure, sync {has_folder, exchanging, condition,
///   uploads, downloads, conflicts, last_error, last_exchange},
///   pending_captures, drafts (file names), lists {today, inbox}, written_at.
/// `lists` is the engine's widget snapshot, so it names the rows Today and
/// Inbox draw; it is absent until a workspace is open.
@MainActor
final class AutomationStateExporter {
    private let destination: URL
    private unowned let workspace: WorkspaceModel
    private var pending: Task<Void, Never>?
    private var sequence = 0
    private var lastWritten: Data?
    private var lists: Any?
    private var listsGeneration = -1

    init(destination: URL, workspace: WorkspaceModel) {
        self.destination = destination
        self.workspace = workspace
    }

    /// Writes now and again after every observed change.
    func start() {
        observe()
        Task { [weak self] in
            while !Task.isCancelled {
                try? await Task.sleep(for: .seconds(1))
                guard let self else { return }
                self.schedule()
            }
        }
    }

    private func observe() {
        withObservationTracking {
            _ = observedState()
        } onChange: { [weak self] in
            Task { @MainActor in
                self?.schedule()
                self?.observe()
            }
        }
        schedule()
    }

    private func schedule() {
        sequence += 1
        let request = sequence
        let previous = pending
        pending = Task {
            await previous?.value
            try? await Task.sleep(for: .milliseconds(250))
            guard request == sequence else { return }
            await write()
        }
    }

    /// Every observable value the document reports, so a change to any one
    /// of them rewrites it.
    private func observedState() -> [String: Any] {
        let sync = workspace.sync
        let screen = workspace.screen
        var state: [String: Any] = [
            "phase": Self.describe(workspace.phase),
            "generation": workspace.generation,
            "screen": ["tab": screen.tab, "sheet": nullable(screen.sheet)],
            "environment": [
                "color_scheme": screen.colorScheme,
                "dynamic_type": screen.dynamicType,
                "voice_over": screen.voiceOver,
            ],
            "failure": nullable(workspace.failure),
            "sync": [
                "has_folder": sync.hasFolder,
                "exchanging": sync.isExchanging,
                "condition": nullable(sync.condition.map { String(describing: $0) }),
                "uploads": sync.outstandingUploads,
                "downloads": sync.outstandingDownloads,
                "conflicts": sync.conflicts,
                "last_error": nullable(sync.lastError),
                "last_exchange": nullable(sync.lastSuccessfulExchange.map { $0.ISO8601Format() }),
            ] as [String: Any],
            "pending_captures": workspace.captures.pending.count,
        ]
        if let location = workspace.location {
            var described: [String: Any] = ["name": location.name]
            if let status = workspace.status {
                described["identity"] = status.workspace
                described["catalog_schema"] = status.schemas.catalog
            }
            state["workspace"] = described
        }
        return state
    }

    private func write() async {
        var document = observedState()
        if let drafts = workspace.drafts {
            let names = (try? FileManager.default.contentsOfDirectory(atPath: drafts.directory.path)) ?? []
            document["drafts"] = names.filter { $0.hasSuffix(".json") }.sorted()
        } else {
            document["drafts"] = [String]()
        }
        if workspace.phase == .ready {
            // The lists change only with the workspace, so the engine is
            // asked again only when its generation moved.
            let generation = workspace.generation
            if lists == nil || listsGeneration != generation,
               let value = try? await workspace.engine.widgetSnapshot(limit: 20),
               let data = try? JSONEncoder().encode(value),
               let object = try? JSONSerialization.jsonObject(with: data) {
                lists = object
                listsGeneration = generation
            }
            document["lists"] = lists
        } else {
            lists = nil
        }
        document["schema"] = "tt-mobile-state/v1"
        // The snapshot's own clock moves on every read; leave it out of the
        // comparison so an unchanged state is not rewritten.
        let options: JSONSerialization.WritingOptions = [.prettyPrinted, .sortedKeys, .withoutEscapingSlashes]
        var compared = document
        if var lists = compared["lists"] as? [String: Any] {
            lists["generated_at"] = nil
            compared["lists"] = lists
        }
        let fingerprint = try? JSONSerialization.data(withJSONObject: compared, options: options)
        guard fingerprint != lastWritten || fingerprint == nil else { return }
        lastWritten = fingerprint
        document["written_at"] = Date().ISO8601Format()
        guard let data = try? JSONSerialization.data(withJSONObject: document, options: options) else { return }
        try? FileManager.default.createDirectory(at: destination.deletingLastPathComponent(), withIntermediateDirectories: true)
        try? data.write(to: destination, options: .atomic)
    }

    private func nullable(_ value: Any?) -> Any { value ?? NSNull() }

    private static func describe(_ phase: WorkspacePhase) -> String {
        switch phase {
        case .loading: "loading"
        case .onboarding: "onboarding"
        case .ready: "ready"
        case .unavailable: "unavailable"
        case .failed: "failed"
        }
    }
}

/// Reports the environment SwiftUI laid the shell out with, and applies a
/// launch's forced appearance.
struct ShellEnvironmentReporter: ViewModifier {
    let screen: ShellScreen
    let appearance: LaunchConfiguration.Appearance?

    func body(content: Content) -> some View {
        content
            .preferredColorScheme(appearance.map { $0 == .dark ? .dark : .light })
            .background {
                // Read below the override, so the report is what was drawn.
                EnvironmentProbe(screen: screen)
            }
    }
}

private struct EnvironmentProbe: View {
    let screen: ShellScreen
    @Environment(\.colorScheme) private var colorScheme
    @Environment(\.dynamicTypeSize) private var dynamicTypeSize
    @Environment(\.accessibilityVoiceOverEnabled) private var voiceOver

    var body: some View {
        Color.clear
            .accessibilityHidden(true)
            .onAppear(perform: report)
            .onChange(of: colorScheme) { report() }
            .onChange(of: dynamicTypeSize) { report() }
            .onChange(of: voiceOver) { report() }
    }

    private func report() {
        screen.colorScheme = colorScheme == .dark ? "dark" : "light"
        screen.dynamicType = Self.name(dynamicTypeSize)
        screen.voiceOver = voiceOver
    }

    private static func name(_ size: DynamicTypeSize) -> String {
        switch size {
        case .xSmall: "xSmall"
        case .small: "small"
        case .medium: "medium"
        case .large: "large"
        case .xLarge: "xLarge"
        case .xxLarge: "xxLarge"
        case .xxxLarge: "xxxLarge"
        case .accessibility1: "accessibility1"
        case .accessibility2: "accessibility2"
        case .accessibility3: "accessibility3"
        case .accessibility4: "accessibility4"
        case .accessibility5: "accessibility5"
        @unknown default: "unknown"
        }
    }
}
