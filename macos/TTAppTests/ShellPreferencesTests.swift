import AppKit
import Carbon
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
        defaults.set("nonsense", forKey: TTActivationPolicy.defaultsKey)
        XCTAssertEqual(TTActivationPolicy.load(from: defaults), .dockAndMenuBar)
        TTActivationPolicy.menuBarOnly.save(to: defaults)
        XCTAssertEqual(TTActivationPolicy.load(from: defaults), .menuBarOnly)
    }

    func testTheModelStartsFromTheDefaultsAndSavesEachChange() {
        let model = PreferencesModel(defaults: defaults)
        XCTAssertEqual(model.exitBehavior, .relaunchOnShow)
        XCTAssertEqual(model.activationPolicy, .dockAndMenuBar)
        XCTAssertEqual(model.hotKey, TTHotKeyPreference.default)
        XCTAssertEqual(model.binaryOverride, "")
        XCTAssertNil(model.binaryOverrideProblem)

        var policies: [TTActivationPolicy] = []
        var hotKeys: [HotKeyCombo?] = []
        model.onActivationPolicyChange = { policies.append($0) }
        model.onHotKeyChange = { hotKeys.append($0) }

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
        XCTAssertTrue(HotKeyCenter.Registration.failed(combo, -1).problem?.contains("error -1") == true)
    }

    func testSystemShortcutsAreReadable() {
        // Whatever the machine's settings, reading them must not fail or
        // yield combos the recorder itself would refuse.
        for combo in SystemHotKeys.enabledCombos() {
            XCTAssertEqual(HotKeyCombo(string: combo.string), combo)
        }
    }
}

private final class FakeStatusDelegate: StatusItemDelegate {
    var isTerminalFrontmost = false
    var canRelaunchTerminal = true
    var isLaunchAtLoginEnabled = false
    var calls: [String] = []
    func toggleTerminal() { calls.append("toggle") }
    func relaunchTerminal() { calls.append("relaunch") }
    func showPreferences(_ sender: Any?) { calls.append("preferences") }
    func toggleLaunchAtLogin() { calls.append("login") }
}

final class StatusItemTests: XCTestCase {
    func testMenuOffersTheShellActionsAndTracksState() throws {
        let delegate = FakeStatusDelegate()
        let controller = StatusItemController(delegate: delegate)
        XCTAssertTrue(try XCTUnwrap(controller.statusItem.button?.image).isTemplate)

        controller.refreshMenu()
        let titles = controller.menu.items.filter { !$0.isSeparatorItem }.map(\.title)
        XCTAssertEqual(titles, ["Show tt", "Relaunch tt", "Preferences…", "Launch at Login", "Quit tt"])
        XCTAssertEqual(controller.menu.items.last?.action, #selector(NSApplication.terminate(_:)))

        delegate.isTerminalFrontmost = true
        delegate.isLaunchAtLoginEnabled = true
        delegate.canRelaunchTerminal = false
        controller.refreshMenu()
        XCTAssertEqual(controller.menu.items[0].title, "Hide tt")
        XCTAssertFalse(controller.menu.items[1].isEnabled)
        XCTAssertEqual(controller.menu.item(withTitle: "Launch at Login")?.state, .on)

        for title in ["Hide tt", "Relaunch tt", "Preferences…", "Launch at Login"] {
            let item = try XCTUnwrap(controller.menu.item(withTitle: title))
            _ = (item.target as? NSObject)?.perform(item.action, with: item)
        }
        XCTAssertEqual(delegate.calls, ["toggle", "relaunch", "preferences", "login"])
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
