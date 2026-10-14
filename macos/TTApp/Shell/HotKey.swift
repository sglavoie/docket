import AppKit
import Carbon

/// One global shortcut: a virtual key code and Carbon modifier bits, as
/// `RegisterEventHotKey` takes them. Its stored spelling is lowercase tokens
/// joined by `+`, modifiers first in the order macOS displays them, e.g.
/// `ctrl+opt+cmd+t`.
struct HotKeyCombo: Equatable, Hashable {
    let keyCode: UInt32
    /// Carbon bits: `controlKey`, `optionKey`, `shiftKey`, `cmdKey`.
    let modifiers: UInt32

    static let modifierMask = UInt32(controlKey | optionKey | shiftKey | cmdKey)

    /// Refuses a combo that would swallow ordinary typing: it needs Control,
    /// Option, or Command, unless the key is a function key.
    init?(keyCode: UInt32, modifiers: UInt32) {
        guard Self.keys[keyCode] != nil else { return nil }
        let modifiers = modifiers & Self.modifierMask
        let chord = modifiers & UInt32(controlKey | optionKey | cmdKey)
        guard chord != 0 || Self.isFunctionKey(keyCode) else { return nil }
        self.keyCode = keyCode
        self.modifiers = modifiers
    }

    /// A key pressed in the recorder.
    init?(event: NSEvent) {
        self.init(keyCode: UInt32(event.keyCode), modifiers: Self.carbonModifiers(event.modifierFlags))
    }

    /// Parses the stored spelling; modifier aliases (`control`, `option`,
    /// `alt`, `command`) and any order or case are accepted.
    init?(string: String) {
        let tokens = string.lowercased().split(separator: "+", omittingEmptySubsequences: false)
            .map { $0.trimmingCharacters(in: .whitespaces) }
        guard let keyName = tokens.last, !keyName.isEmpty,
              let keyCode = Self.keyCodesByName[keyName] else { return nil }
        var modifiers: UInt32 = 0
        for token in tokens.dropLast() {
            guard let bit = Self.modifierAliases[token], modifiers & bit == 0 else { return nil }
            modifiers |= bit
        }
        self.init(keyCode: keyCode, modifiers: modifiers)
    }

    /// The canonical stored spelling.
    var string: String {
        let names = Self.modifierOrder.filter { modifiers & $0.bit != 0 }.map(\.name)
        return (names + [Self.keys[keyCode]?.name ?? ""]).joined(separator: "+")
    }

    /// The menu-style label, e.g. `⌃⌥⌘T`.
    var displayString: String {
        Self.modifierOrder.filter { modifiers & $0.bit != 0 }.map(\.symbol).joined()
            + (Self.keys[keyCode]?.symbol ?? "?")
    }

    /// The combo as a menu item spells one: the key's character and its AppKit
    /// modifiers. Carbon takes the press before any menu sees it, so the item
    /// shows the shortcut without answering it a second time.
    var menuKeyEquivalent: (key: String, modifiers: NSEvent.ModifierFlags) {
        var flags: NSEvent.ModifierFlags = []
        if modifiers & UInt32(controlKey) != 0 { flags.insert(.control) }
        if modifiers & UInt32(optionKey) != 0 { flags.insert(.option) }
        if modifiers & UInt32(shiftKey) != 0 { flags.insert(.shift) }
        if modifiers & UInt32(cmdKey) != 0 { flags.insert(.command) }
        return (Self.menuKeys[keyCode] ?? Self.keys[keyCode]?.symbol.lowercased() ?? "", flags)
    }

    static func carbonModifiers(_ flags: NSEvent.ModifierFlags) -> UInt32 {
        var bits: UInt32 = 0
        if flags.contains(.control) { bits |= UInt32(controlKey) }
        if flags.contains(.option) { bits |= UInt32(optionKey) }
        if flags.contains(.shift) { bits |= UInt32(shiftKey) }
        if flags.contains(.command) { bits |= UInt32(cmdKey) }
        return bits
    }

    static func isFunctionKey(_ keyCode: UInt32) -> Bool {
        guard let name = keys[keyCode]?.name, name.hasPrefix("f") else { return false }
        return Int(name.dropFirst()) != nil
    }

    // MARK: Tables

    private static let modifierOrder: [(bit: UInt32, name: String, symbol: String)] = [
        (UInt32(controlKey), "ctrl", "⌃"),
        (UInt32(optionKey), "opt", "⌥"),
        (UInt32(shiftKey), "shift", "⇧"),
        (UInt32(cmdKey), "cmd", "⌘"),
    ]

    private static let modifierAliases: [String: UInt32] = [
        "ctrl": UInt32(controlKey), "control": UInt32(controlKey),
        "opt": UInt32(optionKey), "option": UInt32(optionKey), "alt": UInt32(optionKey),
        "shift": UInt32(shiftKey),
        "cmd": UInt32(cmdKey), "command": UInt32(cmdKey),
    ]

    /// Keys by their US-layout virtual key code, as the hardware reports them.
    static let keys: [UInt32: (name: String, symbol: String)] = {
        var table: [Int: (String, String)] = [
            kVK_ANSI_A: ("a", "A"), kVK_ANSI_B: ("b", "B"), kVK_ANSI_C: ("c", "C"),
            kVK_ANSI_D: ("d", "D"), kVK_ANSI_E: ("e", "E"), kVK_ANSI_F: ("f", "F"),
            kVK_ANSI_G: ("g", "G"), kVK_ANSI_H: ("h", "H"), kVK_ANSI_I: ("i", "I"),
            kVK_ANSI_J: ("j", "J"), kVK_ANSI_K: ("k", "K"), kVK_ANSI_L: ("l", "L"),
            kVK_ANSI_M: ("m", "M"), kVK_ANSI_N: ("n", "N"), kVK_ANSI_O: ("o", "O"),
            kVK_ANSI_P: ("p", "P"), kVK_ANSI_Q: ("q", "Q"), kVK_ANSI_R: ("r", "R"),
            kVK_ANSI_S: ("s", "S"), kVK_ANSI_T: ("t", "T"), kVK_ANSI_U: ("u", "U"),
            kVK_ANSI_V: ("v", "V"), kVK_ANSI_W: ("w", "W"), kVK_ANSI_X: ("x", "X"),
            kVK_ANSI_Y: ("y", "Y"), kVK_ANSI_Z: ("z", "Z"),
            kVK_ANSI_0: ("0", "0"), kVK_ANSI_1: ("1", "1"), kVK_ANSI_2: ("2", "2"),
            kVK_ANSI_3: ("3", "3"), kVK_ANSI_4: ("4", "4"), kVK_ANSI_5: ("5", "5"),
            kVK_ANSI_6: ("6", "6"), kVK_ANSI_7: ("7", "7"), kVK_ANSI_8: ("8", "8"),
            kVK_ANSI_9: ("9", "9"),
            kVK_ANSI_Minus: ("minus", "-"), kVK_ANSI_Equal: ("equal", "="),
            kVK_ANSI_LeftBracket: ("leftbracket", "["), kVK_ANSI_RightBracket: ("rightbracket", "]"),
            kVK_ANSI_Quote: ("quote", "'"), kVK_ANSI_Semicolon: ("semicolon", ";"),
            kVK_ANSI_Backslash: ("backslash", "\\"), kVK_ANSI_Comma: ("comma", ","),
            kVK_ANSI_Slash: ("slash", "/"), kVK_ANSI_Period: ("period", "."),
            kVK_ANSI_Grave: ("grave", "`"),
            kVK_Return: ("return", "↩"), kVK_Tab: ("tab", "⇥"), kVK_Space: ("space", "Space"),
            kVK_Delete: ("delete", "⌫"), kVK_ForwardDelete: ("forwarddelete", "⌦"),
            kVK_Escape: ("escape", "⎋"),
            kVK_LeftArrow: ("left", "←"), kVK_RightArrow: ("right", "→"),
            kVK_UpArrow: ("up", "↑"), kVK_DownArrow: ("down", "↓"),
            kVK_Home: ("home", "↖"), kVK_End: ("end", "↘"),
            kVK_PageUp: ("pageup", "⇞"), kVK_PageDown: ("pagedown", "⇟"),
        ]
        let functionKeys = [
            kVK_F1, kVK_F2, kVK_F3, kVK_F4, kVK_F5, kVK_F6, kVK_F7, kVK_F8, kVK_F9, kVK_F10,
            kVK_F11, kVK_F12, kVK_F13, kVK_F14, kVK_F15, kVK_F16, kVK_F17, kVK_F18, kVK_F19, kVK_F20,
        ]
        for (index, code) in functionKeys.enumerated() {
            table[code] = ("f\(index + 1)", "F\(index + 1)")
        }
        return Dictionary(uniqueKeysWithValues: table.map { (UInt32($0.key), (name: $0.value.0, symbol: $0.value.1)) })
    }()

    /// The keys a menu spells with something other than their lowercased symbol.
    private static let menuKeys: [UInt32: String] = {
        func unicode(_ code: Int) -> String { String(UnicodeScalar(UInt32(code)).map(Character.init) ?? " ") }
        var table: [Int: String] = [
            kVK_Return: "\r", kVK_Tab: "\t", kVK_Space: " ",
            kVK_Delete: "\u{8}", kVK_ForwardDelete: "\u{7f}", kVK_Escape: "\u{1b}",
            kVK_LeftArrow: unicode(NSLeftArrowFunctionKey), kVK_RightArrow: unicode(NSRightArrowFunctionKey),
            kVK_UpArrow: unicode(NSUpArrowFunctionKey), kVK_DownArrow: unicode(NSDownArrowFunctionKey),
            kVK_Home: unicode(NSHomeFunctionKey), kVK_End: unicode(NSEndFunctionKey),
            kVK_PageUp: unicode(NSPageUpFunctionKey), kVK_PageDown: unicode(NSPageDownFunctionKey),
        ]
        for (code, entry) in keys where isFunctionKey(code) {
            guard let number = Int(entry.name.dropFirst()) else { continue }
            table[Int(code)] = unicode(NSF1FunctionKey + number - 1)
        }
        return Dictionary(uniqueKeysWithValues: table.map { (UInt32($0.key), $0.value) })
    }()

    private static let keyCodesByName: [String: UInt32] =
        Dictionary(uniqueKeysWithValues: keys.map { ($0.value.name, $0.key) })
}

/// The global shortcut preference, `UserDefaults` key `ttHotKey`. Absent
/// means the default combo; an empty string means turned off; anything
/// unparseable reads as the default.
/// The quick-capture shortcut: `ctrl+opt+cmd+a` by default, after tt's own
/// capture key. Persisted in `UserDefaults` key `ttCaptureHotKey`, spelled and
/// cleared like `ttHotKey`.
enum TTCaptureHotKeyPreference {
    static let defaultsKey = "ttCaptureHotKey"
    static let `default` = HotKeyCombo(string: "ctrl+opt+cmd+a")!

    static func load(from defaults: UserDefaults = .standard) -> HotKeyCombo? {
        guard let stored = defaults.string(forKey: defaultsKey) else { return Self.default }
        if stored.isEmpty { return nil }
        return HotKeyCombo(string: stored) ?? Self.default
    }

    static func save(_ combo: HotKeyCombo?, to defaults: UserDefaults = .standard) {
        defaults.set(combo?.string ?? "", forKey: defaultsKey)
    }
}

enum TTHotKeyPreference {
    static let defaultsKey = "ttHotKey"
    static let `default` = HotKeyCombo(string: "ctrl+opt+cmd+t")!

    static func load(from defaults: UserDefaults = .standard) -> HotKeyCombo? {
        guard let stored = defaults.string(forKey: defaultsKey) else { return Self.default }
        if stored.isEmpty { return nil }
        return HotKeyCombo(string: stored) ?? Self.default
    }

    static func save(_ combo: HotKeyCombo?, to defaults: UserDefaults = .standard) {
        defaults.set(combo?.string ?? "", forKey: defaultsKey)
    }
}

extension NSMenuItem {
    /// Shows `combo` as this item's shortcut; nil shows none.
    func setShortcut(_ combo: HotKeyCombo?) {
        let equivalent = combo?.menuKeyEquivalent
        keyEquivalent = equivalent?.key ?? ""
        keyEquivalentModifierMask = equivalent?.modifiers ?? []
    }
}

/// Registers one global shortcut with Carbon's `RegisterEventHotKey`, which
/// needs no Accessibility permission, and calls `onPress` on the main thread.
final class HotKeyCenter {
    enum Registration: Equatable {
        case none
        case registered(HotKeyCombo)
        case failed(HotKeyCombo, OSStatus)

        /// What Preferences says about it; nil when all is well.
        var problem: String? {
            guard case .failed(let combo, let status) = self else { return nil }
            switch Int(status) {
            case eventHotKeyExistsErr:
                return "\(combo.displayString) is already taken by another app. Choose another shortcut."
            case eventHotKeyInvalidErr:
                return "\(combo.displayString) cannot be used as a global shortcut."
            default:
                return "\(combo.displayString) could not be registered (error \(status))."
            }
        }
    }

    /// `'ttHK'`, so events from another center's registration are ignored.
    static let signature: OSType = 0x7474_484B
    private static var nextID: UInt32 = 1

    var onPress: (() -> Void)?
    private(set) var registration: Registration = .none
    /// The shortcut that is live right now; nil while off, refused, or being recorded.
    var registeredCombo: HotKeyCombo? {
        guard case .registered(let combo) = registration else { return nil }
        return combo
    }
    /// What Carbon reports back on a press of this center's shortcut.
    var hotKeyID: EventHotKeyID { EventHotKeyID(signature: Self.signature, id: id) }
    private let id: UInt32
    private var hotKeyRef: EventHotKeyRef?
    private var handlerRef: EventHandlerRef?

    init() {
        id = Self.nextID
        Self.nextID += 1
        var spec = EventTypeSpec(eventClass: OSType(kEventClassKeyboard), eventKind: UInt32(kEventHotKeyPressed))
        let status = InstallEventHandler(
            GetApplicationEventTarget(),
            { _, event, userData in
                guard let event, let userData else { return OSStatus(eventNotHandledErr) }
                var hotKeyID = EventHotKeyID()
                let status = GetEventParameter(
                    event, EventParamName(kEventParamDirectObject), EventParamType(typeEventHotKeyID),
                    nil, MemoryLayout<EventHotKeyID>.size, nil, &hotKeyID
                )
                let center = Unmanaged<HotKeyCenter>.fromOpaque(userData).takeUnretainedValue()
                guard status == noErr, hotKeyID.signature == HotKeyCenter.signature,
                      hotKeyID.id == center.id else { return OSStatus(eventNotHandledErr) }
                center.onPress?()
                return noErr
            },
            1, &spec, Unmanaged.passUnretained(self).toOpaque(), &handlerRef
        )
        if status != noErr {
            NSLog("tt: installing the hotkey handler failed (%d)", status)
        }
    }

    deinit {
        unregister()
        if let handlerRef {
            RemoveEventHandler(handlerRef)
        }
    }

    /// Replaces the current registration; nil turns the shortcut off.
    @discardableResult
    func register(_ combo: HotKeyCombo?) -> Registration {
        unregister()
        guard let combo else { return registration }
        var ref: EventHotKeyRef?
        let status = RegisterEventHotKey(
            combo.keyCode, combo.modifiers, hotKeyID,
            GetApplicationEventTarget(), 0, &ref
        )
        if status == noErr, let ref {
            hotKeyRef = ref
            registration = .registered(combo)
        } else {
            registration = .failed(combo, status)
        }
        return registration
    }

    func unregister() {
        if let hotKeyRef {
            UnregisterEventHotKey(hotKeyRef)
            self.hotKeyRef = nil
        }
        registration = .none
    }
}

/// Shortcuts a global one would take from every app: tt's own Command-only
/// menu shortcuts and the standard few macOS and its apps rely on.
enum ReservedHotKeys {
    static let standard: Set<HotKeyCombo> = Set(
        ["cmd+q", "cmd+w", "cmd+c", "cmd+v", "cmd+x", "cmd+z", "cmd+a", "cmd+tab", "cmd+space", "cmd+comma"]
            .compactMap(HotKeyCombo.init(string:))
    )

    /// Why `combo` is refused, or nil when it is free. The Quick Capture item
    /// shows the capture shortcut itself, so it reserves nothing.
    static func problem(for combo: HotKeyCombo, menu: NSMenu?) -> String? {
        let commandOnly = combo.modifiers & UInt32(cmdKey) != 0
            && combo.modifiers & UInt32(controlKey | optionKey) == 0
        guard commandOnly else { return nil }
        let equivalent = combo.menuKeyEquivalent
        if let menu, let item = MainMenu.item(in: menu, key: equivalent.key, modifiers: equivalent.modifiers),
           item.action != MainMenu.quickCapture {
            let title = item.title.hasSuffix("…") ? String(item.title.dropLast()) : item.title
            return "\(combo.displayString) is tt's \(title) shortcut. Choose another shortcut."
        }
        if standard.contains(combo) {
            return "\(combo.displayString) is a standard macOS shortcut other apps rely on. Choose another shortcut."
        }
        return nil
    }
}

/// Shortcuts macOS itself owns (System Settings › Keyboard › Shortcuts). Carbon
/// cannot see these as conflicts, so Preferences warns about them itself.
enum SystemHotKeys {
    static func enabledCombos() -> Set<HotKeyCombo> {
        var unmanaged: Unmanaged<CFArray>?
        guard CopySymbolicHotKeys(&unmanaged) == noErr,
              let entries = unmanaged?.takeRetainedValue() as? [[String: Any]] else { return [] }
        var combos = Set<HotKeyCombo>()
        for entry in entries {
            guard (entry[kHISymbolicHotKeyEnabled as String] as? Bool) == true,
                  let code = (entry[kHISymbolicHotKeyCode as String] as? NSNumber)?.uint32Value,
                  let modifiers = (entry[kHISymbolicHotKeyModifiers as String] as? NSNumber)?.uint32Value,
                  let combo = HotKeyCombo(keyCode: code, modifiers: modifiers) else { continue }
            combos.insert(combo)
        }
        return combos
    }

    static func conflicts(with combo: HotKeyCombo) -> Bool {
        enabledCombos().contains(combo)
    }
}
