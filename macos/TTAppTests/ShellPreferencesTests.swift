import AppKit
import Carbon
import UserNotifications
import XCTest
@testable import tt

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
        XCTAssertEqual(TTHotKeyPreference.load(from: defaults)?.string, "ctrl+opt+cmd+t")
        TTHotKeyPreference.save(nil, to: defaults)
        XCTAssertNil(TTHotKeyPreference.load(from: defaults))
        TTHotKeyPreference.save(HotKeyCombo(string: "shift+cmd+space"), to: defaults)
        XCTAssertEqual(defaults.string(forKey: TTHotKeyPreference.defaultsKey), "shift+cmd+space")
        XCTAssertEqual(TTHotKeyPreference.load(from: defaults)?.string, "shift+cmd+space")
        defaults.set("shift+t", forKey: TTHotKeyPreference.defaultsKey)
        XCTAssertEqual(TTHotKeyPreference.load(from: defaults), TTHotKeyPreference.default)
    }

    func testActivationPolicyDefaultsToTheDockAndRoundTrips() {
        XCTAssertEqual(TTActivationPolicy.load(from: defaults), .dockAndMenuBar)
        XCTAssertEqual(TTActivationPolicy.dockAndMenuBar.appKitPolicy, .regular)
        XCTAssertEqual(TTActivationPolicy.menuBarOnly.appKitPolicy, .accessory)
        XCTAssertEqual(TTActivationPolicy.dockOnly.appKitPolicy, .regular)
        XCTAssertEqual(TTActivationPolicy.allCases.filter { !$0.showsStatusItem }, [.dockOnly])
        defaults.set("nonsense", forKey: TTActivationPolicy.defaultsKey)
        XCTAssertEqual(TTActivationPolicy.load(from: defaults), .dockAndMenuBar)
        TTActivationPolicy.menuBarOnly.save(to: defaults)
        XCTAssertEqual(TTActivationPolicy.load(from: defaults), .menuBarOnly)
        TTActivationPolicy.dockOnly.save(to: defaults)
        XCTAssertEqual(defaults.string(forKey: TTActivationPolicy.defaultsKey), "dock")
        XCTAssertEqual(TTActivationPolicy.load(from: defaults), .dockOnly)
    }

    func testDefaultFontSizeIsUnsetUntilSavedAndRefusesNonsense() {
        XCTAssertNil(TTDefaultFontSize.load(from: defaults))
        TTDefaultFontSize.save(15, to: defaults)
        XCTAssertEqual(TTDefaultFontSize.load(from: defaults), 15)
        TTDefaultFontSize.save(500, to: defaults)
        XCTAssertEqual(TTDefaultFontSize.load(from: defaults), TTDefaultFontSize.range.upperBound)
        for bad: Any in ["big", 2, 0, -4] {
            defaults.set(bad, forKey: TTDefaultFontSize.defaultsKey)
            XCTAssertNil(TTDefaultFontSize.load(from: defaults), "\(bad)")
        }
        TTDefaultFontSize.save(nil, to: defaults)
        XCTAssertNil(defaults.object(forKey: TTDefaultFontSize.defaultsKey))
        XCTAssertEqual(TTDefaultFontSize.label(14), "14 pt")
        XCTAssertEqual(TTDefaultFontSize.label(13.5), "13.5 pt")
    }

    func testEnvironmentOverridesKeepKnownSingleLineValuesOnly() {
        TTEnvironmentOverrides.save([
            "EDITOR": "  hx  ", "VISUAL": "", "PATH": "a\nb", "HOME": "/elsewhere", "TT_DATA_DIR": "~/tt",
        ], to: defaults)
        XCTAssertEqual(TTEnvironmentOverrides.load(from: defaults), ["EDITOR": "hx", "TT_DATA_DIR": "~/tt"])
        TTEnvironmentOverrides.save([:], to: defaults)
        XCTAssertNil(defaults.object(forKey: TTEnvironmentOverrides.defaultsKey))
        XCTAssertEqual(TTEnvironmentOverrides.names.first, "PATH")
    }

    func testOverridesReplaceTheShellsValuesAndKeepTheRest() {
        let shell = LaunchEnvironment(home: "/Users/me", path: "/bin", forwarded: ["EDITOR": "nvim", "VISUAL": "nvim"])
        let applied = shell.applying(["PATH": "/opt/bin:/bin", "EDITOR": "hx", "TT_DATA_DIR": "/data", "HOME": "/x"])
        XCTAssertEqual(applied.commandEnvironment, [
            "HOME": "/Users/me", "PATH": "/opt/bin:/bin", "EDITOR": "hx", "VISUAL": "nvim", "TT_DATA_DIR": "/data",
        ])
        XCTAssertEqual(applied.value(of: "PATH"), "/opt/bin:/bin")
        XCTAssertNil(shell.value(of: "TT_EDITOR"))
    }

    func testAnEnvironmentEditAwaitsARelaunchOnlyWhileTheOldValuesRun() {
        let model = PreferencesModel(defaults: defaults)
        var changes = 0
        model.onEnvironmentChange = { changes += 1 }
        model.detectedEnvironment = LaunchEnvironment(home: "/h", path: "/bin", forwarded: ["EDITOR": "nvim"])
        XCTAssertFalse(model.environmentAwaitsRelaunch, "no session runs")

        model.sessionEnvironment = model.effectiveEnvironment?.commandEnvironment
        XCTAssertFalse(model.environmentAwaitsRelaunch)

        model.environmentOverrides = ["EDITOR": "hx"]
        XCTAssertTrue(model.environmentAwaitsRelaunch)
        XCTAssertEqual(changes, 1)
        XCTAssertEqual(PreferencesModel(defaults: defaults).environmentOverrides, ["EDITOR": "hx"], "saved")

        model.environmentOverrides = [:]
        XCTAssertFalse(model.environmentAwaitsRelaunch, "back to what runs")
    }

    func testABinaryAwaitsARelaunchOnlyWhenTheNextOneIsAnotherPath() {
        let bundled = TTBinaryResolver.Resolution(url: URL(fileURLWithPath: "/a/bin/tt"), source: .bundled)
        let same = TTBinaryResolver.Resolution(url: URL(fileURLWithPath: "/a/bin/../bin/tt"), source: .userDefaultsOverride)
        let other = TTBinaryResolver.Resolution(url: URL(fileURLWithPath: "/b/tt"), source: .userDefaultsOverride)
        XCTAssertFalse(PreferencesModel.binaryAwaitsRelaunch(running: nil, next: other), "no session runs")
        XCTAssertFalse(PreferencesModel.binaryAwaitsRelaunch(running: bundled, next: nil), "the last tt keeps running")
        XCTAssertFalse(PreferencesModel.binaryAwaitsRelaunch(running: bundled, next: same), "one path, two spellings")
        XCTAssertTrue(PreferencesModel.binaryAwaitsRelaunch(running: bundled, next: other))

        let model = PreferencesModel(defaults: defaults)
        var next: TTBinaryResolver.Resolution? = bundled
        model.resolveNextBinary = { _ in next }
        model.binaryOverride = "/b/tt"
        XCTAssertFalse(model.binaryAwaitsRelaunch, "nothing runs yet")

        model.runningBinary = bundled
        model.refreshBinaryStatus()
        XCTAssertFalse(model.binaryAwaitsRelaunch)

        next = other
        model.binaryOverride = "/b/tt "
        XCTAssertTrue(model.binaryAwaitsRelaunch, "the edit is asked about as it is made")

        model.runningBinary = other
        model.refreshBinaryStatus()
        XCTAssertFalse(model.binaryAwaitsRelaunch, "the relaunched tt is the one named")
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

    func testBinaryStatusNamesTheRunningVersionAndAnOlderCopy() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent("tt-skew-\(UUID().uuidString)")
        defer { try? FileManager.default.removeItem(at: dir) }
        let bundled = dir.appendingPathComponent("bundled/tt")
        let local = dir.appendingPathComponent("local/tt")
        for url in [bundled, local] {
            try FileManager.default.createDirectory(at: url.deletingLastPathComponent(), withIntermediateDirectories: true)
            try "#!/bin/sh\n".write(to: url, atomically: true, encoding: .utf8)
            try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: url.path)
        }
        let old = "tt version a (commit: a, built: 2026-01-01T00:00:00Z)"
        let new = "tt version b (commit: b, built: 2026-02-01T00:00:00Z)"
        let versions = [bundled.path: new, local.path: old]

        let model = PreferencesModel(defaults: defaults)
        model.bundledBinaryPath = bundled.path
        model.localBinaryPath = local.path
        model.readVersion = { versions[$0].flatMap(TTVersion.parse) }
        model.refreshBinaryStatus()
        XCTAssertNil(model.runningBinaryNote, "nothing runs yet")

        model.runningBinary = .init(url: bundled, source: .bundled)
        model.runningVersion = TTVersion.parse(new)
        model.refreshBinaryStatus()
        XCTAssertEqual(model.runningBinaryNote, "Running \(bundled.path): \(new)")
        XCTAssertTrue(model.binarySkewNote?.contains("is older (\(old))") ?? false)

        model.runningBinary = .init(url: local, source: .newerLocalBin)
        model.readVersion = { [bundled.path: old, local.path: new][$0].flatMap(TTVersion.parse) }
        model.refreshBinaryStatus()
        XCTAssertTrue(model.binarySkewNote?.hasPrefix("Chosen over the older bundled copy") ?? false)

        model.runningBinary = .init(url: bundled, source: .bundled)
        model.readVersion = { _ in TTVersion.parse(new) }
        model.refreshBinaryStatus()
        XCTAssertNil(model.binarySkewNote, "matching copies say nothing")
    }

    func testSamePathReplacementKeepsRunningVersionUntilRelaunch() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: dir) }
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        let path = dir.appendingPathComponent("tt")
        try "old executable".write(to: path, atomically: true, encoding: .utf8)
        let resolution = TTBinaryResolver.Resolution(url: path, source: .userDefaultsOverride)
        let old = TTVersion.parse("tt version old")!
        let new = TTVersion.parse("tt version new")!
        let model = PreferencesModel(defaults: defaults)
        model.runningBinary = resolution
        model.runningVersion = old
        model.runningFingerprint = TTBinaryFingerprint.read(path.path)
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
        model.runningFingerprint = TTBinaryFingerprint.read(path.path)
        model.refreshBinaryStatus()
        XCTAssertFalse(model.binaryAwaitsRelaunch)
        XCTAssertNil(model.nextBinaryNote)
        XCTAssertTrue(model.runningBinaryNote?.hasSuffix(new.line) ?? false)
    }

    func testTheModelStartsFromTheDefaultsAndSavesEachChange() {
        let model = PreferencesModel(defaults: defaults)
        XCTAssertEqual(model.exitBehavior, .relaunchOnShow)
        XCTAssertEqual(model.activationPolicy, .dockAndMenuBar)
        XCTAssertEqual(model.hotKey, TTHotKeyPreference.default)
        XCTAssertEqual(model.binaryOverride, "")
        XCTAssertNil(model.binaryOverrideProblem)
        XCTAssertNil(model.defaultFontSize)

        var policies: [TTActivationPolicy] = []
        var hotKeys: [HotKeyCombo?] = []
        var fontSizes: [Float?] = []
        model.onActivationPolicyChange = { policies.append($0) }
        model.onHotKeyChange = { hotKeys.append($0) }
        model.onDefaultFontSizeChange = { fontSizes.append($0) }

        model.defaultFontSize = 16
        model.defaultFontSize = 16
        XCTAssertEqual(TTDefaultFontSize.load(from: defaults), 16)
        XCTAssertEqual(PreferencesModel(defaults: defaults).defaultFontSize, 16, "the next launch starts from it")
        model.defaultFontSize = nil
        XCTAssertEqual(fontSizes, [16, nil])
        XCTAssertNil(defaults.object(forKey: TTDefaultFontSize.defaultsKey))

        model.activationPolicy = .menuBarOnly
        model.activationPolicy = .menuBarOnly
        model.hotKey = nil
        model.exitBehavior = .quitApp
        model.binaryOverride = " /no/such/tt "

        XCTAssertEqual(policies, [.menuBarOnly], "an unchanged value applies nothing")
        XCTAssertEqual(hotKeys, [nil])
        XCTAssertEqual(TTActivationPolicy.load(from: defaults), .menuBarOnly)
        XCTAssertNil(TTHotKeyPreference.load(from: defaults))
        XCTAssertEqual(TTExitBehavior.load(from: defaults), .quitApp)
        XCTAssertEqual(defaults.string(forKey: TTBinaryResolver.overrideDefaultsKey), "/no/such/tt")
        XCTAssertNotNil(model.binaryOverrideProblem)

        model.binaryOverride = "/bin/sh"
        XCTAssertNil(model.binaryOverrideProblem)
        model.binaryOverride = "  "
        XCTAssertNil(defaults.object(forKey: TTBinaryResolver.overrideDefaultsKey))
    }

    /// Restore Default shows while a shortcut is another combo or off, and
    /// never while recording.
    func testRestoreDefaultIsOfferedOnlyAwayFromTheDefault() throws {
        for preset in [TTHotKeyPreference.default, TTCaptureHotKeyPreference.default] {
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
        model.readNotificationAuthorization = { $0(.authorized) }
        model.ghosttyConfigReport = problems
        model.refresh()
        XCTAssertNil(model.ghosttyConfigReport)
    }

    /// Settings reads macOS's answer as it opens; only a refusal is shown.
    func testDeniedNotificationsAreReadWhenSettingsOpens() throws {
        let model = PreferencesModel(defaults: defaults)
        var status = UNAuthorizationStatus.denied
        model.readNotificationAuthorization = { $0(status) }
        XCTAssertFalse(model.notificationsDenied)
        model.refresh()
        XCTAssertTrue(model.notificationsDenied)
        status = .notDetermined
        model.refresh()
        XCTAssertFalse(model.notificationsDenied, "not yet asked is not refused")

        // Off the main thread too, as UserNotifications answers.
        status = .denied
        model.readNotificationAuthorization = { done in DispatchQueue.global().async { done(status) } }
        model.refresh()
        let deadline = Date().addingTimeInterval(2)
        while !model.notificationsDenied, Date() < deadline {
            RunLoop.main.run(until: Date().addingTimeInterval(0.01))
        }
        XCTAssertTrue(model.notificationsDenied)

        XCTAssertEqual(NotificationSettingsLink.url(bundleIdentifier: "io.github.sglavoie.tt").absoluteString,
                       "x-apple.systempreferences:com.apple.Notifications-Settings.extension?id=io.github.sglavoie.tt")
    }

    func testTheExitChoicesAreWorded() {
        XCTAssertEqual(TTExitBehavior.allCases.map(\.label), ["Hide, and start a fresh tt next time", "Quit tt"])
    }

    func testFollowingTheAppearanceDefaultsOnAndSavesEachChange() {
        XCTAssertTrue(TTFollowAppearancePreference.load(from: defaults))
        let model = PreferencesModel(defaults: defaults)
        XCTAssertTrue(model.followAppearance)

        var changes: [Bool] = []
        model.onFollowAppearanceChange = { changes.append($0) }
        model.followAppearance = false
        model.followAppearance = false
        XCTAssertEqual(changes, [false], "an unchanged value applies nothing")
        XCTAssertFalse(TTFollowAppearancePreference.load(from: defaults))
        XCTAssertFalse(PreferencesModel(defaults: defaults).followAppearance, "the next launch starts from it")

        model.followAppearance = true
        XCTAssertEqual(changes, [false, true])
        XCTAssertTrue(TTFollowAppearancePreference.load(from: defaults))
    }
}

final class TerminalToggleTests: XCTestCase {
    func testHidesOnlyWhenTheWindowIsAlreadyInFront() {
        XCTAssertEqual(TerminalToggle.action(appIsActive: true, windowIsVisible: true, windowIsKey: true), .hide)
        XCTAssertEqual(TerminalToggle.action(appIsActive: false, windowIsVisible: true, windowIsKey: true), .show)
        XCTAssertEqual(TerminalToggle.action(appIsActive: true, windowIsVisible: false, windowIsKey: false), .show)
        // Preferences in front: the shortcut brings tt forward rather than hiding.
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
        XCTAssertEqual(registerCapture(first), "⌃⌥⇧⌘F19 already shows and hides tt. Choose another shortcut.")
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
                       "⌘Q is tt's Quit tt shortcut. Choose another shortcut.")
        XCTAssertEqual(ReservedHotKeys.problem(for: try combo("cmd+comma"), menu: menu),
                       "⌘, is tt's Settings shortcut. Choose another shortcut.")
        XCTAssertEqual(ReservedHotKeys.problem(for: try combo("cmd+h"), menu: menu),
                       "⌘H is tt's Hide tt shortcut. Choose another shortcut.")
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
        XCTAssertEqual(problem, "⌘W is tt's Close shortcut. Choose another shortcut.")
        XCTAssertEqual(center.registration, .none)
    }
}

private final class FakeStatusDelegate: StatusItemDelegate {
    var isTerminalFrontmost = false
    var canRelaunchTerminal = true
    var isLaunchAtLoginEnabled = false
    var pulseSummary: String?
    var pulse: TTPulse?
    var toggleHotKey: HotKeyCombo?
    var captureHotKey: HotKeyCombo?
    var calls: [String] = []
    func toggleTerminal() { calls.append("toggle") }
    func showView(_ view: TTView) { calls.append("view:\(view.rawValue)") }
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
            "Show tt", "Quick Capture…", "Relaunch tt", "Settings…", "Launch at Login", "About tt", "tt Guide", "Quit tt",
        ])
        XCTAssertEqual(controller.menu.items.last?.action, #selector(NSApplication.terminate(_:)))
        XCTAssertEqual(controller.menu.items.last?.keyEquivalent, "q")
        XCTAssertEqual(controller.menu.items.last?.keyEquivalentModifierMask, .command)
        XCTAssertEqual(controller.menu.item(withTitle: "Settings…")?.keyEquivalent, ",")

        delegate.isTerminalFrontmost = true
        delegate.isLaunchAtLoginEnabled = true
        delegate.canRelaunchTerminal = false
        controller.refreshMenu()
        XCTAssertNotNil(controller.menu.item(withTitle: "Hide tt"))
        // Always offered: the panel itself says when tt cannot take a capture.
        XCTAssertTrue(controller.menu.item(withTitle: "Quick Capture…")?.isEnabled ?? false)
        XCTAssertFalse(controller.menu.item(withTitle: "Relaunch tt")?.isEnabled ?? true)
        XCTAssertEqual(controller.menu.item(withTitle: "Launch at Login")?.state, .on)

        for title in ["Hide tt", "Quick Capture…", "Relaunch tt", "Settings…", "Launch at Login", "About tt", "tt Guide"] {
            let item = try XCTUnwrap(controller.menu.item(withTitle: title))
            _ = (item.target as? NSObject)?.perform(item.action, with: item)
        }
        XCTAssertEqual(delegate.calls, ["toggle", "capture", "relaunch", "preferences", "login", "about", "guide"])
    }

    /// Each count still owed leads the menu as an enabled item showing tt in
    /// its mode; a zero count takes no row, nor does a pulse tt never sent.
    /// The accessibility value keeps the one-line pulse.
    func testMenuLeadsWithOneItemPerOwedCount() throws {
        let delegate = FakeStatusDelegate()
        let controller = StatusItemController(delegate: delegate)
        controller.refreshMenu()
        XCTAssertTrue(controller.pulseItems.isEmpty)
        XCTAssertTrue(controller.menu.items[0].isSeparatorItem)
        XCTAssertTrue(controller.menu.items[0].isHidden)

        delegate.pulse = TTPulse(inbox: 2, today: 0, overdue: 1, done: 4)
        delegate.pulseSummary = "inbox 2 · overdue 1"
        controller.refreshMenu()
        XCTAssertEqual(controller.menu.items.prefix(2).map(\.title), ["Inbox 2", "Overdue 1"])
        // Plain words and digits, so VoiceOver reads each as it is shown.
        for item in controller.pulseItems {
            XCTAssertTrue(item.title.allSatisfy { $0.isLetter || $0.isNumber || $0 == " " }, item.title)
            XCTAssertNil(item.attributedTitle)
        }
        XCTAssertEqual(controller.statusItem.button?.accessibilityValue() as? String, "inbox 2 · overdue 1")
        XCTAssertTrue(controller.menu.items[2].isSeparatorItem)
        XCTAssertFalse(controller.menu.items[2].isHidden)
        XCTAssertEqual(controller.menu.items[3].title, "Show tt")
        for item in controller.menu.items.prefix(2) {
            XCTAssertTrue(item.isEnabled)
            XCTAssertFalse(item.isHidden)
            _ = (item.target as? NSObject)?.perform(item.action, with: item)
        }
        XCTAssertEqual(delegate.calls, ["view:inbox", "view:due"])

        delegate.pulse = TTPulse(inbox: 0, today: 3, overdue: 0, done: 0)
        delegate.pulseSummary = "today 3"
        controller.refreshPulse()
        XCTAssertEqual(controller.pulseItems.map(\.title), ["Today 3"])
        XCTAssertEqual(controller.menu.items.filter { $0.title.hasPrefix("Inbox") }.count, 0)

        delegate.pulse = TTPulse(inbox: 0, today: 0, overdue: 0, done: 2)
        delegate.pulseSummary = nil
        controller.refreshPulse()
        XCTAssertTrue(controller.pulseItems.isEmpty)
        XCTAssertNil(controller.statusItem.button?.accessibilityValue())
        XCTAssertTrue(controller.menu.items[0].isSeparatorItem)
        XCTAssertTrue(controller.menu.items[0].isHidden)
    }

    /// The icon is named tt, carries the pulse as its value, and says what
    /// its clicks do.
    func testStatusButtonIsLabeledForVoiceOver() throws {
        let delegate = FakeStatusDelegate()
        let controller = StatusItemController(delegate: delegate)
        let button = try XCTUnwrap(controller.statusItem.button)
        XCTAssertEqual(button.accessibilityLabel(), "tt")
        XCTAssertEqual(button.accessibilityHelp(), "Click to show or hide tt; Control-click for the menu")
        XCTAssertNil(button.accessibilityValue())

        delegate.pulse = TTPulse(inbox: 1, today: 2, overdue: 0, done: 0)
        delegate.pulseSummary = "inbox 1 · today 2"
        controller.refreshMenu()
        XCTAssertEqual(button.accessibilityLabel(), "tt")
        XCTAssertEqual(button.accessibilityValue() as? String, "inbox 1 · today 2")
    }

    func testMenuShowsTheLiveShortcuts() throws {
        let delegate = FakeStatusDelegate()
        let controller = StatusItemController(delegate: delegate)
        controller.refreshMenu()
        let toggle = try XCTUnwrap(controller.menu.item(withTitle: "Show tt"))
        let capture = try XCTUnwrap(controller.menu.item(withTitle: "Quick Capture…"))
        XCTAssertEqual([toggle.keyEquivalent, capture.keyEquivalent], ["", ""])

        delegate.toggleHotKey = TTHotKeyPreference.default
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
        XCTAssertEqual(TTBadgePlacement(policy: .dockAndMenuBar, enabled: true, label: "2"),
                       TTBadgePlacement(dock: "2", statusItem: nil))
        XCTAssertEqual(TTBadgePlacement(policy: .dockOnly, enabled: true, label: "2"),
                       TTBadgePlacement(dock: "2", statusItem: nil))
        XCTAssertEqual(TTBadgePlacement(policy: .menuBarOnly, enabled: true, label: "2"),
                       TTBadgePlacement(dock: nil, statusItem: "2"))
        XCTAssertEqual(TTBadgePlacement(policy: .menuBarOnly, enabled: false, label: "2"),
                       TTBadgePlacement(dock: nil, statusItem: nil))
        XCTAssertEqual(TTBadgePlacement(policy: .dockAndMenuBar, enabled: true, label: nil),
                       TTBadgePlacement(dock: nil, statusItem: nil))
    }

    /// The Dock menu carries the status menu's commands: the counts owed,
    /// Show/Hide, Quick Capture…, and Relaunch tt.
    func testTheDockMenuOffersThePulseAndTheShellActions() throws {
        let target = NSObject()
        func titles(_ menu: NSMenu) -> [String] { menu.items.map { $0.isSeparatorItem ? "-" : $0.title } }

        let empty = AppDelegate.dockMenu(pulse: nil, terminalFrontmost: false, canRelaunch: true, target: target)
        XCTAssertEqual(titles(empty), ["Show tt", "Quick Capture…", "Relaunch tt"])

        let menu = AppDelegate.dockMenu(
            pulse: TTPulse(inbox: 2, today: 0, overdue: 1, done: 5), terminalFrontmost: true, canRelaunch: false,
            target: target
        )
        XCTAssertEqual(titles(menu), ["Inbox 2", "Overdue 1", "-", "Hide tt", "Quick Capture…", "Relaunch tt"])
        XCTAssertEqual(menu.items.prefix(2).compactMap { $0.representedObject as? String }, ["inbox", "due"])
        XCTAssertTrue(menu.items.allSatisfy { $0.isSeparatorItem || $0.target === target })
        XCTAssertFalse(menu.item(withTitle: "Relaunch tt")?.isEnabled ?? true)
        XCTAssertEqual(menu.item(withTitle: "Hide tt")?.action, #selector(AppDelegate.toggleFromMenu(_:)))
        XCTAssertEqual(menu.items[0].action, #selector(AppDelegate.showViewFromMenu(_:)))
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
