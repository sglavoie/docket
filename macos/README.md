# tt.app

A standalone macOS app that runs `tt` in an embedded Ghostty terminal: its own
Dock icon, a menu-bar icon, and a global shortcut that shows or hides it. It
spawns the ordinary `tt` binary on a pty; nothing here changes the Go program,
and none of it runs under `just ci`.

## Prerequisites

- macOS 14 or later, Apple silicon or Intel.
- Xcode (the full app, not only the Command Line Tools), with `xcodebuild`
  selected: `xcode-select -p` should name `Xcode.app`.
- Go and [`just`](https://github.com/casey/just), as for `tt` itself.
- `git`, `curl`, `shasum`, `tar`, `unzip`: present on a stock Mac.

Nothing else is installed by hand. The first build downloads the Zig release
and XcodeGen version pinned in `GHOSTTY_VERSION` and `XCODEGEN_VERSION`, checks
their SHA-256, and caches them under `macos/.cache/` (gitignored). Building
GhosttyKit takes several minutes the first time and is skipped afterwards
while the pin and the patches are unchanged.

## Build and install

```bash
just mac-install        # Release build, bundled tt, ad-hoc signed, into /Applications
```

That is the whole install from a clean clone. It runs `just mac-app Release`,
which runs `just build` (so the bundled `tt` carries the same version ldflags
as `just install`'s), builds the app, copies `tt` to
`tt.app/Contents/Resources/bin/tt`, signs, and then copies
`macos/build/tt.app` to `/Applications`. It refuses while tt.app is running,
since replacing the bundle would end its session: quit it from the menu-bar
icon first. `TT_APP_INSTALL_DIR` installs elsewhere.

The other recipes:

| Recipe | Does |
| --- | --- |
| `just mac-ghosttykit` | Build and vendor `Vendor/GhosttyKit.xcframework` and `Vendor/ghostty.h` |
| `just mac-app` | Debug build into `macos/build/tt.app`, no bundled `tt` |
| `just mac-app Release` | Release build with the bundled `tt` |
| `just mac-test` | `xcodebuild test` over `TTAppTests` (hosted in the app) |
| `just mac-install` | Release build, then install to `/Applications` |

`macos/tt.xcodeproj` is generated from `project.yml` on every build and is
gitignored: edit the spec, not the project.

### Which tt runs

The first executable of: the Preferences override (`ttBinaryPath`), the
bundled `Contents/Resources/bin/tt`, `~/.local/bin/tt`, then `PATH`. A Release
build always bundles one, so an installed app runs the `tt` it was built with,
with one exception: a `~/.local/bin/tt` whose `--version` reports a later
`built:` time than the bundled copy's runs instead, since the older binary
refuses a workspace the newer one has migrated. A Debug build bundles none and
falls through to `just install`'s copy. Preferences names the binary running,
its version, and an older copy of the pair when there is one.

### Environment

A Finder-launched app inherits almost no environment, so tt.app asks your
login shell, interactively (`$SHELL -l -i`), for `PATH`, `EDITOR`, `VISUAL`,
`TT_EDITOR`, and `TT_DATA_DIR`, and passes those to tt: `Ctrl+E` opens the
editor your terminal would, and `TT_DATA_DIR` picks the same workspace. A
shell that cannot run interactively within five seconds is asked again as a
plain login shell.

### Signing and notarization

`scripts/sign.sh` signs the bundled `tt` and then the app. By default the
signature is ad-hoc: the app runs on the Mac that built it, and Gatekeeper
(`spctl`) rejects it anywhere else. To sign for distribution:

```bash
export TT_CODESIGN_IDENTITY="Developer ID Application: Your Name (TEAMID)"
xcrun notarytool store-credentials tt-notary   # once; prompts for the Apple ID
export TT_NOTARY_PROFILE=tt-notary
just mac-app Release
```

With a Developer ID, both binaries are signed with the hardened runtime and a
secure timestamp, which notarization requires. With `TT_NOTARY_PROFILE` too,
the app is zipped, submitted with `notarytool --wait`, and the ticket stapled.
No entitlements are needed: the app only spawns `tt`, and Ghostty's own
entitlements exist for the arbitrary shells Ghostty.app hosts.

## Using the app

### Global shortcut

`Ctrl+Opt+Cmd+T` by default. It shows tt from any app and hides it when tt is
already in front; with no terminal (a launch error) it fronts the error window.
Change or clear it in Preferences (`Cmd+,`, shown as Settings… in the app
menu): click the recorder and press the combination. A combination needs
Ctrl, Opt, or Cmd unless the key is a function key, and Preferences warns when
another app or macOS itself already owns it.

It is stored as the `ttHotKey` default, lowercase and `+`-joined:

```bash
defaults write io.github.sglavoie.tt ttHotKey 'ctrl+opt+cmd+t'
defaults write io.github.sglavoie.tt ttHotKey ''    # off
defaults delete io.github.sglavoie.tt ttHotKey      # back to the default
```

The app registers it itself, with no Accessibility permission, so a Karabiner
or Raycast rule that focused Ghostty for tt can go.

### Menu-bar icon and window

Clicking the menu-bar icon shows or hides tt; right-click or Control-click
opens its menu: Show/Hide, Relaunch tt, Preferences…, Launch at Login, Quit.
Closing the window hides it and keeps the session; the next show returns to
it. Preferences also chooses whether tt appears in the Dock and menu bar or
the menu bar only (`ttActivationPolicy`: `regular` or `accessory`).

When `tt` exits cleanly (`q`), the window hides and the next show starts a
fresh `tt`, or, if Preferences says so, the app quits (`ttExitBehavior`:
`relaunch` or `quit`). A failed exit or a crash always stays on screen with
its status and a Relaunch button.

### Terminal config

The terminal reads `~/.config/tt-app/ghostty.conf`, in Ghostty's own config
syntax, and not Ghostty.app's config. To share that one, write:

```text
config-file = ~/.config/ghostty/config
```

Preferences has Open and Reload for it. The app always overrides `command`,
`wait-after-command`, and `confirm-close-surface`, since it owns the process
tt runs in.

### Drag and drop

Dropping files on the window types their paths, shell-escaped and
space-separated, as a paste would; URLs and text drop as text. Type `&` in
capture and drop a file to attach it, or drop one into the Attachments
panel's add prompt.

### Window background

tt announces its theme background to the terminal (OSC 11), and the title bar
takes that color, with light or dark title text to match, so the window reads
as one surface. Before tt sets it, the title bar uses the Ghostty config's
`background`.

### Default zoom

`Cmd+=` and `Cmd+-` zoom the terminal for the running session only; every
launch, and `Cmd+0`, return to the default zoom. Preferences sets it as a font
size in points, applied at once even over a zoomed window, and it overrides
the config file's `font-size`; Reset hands the size back to the config. It is
stored as the `ttDefaultFontSize` default, 6 to 72:

```bash
defaults write io.github.sglavoie.tt ttDefaultFontSize -float 15
defaults delete io.github.sglavoie.tt ttDefaultFontSize   # use the config's
```

Ghostty's standard themes ship inside the app, so `theme = Catppuccin Mocha`
works without Ghostty.app installed; `build-ghosttykit.sh` vendors them, with
Ghostty's terminfo and shell integration, into `Vendor/resources/`.

## Layout

```text
project.yml                 XcodeGen spec
GHOSTTY_VERSION             pinned Ghostty tag, Zig version, Zig checksums
XCODEGEN_VERSION            pinned XcodeGen version and checksum
patches/ghostty/            patches applied to the pinned tag (see its README)
scripts/                    build-ghosttykit, generate-project, build-app,
                            test-app, sign, install-app, make-icons
TTApp/App/                  AppDelegate, menus, lifecycle
TTApp/Ghostty/              runtime glue, config, surface view (ported, trimmed)
TTApp/Shell/                window, status item, hotkey, preferences, tt resolution
TTApp/Resources/            Info.plist, tt-launch, Assets.xcassets
TTAppTests/                 XCTest
NOTICE-ghostty              upstream MIT notices for the ported sources and themes
```

The icons are drawn by `scripts/make-icons.swift` and committed: rerun
`swift macos/scripts/make-icons.swift` after changing the artwork there,
never edit the PNGs by hand. `StatusIcon` is a template image, so macOS tints
it for light, dark, and highlighted menu bars.

## Upgrading Ghostty

The embedding API in `ghostty.h` is not stable, so the app pins one tag and
the Swift under `TTApp/Ghostty/` is a trimmed port of that tag's macOS app.
Staying pinned is fine; upgrade deliberately:

1. **Bump the pin.** In `GHOSTTY_VERSION`, set `GHOSTTY_TAG`, set
   `ZIG_VERSION` to the tag's `build.zig.zon` `minimum_zig_version`, and copy
   both `ZIG_SHA256_*` values from `ziglang.org/download/index.json` for that
   release.
2. **Re-check the patches.** Read `patches/ghostty/README.md`: delete each patch
   the new tag already carries and refresh any that no longer apply.
3. **Rebuild.** `GHOSTTYKIT_FORCE=1 just mac-ghosttykit`, then `just mac-app`.
   Compile errors here are API churn; fix them in `TTApp/Ghostty/`.
4. **Diff upstream against the port.** With the old and new tags checked out
   (the build leaves the new one in `macos/.cache/ghostty-<tag>/`):

   ```bash
   git -C macos/.cache/ghostty-NEW diff OLD NEW -- include/ghostty.h \
     macos/Sources/Ghostty/Surface\ View/SurfaceView_AppKit.swift \
     macos/Sources/Ghostty/Ghostty.App.swift \
     macos/Sources/Ghostty/Ghostty.Config.swift \
     macos/Sources/Ghostty/Ghostty.Input.swift
   ```

   The build clones one tag shallowly, so fetch the old tag first
   (`git -C macos/.cache/ghostty-NEW fetch --depth 1 origin tag OLD`). Carry
   every change to key, IME, mouse, clipboard, and runtime-callback handling
   into the matching file under `TTApp/Ghostty/`; a new or renamed
   `ghostty_*` function or enum case in `ghostty.h` is where to look first.
5. **Test.** `just mac-test`, then `just mac-install`.
6. **Rerun the key checklist** in the installed app, side by side with
   Ghostty.app on the same keys:
   - `Ctrl+M` toggles tt's menu bar and is not Enter.
   - `Ctrl+I` is not `Tab`; `Shift+Enter` and `Ctrl+J` do what they do in
     Ghostty.app.
   - `é`, `ç`, and dead-key `^` then `e` type correctly into capture (`a`).
   - Drag-select and `Cmd+C`, then `Cmd+V`, round-trip text.
   - Scrolling, the cursor shape, and a paste confirmation behave as in
     Ghostty.app.
   - The global shortcut toggles tt from another app; closing and reopening
     keeps the session; `q` follows the exit preference.
