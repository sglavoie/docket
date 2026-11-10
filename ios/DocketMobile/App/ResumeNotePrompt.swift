import SwiftUI
import DocketEngine

/// Pausing work asks one question first: where did you leave off? The answer
/// is the task's resume note, shown under "Where you left off" in its details
/// and read back when the task is resumed. The prompt is prefilled with the
/// note already stored, saving it unchanged writes nothing, and the pause
/// still happens without a note.
struct ResumeNotePrompt: ViewModifier {
    /// The task about to be paused; nil hides the prompt.
    @Binding var task: TaskSnapshot?
    let workspace: WorkspaceModel
    /// Pauses the task, after its note is saved. Receives the refusal of the
    /// note save, if any; the pause is not attempted then.
    let pause: (TaskSnapshot, String?) -> Void
    @State private var note = ""

    func body(content: Content) -> some View {
        content
            .alert("Where did you leave off?", isPresented: Binding(
                get: { task != nil },
                set: { if !$0 { task = nil } }
            ), presenting: task) { task in
                TextField("Next step, open question…", text: $note)
                    .accessibilityIdentifier("resume.note")
                Button("Pause") { save(task) }
                    .accessibilityIdentifier("resume.pause")
                Button("Pause Without Note") { pause(task, nil) }
                    .accessibilityIdentifier("resume.skip")
                Button("Cancel", role: .cancel) {}
            } message: { _ in
                Text("A line for future you. It shows when you come back to this task.")
            }
            .onChange(of: task) { _, task in
                note = task?.resumeNote ?? ""
            }
    }

    private func save(_ task: TaskSnapshot) {
        let trimmed = Self.normalize(note)
        guard trimmed != (task.resumeNote ?? "") else {
            pause(task, nil)
            return
        }
        Task {
            let refusal = await workspace.edit(task.locator, TaskEdit(resumeNote: trimmed))
            pause(task, refusal)
        }
    }

    /// One trimmed line of at most 240 characters: what the engine's gate
    /// accepts, so the prompt never offers a note the save would refuse.
    static func normalize(_ text: String) -> String {
        let line = text.components(separatedBy: .newlines).joined(separator: " ")
            .trimmingCharacters(in: .whitespacesAndNewlines)
        return String(String.UnicodeScalarView(line.unicodeScalars.prefix(240)))
    }
}

extension View {
    func resumeNotePrompt(
        for task: Binding<TaskSnapshot?>, workspace: WorkspaceModel,
        pause: @escaping (TaskSnapshot, String?) -> Void
    ) -> some View {
        modifier(ResumeNotePrompt(task: task, workspace: workspace, pause: pause))
    }
}
