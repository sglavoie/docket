import CryptoKit
import Foundation
import Observation
import TTEngine
import TTReminders
import TTShared
import TTSync
import UIKit

/// Where the app stands with its workspace. Each phase has its own screen, so
/// a slow open, a first launch, a workspace that went missing, and one this
/// build cannot read never look alike.
enum WorkspacePhase: Equatable {
    case loading
    case onboarding
    case ready
    /// The selected workspace is not where it was left.
    case unavailable(String)
    /// The engine refused the workspace.
    case failed(String)
}

/// The app's one open workspace: chosen at onboarding, opened through the
/// engine, and reread after every commit and every import. `generation`
/// moves whenever the workspace may have changed, which is what every list
/// reloads on.
@MainActor
@Observable
final class WorkspaceModel {
    private(set) var phase: WorkspacePhase = .loading
    private(set) var status: WorkspaceStatus?
    private(set) var location: WorkspaceLocation? {
        didSet {
            if oldValue?.runtime != location?.runtime {
                actionReceipt = nil
                undoneReceipt = nil
                if let location {
                    let store = draftStores[location.runtime] ?? DraftStore(location: location)
                    draftStores[location.runtime] = store
                    drafts = store
                } else {
                    drafts = nil
                }
            }
        }
    }
    // A workspace switch must not drop recovery text whose disk write failed.
    @ObservationIgnored private var draftStores: [URL: DraftStore] = [:]
    private(set) var generation = 0
    private(set) var actionReceipt: TaskActionReceipt?
    private(set) var undoneReceipt: TaskActionReceipt?
    private(set) var undoing = false
    /// The last mutation refusal, shown until the next one succeeds.
    private(set) var failure: String?
    private(set) var isJoining = false
    private(set) var joinFailure: String?
    private(set) var isDeletingWorkspace = false
    private(set) var workspaceDeletionFailure: String?

    let engine = TTEngine()
    let sync = SyncModel()
    let launch = LaunchConfiguration.current
    let reminders: ReminderModel
    let captures: CaptureInboxModel
    /// What the shell is showing, for the semantic state export.
    let screen = ShellScreen()
    @ObservationIgnored private var exporter: AutomationStateExporter?
    /// A place a widget, a shortcut, or a link asked the app to open, which
    /// the shell shows once a workspace is open.
    var linkRequest: DeepLink?
    /// The open in flight, which a notification action launching the app
    /// in the background waits for rather than opening a second time.
    @ObservationIgnored private var starting: Task<Void, Never>?

    /// The engine serves one workspace per process, so a hosted test that
    /// opens its own must not race the app for it.
    private static var isHostingTests: Bool {
        ProcessInfo.processInfo.environment["XCTestConfigurationFilePath"] != nil
    }

    init() {
        reminders = ReminderModel(root: LaunchConfiguration.current.root)
        captures = CaptureInboxModel(
            container: LaunchConfiguration.current.sharedContainer,
            holdImports: LaunchConfiguration.current.holdCaptureImports
        )
        sync.onImport = { [weak self] in self?.changed(.incomingChanges) }
        if let destination = LaunchConfiguration.current.stateExport {
            exporter = AutomationStateExporter(destination: destination, workspace: self)
            exporter?.start()
        }
    }

    /// Marks the workspace as possibly changed: every list rereads, and the
    /// pending reminders are reconciled with what changed.
    private func changed(_ reason: ReconcileReason) {
        generation += 1
        reminders.reconcile(reason)
        captures.publishSnapshot(engine: engine)
    }

    /// Imports what the share extension and shortcuts saved. Each capture
    /// lands exactly once, however often this runs or was interrupted.
    func importCaptures() async {
        guard phase == .ready else {
            captures.refreshPending()
            return
        }
        if await captures.importPending(engine: engine) {
            changed(.localChange)
            sync.exchange()
        }
    }

    /// Opens what a `tt://` link names. A task link minted for another
    /// workspace is refused rather than guessed at.
    func open(url: URL) {
        guard let link = DeepLink(url: url) else {
            failure = "tt cannot open \(url.absoluteString)."
            return
        }
        linkRequest = link
    }

    /// Whether a task link belongs to the open workspace.
    func accepts(_ link: DeepLink) -> Bool {
        if case .task(let workspaceID, _, _) = link {
            return status?.workspace == workspaceID
        }
        return true
    }

    func refuseLink() {
        failure = "That link belongs to another workspace than the one open on this iPhone."
        linkRequest = nil
    }

    /// Opens the selected workspace unless it is open, waiting for an open
    /// already in flight. Answers whether a workspace is open afterwards.
    func ensureReady() async -> Bool {
        if phase == .ready { return true }
        await start()
        return phase == .ready
    }

    var existingLocations: [WorkspaceLocation] {
        // Disk changes need an observable dependency to refresh the picker.
        _ = generation
        return launch.existingLocations()
    }

    /// Called only after confirmation in the workspace picker, where no
    /// workspace is open and no editor can submit another change.
    func deleteWorkspace(_ target: WorkspaceLocation) async {
        guard phase == .onboarding, !isJoining, !isDeletingWorkspace else { return }
        isDeletingWorkspace = true
        workspaceDeletionFailure = nil
        defer { isDeletingWorkspace = false }
        await sync.waitForExchanges()
        do {
            // Closing serializes behind pending engine calls and refuses any
            // late editor/import request before the directories disappear.
            if await engine.workspaceID != nil { try await engine.close() }
            await captures.settle()
            captures.clearSnapshot()
            let launch = launch
            try await Task.detached(priority: .userInitiated) {
                try launch.delete(target)
            }.value
            draftStores[target.runtime] = nil
            generation += 1
        } catch {
            workspaceDeletionFailure = "Could not delete “\(target.name)”: \(Self.describe(error))"
        }
    }

    func start() async {
        guard !isDeletingWorkspace else { return }
        if let starting {
            await starting.value
            return
        }
        let task = Task { await self.performStart() }
        starting = task
        await task.value
        starting = nil
    }

    private func performStart() async {
        guard !Self.isHostingTests else { return }
        phase = .loading
        if let gate = launch.openGate {
            while FileManager.default.fileExists(atPath: gate.path) {
                do { try await Task.sleep(for: .milliseconds(100)) }
                catch { return }
            }
        }
        if launch.openDelay > .zero {
            try? await Task.sleep(for: launch.openDelay)
        }
        guard let selected = launch.selectedLocation() else {
            if let seed = launch.reminderSeed {
                await seedReminder(seed)
                return
            }
            phase = .onboarding
            return
        }
        await open(selected, create: false)
    }

    /// Opens a workspace on this device. Without `create`, a missing
    /// workspace is reported unavailable rather than silently recreated empty.
    func open(_ location: WorkspaceLocation, create: Bool) async {
        guard !isDeletingWorkspace else { return }
        phase = .loading
        // A pre-v24 workspace still keeps .tt/ until the engine migrates it.
        let marked = [".docket", ".tt"].contains { FileManager.default.fileExists(atPath: location.data.appendingPathComponent($0).path) }
        if !create, !marked {
            phase = .unavailable("The workspace “\(location.name)” is no longer on this iPhone.")
            return
        }
        do {
            try FileManager.default.createDirectory(at: location.runtime, withIntermediateDirectories: true)
            let device = try deviceID(for: location, creating: create)
            let opened = try await engine.open(
                dataDirectory: location.data, runtimeDirectory: location.runtime,
                deviceID: device, createIfMissing: create
            )
            status = opened
            try launch.select(location)
            self.location = location
            failure = nil
            sync.attach(location: location, workspaceID: opened.workspace, exchangeOverride: launch.exchangeOverride)
            sync.exchange()
            generation += 1
            phase = .ready
            reminders.attach(engine: engine, commands: WorkspaceReminderCommands(workspace: self))
            await resumeImports()
            await importCaptures()
            captures.publishSnapshot(engine: engine)
        } catch {
            status = nil
            phase = .failed(Self.describe(error))
        }
    }

    /// UI-test seam: a disposable workspace holding one task whose reminder
    /// fires shortly, with reminders turned on, so a device test can watch a
    /// real notification arrive and act on it. Production never takes it.
    private func seedReminder(_ seed: LaunchConfiguration.ReminderSeed) async {
        await createWorkspace()
        guard phase == .ready else { return }
        do {
            let locator = try await engine.capture(title: seed.title).task.locator
            var settings = ReminderSettings(enabled: true, checkInHour: 9, checkInMinute: 0)
            let fire = Date().addingTimeInterval(seed.delay)
            switch seed.kind {
            case .checkIn:
                // A check-in is a day announced at a whole minute: the first
                // whole minute after the delay, today.
                let next = Calendar.current.dateComponents([.year, .month, .day, .hour, .minute], from: fire.addingTimeInterval(59))
                settings.checkInHour = next.hour ?? 9
                settings.checkInMinute = next.minute ?? 0
                let day = String(format: "%04d-%02d-%02d", next.year ?? 0, next.month ?? 0, next.day ?? 0)
                _ = try await engine.checkIn([locator], .set(date: day, repeatDays: 0))
            case .deferral:
                _ = try await engine.edit(locator, TaskEdit(deferUntil: ISO8601DateFormatter().string(from: fire)))
            }
            changed(.localChange)
            await reminders.update(settings)
        } catch {
            failure = Self.describe(error)
        }
    }

    /// Starts a new, empty workspace kept on this iPhone.
    func createWorkspace() async {
        await open(launch.location(named: launch.unusedName("local")), create: true)
    }

    /// Joins the workspace a desktop enrolled in the exchange `folder`: the
    /// transport downloads its history and the engine builds the working copy
    /// from it, then the app opens it like any other.
    func join(folder: URL?) async {
        guard !isDeletingWorkspace else { return }
        let access: SyncModel.FolderChoice
        if let folder {
            access = .picked(folder)
        } else if let override = launch.exchangeOverride {
            access = .direct(override)
        } else {
            joinFailure = "Choose the exchange folder the Mac enrolled the workspace in."
            return
        }
        isJoining = true
        joinFailure = nil
        let location = launch.location(named: launch.unusedName("joined"))
        do {
            let device = try deviceID(for: location, creating: true)
            try await sync.join(choice: access, location: location, deviceID: device)
            isJoining = false
            await open(location, create: false)
        } catch {
            isJoining = false
            joinFailure = Self.describe(error)
        }
    }

    /// Returns to onboarding without touching any workspace's files.
    func chooseAnotherWorkspace() {
        try? launch.select(nil)
        sync.detach()
        reminders.detach()
        captures.clearSnapshot()
        location = nil
        status = nil
        workspaceDeletionFailure = nil
        phase = .onboarding
    }

    func retry() async {
        await start()
    }

    /// This workspace's unsaved drafts, or nil before one is open.
    private(set) var drafts: DraftStore?

    /// Captures shorthand text. Returns the refusal in a person's words, or
    /// nil once every line committed.
    func capture(text: String, projectID: String?) async -> String? {
        await attempt { _ = try await self.engine.capture(text: text, projectID: projectID) }
    }

    /// Captures one structured task.
    func capture(_ fields: CaptureFields) async -> String? {
        await attempt { _ = try await self.engine.capture(fields) }
    }

    func previewCapture(text: String, projectID: String?) async -> CapturePreview? {
        try? await engine.previewCapture(text: text, projectID: projectID)
    }

    func saveCaptureDraft(_ draft: CaptureDraft) async -> EditorFailure? {
        let error = await attemptError {
            try await waitForSimulatorEditorSave()
            switch draft.mode {
            case .quick:
                _ = try await self.engine.capture(text: draft.text, projectID: nil)
            case .form:
                _ = try await self.engine.capture(draft.fields.capture(projectID: draft.projectID, destination: draft.destination))
            }
        }
        return error.map(EditorFailure.init)
    }

    func saveEditDraft(_ locator: TaskLocator, _ draft: EditDraft) async -> EditorFailure? {
        let error = await attemptError {
            try await waitForSimulatorEditorSave()
            let result = try await self.engine.edit(locator, draft.fields.edit(from: draft.original))
            self.recordReceipt("Saved changes to “\(result.task.title)”", result.committed)
        }
        return error.map(EditorFailure.init)
    }

    func edit(_ locator: TaskLocator, _ edit: TaskEdit) async -> String? {
        await attempt {
            let result = try await self.engine.edit(locator, edit)
            self.recordReceipt("Saved changes to “\(result.task.title)”", result.committed)
        }
    }

    func plan(_ locator: TaskLocator, _ action: PlanAction) async -> String? {
        switch await listAction(locator, .plan(action)) {
        case .success: nil
        case .failure(let failure): failure.message
        }
    }

    func transition(_ locator: TaskLocator, to state: WorkflowState) async -> String? {
        switch await listAction(locator, .transition(state)) {
        case .success: nil
        case .failure(let failure): failure.message
        }
    }

    func listAction(_ locator: TaskLocator, _ action: TaskListAction) async -> Result<TaskActionReceipt, TaskActionFailure> {
        actionReceipt = nil
        do {
            let result: TaskMutation
            switch action {
            case .transition(let state): result = try await engine.transition(locator, to: state)
            case .plan(let plan): result = try await engine.plan(locator, plan)
            }
            changed(.localChange)
            sync.exchange()
            return .success(recordReceipt(action.message(title: result.task.title), result.committed))
        } catch {
            return .failure(TaskActionFailure(message: Self.describe(error)))
        }
    }

    func undoReceipt(_ receipt: TaskActionReceipt) async -> String? {
        guard !undoing else { return "Undo is already in progress." }
        guard let transactionID = receipt.transactionID else { return "This action has no change to undo." }
        undoing = true
        defer { undoing = false }
        return await attempt(clearingReceipt: false) {
            _ = try await self.engine.undo(transactionID: transactionID)
            self.undoneReceipt = receipt
            if self.actionReceipt?.id == receipt.id {
                self.actionReceipt = TaskActionReceipt(message: "Undone: \(receipt.message)", transactionID: nil)
                UIAccessibility.post(notification: .announcement, argument: self.actionReceipt?.message)
            }
        }
    }

    func dismissReceipt(_ id: UUID) {
        if actionReceipt?.id == id { actionReceipt = nil }
    }

    @discardableResult
    private func recordReceipt(_ message: String, _ committed: CommittedEffect, moves: [AddressMove] = []) -> TaskActionReceipt {
        let receipt = TaskActionReceipt(message: committed.transactionID.isEmpty ? "No changes were needed." : message,
                                        transactionID: committed.transactionID.isEmpty ? nil : committed.transactionID,
                                        moves: moves)
        actionReceipt = receipt
        UIAccessibility.post(notification: .announcement, argument: receipt.message)
        return receipt
    }

    func checklist(_ locator: TaskLocator, _ action: ChecklistAction, seen: [ChecklistItem]) async -> String? {
        await attempt {
            let result = try await self.engine.checklist(locator, action, seen: seen)
            self.recordReceipt("Updated checklist for “\(result.task.title)”", result.committed)
        }
    }

    func checkIn(_ locators: [TaskLocator], _ action: CheckInAction) async -> String? {
        await attempt {
            let result = try await self.engine.checkIn(locators, action)
            let verb = if case .acknowledge = action { "Reviewed" } else { "Updated check-in for" }
            self.recordReceipt("\(verb) \(result.tasks.count) \(result.tasks.count == 1 ? "task" : "tasks")", result.committed)
        }
    }

    func reorder(_ locator: TaskLocator, in kind: TaskOrder, order: [TaskLocator]) async -> String? {
        await attempt {
            let result = try await self.engine.reorder(locator, in: kind, order: order)
            self.recordReceipt("Updated task order", result.committed)
        }
    }

    func hierarchy(_ locator: TaskLocator, _ action: HierarchyAction) async -> String? {
        await attempt {
            let result = try await self.engine.hierarchy(locator, action)
            self.recordReceipt("Updated outline for “\(result.task.title)”", result.committed)
        }
    }

    /// Applies one action to a selection. A move reports where each task
    /// went, which a screen showing one of them follows.
    func bulk(_ locators: [TaskLocator], _ action: BulkAction) async -> (refusal: String?, moves: [AddressMove]) {
        var moves: [AddressMove] = []
        let refusal = await attempt {
            let result = try await self.engine.bulk(locators, action)
            moves = result.moves
            let count = result.tasks.count
            let subject = "\(count) \(count == 1 ? "task" : "tasks")"
            let message: String = switch action {
            case .plan(let plan): TaskListAction.plan(plan).message(count: count)
            case .state(let state): TaskListAction.transition(state).message(count: count)
            case .tags: "Updated tags on \(subject)"
            case .move: "Moved \(subject) to another project"
            }
            self.recordReceipt(message, result.committed, moves: moves)
        }
        return (refusal, moves)
    }

    func project(_ projectID: String?, _ action: ProjectAction) async -> String? {
        await attempt { _ = try await self.engine.project(projectID, action) }
    }

    // MARK: Rich content

    func comment(_ owner: CommentOwner, _ action: CommentAction) async -> String? {
        await attempt {
            let result = try await self.engine.comment(owner, action)
            if let task = result.task {
                self.recordReceipt("Updated comments on “\(task.title)”", result.committed)
            }
        }
    }

    /// Runs one note action, returning the note as saved.
    func note(_ action: NoteAction) async -> (refusal: String?, note: NoteRecord?, stale: Bool) {
        var saved: NoteRecord?
        var stale = false
        let refusal = await attempt {
            do {
                saved = try await self.engine.note(action).note
            } catch let failure as BridgeError where failure.code == .staleRevision {
                stale = true
                throw failure
            }
        }
        return (refusal, saved, stale)
    }

    func attachment(_ locator: TaskLocator, _ action: AttachmentAction) async -> String? {
        await attempt {
            let result = try await self.engine.attachment(locator, action)
            self.recordReceipt("Updated attachments on “\(result.task.title)”", result.committed)
        }
    }

    /// This workspace's staged attachment imports, or nil before one is open.
    var importer: AttachmentImporter? { location.map(AttachmentImporter.init(location:)) }

    /// Imports picked files into a task: each is copied into staging while
    /// its grant is valid, then committed. Returns the first refusal.
    func importFiles(_ urls: [URL], to task: TaskLocator) async -> String? {
        guard let importer else { return "No workspace is open." }
        var refusal: String?
        for url in urls {
            do {
                let pending = try await Task.detached { try importer.stage(file: url, for: task) }.value
                if let failure = await commitImport(pending, importer) { refusal = refusal ?? failure }
            } catch {
                refusal = refusal ?? Self.describe(error)
            }
        }
        changed(.localChange)
        return refusal
    }

    /// Imports a photo's bytes into a task through the same staging.
    func importPhoto(_ data: Data, filename: String, to task: TaskLocator) async -> String? {
        guard let importer else { return "No workspace is open." }
        do {
            let pending = try await Task.detached { try importer.stage(data: data, filename: filename, for: task) }.value
            let refusal = await commitImport(pending, importer)
            changed(.localChange)
            return refusal
        } catch {
            return Self.describe(error)
        }
    }

    /// Finishes every import a termination interrupted, and sweeps copies
    /// that were never recorded.
    func resumeImports() async {
        guard let importer, !launch.holdImportsBeforeCommit else { return }
        importer.sweep()
        let pending = importer.pending()
        guard !pending.isEmpty else { return }
        for record in pending {
            if let refusal = await commitImport(record, importer) {
                failure = "An interrupted import of \(record.filename) could not finish: \(refusal)"
            }
        }
        changed(.localChange)
    }

    private func commitImport(_ pending: AttachmentImporter.Pending, _ importer: AttachmentImporter) async -> String? {
        guard !launch.holdImportsBeforeCommit else { return nil }
        do {
            let result = try await importer.commit(pending, engine: engine)
            if result.outcome == "added" {
                recordReceipt("Attached “\(pending.filename)” to “\(result.task.title)”", result.committed)
            }
            sync.exchange()
            return nil
        } catch {
            return Self.describe(error)
        }
    }

    // MARK: Sync conflicts and recovery

    /// Every open sync conflict, read from this iPhone's own history: it
    /// answers with the exchange unreachable.
    func conflicts() async -> Result<[SyncConflict], ConflictFailure> {
        do {
            return .success(try await engine.conflicts(deviceNames: sync.deviceNames))
        } catch {
            return .failure(ConflictFailure(message: Self.describe(error), stale: false))
        }
    }

    /// Resolves a conflict. A conflict that changed since it was read is
    /// refused as stale and nothing is written; the caller re-reads it.
    func resolve(_ conflict: SyncConflict, _ choice: ConflictChoice) async -> Result<ConflictResolution, ConflictFailure> {
        do {
            let resolution = try await engine.resolve(conflict: conflict.id, choice)
            sync.noteConflicts(resolution.remaining)
            changed(.localChange)
            sync.exchange()
            return .success(resolution)
        } catch let failure as BridgeError where failure.code == .staleRevision || failure.code == .notFound {
            // An edit observed by the next pass changes the conflict; ask for
            // that pass so the re-read sees it.
            sync.exchange()
            return .failure(ConflictFailure(message: failure.message, stale: true))
        } catch {
            return .failure(ConflictFailure(message: Self.describe(error), stale: false))
        }
    }

    /// Writes a diagnostics document — the engine's status, outbox, and
    /// conflict structure, and the transport's queues and last error — under
    /// this workspace's device directory and returns it for sharing. It names
    /// files and devices but holds no task or note content.
    func exportDiagnostics() async -> Result<URL, ConflictFailure> {
        guard let location else { return .failure(ConflictFailure(message: "No workspace is open.", stale: false)) }
        var document = DiagnosticsDocument(
            exportedAt: Date(),
            app: Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? "unknown",
            appExecutables: DiagnosticsDocument.appExecutableDigests(),
            workspace: location.name,
            transport: sync.diagnostics()
        )
        do {
            document.engine = try await engine.syncDiagnostics(deviceNames: sync.deviceNames)
        } catch {
            document.engineError = Self.describe(error)
        }
        do {
            let directory = location.runtime.appendingPathComponent("diagnostics", isDirectory: true)
            try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
            let stamp = ISO8601DateFormatter().string(from: document.exportedAt).replacingOccurrences(of: ":", with: "")
            let file = directory.appendingPathComponent("tt-sync-diagnostics-\(stamp).json")
            let encoder = JSONEncoder()
            encoder.outputFormatting = [.prettyPrinted, .sortedKeys, .withoutEscapingSlashes]
            encoder.dateEncodingStrategy = .iso8601
            try encoder.encode(document).write(to: file, options: .atomic)
            return .success(file)
        } catch {
            return .failure(ConflictFailure(message: "The diagnostics could not be written: \(error.localizedDescription)", stale: false))
        }
    }

    /// Runs one mutation for a screen that shows its own refusal, so the
    /// refusal stays beside the draft it refused rather than in the banner.
    private func attempt(clearingReceipt: Bool = true, _ mutation: @escaping () async throws -> Void) async -> String? {
        await attemptError(clearingReceipt: clearingReceipt, mutation).map(Self.describe)
    }

    private func attemptError(clearingReceipt: Bool = true, _ mutation: @escaping () async throws -> Void) async -> Error? {
        if clearingReceipt { actionReceipt = nil }
        do {
            try await mutation()
            changed(.localChange)
            sync.exchange()
            return nil
        } catch {
            return error
        }
    }

    func complete(_ task: TaskSnapshot) async {
        _ = await perform { _ = try await self.engine.transition(task.locator, to: .done) }
    }

    private func perform(_ mutation: @escaping () async throws -> Void) async -> Bool {
        do {
            try await mutation()
            failure = nil
            changed(.localChange)
            sync.exchange()
            return true
        } catch {
            failure = Self.describe(error)
            return false
        }
    }

    /// Exchanges, then rereads what an import may have changed.
    func refresh() async {
        sync.exchange()
        await importCaptures()
        changed(.foreground)
    }

    static func describe(_ error: Error) -> String {
        if let failure = error as? BridgeError {
            switch failure.code {
            case .schemaTooNew: return "This workspace was written by a newer tt. Update tt on this iPhone to open it. (\(failure.message))"
            case .migrationRequired: return "This workspace needs a migration only the desktop runs. (\(failure.message))"
            case .validation: return failure.explanation
            default: return failure.message
            }
        }
        return String(describing: error)
    }

    /// Existing copies retain their author; each new copy gets a fresh one.
    private func deviceID(for location: WorkspaceLocation, creating: Bool) throws -> String {
        if let override = launch.deviceIDOverride {
            return override
        }
        return try WorkingCopyIdentity.resolve(
            dataDirectory: location.data, runtimeDirectory: location.runtime,
            legacyDeviceID: UserDefaults.standard.string(forKey: "tt.device-id"), creating: creating
        )
    }
}

/// A conflict read or resolution that did not go through. `stale` means the
/// conflict changed since it was read: nothing was written, and re-reading
/// shows it as it is now.
struct ConflictFailure: Error, Equatable {
    let message: String
    let stale: Bool
}

/// One diagnostics export: what the app, the transport, and the engine knew.
struct DiagnosticsDocument: Encodable {
    var format = "tt-mobile-sync-diagnostics/v1"
    var exportedAt: Date
    var app: String
    /// The running build's executables by name, so a device qualification
    /// can tell which build wrote this export.
    var appExecutables: [String: String]
    var workspace: String
    var transport: TransportDiagnostics
    var engine: JSONValue?
    var engineError: String?

    enum CodingKeys: String, CodingKey {
        case format, app, workspace, transport, engine
        case exportedAt = "exported_at"
        case appExecutables = "app_executables"
        case engineError = "engine_error"
    }

    static func appExecutableDigests() -> [String: String] {
        let bundle = Bundle.main.bundleURL
        var out: [String: String] = [:]
        guard let name = Bundle.main.executableURL?.lastPathComponent else { return out }
        for rel in [name, "\(name).debug.dylib"] {
            if let data = try? Data(contentsOf: bundle.appendingPathComponent(rel)) {
                out[rel] = "sha256:" + SHA256.hash(data: data).map { String(format: "%02x", $0) }.joined()
            }
        }
        return out
    }
}
