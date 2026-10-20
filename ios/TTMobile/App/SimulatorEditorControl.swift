import Foundation

/// UI tests may hold a real editor submission before it reaches the engine,
/// then release it after typing a newer draft. No outcome is fabricated.
func waitForSimulatorEditorSave() async throws {
    #if DEBUG && targetEnvironment(simulator)
    let environment = ProcessInfo.processInfo.environment
    guard environment["TT_UITEST_EDITOR_SAVE_GATE"] == "1",
          let root = environment["TT_UITEST_ROOT"], !root.isEmpty else { return }
    let gate = URL(fileURLWithPath: root).appendingPathComponent("editor-save.release")
    for _ in 0..<600 {
        if FileManager.default.fileExists(atPath: gate.path) { return }
        try await Task.sleep(for: .milliseconds(50))
    }
    throw NSError(domain: "tt.simulator.editor", code: 1,
                  userInfo: [NSLocalizedDescriptionKey: "The simulator test did not release the editor save."])
    #endif
}
