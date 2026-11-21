import AppKit
import Carbon
import XCTest
@testable import Docket

final class HotKeyComboTests: XCTestCase {
    func testEveryKeyAndModifierSetRoundTripsThroughItsString() {
        let modifierBits = [controlKey, optionKey, shiftKey, cmdKey].map(UInt32.init)
        for keyCode in HotKeyCombo.keys.keys {
            for mask in 0..<(1 << modifierBits.count) {
                let modifiers = modifierBits.enumerated()
                    .filter { mask & (1 << $0.offset) != 0 }
                    .reduce(UInt32(0)) { $0 | $1.element }
                guard let combo = HotKeyCombo(keyCode: keyCode, modifiers: modifiers) else { continue }
                XCTAssertEqual(HotKeyCombo(string: combo.string), combo, combo.string)
                XCTAssertEqual(combo.keyCode, keyCode)
                XCTAssertEqual(combo.modifiers, modifiers)
            }
        }
    }

    func testStringsParseToCarbonKeyCodesAndModifiers() throws {
        let combo = try XCTUnwrap(HotKeyCombo(string: "ctrl+opt+cmd+t"))
        XCTAssertEqual(combo.keyCode, UInt32(kVK_ANSI_T))
        XCTAssertEqual(combo.modifiers, UInt32(controlKey | optionKey | cmdKey))
        XCTAssertEqual(combo.displayString, "⌃⌥⌘T")

        // Aliases, case, spacing, and order all land on the canonical form.
        XCTAssertEqual(HotKeyCombo(string: "Command + Shift + Space")?.string, "shift+cmd+space")
        XCTAssertEqual(HotKeyCombo(string: "cmd+alt+control+f5")?.string, "ctrl+opt+cmd+f5")
        XCTAssertEqual(HotKeyCombo(string: "cmd+minus")?.displayString, "⌘-")
        XCTAssertEqual(HotKeyCombo(string: "f13")?.keyCode, UInt32(kVK_F13))
    }

    func testCombosThatWouldSwallowTypingOrDoNotParseAreRefused() {
        for bad in ["", "t", "shift+t", "cmd+", "+t", "cmd+cmd+t", "hyper+t", "cmd+t+x", "cmd+nosuchkey", "cmd++"] {
            XCTAssertNil(HotKeyCombo(string: bad), bad)
        }
        XCTAssertNil(HotKeyCombo(keyCode: 0xFF, modifiers: UInt32(cmdKey)))
        XCTAssertNotNil(HotKeyCombo(string: "shift+f1"))
    }

    func testRecordedEventsCarryTheirModifiers() throws {
        let event = try XCTUnwrap(NSEvent.keyEvent(
            with: .keyDown, location: .zero, modifierFlags: [.command, .option, .capsLock],
            timestamp: 0, windowNumber: 0, context: nil, characters: "k",
            charactersIgnoringModifiers: "k", isARepeat: false, keyCode: UInt16(kVK_ANSI_K)
        ))
        XCTAssertEqual(HotKeyCombo(event: event)?.string, "opt+cmd+k")
    }
}

final class ShellPreferenceDefaultsTests: XCTestCase {
    private var suite = ""
    private var defaults: UserDefaults!

    override func setUpWithError() throws {
        suite = "tt-tests-\(UUID().uuidString)"
        defaults = try XCTUnwrap(UserDefaults(suiteName: suite))
    }

    override func tearDown() {
        defaults.removePersistentDomain(forName: suite)
    }

    func testHotKeyDefaultsOnCanBeTurnedOffAndRoundTrips() {
        XCTAssertEqual(DocketHotKeyPreference.load(from: defaults)?.string, "ctrl+opt+cmd+t")
        DocketHotKeyPreference.save(nil, to: defaults)
        XCTAssertNil(DocketHotKeyPreference.load(from: defaults))
        DocketHotKeyPreference.save(HotKeyCombo(string: "shift+cmd+space"), to: defaults)
        XCTAssertEqual(defaults.string(forKey: DocketHotKeyPreference.defaultsKey), "shift+cmd+space")
        XCTAssertEqual(DocketHotKeyPreference.load(from: defaults)?.string, "shift+cmd+space")
        defaults.set("shift+t", forKey: DocketHotKeyPreference.defaultsKey)
        XCTAssertEqual(DocketHotKeyPreference.load(from: defaults), DocketHotKeyPreference.default)
    }

    func testActivationPolicyDefaultsToTheDockAndRoundTrips() {
        XCTAssertEqual(DocketActivationPolicy.load(from: defaults), .dockAndMenuBar)
        XCTAssertEqual(DocketActivationPolicy.dockAndMenuBar.appKitPolicy, .regular)
        XCTAssertEqual(DocketActivationPolicy.menuBarOnly.appKitPolicy, .accessory)
        XCTAssertEqual(DocketActivationPolicy.dockOnly.appKitPolicy, .regular)
        XCTAssertEqual(DocketActivationPolicy.allCases.filter { !$0.showsStatusItem }, [.dockOnly])
        defaults.set("nonsense", forKey: DocketActivationPolicy.defaultsKey)
        XCTAssertEqual(DocketActivationPolicy.load(from: defaults), .dockAndMenuBar)
        DocketActivationPolicy.menuBarOnly.save(to: defaults)
        XCTAssertEqual(DocketActivationPolicy.load(from: defaults), .menuBarOnly)
        DocketActivationPolicy.dockOnly.save(to: defaults)
        XCTAssertEqual(defaults.string(forKey: DocketActivationPolicy.defaultsKey), "dock")
        XCTAssertEqual(DocketActivationPolicy.load(from: defaults), .dockOnly)
    }

    func testDefaultFontSizeIsUnsetUntilSavedAndRefusesNonsense() {
        XCTAssertNil(DocketDefaultFontSize.load(from: defaults))
        DocketDefaultFontSize.save(15, to: defaults)
        XCTAssertEqual(DocketDefaultFontSize.load(from: defaults), 15)
        DocketDefaultFontSize.save(500, to: defaults)
        XCTAssertEqual(DocketDefaultFontSize.load(from: defaults), DocketDefaultFontSize.range.upperBound)
        for bad: Any in ["big", 2, 0, -4] {
            defaults.set(bad, forKey: DocketDefaultFontSize.defaultsKey)
            XCTAssertNil(DocketDefaultFontSize.load(from: defaults), "\(bad)")
        }
        DocketDefaultFontSize.save(nil, to: defaults)
        XCTAssertNil(defaults.object(forKey: DocketDefaultFontSize.defaultsKey))
        XCTAssertEqual(DocketDefaultFontSize.label(14), "14 pt")
        XCTAssertEqual(DocketDefaultFontSize.label(13.5), "13.5 pt")
    }

    func testEnvironmentOverridesKeepKnownSingleLineValuesOnly() {
        DocketEnvironmentOverrides.save([
            "DOCKET_DB": "  ~/docket.db  ", "PATH": "a\nb", "HOME": "/elsewhere", "EDITOR": "hx",
        ], to: defaults)
        XCTAssertEqual(DocketEnvironmentOverrides.load(from: defaults), ["DOCKET_DB": "~/docket.db"])
        DocketEnvironmentOverrides.save(["PATH": ""], to: defaults)
        XCTAssertNil(defaults.object(forKey: DocketEnvironmentOverrides.defaultsKey))
        XCTAssertEqual(DocketEnvironmentOverrides.names, ["PATH", "DOCKET_DB"])
    }

    func testOverridesReplaceTheShellsValuesAndKeepTheRest() {
        let shell = LaunchEnvironment(home: "/Users/me", path: "/bin", forwarded: ["DOCKET_DB": "/shell.db"])
        let applied = shell.applying(["PATH": "/opt/bin:/bin", "HOME": "/x", "EDITOR": "hx"])
        XCTAssertEqual(applied.commandEnvironment, [
            "HOME": "/Users/me", "PATH": "/opt/bin:/bin", "DOCKET_DB": "/shell.db", "DOCKET_BELL": "1",
        ])
        XCTAssertEqual(applied.value(of: "PATH"), "/opt/bin:/bin")
        XCTAssertEqual(shell.applying(["DOCKET_DB": "/mine.db"]).value(of: "DOCKET_DB"), "/mine.db")
        XCTAssertNil(shell.value(of: "EDITOR"))
    }

    /// Only the database override is forwarded, and nothing is given a value
    /// neither the shell nor Settings set; the bell Docket rings on a row done is
    /// always asked for, since Settings decides what it plays.
    func testTheDatabaseOverrideIsForwardedAndNothingIsDefaulted() {
        XCTAssertEqual(LaunchEnvironment.forwardedNames, ["DOCKET_DB"])
        XCTAssertEqual(LaunchEnvironment(home: "/h", path: "/bin").commandEnvironment,
                       ["HOME": "/h", "PATH": "/bin", "DOCKET_BELL": "1"])
        XCTAssertEqual(
            LaunchEnvironment.current(processEnvironment: ["HOME": "/h", "SHELL": "/usr/bin/false", "DOCKET_DB": "/x.db"])
                .commandEnvironment["DOCKET_DB"],
            "/x.db", "an inherited value is forwarded"
        )
    }

    func testTerminalFontDefaultsToBuiltInAndPersists() {
        XCTAssertEqual(DocketTerminalFontPreference.load(from: defaults), .builtIn)
        let model = PreferencesModel(defaults: defaults)
        XCTAssertEqual(model.terminalFont, .builtIn)
        XCTAssertEqual(model.fontChoice, PreferencesModel.builtInFontTag)
        var changes = 0
        model.onTerminalFontChange = { changes += 1 }

        model.fontChoice = PreferencesModel.configFontTag
        XCTAssertNil(model.terminalFont)
        XCTAssertEqual(defaults.string(forKey: DocketTerminalFontPreference.defaultsKey), "ghostty-config")
        XCTAssertNil(PreferencesModel(defaults: defaults).terminalFont)

        model.fontChoice = "Menlo"
        XCTAssertEqual(model.terminalFont, .family("Menlo"))
        XCTAssertEqual(DocketTerminalFontPreference.load(from: defaults), .family("Menlo"))
        XCTAssertTrue(model.fontChoices.contains("Menlo"), "a saved family stays listed")

        model.fontChoice = PreferencesModel.builtInFontTag
        XCTAssertNil(defaults.object(forKey: DocketTerminalFontPreference.defaultsKey), "absence is the built-in")
        XCTAssertEqual(changes, 3)

        defaults.set("Bad\nName", forKey: DocketTerminalFontPreference.defaultsKey)
        XCTAssertEqual(DocketTerminalFontPreference.load(from: defaults), .builtIn)
    }

    func testAnEnvironmentEditAwaitsARelaunchOnlyWhileTheOldValuesRun() {
        let model = PreferencesModel(defaults: defaults)
        var changes = 0
        model.onEnvironmentChange = { changes += 1 }
        model.detectedEnvironment = LaunchEnvironment(home: "/h", path: "/bin", forwarded: ["DOCKET_DB": "/shell.db"])
        XCTAssertFalse(model.environmentAwaitsRelaunch, "no session runs")

        model.sessionEnvironment = model.effectiveEnvironment?.commandEnvironment
        XCTAssertFalse(model.environmentAwaitsRelaunch)

        model.environmentOverrides = ["DOCKET_DB": "/mine.db"]
        XCTAssertTrue(model.environmentAwaitsRelaunch)
        XCTAssertEqual(changes, 1)
        XCTAssertEqual(PreferencesModel(defaults: defaults).environmentOverrides, ["DOCKET_DB": "/mine.db"], "saved")

        model.environmentOverrides = [:]
        XCTAssertFalse(model.environmentAwaitsRelaunch, "back to what runs")
    }

    func testABinaryAwaitsARelaunchOnlyWhenTheNextOneIsAnotherPath() {
        let bundled = DocketBinaryResolver.Resolution(url: URL(fileURLWithPath: "/a/bin/docket"), source: .bundled)
        let same = DocketBinaryResolver.Resolution(url: URL(fileURLWithPath: "/a/bin/../bin/docket"), source: .userDefaultsOverride)
        let other = DocketBinaryResolver.Resolution(url: URL(fileURLWithPath: "/b/docket"), source: .userDefaultsOverride)
        XCTAssertFalse(PreferencesModel.binaryAwaitsRelaunch(running: nil, next: other), "no session runs")
        XCTAssertFalse(PreferencesModel.binaryAwaitsRelaunch(running: bundled, next: nil), "the last Docket keeps running")
        XCTAssertFalse(PreferencesModel.binaryAwaitsRelaunch(running: bundled, next: same), "one path, two spellings")
        XCTAssertTrue(PreferencesModel.binaryAwaitsRelaunch(running: bundled, next: other))

        let model = PreferencesModel(defaults: defaults)
        var next: DocketBinaryResolver.Resolution? = bundled
        model.resolveNextBinary = { _ in next }
        model.binaryOverride = "/b/docket"
        XCTAssertFalse(model.binaryAwaitsRelaunch, "nothing runs yet")

        model.runningBinary = bundled
        model.refreshBinaryStatus()
        XCTAssertFalse(model.binaryAwaitsRelaunch)

        next = other
        model.binaryOverride = "/b/docket "
        XCTAssertTrue(model.binaryAwaitsRelaunch, "the edit is asked about as it is made")

        model.runningBinary = other
        model.refreshBinaryStatus()
        XCTAssertFalse(model.binaryAwaitsRelaunch, "the relaunched Docket is the one named")
    }

    func testALaunchAtLoginFailureIsTheErrorAndWhereToChangeIt() {
        let error = NSError(domain: "t", code: 1, userInfo: [NSLocalizedDescriptionKey: "Operation not permitted."])
        XCTAssertEqual(PreferencesModel.launchAtLoginFailure(error),
                       "Operation not permitted.\nSystem Settings › General › Login Items")
    }

    func testSettingsIsFourTabsInAWindowThatRemembersItsFrame() throws {
        XCTAssertEqual(SettingsTab.allCases.map(\.title), ["General", "Shortcuts", "Terminal", "Advanced"])
        XCTAssertEqual(PreferencesWindowController.frameAutosaveName, "Settings")

        let unsaved = PreferencesWindowController(model: PreferencesModel(defaults: defaults), frameAutosaveName: nil)
        XCTAssertEqual(unsaved.window.title, "Settings")
        XCTAssertEqual(unsaved.window.frameAutosaveName, "")

        let name = "tt-tests-settings-\(UUID().uuidString)"
        defer { UserDefaults.standard.removeObject(forKey: "NSWindow Frame \(name)") }
        let saved = PreferencesWindowController(model: PreferencesModel(defaults: defaults), frameAutosaveName: name)
        XCTAssertEqual(saved.window.frameAutosaveName, name)
        saved.window.setFrameOrigin(NSPoint(x: 123, y: 321))
        saved.window.saveFrame(usingName: name)
        saved.window.setFrameAutosaveName("")

        let reopened = PreferencesWindowController(model: PreferencesModel(defaults: defaults), frameAutosaveName: name)
        XCTAssertEqual(reopened.window.frame.origin, NSPoint(x: 123, y: 321), "opens where it was left, not centered")
        reopened.window.setFrameAutosaveName("")
    }

    func testBinaryStatusComparesTheBundledCopyWithLocalBin() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent("tt-skew-\(UUID().uuidString)")
        defer { try? FileManager.default.removeItem(at: dir) }
        let bundled = dir.appendingPathComponent("bundled/docket")
        let local = dir.appendingPathComponent("local/docket")
        for url in [bundled, local] {
            try FileManager.default.createDirectory(at: url.deletingLastPathComponent(), withIntermediateDirectories: true)
            try "#!/bin/sh\n".write(to: url, atomically: true, encoding: .utf8)
            try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: url.path)
        }
        let old = "docket version a (commit: a, built: 2026-01-01T00:00:00Z)"
        let new = "docket version b (commit: b, built: 2026-02-01T00:00:00Z)"
        let versions = [bundled.path: new, local.path: old]

        let model = PreferencesModel(defaults: defaults)
        model.bundledBinaryPath = bundled.path
        model.localBinaryPath = local.path
        model.readVersion = { versions[$0].flatMap(DocketVersion.parse) }
        model.refreshBinaryStatus()
        XCTAssertNil(model.runningBinaryNote, "nothing runs yet")

        model.runningBinary = .init(url: bundled, source: .bundled)
        model.runningVersion = DocketVersion.parse(new)
        model.refreshBinaryStatus()
        XCTAssertEqual(model.runningBinaryNote, "Running \(bundled.path): \(new)")
        XCTAssertTrue(model.binarySkewNote?.contains("is older (\(old))") ?? false)

        model.runningVersion = DocketVersion.parse(old)
        model.readVersion = { [bundled.path: old, local.path: new][$0].flatMap(DocketVersion.parse) }
        model.refreshBinaryStatus()
        XCTAssertTrue(model.binarySkewNote?.contains("is newer (\(new))") ?? false)

        model.runningBinary = .init(url: local, source: .localBin)
        model.runningVersion = DocketVersion.parse(new)
        model.refreshBinaryStatus()
        XCTAssertNil(model.binarySkewNote, "only the bundled copy is compared")

        model.runningBinary = .init(url: bundled, source: .bundled)
        model.readVersion = { _ in DocketVersion.parse(new) }
        model.refreshBinaryStatus()
        XCTAssertNil(model.binarySkewNote, "matching copies say nothing")
    }

    func testSamePathReplacementKeepsRunningVersionUntilRelaunch() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: dir) }
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        let path = dir.appendingPathComponent("docket")
        try "old executable".write(to: path, atomically: true, encoding: .utf8)
        let resolution = DocketBinaryResolver.Resolution(url: path, source: .userDefaultsOverride)
        let old = DocketVersion.parse("docket version old")!
        let new = DocketVersion.parse("docket version new")!
        let model = PreferencesModel(defaults: defaults)
        model.runningBinary = resolution
        model.runningVersion = old
        model.runningFingerprint = DocketBinaryFingerprint.read(path.path)
        model.resolveNextBinary = { _ in resolution }
        model.readVersion = { _ in old }
        model.refreshBinaryStatus()
        XCTAssertFalse(model.binaryAwaitsRelaunch)

        try "new executable with the same version label".write(to: path, atomically: true, encoding: .utf8)
        model.refreshBinaryStatus()
        XCTAssertTrue(model.binaryAwaitsRelaunch, "development builds may have identical version strings")
        model.readVersion = { _ in new }
        model.refreshBinaryStatus()
        XCTAssertTrue(model.runningBinaryNote?.hasSuffix(old.line) ?? false)
        XCTAssertTrue(model.nextBinaryNote?.contains(new.line) ?? false)
        model.runningVersion = new
        model.runningFingerprint = DocketBinaryFingerprint.read(path.path)
        model.refreshBinaryStatus()
        XCTAssertFalse(model.binaryAwaitsRelaunch)
        XCTAssertNil(model.nextBinaryNote)
        XCTAssertTrue(model.runningBinaryNote?.hasSuffix(new.line) ?? false)
    }

    func testTheModelStartsFromTheDefaultsAndSavesEachChange() {
        let model = PreferencesModel(defaults: defaults)
        XCTAssertEqual(model.exitBehavior, .relaunchOnShow)
        XCTAssertEqual(model.activationPolicy, .dockAndMenuBar)
        XCTAssertEqual(model.hotKey, DocketHotKeyPreference.default)
        XCTAssertEqual(model.binaryOverride, "")
        XCTAssertNil(model.binaryOverrideProblem)
        XCTAssertNil(model.defaultFontSize)

        var policies: [DocketActivationPolicy] = []
        var hotKeys: [HotKeyCombo?] = []
        var fontSizes: [Float?] = []
        model.onActivationPolicyChange = { policies.append($0) }
        model.onHotKeyChange = { hotKeys.append($0) }
        model.onDefaultFontSizeChange = { fontSizes.append($0) }

        model.defaultFontSize = 16
        model.defaultFontSize = 16
        XCTAssertEqual(DocketDefaultFontSize.load(from: defaults), 16)
        XCTAssertEqual(PreferencesModel(defaults: defaults).defaultFontSize, 16, "the next launch starts from it")
        model.defaultFontSize = nil
        XCTAssertEqual(fontSizes, [16, nil])
        XCTAssertNil(defaults.object(forKey: DocketDefaultFontSize.defaultsKey))

        model.activationPolicy = .menuBarOnly
        model.activationPolicy = .menuBarOnly
        model.hotKey = nil
        model.exitBehavior = .quitApp
        model.binaryOverride = " /no/such/docket "

        XCTAssertEqual(policies, [.menuBarOnly], "an unchanged value applies nothing")
        XCTAssertEqual(hotKeys, [nil])
        XCTAssertEqual(DocketActivationPolicy.load(from: defaults), .menuBarOnly)
        XCTAssertNil(DocketHotKeyPreference.load(from: defaults))
        XCTAssertEqual(DocketExitBehavior.load(from: defaults), .quitApp)
        XCTAssertEqual(defaults.string(forKey: DocketBinaryResolver.overrideDefaultsKey), "/no/such/docket")
        XCTAssertNotNil(model.binaryOverrideProblem)

        model.binaryOverride = "/bin/sh"
        XCTAssertNil(model.binaryOverrideProblem)
        model.binaryOverride = "  "
        XCTAssertNil(defaults.object(forKey: DocketBinaryResolver.overrideDefaultsKey))
    }

    /// Restore Default shows while a shortcut is another combo or off, and
    /// never while recording.
    func testRestoreDefaultIsOfferedOnlyAwayFromTheDefault() throws {
        for preset in [DocketHotKeyPreference.default, DocketCaptureHotKeyPreference.default] {
            XCTAssertFalse(HotKeyRecorderView.offersRestoreDefault(combo: preset, default: preset, recording: false))
            XCTAssertTrue(HotKeyRecorderView.offersRestoreDefault(combo: nil, default: preset, recording: false))
            let other = try XCTUnwrap(HotKeyCombo(string: "ctrl+opt+f5"))
            XCTAssertTrue(HotKeyRecorderView.offersRestoreDefault(combo: other, default: preset, recording: false))
            XCTAssertFalse(HotKeyRecorderView.offersRestoreDefault(combo: other, default: preset, recording: true))
        }
    }

    /// Reload says what it did: applied, applied with Ghostty's diagnostics,
    /// or not applied; reopening Settings clears it.
    func testReloadReportsWhatItFound() {
        XCTAssertEqual(GhosttyConfigReport(applied: true, diagnostics: []).text, "Reloaded.")
        XCTAssertFalse(GhosttyConfigReport(applied: true, diagnostics: []).warning)
        let problems = GhosttyConfigReport(applied: true, diagnostics: ["unknown field: x", "invalid value: y"])
        XCTAssertTrue(problems.warning)
        XCTAssertEqual(problems.text,
                       "Reloaded, with 2 problems in the config:\n• unknown field: x\n• invalid value: y")
        XCTAssertTrue(GhosttyConfigReport(applied: false, diagnostics: []).warning)

        let model = PreferencesModel(defaults: defaults)
        model.ghosttyConfigReport = problems
        model.refresh()
        XCTAssertNil(model.ghosttyConfigReport)
    }

    func testTheExitChoicesAreWorded() {
        XCTAssertEqual(DocketExitBehavior.allCases.map(\.label), ["Hide, and start a fresh Docket next time", "Quit Docket"])
    }

    func testFollowingTheAppearanceDefaultsOnAndSavesEachChange() {
        XCTAssertTrue(DocketFollowAppearancePreference.load(from: defaults))
        let model = PreferencesModel(defaults: defaults)
        XCTAssertTrue(model.followAppearance)

        var changes: [Bool] = []
        model.onFollowAppearanceChange = { changes.append($0) }
        model.followAppearance = false
        model.followAppearance = false
        XCTAssertEqual(changes, [false], "an unchanged value applies nothing")
        XCTAssertFalse(DocketFollowAppearancePreference.load(from: defaults))
        XCTAssertFalse(PreferencesModel(defaults: defaults).followAppearance, "the next launch starts from it")

        model.followAppearance = true
        XCTAssertEqual(changes, [false, true])
        XCTAssertTrue(DocketFollowAppearancePreference.load(from: defaults))
    }

    func testTheDoneSoundDefaultsOffAndSavesEachChange() {
        XCTAssertEqual(DocketDoneSound.load(from: defaults), .none)
        XCTAssertNil(DocketDoneSound.none.sound, "none plays nothing")
        let model = PreferencesModel(defaults: defaults)
        XCTAssertEqual(model.doneSound, .none)

        model.doneSound = .glass
        XCTAssertEqual(DocketDoneSound.load(from: defaults), .glass)
        XCTAssertEqual(PreferencesModel(defaults: defaults).doneSound, .glass, "the next launch starts from it")

        defaults.set("Sosumi?", forKey: DocketDoneSound.defaultsKey)
        XCTAssertEqual(DocketDoneSound.load(from: defaults), .none, "an unknown value reads as off")
        XCTAssertEqual(DocketDoneSound.allCases.map(\.label), ["None", "Glass", "Hero", "Pop", "Purr", "Tink"])
        for choice in DocketDoneSound.allCases where choice != .none {
            XCTAssertNotNil(choice.sound, "macOS has \(choice.rawValue)")
        }
    }

    func testTheOverdueBadgeDefaultsOnAndSavesEachChange() {
        XCTAssertTrue(DocketDockBadgePreference.load(from: defaults))
        let model = PreferencesModel(defaults: defaults)
        XCTAssertTrue(model.dockBadge)

        var changes: [Bool] = []
        model.onDockBadgeChange = { changes.append($0) }
        model.dockBadge = false
        model.dockBadge = false
        XCTAssertEqual(changes, [false], "an unchanged value applies nothing")
        XCTAssertFalse(DocketDockBadgePreference.load(from: defaults))
        XCTAssertFalse(PreferencesModel(defaults: defaults).dockBadge, "the next launch starts from it")
    }

    func testTerminalThemeSelectionPersistsAndNotifiesOnlyOnChanges() throws {
        let model = PreferencesModel(defaults: defaults)
        XCTAssertNil(model.terminalTheme)
        var changes = 0
        model.onTerminalThemeChange = { changes += 1 }

        model.themeName = "Catppuccin Latte"
        model.themeName = "Catppuccin Latte"
        XCTAssertEqual(changes, 1)
        XCTAssertEqual(DocketTerminalThemePreference.load(from: defaults)?.configValue, "Catppuccin Latte")

        model.separateDarkTheme = true
        model.darkThemeName = "Catppuccin Mocha"
        XCTAssertEqual(PreferencesModel(defaults: defaults).terminalTheme?.configValue,
                       "light:Catppuccin Latte,dark:Catppuccin Mocha")
        XCTAssertEqual(changes, 3)

        model.themeName = nil
        XCTAssertNil(defaults.object(forKey: DocketTerminalThemePreference.defaultsKey))
        XCTAssertEqual(changes, 4)
    }

    func testTerminalThemeSyntaxAndThemeDiscovery() throws {
        XCTAssertEqual(TerminalTheme(configValue: "dark:Mocha,light:Latte")?.configValue,
                       "light:Latte,dark:Mocha")
        for bad in ["", "light:Latte", "light:Latte,dark:", "light:Latte,light:Mocha", "A,B", "A\ncommand = /bin/sh"] {
            XCTAssertNil(TerminalTheme(configValue: bad), bad)
        }

        let root = FileManager.default.temporaryDirectory.appendingPathComponent("tt-themes-\(UUID().uuidString)")
        defer { try? FileManager.default.removeItem(at: root) }
        let bundled = root.appendingPathComponent("bundled")
        let user = root.appendingPathComponent("user")
        try FileManager.default.createDirectory(at: bundled, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: user, withIntermediateDirectories: true)
        for (directory, names) in [(bundled, ["Latte", "Mocha", ".hidden"]), (user, ["Mocha", "Custom"])] {
            for name in names {
                try "background = #000000\n".write(to: directory.appendingPathComponent(name), atomically: true, encoding: .utf8)
            }
        }
        XCTAssertEqual(TerminalTheme.available(bundled: bundled, user: user), ["Custom", "Latte", "Mocha"])
    }
}

final class TerminalToggleTests: XCTestCase {
    func testHidesOnlyWhenTheWindowIsAlreadyInFront() {
        XCTAssertEqual(TerminalToggle.action(appIsActive: true, windowIsVisible: true, windowIsKey: true), .hide)
        XCTAssertEqual(TerminalToggle.action(appIsActive: false, windowIsVisible: true, windowIsKey: true), .show)
        XCTAssertEqual(TerminalToggle.action(appIsActive: true, windowIsVisible: false, windowIsKey: false), .show)
        // Preferences in front: the shortcut brings Docket forward rather than hiding.
        XCTAssertEqual(TerminalToggle.action(appIsActive: true, windowIsVisible: true, windowIsKey: false), .show)
    }
}

final class HotKeyCenterTests: XCTestCase {
    func testRegistersReplacesAndTurnsOff() throws {
        let center = HotKeyCenter()
        let first = try XCTUnwrap(HotKeyCombo(string: "ctrl+opt+shift+cmd+f19"))
        let second = try XCTUnwrap(HotKeyCombo(string: "ctrl+opt+shift+cmd+f18"))
        XCTAssertEqual(center.register(first), .registered(first))
        XCTAssertNil(center.registration.problem)
        XCTAssertEqual(center.register(second), .registered(second))
        XCTAssertEqual(center.register(nil), .none)
        center.unregister()
        XCTAssertEqual(center.registration, .none)
    }

    /// A press arrives as a Carbon event naming the registration; only the
    /// center that owns that ID answers it.
    func testAPressReachesOnlyItsOwnCenter() throws {
        let center = HotKeyCenter()
        let other = HotKeyCenter()
        var presses: [String] = []
        center.onPress = { presses.append("center") }
        other.onPress = { presses.append("other") }

        var eventRef: EventRef?
        XCTAssertEqual(CreateEvent(nil, OSType(kEventClassKeyboard), UInt32(kEventHotKeyPressed), 0, 0, &eventRef), noErr)
        let event = try XCTUnwrap(eventRef)
        defer { ReleaseEvent(event) }
        var id = center.hotKeyID
        XCTAssertEqual(SetEventParameter(event, EventParamName(kEventParamDirectObject), EventParamType(typeEventHotKeyID),
                                         MemoryLayout<EventHotKeyID>.size, &id), noErr)
        XCTAssertEqual(SendEventToEventTarget(event, GetApplicationEventTarget()), noErr)
        XCTAssertEqual(presses, ["center"])
    }

    func testFailuresNameTheShortcut() throws {
        let combo = try XCTUnwrap(HotKeyCombo(string: "ctrl+opt+cmd+t"))
        let taken = HotKeyCenter.Registration.failed(combo, OSStatus(eventHotKeyExistsErr)).problem
        XCTAssertEqual(taken, "⌃⌥⌘T is already taken by another app. Choose another shortcut.")
        XCTAssertEqual(HotKeyCenter.Registration.failed(combo, -1).problem,
                       "macOS would not register ⌃⌥⌘T as a global shortcut. Choose another shortcut.")
    }

    func testSystemShortcutsAreReadable() {
        // Whatever the machine's settings, reading them must not fail or
        // yield combos the recorder itself would refuse.
        for combo in SystemHotKeys.enabledCombos() {
            XCTAssertEqual(HotKeyCombo(string: combo.string), combo)
        }
    }
}

final class HotKeyConflictTests: XCTestCase {
    private func combo(_ string: String) throws -> HotKeyCombo {
        try XCTUnwrap(HotKeyCombo(string: string))
    }

    /// Each shortcut refuses the combo the other holds live, naming what that
    /// one does, and a refused one registers once the other moves away.
    func testTheTwoShortcutsRefuseEachOther() throws {
        let toggle = HotKeyCenter()
        let capture = HotKeyCenter()
        let first = try combo("ctrl+opt+shift+cmd+f19")
        let second = try combo("ctrl+opt+shift+cmd+f18")
        func registerToggle(_ combo: HotKeyCombo?) -> String? {
            AppDelegate.register(combo, on: toggle, beside: capture, otherUse: AppDelegate.captureHotKeyUse, menu: nil)
        }
        func registerCapture(_ combo: HotKeyCombo?) -> String? {
            AppDelegate.register(combo, on: capture, beside: toggle, otherUse: AppDelegate.toggleHotKeyUse, menu: nil)
        }

        XCTAssertNil(registerToggle(first))
        XCTAssertEqual(registerCapture(first), "⌃⌥⇧⌘F19 already shows and hides Docket. Choose another shortcut.")
        XCTAssertNil(capture.registeredCombo)

        XCTAssertNil(registerToggle(second))
        XCTAssertNil(registerCapture(first), "the re-registration clears the stale warning")
        XCTAssertEqual(capture.registeredCombo, first)

        XCTAssertEqual(registerToggle(first), "⌃⌥⇧⌘F19 already opens quick capture. Choose another shortcut.")
        XCTAssertNil(toggle.registeredCombo)
        XCTAssertNil(registerCapture(second))
        XCTAssertNil(registerToggle(first))
        XCTAssertEqual(toggle.registeredCombo, first)
    }

    func testTheMenusCommandShortcutsAreRefused() throws {
        let menu = MainMenu.make(captureHotKey: try combo("shift+cmd+k"))
        XCTAssertEqual(ReservedHotKeys.problem(for: try combo("cmd+q"), menu: menu),
                       "⌘Q is Docket's Quit Docket shortcut. Choose another shortcut.")
        XCTAssertEqual(ReservedHotKeys.problem(for: try combo("cmd+comma"), menu: menu),
                       "⌘, is Docket's Settings shortcut. Choose another shortcut.")
        XCTAssertEqual(ReservedHotKeys.problem(for: try combo("cmd+h"), menu: menu),
                       "⌘H is Docket's Hide Docket shortcut. Choose another shortcut.")
        XCTAssertNil(ReservedHotKeys.problem(for: try combo("shift+cmd+k"), menu: menu),
                     "Quick Capture's item shows the capture shortcut and reserves nothing")
        XCTAssertNil(ReservedHotKeys.problem(for: try combo("opt+cmd+h"), menu: menu),
                     "only a Command-only combo is checked against the menu")
        XCTAssertNil(ReservedHotKeys.problem(for: try combo("shift+cmd+q"), menu: menu))
    }

    func testTheStandardShortcutsAreRefusedWithoutAMenu() throws {
        for string in ["cmd+q", "cmd+w", "cmd+c", "cmd+v", "cmd+x", "cmd+z", "cmd+a", "cmd+tab", "cmd+space", "cmd+comma"] {
            let combo = try combo(string)
            XCTAssertEqual(ReservedHotKeys.problem(for: combo, menu: nil),
                           "\(combo.displayString) is a standard macOS shortcut other apps rely on. Choose another shortcut.",
                           string)
        }
        XCTAssertNil(ReservedHotKeys.problem(for: try combo("cmd+t"), menu: nil))
        XCTAssertNil(ReservedHotKeys.problem(for: try combo("ctrl+cmd+q"), menu: nil))
    }

    func testARefusedShortcutIsNotRegistered() throws {
        let center = HotKeyCenter()
        let problem = AppDelegate.register(try combo("cmd+w"), on: center, beside: nil,
                                           otherUse: AppDelegate.captureHotKeyUse, menu: MainMenu.make())
        XCTAssertEqual(problem, "⌘W is Docket's Close shortcut. Choose another shortcut.")
        XCTAssertEqual(center.registration, .none)
    }
}

private final class FakeStatusDelegate: StatusItemDelegate {
    var isTerminalFrontmost = false
    var canRelaunchTerminal = true
    var isLaunchAtLoginEnabled = false
    var overdueSummary: String?
    var toggleHotKey: HotKeyCombo?
    var captureHotKey: HotKeyCombo?
    var calls: [String] = []
    func toggleTerminal() { calls.append("toggle") }
    func relaunchTerminal() { calls.append("relaunch") }
    func showQuickCapture(_ sender: Any?) { calls.append("capture") }
    func showPreferences(_ sender: Any?) { calls.append("preferences") }
    func showAbout(_ sender: Any?) { calls.append("about") }
    func showGuide(_ sender: Any?) { calls.append("guide") }
    func toggleLaunchAtLogin() { calls.append("login") }
}

final class StatusItemTests: XCTestCase {
    func testMenuOffersTheShellActionsAndTracksState() throws {
        let delegate = FakeStatusDelegate()
        let controller = StatusItemController(delegate: delegate)
        XCTAssertTrue(try XCTUnwrap(controller.statusItem.button?.image).isTemplate)

        controller.refreshMenu()
        let titles = controller.menu.items.filter { !$0.isSeparatorItem && !$0.isHidden }.map(\.title)
        XCTAssertEqual(titles, [
            "Show Docket", "Quick Capture…", "Relaunch Docket", "Settings…", "Launch at Login", "About Docket",
            "Docket Specification", "Quit Docket",
        ])
        XCTAssertEqual(controller.menu.items.last?.action, #selector(NSApplication.terminate(_:)))
        XCTAssertEqual(controller.menu.items.last?.keyEquivalent, "q")
        XCTAssertEqual(controller.menu.items.last?.keyEquivalentModifierMask, .command)
        XCTAssertEqual(controller.menu.item(withTitle: "Settings…")?.keyEquivalent, ",")

        delegate.isTerminalFrontmost = true
        delegate.isLaunchAtLoginEnabled = true
        delegate.canRelaunchTerminal = false
        controller.refreshMenu()
        XCTAssertNotNil(controller.menu.item(withTitle: "Hide Docket"))
        // Always offered: the panel itself says when Docket cannot take a capture.
        XCTAssertTrue(controller.menu.item(withTitle: "Quick Capture…")?.isEnabled ?? false)
        XCTAssertFalse(controller.menu.item(withTitle: "Relaunch Docket")?.isEnabled ?? true)
        XCTAssertEqual(controller.menu.item(withTitle: "Launch at Login")?.state, .on)

        for title in ["Hide Docket", "Quick Capture…", "Relaunch Docket", "Settings…", "Launch at Login", "About Docket", "Docket Specification"] {
            let item = try XCTUnwrap(controller.menu.item(withTitle: title))
            _ = (item.target as? NSObject)?.perform(item.action, with: item)
        }
        XCTAssertEqual(delegate.calls, ["toggle", "capture", "relaunch", "preferences", "login", "about", "guide"])
    }

    /// The icon is named Docket, carries the overdue count as its value, and
    /// says what its clicks do.
    func testStatusButtonIsLabeledForVoiceOver() throws {
        let delegate = FakeStatusDelegate()
        let controller = StatusItemController(delegate: delegate)
        let button = try XCTUnwrap(controller.statusItem.button)
        XCTAssertEqual(button.accessibilityLabel(), "Docket")
        XCTAssertEqual(button.accessibilityHelp(), "Click to show or hide Docket; Control-click for the menu")
        XCTAssertNil(button.accessibilityValue())

        delegate.overdueSummary = "2 overdue"
        controller.refreshMenu()
        XCTAssertEqual(button.accessibilityLabel(), "Docket")
        XCTAssertEqual(button.accessibilityValue() as? String, "2 overdue")

        delegate.overdueSummary = nil
        controller.refreshSummary()
        XCTAssertNil(button.accessibilityValue())
    }

    func testMenuShowsTheLiveShortcuts() throws {
        let delegate = FakeStatusDelegate()
        let controller = StatusItemController(delegate: delegate)
        controller.refreshMenu()
        let toggle = try XCTUnwrap(controller.menu.item(withTitle: "Show Docket"))
        let capture = try XCTUnwrap(controller.menu.item(withTitle: "Quick Capture…"))
        XCTAssertEqual([toggle.keyEquivalent, capture.keyEquivalent], ["", ""])

        delegate.toggleHotKey = DocketHotKeyPreference.default
        delegate.captureHotKey = HotKeyCombo(string: "opt+space")
        controller.refreshMenu()
        XCTAssertEqual(toggle.keyEquivalent, "t")
        XCTAssertEqual(toggle.keyEquivalentModifierMask, [.control, .option, .command])
        XCTAssertEqual(capture.keyEquivalent, " ")
        XCTAssertEqual(capture.keyEquivalentModifierMask, [.option])

        delegate.toggleHotKey = nil
        controller.refreshMenu()
        XCTAssertEqual(toggle.keyEquivalent, "")
    }

    /// With no Dock icon the overdue count sits beside the menu-bar icon,
    /// and goes when it reaches zero.
    func testTheCountShowsBesideTheIcon() throws {
        let controller = StatusItemController(delegate: FakeStatusDelegate())
        XCTAssertNil(controller.shownCount)
        controller.showCount("3")
        XCTAssertEqual(controller.shownCount, "3")
        XCTAssertEqual(controller.statusItem.length, NSStatusItem.variableLength)
        XCTAssertEqual(controller.statusItem.button?.imagePosition, .imageLeading)
        XCTAssertNotNil(controller.statusItem.button?.image)
        controller.showCount(nil)
        XCTAssertNil(controller.shownCount)
        XCTAssertEqual(controller.statusItem.length, NSStatusItem.squareLength)
    }

    func testTheBadgeGoesWhereThereIsAnIconForIt() {
        XCTAssertEqual(DocketBadgePlacement(policy: .dockAndMenuBar, enabled: true, count: 2),
                       DocketBadgePlacement(dock: "2", statusItem: nil))
        XCTAssertEqual(DocketBadgePlacement(policy: .dockOnly, enabled: true, count: 2),
                       DocketBadgePlacement(dock: "2", statusItem: nil))
        XCTAssertEqual(DocketBadgePlacement(policy: .menuBarOnly, enabled: true, count: 2),
                       DocketBadgePlacement(dock: nil, statusItem: "2"))
        for policy in DocketActivationPolicy.allCases {
            XCTAssertEqual(DocketBadgePlacement(policy: policy, enabled: false, count: 2),
                           DocketBadgePlacement(dock: nil, statusItem: nil), "turned off")
            XCTAssertEqual(DocketBadgePlacement(policy: policy, enabled: true, count: 0),
                           DocketBadgePlacement(dock: nil, statusItem: nil), "nothing overdue")
            XCTAssertEqual(DocketBadgePlacement(policy: policy, enabled: true, count: nil),
                           DocketBadgePlacement(dock: nil, statusItem: nil), "count unknown")
        }
    }

    /// The Dock menu carries the status menu's commands: the overdue line,
    /// Show/Hide, Quick Capture…, and Relaunch Docket.
    func testTheDockMenuOffersTheOverdueLineAndTheShellActions() throws {
        let target = NSObject()
        func titles(_ menu: NSMenu) -> [String] { menu.items.map { $0.isSeparatorItem ? "-" : $0.title } }

        let empty = AppDelegate.dockMenu(overdue: nil, terminalFrontmost: false, canRelaunch: true, target: target)
        XCTAssertEqual(titles(empty), ["Show Docket", "Quick Capture…", "Relaunch Docket"])
        XCTAssertTrue(empty.item(withTitle: "Relaunch Docket")?.isEnabled ?? false)

        let menu = AppDelegate.dockMenu(overdue: "3 overdue", terminalFrontmost: true, canRelaunch: false, target: target)
        XCTAssertEqual(titles(menu), ["3 overdue", "-", "Hide Docket", "Quick Capture…", "Relaunch Docket"])
        XCTAssertFalse(menu.items[0].isEnabled, "the overdue line is a label")
        XCTAssertTrue(menu.items.dropFirst(2).allSatisfy { $0.target === target })
        XCTAssertFalse(menu.item(withTitle: "Relaunch Docket")?.isEnabled ?? true)
        XCTAssertEqual(menu.item(withTitle: "Hide Docket")?.action, #selector(AppDelegate.toggleFromMenu(_:)))
        XCTAssertEqual(menu.item(withTitle: "Quick Capture…")?.action, #selector(AppDelegate.showQuickCapture(_:)))
    }

    func testRightAndControlClicksOpenTheMenu() throws {
        func mouse(_ type: NSEvent.EventType, _ flags: NSEvent.ModifierFlags = []) -> NSEvent? {
            NSEvent.mouseEvent(with: type, location: .zero, modifierFlags: flags, timestamp: 0,
                               windowNumber: 0, context: nil, eventNumber: 0, clickCount: 1, pressure: 0)
        }
        XCTAssertFalse(StatusItemController.opensMenu(mouse(.leftMouseUp)))
        XCTAssertTrue(StatusItemController.opensMenu(mouse(.rightMouseUp)))
        XCTAssertTrue(StatusItemController.opensMenu(mouse(.leftMouseUp, .control)))
    }
}
