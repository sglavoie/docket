import SwiftUI
import TTShared
import UIKit

/// The share sheet's tt entry. It reads what was shared — words, links, and
/// files — proposes a task, and saves it to the durable capture inbox in the
/// App Group container. It never opens the workspace: the app's engine adds
/// the capture to the Inbox when tt next opens, and until then the capture is
/// shown there as waiting.
final class ShareViewController: UIViewController {
    /// One identity for everything this share does, so saving again after a
    /// failure replaces its own unfinished copy instead of adding another.
    private let captureID = UUID()

    override func viewDidLoad() {
        super.viewDidLoad()
        let providers = (extensionContext?.inputItems as? [NSExtensionItem] ?? [])
            .flatMap { $0.attachments ?? [] }
        let model = ShareModel(captureID: captureID, providers: providers) { [weak self] in
            self?.extensionContext?.completeRequest(returningItems: nil)
        } cancel: { [weak self] in
            self?.extensionContext?.cancelRequest(withError: NSError(domain: NSCocoaErrorDomain, code: NSUserCancelledError))
        }
        let host = UIHostingController(rootView: ShareView(model: model))
        addChild(host)
        host.view.frame = view.bounds
        host.view.autoresizingMask = [.flexibleWidth, .flexibleHeight]
        view.addSubview(host.view)
        host.didMove(toParent: self)
    }
}

@MainActor
@Observable
final class ShareModel {
    enum Phase: Equatable {
        case loading
        case editing
        case saved
        case failed(String)
    }

    var phase: Phase = .loading
    var title = ""
    var note = ""
    private(set) var items = SharedItems()

    private let captureID: UUID
    private let providers: [NSItemProvider]
    private let finish: () -> Void
    let cancel: () -> Void
    private let container = SharedContainer.root()

    init(captureID: UUID, providers: [NSItemProvider], finish: @escaping () -> Void, cancel: @escaping () -> Void) {
        self.captureID = captureID
        self.providers = providers
        self.finish = finish
        self.cancel = cancel
    }

    func load() async {
        guard let container else {
            phase = .failed("tt’s shared storage is not available, so nothing can be saved from here.")
            return
        }
        let scratch = container.appendingPathComponent("share-scratch/\(captureID.uuidString.lowercased())", isDirectory: true)
        items = await SharedItems.load(providers, scratch: scratch)
        let draft = ShareDraft(proposing: items)
        title = draft.title
        note = draft.note
        phase = items.isEmpty ? .failed("tt cannot read what was shared.") : .editing
    }

    var canSave: Bool {
        !title.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty || !items.urls.isEmpty || !items.files.isEmpty
    }

    func save() {
        guard let container else { return }
        let workspace = WidgetSnapshotStore(container: container).read()?.workspace
        do {
            try ShareDraft(title: title, note: note).save(items, into: CaptureInbox(container: container), id: captureID, workspace: workspace)
            try? FileManager.default.removeItem(at: container.appendingPathComponent("share-scratch/\(captureID.uuidString.lowercased())"))
            phase = .saved
            Task {
                try? await Task.sleep(for: .milliseconds(900))
                finish()
            }
        } catch {
            phase = .failed("The capture could not be saved: \(error.localizedDescription). Try again.")
        }
    }
}

struct ShareView: View {
    @Bindable var model: ShareModel

    var body: some View {
        NavigationStack {
            Form {
                switch model.phase {
                case .loading:
                    ProgressView("Reading what was shared…")
                case .saved:
                    Label("Saved. tt will add it to your Inbox the next time it opens.", systemImage: "checkmark.circle")
                        .accessibilityIdentifier("share.saved")
                case .failed(let message):
                    Text(message).foregroundStyle(.red)
                    if model.canSave { fields }
                case .editing:
                    fields
                }
            }
            .navigationTitle("Save to tt")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { model.cancel() }
                }
                ToolbarItem(placement: .confirmationAction) {
                    Button("Save") { model.save() }
                        .disabled(!model.canSave || model.phase == .saved || model.phase == .loading)
                        .accessibilityIdentifier("share.save")
                }
            }
        }
        .task { await model.load() }
    }

    @ViewBuilder
    private var fields: some View {
        Section {
            TextField("Title", text: $model.title, axis: .vertical)
                .accessibilityIdentifier("share.title")
            TextField("Note", text: $model.note, axis: .vertical)
        }
        if !model.items.urls.isEmpty || !model.items.files.isEmpty {
            Section("Attached") {
                ForEach(model.items.urls, id: \.self) { url in
                    Label(url.absoluteString, systemImage: "link").lineLimit(1)
                }
                ForEach(model.items.files, id: \.self) { file in
                    Label(file.filename, systemImage: "doc").lineLimit(1)
                }
            }
        }
        Section {
            Text("Goes to your Inbox. It is saved on this iPhone now and becomes a task when tt opens.")
                .font(.footnote)
                .foregroundStyle(.secondary)
        }
    }
}
