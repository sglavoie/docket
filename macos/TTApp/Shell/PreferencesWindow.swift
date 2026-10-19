import AppKit
import SwiftUI
import UserNotifications

/// What Settings › Terminal says under Reload: that the config was applied,
/// with anything Ghostty found wrong in it, or that it was not.
struct GhosttyConfigReport: Equatable {
    let lines: [String]
    let warning: Bool

    init(applied: Bool, diagnostics: [String]) {
        guard applied else {
            lines = ["Could not reload; the terminal keeps its previous config."]
            warning = true
            return
        }
        guard !diagnostics.isEmpty else {
            lines = ["Reloaded."]
            warning = false
            return
        }
        let noun = diagnostics.count == 1 ? "problem" : "problems"
        lines = ["Reloaded, with \(diagnostics.count) \(noun) in the config:"] + diagnostics.map { "• " + $0 }
        warning = true
    }

    var text: String { lines.joined(separator: "\n") }
}

/// Where System Settings shows tt.app's notification permission.
enum NotificationSettingsLink {
    static func url(bundleIdentifier: String? = Bundle.main.bundleIdentifier) -> URL {
        let base = "x-apple.systempreferences:com.apple.Notifications-Settings.extension"
        guard let bundleIdentifier else { return URL(string: base)! }
        return URL(string: base + "?id=" + bundleIdentifier) ?? URL(string: base)!
    }

    static let deniedNote = "macOS has notifications for tt.app turned off, so none will show. "
        + "Allow them in System Settings › Notifications."
}

/// Everything Settings edits. Each change is saved to `UserDefaults` as it
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

    /// The quick-capture panel's shortcut; nil is turned off.
    @Published var captureHotKey: HotKeyCombo? {
        didSet {
            guard captureHotKey != oldValue else { return }
            TTCaptureHotKeyPreference.save(captureHotKey, to: defaults)
            onCaptureHotKeyChange?(captureHotKey)
        }
    }

    /// tt reports arrivals; turning it off also silences the running one.
    @Published var notificationsEnabled: Bool {
        didSet {
            guard notificationsEnabled != oldValue else { return }
            TTNotificationsPreference.save(notificationsEnabled, to: defaults)
        }
    }

    /// tt's theme follows macOS's light or dark appearance.
    @Published var followAppearance: Bool {
        didSet {
            guard followAppearance != oldValue else { return }
            TTFollowAppearancePreference.save(followAppearance, to: defaults)
            onFollowAppearanceChange?(followAppearance)
        }
    }

    /// The Dock icon carries tt's overdue count.
    @Published var dockBadge: Bool {
        didSet {
            guard dockBadge != oldValue else { return }
            TTDockBadgePreference.save(dockBadge, to: defaults)
            onDockBadgeChange?(dockBadge)
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

    /// A selected Ghostty theme, or nil to use the Ghostty config.
    @Published var terminalTheme: TerminalTheme? {
        didSet {
            guard terminalTheme != oldValue else { return }
            TTTerminalThemePreference.save(terminalTheme, to: defaults)
            onTerminalThemeChange?()
        }
    }

    @Published var availableThemes: [String]

    var themeName: String? {
        get { terminalTheme?.light }
        set { terminalTheme = newValue.flatMap { TerminalTheme($0, dark: terminalTheme?.dark) } }
    }

    var separateDarkTheme: Bool {
        get { terminalTheme?.dark != nil }
        set {
            guard var theme = terminalTheme else { return }
            theme.dark = newValue ? theme.light : nil
            terminalTheme = theme
        }
    }

    var darkThemeName: String {
        get { terminalTheme.map { $0.dark ?? $0.light } ?? "" }
        set {
            guard let light = terminalTheme?.light,
                  let theme = TerminalTheme(light, dark: newValue) else { return }
            terminalTheme = theme
        }
    }

    /// Keep a saved choice visible even if its theme file was removed.
    var themeChoices: [String] {
        var choices = availableThemes
        for name in [terminalTheme?.light, terminalTheme?.dark].compactMap({ $0 }) where !choices.contains(name) {
            choices.append(name)
        }
        return choices
    }

    /// The size the terminal uses while `defaultFontSize` is nil: the Ghostty
    /// config's, set by whoever loads it.
    @Published var configFontSize: Float = 13

    /// Set by whoever registers the shortcut: why it is not working, or what
    /// it collides with.
    @Published var hotKeyProblem: String?
    @Published var captureHotKeyProblem: String?

    @Published var binaryOverride: String {
        didSet {
            let trimmed = binaryOverride.trimmingCharacters(in: .whitespacesAndNewlines)
            if trimmed.isEmpty {
                defaults.removeObject(forKey: TTBinaryResolver.overrideDefaultsKey)
            } else {
                defaults.set(trimmed, forKey: TTBinaryResolver.overrideDefaultsKey)
            }
            refreshBinaryRelaunch()
        }
    }

    /// What the login shell reported at launch, set by the app delegate.
    @Published var detectedEnvironment: LaunchEnvironment?

    /// Values typed in Settings in place of the shell's; blank uses the shell's.
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
    /// its notes are read when Settings opens, not at launch.
    var runningBinary: TTBinaryResolver.Resolution?
    var runningVersion: TTVersion?
    var runningFingerprint: TTBinaryFingerprint?
    /// The tt the next session would run, resolved the way a session resolves
    /// it; nil when none resolves.
    var resolveNextBinary: (_ searchPath: String) -> TTBinaryResolver.Resolution?
    /// The next tt is another binary than the running one.
    @Published private(set) var binaryAwaitsRelaunch = false
    @Published private(set) var nextBinaryNote: String?
    /// Which tt runs and its version.
    @Published private(set) var runningBinaryNote: String?
    /// A second copy that disagrees with the running one, and what to do.
    @Published private(set) var binarySkewNote: String?
    /// Where the bundled and `~/.local/bin` copies live; the app's own by default.
    var bundledBinaryPath = Bundle.main.resourceURL?.appendingPathComponent("bin/tt").path
    var localBinaryPath = FileManager.default.homeDirectoryForCurrentUser.appendingPathComponent(".local/bin/tt").path
    var readVersion: (String) -> TTVersion? = { TTVersion.read(binary: $0) }

    /// What the last Reload in Settings › Terminal found; nil until one.
    @Published var ghosttyConfigReport: GhosttyConfigReport?

    /// macOS refuses tt.app's notifications, read as Settings opens.
    @Published private(set) var notificationsDenied = false
    /// Reads macOS's answer for tt.app's notifications; tests answer it.
    var readNotificationAuthorization: (@escaping (UNAuthorizationStatus) -> Void) -> Void = { done in
        UNUserNotificationCenter.current().getNotificationSettings { settings in
            done(settings.authorizationStatus)
        }
    }

    @Published private(set) var launchAtLogin = false
    @Published private(set) var launchAtLoginNote: String?

    var onActivationPolicyChange: ((TTActivationPolicy) -> Void)?
    var onHotKeyChange: ((HotKeyCombo?) -> Void)?
    var onCaptureHotKeyChange: ((HotKeyCombo?) -> Void)?
    var onDefaultFontSizeChange: ((Float?) -> Void)?
    var onTerminalThemeChange: (() -> Void)?
    var onDockBadgeChange: ((Bool) -> Void)?
    var onFollowAppearanceChange: ((Bool) -> Void)?
    /// True while the recorder listens, so the live shortcuts can step aside
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
        captureHotKey = TTCaptureHotKeyPreference.load(from: defaults)
        defaultFontSize = TTDefaultFontSize.load(from: defaults)
        terminalTheme = TTTerminalThemePreference.load(from: defaults)
        availableThemes = TerminalTheme.available()
        notificationsEnabled = TTNotificationsPreference.load(from: defaults)
        dockBadge = TTDockBadgePreference.load(from: defaults)
        followAppearance = TTFollowAppearancePreference.load(from: defaults)
        binaryOverride = defaults.string(forKey: TTBinaryResolver.overrideDefaultsKey) ?? ""
        environmentOverrides = TTEnvironmentOverrides.load(from: defaults)
        resolveNextBinary = { [defaults] searchPath in
            try? TTBinaryResolver.live(searchPath: searchPath, defaults: defaults).resolve().get()
        }
    }

    /// Re-reads what may have changed elsewhere, e.g. the login item in System
    /// Settings or a `defaults write`.
    func refresh() {
        exitBehavior = .load(from: defaults)
        terminalTheme = TTTerminalThemePreference.load(from: defaults)
        availableThemes = TerminalTheme.available()
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
        refreshNotificationAuthorization()
        ghosttyConfigReport = nil
    }

    /// Asks macOS whether tt.app may notify, off the main thread, and
    /// publishes the answer on it. Not yet asked is not denied.
    func refreshNotificationAuthorization() {
        readNotificationAuthorization { [weak self] status in
            let denied = status == .denied
            let publish: () -> Void = { self?.notificationsDenied = denied }
            if Thread.isMainThread { publish() } else { DispatchQueue.main.async(execute: publish) }
        }
    }

    /// Reads the running binary's version and, when it is one of the bundled
    /// and `~/.local/bin` pair, how the other one compares.
    func refreshBinaryStatus() {
        refreshBinaryRelaunch()
        guard let running = runningBinary else {
            runningBinaryNote = nil
            binarySkewNote = nil
            return
        }
        let path = running.url.path
        let version = runningVersion
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

    /// Asked as the override changes and as Settings opens, never per
    /// redraw: resolving may read two binaries' versions.
    func refreshBinaryRelaunch() {
        let next = runningBinary == nil ? nil : resolveNextBinary(effectiveEnvironment?.path ?? "")
        let nextVersion = next.flatMap { readVersion($0.url.path) }
        let nextFingerprint = next.flatMap { TTBinaryFingerprint.read($0.url.path) }
        let versionChanged = runningVersion != nil && nextVersion != nil && runningVersion != nextVersion
        let fileChanged = runningFingerprint != nil && nextFingerprint != nil && runningFingerprint != nextFingerprint
        let awaits = Self.binaryAwaitsRelaunch(running: runningBinary, next: next) || versionChanged || fileChanged
        nextBinaryNote = awaits ? "Available after relaunch: \(nextVersion?.line ?? "updated executable")" : nil
        if awaits != binaryAwaitsRelaunch {
            binaryAwaitsRelaunch = awaits
        }
    }

    /// A running tt and a resolvable next one at different paths. One that no
    /// longer resolves relaunches nothing: the last tt keeps running.
    static func binaryAwaitsRelaunch(
        running: TTBinaryResolver.Resolution?,
        next: TTBinaryResolver.Resolution?
    ) -> Bool {
        guard let running, let next else { return false }
        return running.url.standardizedFileURL.path != next.url.standardizedFileURL.path
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
            let message = Self.launchAtLoginFailure(error)
            launchAtLoginNote = message
            return message
        }
    }

    /// The error, then where the login item can be changed by hand.
    static func launchAtLoginFailure(_ error: Error) -> String {
        "\(error.localizedDescription)\nSystem Settings › General › Login Items"
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

/// The SwiftUI Settings window, where it was last left.
final class PreferencesWindowController: NSObject, NSWindowDelegate {
    static let frameAutosaveName = "Settings"

    let model: PreferencesModel
    let recorder = HotKeyRecorder()
    let captureRecorder = HotKeyRecorder()
    private let frameAutosaveName: String?
    private(set) lazy var window: NSWindow = makeWindow()

    /// Tests pass no autosave name, so they leave the saved frame alone.
    init(model: PreferencesModel, frameAutosaveName: String? = PreferencesWindowController.frameAutosaveName) {
        self.model = model
        self.frameAutosaveName = frameAutosaveName
        super.init()
    }

    func show() {
        model.refresh()
        NSApp.activate(ignoringOtherApps: true)
        window.makeKeyAndOrderFront(nil)
    }

    private func makeWindow() -> NSWindow {
        let host = NSHostingController(
            rootView: PreferencesView(model: model, recorder: recorder, captureRecorder: captureRecorder)
        )
        // Each tab is its own height; the window follows the one chosen.
        host.sizingOptions = .preferredContentSize
        let window = NSWindow(contentViewController: host)
        window.title = "Settings"
        window.styleMask = [.titled, .closable]
        window.isReleasedWhenClosed = false
        window.delegate = self
        if let frameAutosaveName {
            if !window.setFrameUsingName(frameAutosaveName) {
                window.center()
            }
            window.setFrameAutosaveName(frameAutosaveName)
        } else {
            window.center()
        }
        return window
    }

    /// A closed window records nothing.
    func windowWillClose(_ notification: Notification) {
        recorder.stop()
        captureRecorder.stop()
    }
}

/// The four Settings tabs, in order.
enum SettingsTab: String, CaseIterable {
    case general, shortcuts, terminal, advanced

    var title: String { rawValue.capitalized }

    var systemImage: String {
        switch self {
        case .general: return "gearshape"
        case .shortcuts: return "keyboard"
        case .terminal: return "terminal"
        case .advanced: return "gearshape.2"
        }
    }
}

struct PreferencesView: View {
    @ObservedObject var model: PreferencesModel
    @ObservedObject var recorder: HotKeyRecorder
    @ObservedObject var captureRecorder: HotKeyRecorder

    var body: some View {
        TabView {
            ForEach(SettingsTab.allCases, id: \.self) { tab in
                content(tab)
                    .tabItem { Label(tab.title, systemImage: tab.systemImage) }
                    .tag(tab)
            }
        }
    }

    @ViewBuilder
    private func content(_ tab: SettingsTab) -> some View {
        switch tab {
        case .general: general
        case .shortcuts: shortcuts
        case .terminal: terminal
        case .advanced: advanced
        }
    }

    private var general: some View {
        tabForm {
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
                Picker("When tt exits", selection: $model.exitBehavior) {
                    ForEach(TTExitBehavior.allCases, id: \.self) { Text($0.label).tag($0) }
                }
                note("A crash or a non-zero exit always keeps tt's last output on screen with a Relaunch button.")
            }

            Section {
                Toggle("Match macOS appearance", isOn: $model.followAppearance)
                note("tt switches between its light and dark themes with the system, without changing "
                    + "the theme it saves. T still toggles it until the next change.")
            }

            Section {
                Toggle("Notify when work arrives", isOn: $model.notificationsEnabled)
                if model.notificationsEnabled, model.notificationsDenied {
                    HStack {
                        note(NotificationSettingsLink.deniedNote, warning: true)
                        Spacer()
                        Button("Open Notification Settings") {
                            NSWorkspace.shared.open(NotificationSettingsLink.url())
                        }
                    }
                }
                note("A check-in whose day comes, a timed deadline that passes, or a deferral that ends, "
                    + "while tt is not in front. Click one to show its task; Acknowledge, Done, and Defer 1 Hour "
                    + "act on it from the notification.")
                Toggle(model.activationPolicy.showsDockIcon
                       ? "Show overdue count on the Dock icon"
                       : "Show overdue count beside the menu-bar icon",
                       isOn: $model.dockBadge)
            }
        }
    }

    private var shortcuts: some View {
        tabForm {
            Section {
                LabeledContent("Show and hide tt") {
                    HotKeyRecorderView(combo: $model.hotKey, defaultCombo: TTHotKeyPreference.default,
                                       model: model, recorder: recorder, other: captureRecorder)
                }
                if let problem = model.hotKeyProblem {
                    note(problem, warning: true)
                }
                note("Works from any app. Pressing it while tt is in front hides tt. "
                    + "A Karabiner rule can send this same shortcut.")
                LabeledContent("Quick capture") {
                    HotKeyRecorderView(combo: $model.captureHotKey, defaultCombo: TTCaptureHotKeyPreference.default,
                                       model: model, recorder: captureRecorder, other: recorder)
                }
                if let problem = model.captureHotKeyProblem {
                    note(problem, warning: true)
                }
                note("Opens a capture field over any app. Enter sends the line to tt, "
                    + "which reads it as its composer would: #tag, @date, %project, $state, !1.")
            }
        }
    }

    private var terminal: some View {
        tabForm {
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

                Picker("Theme", selection: $model.themeName) {
                    Text("From the Ghostty config").tag(String?.none)
                    Divider()
                    ForEach(model.themeChoices, id: \.self) { Text($0).tag(String?.some($0)) }
                }
                Toggle("Use another theme in Dark Mode", isOn: $model.separateDarkTheme)
                    .disabled(model.themeName == nil)
                if model.separateDarkTheme {
                    Picker("Dark Mode theme", selection: $model.darkThemeName) {
                        ForEach(model.themeChoices, id: \.self) { Text($0).tag($0) }
                    }
                }
                note((model.themeName == nil
                    ? "Uses the Ghostty config's theme. "
                    : "Overrides the Ghostty config's theme; colors it sets individually still apply. ")
                    + "Changing this recolors the open terminal without changing its zoom.")
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
                if let report = model.ghosttyConfigReport {
                    note(report.text, warning: report.warning)
                        .textSelection(.enabled)
                }
            }
        }
    }

    private var advanced: some View {
        tabForm {
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
                if model.binaryAwaitsRelaunch {
                    HStack {
                        note(model.nextBinaryNote ?? "A different tt is available after relaunch.", warning: true)
                        Spacer()
                        Button("Relaunch tt") { model.onRelaunch?() }
                    }
                }
                note("Leave empty to use the bundled tt, ~/.local/bin/tt, then PATH; "
                    + "a ~/.local/bin/tt built after the bundled one wins. "
                    + "Takes effect when tt next starts.")
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
        }
    }

    private func tabForm<Content: View>(@ViewBuilder _ content: () -> Content) -> some View {
        Form(content: content)
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
    @Binding var combo: HotKeyCombo?
    /// What Restore Default sets, offered only while the combo differs.
    let defaultCombo: HotKeyCombo
    @ObservedObject var model: PreferencesModel
    @ObservedObject var recorder: HotKeyRecorder
    /// The other shortcut's recorder, stopped when this one starts.
    let other: HotKeyRecorder

    var body: some View {
        HStack {
            Button(action: toggleRecording) {
                Text(label)
                    .frame(minWidth: 140)
                    .foregroundStyle(recorder.isRecording ? Color.accentColor : Color.primary)
            }
            if combo != nil, !recorder.isRecording {
                Button("Clear") { combo = nil }
            }
            if Self.offersRestoreDefault(combo: combo, default: defaultCombo, recording: recorder.isRecording) {
                Button("Restore Default") { combo = defaultCombo }
                    .help("Use \(defaultCombo.displayString)")
            }
        }
        .onDisappear { recorder.stop() }
    }

    /// Restore Default shows while the shortcut is another one, or off, and
    /// nothing is being recorded.
    static func offersRestoreDefault(combo: HotKeyCombo?, default defaultCombo: HotKeyCombo, recording: Bool) -> Bool {
        !recording && combo != defaultCombo
    }

    private var label: String {
        if recorder.isRecording {
            return recorder.hint ?? "Type a shortcut…"
        }
        return combo?.displayString ?? "Record Shortcut"
    }

    private func toggleRecording() {
        if recorder.isRecording {
            recorder.stop()
            return
        }
        other.stop()
        let binding = $combo
        recorder.start(
            onStateChange: { [weak model] recording in model?.onRecordingChange?(recording) },
            onRecord: { binding.wrappedValue = $0 }
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
