import AppKit
import SwiftUI

/// Everything Preferences edits. Each change is saved to `UserDefaults` as it
/// is made; the app delegate applies the live ones through the callbacks.
final class PreferencesModel: ObservableObject {
    let defaults: UserDefaults

    @Published var exitBehavior: TTExitBehavior {
        didSet {
            guard exitBehavior != oldValue else { return }
            exitBehavior.save(to: defaults)
        }
    }

    @Published var activationPolicy: TTActivationPolicy {
        didSet {
            guard activationPolicy != oldValue else { return }
            activationPolicy.save(to: defaults)
            onActivationPolicyChange?(activationPolicy)
        }
    }

    /// nil is turned off.
    @Published var hotKey: HotKeyCombo? {
        didSet {
            guard hotKey != oldValue else { return }
            TTHotKeyPreference.save(hotKey, to: defaults)
            onHotKeyChange?(hotKey)
        }
    }

    /// The zoom tt.app launches at; nil leaves it to the Ghostty config.
    @Published var defaultFontSize: Float? {
        didSet {
            guard defaultFontSize != oldValue else { return }
            TTDefaultFontSize.save(defaultFontSize, to: defaults)
            onDefaultFontSizeChange?(defaultFontSize)
        }
    }

    /// The size the terminal uses while `defaultFontSize` is nil: the Ghostty
    /// config's, set by whoever loads it.
    @Published var configFontSize: Float = 13

    /// Set by whoever registers the shortcut: why it is not working, or what
    /// it collides with.
    @Published var hotKeyProblem: String?

    @Published var binaryOverride: String {
        didSet {
            let trimmed = binaryOverride.trimmingCharacters(in: .whitespacesAndNewlines)
            if trimmed.isEmpty {
                defaults.removeObject(forKey: TTBinaryResolver.overrideDefaultsKey)
            } else {
                defaults.set(trimmed, forKey: TTBinaryResolver.overrideDefaultsKey)
            }
        }
    }

    /// What the login shell reported at launch, set by the app delegate.
    @Published var detectedEnvironment: LaunchEnvironment?

    /// Values typed in Preferences in place of the shell's; blank uses the shell's.
    @Published var environmentOverrides: [String: String] {
        didSet {
            guard environmentOverrides != oldValue else { return }
            TTEnvironmentOverrides.save(environmentOverrides, to: defaults)
            onEnvironmentChange?()
        }
    }

    /// The environment the running tt started with; nil while none runs.
    @Published var sessionEnvironment: [String: String]?

    /// What the next tt starts with.
    var effectiveEnvironment: LaunchEnvironment? {
        detectedEnvironment?.applying(environmentOverrides)
    }

    /// The running tt started with other values than the next one would.
    var environmentAwaitsRelaunch: Bool {
        guard let session = sessionEnvironment, let next = effectiveEnvironment else { return false }
        return next.commandEnvironment != session
    }

    /// The binary this session runs, set by the app delegate once resolved;
    /// its notes are read when Preferences opens, not at launch.
    var runningBinary: TTBinaryResolver.Resolution?
    /// Which tt runs and its version.
    @Published private(set) var runningBinaryNote: String?
    /// A second copy that disagrees with the running one, and what to do.
    @Published private(set) var binarySkewNote: String?
    /// Where the bundled and `~/.local/bin` copies live; the app's own by default.
    var bundledBinaryPath = Bundle.main.resourceURL?.appendingPathComponent("bin/tt").path
    var localBinaryPath = FileManager.default.homeDirectoryForCurrentUser.appendingPathComponent(".local/bin/tt").path
    var readVersion: (String) -> TTVersion? = { TTVersion.read(binary: $0) }

    @Published private(set) var launchAtLogin = false
    @Published private(set) var launchAtLoginNote: String?

    var onActivationPolicyChange: ((TTActivationPolicy) -> Void)?
    var onHotKeyChange: ((HotKeyCombo?) -> Void)?
    var onDefaultFontSizeChange: ((Float?) -> Void)?
    /// True while the recorder listens, so the live shortcut can step aside
    /// and be recorded like any other key.
    var onRecordingChange: ((Bool) -> Void)?
    var onOpenGhosttyConfig: (() -> Void)?
    var onReloadGhosttyConfig: (() -> Void)?
    var onEnvironmentChange: (() -> Void)?
    var onRelaunch: (() -> Void)?

    init(defaults: UserDefaults = .standard) {
        self.defaults = defaults
        exitBehavior = .load(from: defaults)
        activationPolicy = .load(from: defaults)
        hotKey = TTHotKeyPreference.load(from: defaults)
        defaultFontSize = TTDefaultFontSize.load(from: defaults)
        binaryOverride = defaults.string(forKey: TTBinaryResolver.overrideDefaultsKey) ?? ""
        environmentOverrides = TTEnvironmentOverrides.load(from: defaults)
    }

    /// Re-reads what may have changed elsewhere, e.g. the login item in System
    /// Settings or a `defaults write`.
    func refresh() {
        exitBehavior = .load(from: defaults)
        let stored = defaults.string(forKey: TTBinaryResolver.overrideDefaultsKey) ?? ""
        if stored != binaryOverride.trimmingCharacters(in: .whitespacesAndNewlines) {
            binaryOverride = stored
        }
        let storedEnvironment = TTEnvironmentOverrides.load(from: defaults)
        if storedEnvironment != TTEnvironmentOverrides.sanitized(environmentOverrides) {
            environmentOverrides = storedEnvironment
        }
        refreshLoginItem()
        refreshBinaryStatus()
    }

    /// Reads the running binary's version and, when it is one of the bundled
    /// and `~/.local/bin` pair, how the other one compares.
    func refreshBinaryStatus() {
        guard let running = runningBinary else {
            runningBinaryNote = nil
            binarySkewNote = nil
            return
        }
        let path = running.url.path
        let version = readVersion(path)
        runningBinaryNote = "Running \(Self.abbreviated(path)): \(version?.line ?? "version unknown")"

        binarySkewNote = nil
        let other: String?
        switch running.source {
        case .bundled: other = localBinaryPath
        case .newerLocalBin: other = bundledBinaryPath
        default: other = nil
        }
        guard let other, TTBinaryResolver.isExecutableRegularFile(other),
              let otherVersion = readVersion(other), otherVersion.line != version?.line else {
            return
        }
        if running.source == .newerLocalBin {
            binarySkewNote = "Chosen over the older bundled copy (\(otherVersion.line)). "
                + "Reinstall tt.app to update it."
        } else if let built = otherVersion.built, let runningBuilt = version?.built, built < runningBuilt {
            binarySkewNote = "\(Self.abbreviated(other)) is older (\(otherVersion.line)) "
                + "and refuses a workspace this one migrates: `just install` updates it."
        }
    }

    static func abbreviated(_ path: String) -> String {
        let home = NSHomeDirectory()
        return path.hasPrefix(home + "/") ? "~" + path.dropFirst(home.count) : path
    }

    func refreshLoginItem() {
        launchAtLogin = LoginItem.isEnabled
        launchAtLoginNote = LoginItem.note
    }

    /// Returns the failure, which the note also shows.
    @discardableResult
    func setLaunchAtLogin(_ enabled: Bool) -> String? {
        do {
            try LoginItem.setEnabled(enabled)
            refreshLoginItem()
            return nil
        } catch {
            refreshLoginItem()
            let message = "Launch at Login could not be changed: \(error.localizedDescription)"
            launchAtLoginNote = message
            return message
        }
    }

    /// Why the override will not work, when it is set; checked the way the
    /// resolver checks it.
    var binaryOverrideProblem: String? {
        let trimmed = binaryOverride.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty else { return nil }
        let path = (trimmed as NSString).expandingTildeInPath
        return TTBinaryResolver.isExecutableRegularFile(path) ? nil : "Not an executable file."
    }
}

/// The SwiftUI Preferences window.
final class PreferencesWindowController: NSObject, NSWindowDelegate {
    let model: PreferencesModel
    let recorder = HotKeyRecorder()
    private(set) lazy var window: NSWindow = makeWindow()

    init(model: PreferencesModel) {
        self.model = model
        super.init()
    }

    func show() {
        model.refresh()
        NSApp.activate(ignoringOtherApps: true)
        window.makeKeyAndOrderFront(nil)
    }

    private func makeWindow() -> NSWindow {
        let window = NSWindow(contentViewController: NSHostingController(
            rootView: PreferencesView(model: model, recorder: recorder)
        ))
        window.title = "tt Preferences"
        window.styleMask = [.titled, .closable]
        window.isReleasedWhenClosed = false
        window.delegate = self
        window.center()
        return window
    }

    /// A closed window records nothing.
    func windowWillClose(_ notification: Notification) {
        recorder.stop()
    }
}

struct PreferencesView: View {
    @ObservedObject var model: PreferencesModel
    @ObservedObject var recorder: HotKeyRecorder

    var body: some View {
        Form {
            Section {
                LabeledContent("Show and hide tt") {
                    HotKeyRecorderView(model: model, recorder: recorder)
                }
                if let problem = model.hotKeyProblem {
                    note(problem, warning: true)
                }
                note("Works from any app. Pressing it while tt is in front hides tt. "
                    + "A Karabiner rule can send this same shortcut.")
            }

            Section {
                Picker("Show tt in", selection: $model.activationPolicy) {
                    ForEach(TTActivationPolicy.allCases, id: \.self) { Text($0.label).tag($0) }
                }
                Toggle("Launch at Login", isOn: Binding(
                    get: { model.launchAtLogin },
                    set: { model.setLaunchAtLogin($0) }
                ))
                if let loginNote = model.launchAtLoginNote {
                    note(loginNote, warning: true)
                }
            }

            Section {
                LabeledContent("Default zoom") {
                    // Reset leads, so the trailing stepper holds still as it appears.
                    HStack {
                        if model.defaultFontSize != nil {
                            Button("Reset") { model.defaultFontSize = nil }
                        }
                        Stepper(
                            TTDefaultFontSize.label(model.defaultFontSize ?? model.configFontSize),
                            value: Binding(
                                get: { model.defaultFontSize ?? model.configFontSize },
                                set: { model.defaultFontSize = $0.rounded() }
                            ),
                            in: TTDefaultFontSize.range
                        )
                    }
                }
                note((model.defaultFontSize == nil
                    ? "The Ghostty config's font size. "
                    : "Overrides the Ghostty config's font-size. ")
                    + "⌘+ and ⌘− zoom until tt.app quits; each launch and ⌘0 return to this size.")
            }

            Section {
                Picker("When tt exits", selection: $model.exitBehavior) {
                    ForEach(TTExitBehavior.allCases, id: \.self) { Text($0.label).tag($0) }
                }
                note("A crash or a non-zero exit always keeps tt's last output on screen with a Relaunch button.")
            }

            Section {
                LabeledContent("tt binary") {
                    HStack {
                        TextField("", text: $model.binaryOverride, prompt: Text("Automatic"))
                            .labelsHidden()
                            .frame(minWidth: 200)
                        Button("Choose…", action: chooseBinary)
                    }
                }
                if let problem = model.binaryOverrideProblem {
                    note(problem, warning: true)
                }
                if let running = model.runningBinaryNote {
                    note(running)
                }
                if let skew = model.binarySkewNote {
                    note(skew, warning: true)
                }
                note("Leave empty to use the bundled tt, ~/.local/bin/tt, then PATH; "
                    + "a ~/.local/bin/tt built after the bundled one wins. "
                    + "Takes effect the next time tt.app starts.")
            }

            Section {
                ForEach(TTEnvironmentOverrides.names, id: \.self) { name in
                    LabeledContent(name) {
                        let detected = model.detectedEnvironment?.value(of: name)
                        TextField("", text: environmentBinding(name), prompt: Text(detected ?? "not set"))
                            .labelsHidden()
                            .font(.system(.body, design: .monospaced))
                            .frame(minWidth: 260)
                            .help(detected.map { "From your login shell: \($0)" } ?? "Your login shell does not set \(name).")
                    }
                }
                if model.environmentAwaitsRelaunch {
                    HStack {
                        note("The running tt started with the previous values.", warning: true)
                        Spacer()
                        Button("Relaunch tt") { model.onRelaunch?() }
                    }
                }
                note("Gray values are what your login shell sets. Type one to give tt "
                    + "another; clear it to go back. tt reads these each time it starts.")
            }

            Section {
                LabeledContent("Ghostty config") {
                    HStack {
                        Button("Open") { model.onOpenGhosttyConfig?() }
                        Button("Reload") { model.onReloadGhosttyConfig?() }
                    }
                }
                note(Ghostty.Config.userConfigURL().path
                    .replacingOccurrences(of: NSHomeDirectory(), with: "~"))
            }
        }
        .formStyle(.grouped)
        .frame(width: 520)
        .fixedSize(horizontal: false, vertical: true)
    }

    private func note(_ text: String, warning: Bool = false) -> some View {
        Text(text)
            .font(.callout)
            .foregroundStyle(warning ? Color.orange : Color.secondary)
            .fixedSize(horizontal: false, vertical: true)
    }

    /// One variable's override; an empty field removes it.
    private func environmentBinding(_ name: String) -> Binding<String> {
        Binding(
            get: { model.environmentOverrides[name] ?? "" },
            set: { value in
                var overrides = model.environmentOverrides
                overrides[name] = value.isEmpty ? nil : value
                model.environmentOverrides = overrides
            }
        )
    }

    private func chooseBinary() {
        let panel = NSOpenPanel()
        panel.canChooseFiles = true
        panel.canChooseDirectories = false
        panel.allowsMultipleSelection = false
        panel.treatsFilePackagesAsDirectories = true
        panel.message = "Choose the tt binary tt.app runs."
        if panel.runModal() == .OK, let url = panel.url {
            model.binaryOverride = url.path
        }
    }
}

/// A button that records the next shortcut typed. Esc cancels; the shortcut
/// must use Control, Option, or Command, unless it is a function key.
struct HotKeyRecorderView: View {
    @ObservedObject var model: PreferencesModel
    @ObservedObject var recorder: HotKeyRecorder

    var body: some View {
        HStack {
            Button(action: toggleRecording) {
                Text(label)
                    .frame(minWidth: 140)
                    .foregroundStyle(recorder.isRecording ? Color.accentColor : Color.primary)
            }
            if model.hotKey != nil, !recorder.isRecording {
                Button("Clear") { model.hotKey = nil }
            }
        }
        .onDisappear { recorder.stop() }
    }

    private var label: String {
        if recorder.isRecording {
            return recorder.hint ?? "Type a shortcut…"
        }
        return model.hotKey?.displayString ?? "Record Shortcut"
    }

    private func toggleRecording() {
        if recorder.isRecording {
            recorder.stop()
            return
        }
        recorder.start(
            onStateChange: { [weak model] recording in model?.onRecordingChange?(recording) },
            onRecord: { [weak model] combo in model?.hotKey = combo }
        )
    }
}

final class HotKeyRecorder: ObservableObject {
    @Published private(set) var isRecording = false
    @Published private(set) var hint: String?
    private var monitor: Any?
    private var onStateChange: ((Bool) -> Void)?

    func start(onStateChange: @escaping (Bool) -> Void, onRecord: @escaping (HotKeyCombo) -> Void) {
        stop()
        self.onStateChange = onStateChange
        isRecording = true
        hint = nil
        onStateChange(true)
        monitor = NSEvent.addLocalMonitorForEvents(matching: .keyDown) { [weak self] event in
            guard let self else { return event }
            let chord = event.modifierFlags.intersection([.command, .control, .option, .shift])
            if Int(event.keyCode) == HotKeyRecorder.escapeKeyCode, chord.isEmpty {
                self.stop()
            } else if let combo = HotKeyCombo(event: event) {
                self.stop()
                onRecord(combo)
            } else {
                self.hint = "Add ⌃, ⌥, or ⌘…"
            }
            return nil
        }
    }

    func stop() {
        if let monitor {
            NSEvent.removeMonitor(monitor)
            self.monitor = nil
        }
        guard isRecording else { return }
        isRecording = false
        hint = nil
        onStateChange?(false)
        onStateChange = nil
    }

    deinit {
        if let monitor {
            NSEvent.removeMonitor(monitor)
        }
    }

    private static let escapeKeyCode = 0x35
}
