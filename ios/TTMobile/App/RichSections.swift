import SwiftUI
import TTEngine

/// A task's or a project's comments. Each change names the comment as this
/// screen drew it — its index or id, and the text read there — so a comment
/// another device edited in the meantime refuses the change instead of
/// receiving it.
struct CommentsSection: View {
    let comments: [CommentRecord]
    /// Opens the editor: a new comment, or an edit of one.
    let compose: (CommentRecord?) -> Void
    let delete: (CommentRecord) -> Void

    @State private var actingOn: CommentRecord?

    var body: some View {
        Section("Comments") {
            ForEach(comments) { comment in
                HStack(alignment: .top) {
                    VStack(alignment: .leading, spacing: 4) {
                        Text(comment.text)
                            .accessibilityIdentifier("comment.\(comment.index)")
                        Text(comment.createdAt.formatted(date: .abbreviated, time: .shortened))
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                    Spacer()
                    Button {
                        actingOn = comment
                    } label: {
                        Image(systemName: "ellipsis.circle")
                    }
                    .buttonStyle(.borderless)
                    .accessibilityLabel("Comment actions")
                    .accessibilityIdentifier("comment.\(comment.index).actions")
                }
            }
            Button("Add Comment") { compose(nil) }
                .accessibilityIdentifier("comments.add")
        }
        .confirmationDialog(
            "Comment", isPresented: Binding(get: { actingOn != nil }, set: { if !$0 { actingOn = nil } }),
            titleVisibility: .hidden, presenting: actingOn
        ) { comment in
            Button("Edit Comment") { afterDialogCloses { compose(comment) } }
            Button("Delete Comment", role: .destructive) { delete(comment) }
        }
    }
}

/// What the comment editor was opened for.
struct CommentRequest: Identifiable {
    let owner: CommentOwner
    let editing: CommentRecord?

    var id: String { "\(ownerKey)-\(editing?.id ?? "new")" }

    var ownerKey: String {
        switch owner {
        case .task(let locator): "\(locator.projectID)-\(locator.taskID)"
        case .project(let id): "project-\(id)"
        }
    }
}

/// Writes or edits one comment. The text is kept as a draft on every change
/// until a save commits or the person cancels, so a refused save, a dismissed
/// sheet, and a killed app all return to it.
struct CommentEditorSheet: View {
    let workspace: WorkspaceModel
    let request: CommentRequest

    @Environment(\.dismiss) private var dismiss
    @State private var text = ""
    @State private var failure: String?
    @State private var saving = false
    @State private var loaded = false

    var body: some View {
        NavigationStack {
            Form {
                if let failure {
                    Text(failure)
                        .foregroundStyle(.red)
                        .accessibilityIdentifier("comment.failure")
                }
                TextEditor(text: $text)
                    .frame(minHeight: 160)
                    .accessibilityIdentifier("comment.text")
            }
            .navigationTitle(request.editing == nil ? "Add Comment" : "Edit Comment")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") {
                        workspace.drafts?.discardComment(request.id)
                        dismiss()
                    }
                    .accessibilityIdentifier("comment.cancel")
                }
                ToolbarItem(placement: .confirmationAction) {
                    Button("Save") { save() }
                        .disabled(saving || text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
                        .accessibilityIdentifier("comment.save")
                }
            }
            .onAppear(perform: restore)
            .onChange(of: text) {
                guard loaded else { return }
                workspace.drafts?.saveComment(
                    CommentDraft(editingIndex: request.editing?.index, editingID: request.editing?.commentID,
                                 original: request.editing?.text, text: text),
                    for: request.id
                )
            }
        }
        .interactiveDismissDisabled(saving)
    }

    private func restore() {
        guard !loaded else { return }
        if let draft = workspace.drafts?.loadComment(request.id), draft.original == request.editing?.text {
            text = draft.text
        } else {
            text = request.editing?.text ?? ""
        }
        loaded = true
    }

    private func save() {
        saving = true
        Task {
            let action: CommentAction = request.editing.map { .edit($0, text: text) } ?? .add(text)
            if let refusal = await workspace.comment(request.owner, action) {
                failure = refusal
                saving = false
                return
            }
            workspace.drafts?.discardComment(request.id)
            dismiss()
        }
    }
}

/// A task's attachments and what this iPhone holds of each: a file on this
/// iPhone opens and shares; one whose bytes are on their way says so and can
/// be downloaded now; a link opens and shares; a code reference is kept whole
/// for the desktop. Imports still being staged or committed show as their own
/// rows.
struct AttachmentsSection: View {
    let detail: TaskDetail
    let sync: SyncModel
    let pending: [AttachmentImporter.Pending]
    let add: () -> Void
    let preview: (URL) -> Void
    let remove: (AttachmentRecord) -> Void

    @State private var actingOn: AttachmentRecord?

    private var attachments: [AttachmentRecord] { detail.attachments ?? [] }

    /// Workspace paths under this task's attachment directory that an
    /// incoming transfer is still waiting on and no attachment here names.
    private var arriving: [String] {
        guard let dir = detail.attachmentDir, !dir.isEmpty else { return [] }
        let known = Set(attachments.compactMap(\.workspacePath))
        return sync.unavailable.filter { $0.hasPrefix(dir + "/") && !known.contains($0) }
    }

    var body: some View {
        Section("Attachments") {
            ForEach(attachments) { attachment in
                row(attachment)
            }
            ForEach(arriving, id: \.self) { path in
                let name = (path as NSString).lastPathComponent
                HStack {
                    Label(name, systemImage: "icloud.and.arrow.down")
                    Spacer()
                    downloadButton(paths: [path], name: name)
                }
                .accessibilityElement(children: .contain)
                .accessibilityIdentifier("attachment.arriving.\(name)")
            }
            ForEach(pending.filter { $0.task == detail.task.locator }, id: \.id) { record in
                Label("Importing \(record.filename)…", systemImage: "arrow.down.doc")
                    .foregroundStyle(.secondary)
                    .accessibilityIdentifier("attachment.importing.\(record.filename)")
            }
            Button("Add Attachment") { add() }
                .accessibilityIdentifier("attachments.add")
        }
        .confirmationDialog(
            actingOn?.name ?? "", isPresented: Binding(get: { actingOn != nil }, set: { if !$0 { actingOn = nil } }),
            titleVisibility: .visible, presenting: actingOn
        ) { attachment in
            Button(attachment.kind == .file ? "Remove Attachment and Its File" : "Remove Attachment", role: .destructive) {
                remove(attachment)
            }
        }
    }

    @ViewBuilder
    private func row(_ attachment: AttachmentRecord) -> some View {
        HStack {
            VStack(alignment: .leading, spacing: 2) {
                primary(attachment)
                Text(status(attachment))
                    .font(.caption)
                    .foregroundStyle(attachment.kind == .file && !attachment.available ? .orange : .secondary)
                    .accessibilityIdentifier("attachment.\(attachment.name).status")
            }
            Spacer()
            if let shared = shareURL(attachment) {
                ShareLink(item: shared) {
                    Image(systemName: "square.and.arrow.up")
                }
                .buttonStyle(.borderless)
                .accessibilityLabel("Share \(attachment.name)")
                .accessibilityIdentifier("attachment.\(attachment.name).share")
            }
            if attachment.kind == .file, !attachment.available {
                downloadButton(paths: attachment.workspacePath.map { [$0] }, name: attachment.name)
            }
            Button {
                actingOn = attachment
            } label: {
                Image(systemName: "ellipsis.circle")
            }
            .buttonStyle(.borderless)
            .accessibilityLabel("Attachment actions")
            .accessibilityIdentifier("attachment.\(attachment.name).actions")
        }
    }

    @ViewBuilder
    private func primary(_ attachment: AttachmentRecord) -> some View {
        switch attachment.kind {
        case .file:
            if attachment.available, let path = attachment.localPath {
                Button {
                    preview(URL(fileURLWithPath: path))
                } label: {
                    Label(attachment.name, systemImage: "doc")
                }
                .accessibilityIdentifier("attachment.\(attachment.name)")
            } else {
                Label(attachment.name, systemImage: "doc.badge.ellipsis")
                    .accessibilityIdentifier("attachment.\(attachment.name)")
            }
        case .url:
            if let url = attachment.url.flatMap(URL.init(string:)) {
                Link(destination: url) {
                    Label(attachment.name, systemImage: "link")
                }
                .accessibilityIdentifier("attachment.\(attachment.name)")
            } else {
                Label(attachment.name, systemImage: "link")
                    .accessibilityIdentifier("attachment.\(attachment.name)")
            }
        case .code:
            Label(attachment.name, systemImage: "chevron.left.forwardslash.chevron.right")
                .accessibilityIdentifier("attachment.\(attachment.name)")
        }
    }

    private func status(_ attachment: AttachmentRecord) -> String {
        switch attachment.kind {
        case .url:
            return "Link"
        case .code:
            let line = attachment.line.map { ":\($0)" } ?? ""
            return "Code reference \(attachment.codePath ?? attachment.name)\(line) · opens on the desktop"
        case .file:
            if attachment.available {
                return "On this iPhone · " + ByteCountFormatter.string(fromByteCount: attachment.size ?? 0, countStyle: .file)
            }
            if let path = attachment.workspacePath, sync.isArriving(path) {
                return "Waiting to download"
            }
            return "Not on this iPhone"
        }
    }

    private func shareURL(_ attachment: AttachmentRecord) -> URL? {
        switch attachment.kind {
        case .file: attachment.available ? attachment.localPath.map { URL(fileURLWithPath: $0) } : nil
        case .url: attachment.url.flatMap(URL.init(string:))
        case .code: nil
        }
    }

    private func downloadButton(paths: [String]?, name: String) -> some View {
        Button("Download") { sync.download(paths: paths) }
            .buttonStyle(.borderless)
            .disabled(!sync.hasFolder)
            .accessibilityIdentifier("attachment.\(name).download")
    }
}
