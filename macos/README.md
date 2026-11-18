# Docket.app

A standalone macOS app that runs `docket` in an embedded Ghostty terminal: its own
window, Dock icon, menu-bar icon, and global shortcuts. It spawns the ordinary
`docket` binary on a pty. Nothing in it changes the outline itself: quick capture
runs `docket add` and the overdue count runs `docket ls`, each one more window on
the database under SPEC.md's Two windows. None of it runs under `just check`.

v2's Docket.app takes v1's place: the same name and bundle ID (`io.github.sglavoie.docket`), so
installing it replaces v1's app and keeps the Settings both share. v1 settings with
nothing behind them in v2 are ignored.

## Prerequisites

- macOS 14 or later, Apple silicon or Intel.
- Xcode (the full app, not only the Command Line Tools), with `xcodebuild`
  selected: `xcode-select -p` should name `Xcode.app`.
- Go and [`just`](https://github.com/casey/just), as for `docket` itself.
- `git`, `curl`, `shasum`, `tar`, `unzip`: present on a stock Mac.

Nothing else is installed by hand. The first build downloads the Zig release
and XcodeGen version pinned in `GHOSTTY_VERSION` and `XCODEGEN_VERSION`, checks
their SHA-256, and caches them under `macos/.cache/` (gitignored). Building
GhosttyKit takes several minutes the first time and is skipped afterwards
while the pin and the patches are unchanged.

## Build and install

```bash
just mac-install        # Release build, bundled docket, signed, into /Applications
```

That is the whole install from a clean clone. It runs `just mac-app Release`,
which runs `just build` (so `docket --version` names the commit and build time),
builds GhosttyKit the first time, builds the app, copies `bin/docket` to
`Docket.app/Contents/Resources/bin/docket`, signs, and copies
`macos/build/Docket.app` to `/Applications`. It refuses while Docket.app is
running, since replacing the bundle would end its session: quit it first.
`DOCKET_APP_INSTALL_DIR` installs elsewhere.

A release installs it with Homebrew: `brew install --cask sglavoie/tap/docket`
puts the notarized, universal Docket.app in `/Applications` and links the
`docket` it bundles as `docket` and `dkt`. `scripts/release.sh` (`just release`)
tags a v2 commit; the Release workflow publishes the CLI archives with GoReleaser,
then its `macos-app` job builds the app with `macos/scripts/package-release.sh`
and writes the cask with `scripts/homebrew_cask.sh`.

| Recipe | Does |
| --- | --- |
| `just mac-ghosttykit` | Build and vendor `Vendor/GhosttyKit.xcframework` and `Vendor/ghostty.h` |
| `just mac-app` | Debug build into `macos/build/Docket.app`, no bundled `docket` |
| `just mac-app Release` | Release build with the bundled `docket` |
| `just mac-test` | `xcodebuild test` over `DocketAppTests` (hosted in the app) |
| `just mac-install` | Release build, then install to `/Applications` |

`macos/Docket.xcodeproj` is generated from `project.yml` on every build and is
gitignored: edit the spec, not the project.

### Which Docket runs

The first executable of: the Settings override (`docketBinaryPath`), the bundled
`Contents/Resources/bin/docket`, `~/.local/bin/docket`, then `PATH`. A Release
build always bundles one, so an installed app runs the `docket` it was built
with; point the override at `~/.local/bin/docket` to run a `just install` build
instead. A Debug build bundles none and falls through to `just install`'s copy.
Settings › Advanced names the binary running and its version, and says when
`~/.local/bin/docket` is older or newer than the bundled copy. A binary choice
takes effect when Docket next starts, and Settings offers Relaunch Docket while
the running one is another binary than the next would be. Quick capture and the
overdue count run the same binary.

If no Docket is found at launch, a window says where it looked and offers Open
Settings…, Try Again, and Quit. When the terminal itself cannot start, the
window names the config file and lists what Ghostty found wrong in it.

### Environment

A Finder-launched app inherits almost no environment, so Docket.app asks your
login shell, interactively (`$SHELL -l -i`), for `PATH` and `DOCKET_DB`, and
passes those to Docket, so it opens the same database a terminal would. A shell
that cannot run interactively within five seconds is asked again as a plain
login shell.

Settings › Advanced lists both, each showing the shell's value in gray. Typing
a value gives Docket that instead, and clearing the field returns to the
shell's. They are stored together as the `docketEnvironment` dictionary:

```bash
defaults write io.github.sglavoie.docket docketEnvironment -dict DOCKET_DB ~/tmp/trial.db
defaults delete io.github.sglavoie.docket docketEnvironment   # use the shell's
```

### Signing and notarization

`scripts/sign.sh` signs the bundled `docket` and then the app. By default it uses
the first Apple Development identity in your keychain (Xcode › Settings ›
Accounts creates one), else signs ad-hoc; either way the app runs on the Mac
that built it, and Gatekeeper (`spctl`) rejects it anywhere else.
`DOCKET_CODESIGN_IDENTITY=-` forces ad-hoc.

The certificate matters for privacy prompts. Docket.app's login-shell probe runs
your shell startup files, so a `~/.zshrc` that reads from `~/Documents` makes
macOS ask whether Docket.app may access Documents. A certificate-signed app keeps
that answer across rebuilds; an ad-hoc one is identified by its hash, so every
`just mac-install` is a new app to macOS and asks again.

To sign for distribution:

```bash
export DOCKET_CODESIGN_IDENTITY="Developer ID Application: Your Name (TEAMID)"
xcrun notarytool store-credentials docket-notary   # once; prompts for the Apple ID
export DOCKET_NOTARY_PROFILE=docket-notary
just mac-app Release
```

With a Developer ID, both binaries are signed with the hardened runtime and a
secure timestamp, which notarization requires. With `DOCKET_NOTARY_PROFILE` too,
the app is zipped, submitted with `notarytool --wait`, and the ticket stapled.
Its one entitlement lets the hardened runtime send the Apple Events ⌘K in quick
capture asks the app in front with; the Share extension is sandboxed and holds
nothing else. `sign.sh` signs each extension with its own entitlements file.

## Using the app

### Global shortcut

`Ctrl+Opt+Cmd+T` by default. It shows Docket from any app and hides it when Docket is
already in front; with no terminal (a launch error) it fronts the error window.
Change or clear it in Settings › Shortcuts (`Cmd+,`): click the recorder and
press the combination. A combination needs Ctrl, Opt, or Cmd unless the key is
a function key, and Settings warns when another app or macOS itself already
owns it. Settings refuses, and leaves unregistered, a combination another Docket
shortcut already holds, a Command-only one a Docket menu item uses, and the
standard `Cmd+Q`, `Cmd+W`, `Cmd+C`, `Cmd+V`, `Cmd+X`, `Cmd+Z`, `Cmd+A`,
`Cmd+Tab`, `Cmd+Space`, and `Cmd+,`; changing one shortcut checks the others
again, so a warning the old combination caused clears. Restore Default appears
beside a shortcut that is not its default, or is off.

It is stored as the `docketHotKey` default, lowercase and `+`-joined:

```bash
defaults write io.github.sglavoie.docket docketHotKey 'ctrl+opt+cmd+t'
defaults write io.github.sglavoie.docket docketHotKey ''    # off
defaults delete io.github.sglavoie.docket docketHotKey      # back to the default
```

The app registers it itself, with no Accessibility permission, so a Karabiner
or Raycast rule that focused Ghostty for Docket can go.

### Menu-bar icon and window

The menu-bar icon shows or hides Docket on a left click, like the global
shortcut. A right click or Control-click opens its menu: Show/Hide, Quick
Capture…, Relaunch Docket, Settings… (`Cmd+,`), Launch at Login, About,
Docket Specification, and Quit Docket. The Dock icon's menu has the overdue
line, Show/Hide, Quick Capture…, and Relaunch Docket.

Settings › General › Show Docket in picks Dock and menu bar (the default), Dock
only, or Menu bar only (`docketActivationPolicy`: `regular`, `dock`, or
`accessory`); it applies at once.

Closing the window hides it; only Quit ends Docket.app. When Docket exits with
`q`, Settings › General › When Docket exits picks between hiding the window and
starting a fresh Docket the next time it is shown (the default), or quitting
Docket.app (`docketExitBehavior`: `relaunch` or `quit`). A crash or a non-zero
exit always keeps Docket's last output on screen with a Relaunch button (Return
presses it). Quitting Docket.app ends Docket with a hangup, as closing a
terminal does; Docket has already saved every change as it was made.

### Quick capture

`Ctrl+Opt+Cmd+A` by default, or Quick Capture… in either menu, opens a one-line
field over whatever app is in front, without bringing Docket.app forward. Enter
runs `docket add` with the line, which reads it as the capture line does:
sigils become fields and a `%name` word files it under a project or heading,
else it goes to the Inbox. The panel says where the row went and closes a moment
later. A line Docket refuses stays in the field with Docket's reason. Esc, or
clicking elsewhere, puts the panel away and keeps the line for next time; an
empty Enter closes it.

An open Docket window sees the row at its next key, or within a minute. Change
or clear the shortcut in Settings › Shortcuts; it is stored as
`docketCaptureHotKey`, spelled like `docketHotKey`.

### Capture from other apps

Each of these opens quick capture holding a line, and none of them sends it:
the line is read, finished and sent with Enter, so nothing from outside adds a
row unseen. A line typed and not yet sent is kept, and what arrives goes after
it. Docket draws a web address in a row as a link that `gl` opens, so an address
goes into the line as it is; with a title in front, the title is selected, so
typing replaces it.

- **⌘K in quick capture** adds what the app in front shows: the Safari, Chrome,
  Brave or Edge tab (its title and address), the messages selected in Mail
  (the first subject and a `message://` link for each) or the items selected in
  Finder (their paths). An empty line takes the title too; a typed one keeps
  its words and gains the links. The hint names what ⌘K adds, and only when the
  app in front is one it reads. The first ⌘K per app asks macOS for permission
  to automate it; a refusal is named on the panel, and is changed in System
  Settings › Privacy & Security › Automation. `gl` opens web addresses only, so a
  Mail link or a path stays text to copy.
- **Services › Capture to Docket** takes the selected text in any app. A
  selection that is only a web address leaves the title to type.
- **Share › Docket**, from Safari, Finder, Preview and any app with a Share
  menu, takes the page's title and address, the files' paths, or the text.
  `DocketShare.appex` shows nothing itself: it opens a `docket://capture` link.
- **`docket://capture?text=…&url=…`** from a script, a Shortcut or
  `open "docket://capture?text=Call%20the%20bank%20@fri"`: the text, then each
  `url` (repeatable). With neither it opens the panel as it was;
  `docket://show` shows Docket. A link that starts Docket.app waits for its
  first session to find Docket before the panel opens.

### Overdue count

Docket.app runs `docket ls '@overdue'` once a minute, when Docket.app stops
being the app in front, after a quick capture, and when a session ends, and
shows how many rows it printed: open rows whose deadline has passed. The count
goes on the Dock icon, or beside the menu-bar icon when there is no Dock icon,
and nothing shows while none are overdue. Settings › General turns it off
(`docketDockBadge`, on by default); off, nothing is run. A run that fails leaves
the count blank rather than stale, and logs why.

### Appearance

With Settings › General › Match macOS appearance on (`docketFollowAppearance`,
the default) and no theme chosen in Settings › Terminal, the terminal takes a
light or dark background and text colour to match macOS, and changes them when
macOS does. Docket asks the terminal for its background at launch, whenever its
window has focus again, once a minute, and when the terminal reports that macOS
turned light or dark (mode 2031, so with or without focus), and paints its light
or dark palette to suit. That report can come before Docket.app has reloaded its
colours, so Docket asks again 300 ms later; Docket.app also reports a focus
change as soon as it recolours a focused window. With a theme chosen, the theme's colours apply instead, and
a separate Dark Mode theme follows macOS through Ghostty itself while following
is on; with following off, the pair's theme for the appearance at launch, or
when following was turned off, stays. With following off and no theme, the
Ghostty config's colours stand.

The title bar takes the terminal's background, with light or dark title text to
match, so the window reads as one surface.

### Terminal config

The terminal reads `~/.config/docket-app/ghostty.conf`, in Ghostty's own config
syntax, and not Ghostty.app's config. To share that one, write:

```text
config-file = ~/.config/ghostty/config
```

Settings has Open and Reload for it; Reload says "Reloaded." or lists what
Ghostty found wrong in the file. The app always overrides `command`,
`wait-after-command`, and `confirm-close-surface`, since it owns the process
Docket runs in. By default it also resets the `font-family` keys to the built-in
font; see [Terminal font](#terminal-font).

### Default zoom

`Cmd+=` and `Cmd+-` zoom the terminal for the running session only; every
launch, and `Cmd+0`, return to the default zoom. Settings sets it as a font
size in points, applied at once even over a zoomed window, and it overrides
the config file's `font-size`; Reset hands the size back to the config. It is
stored as the `docketDefaultFontSize` default, 6 to 72:

```bash
defaults write io.github.sglavoie.docket docketDefaultFontSize -float 15
defaults delete io.github.sglavoie.docket docketDefaultFontSize   # use the config's
```

### Terminal font

Settings › Terminal › Font defaults to JetBrains Mono Nerd Font (built in), so
every Docket.app looks the same whatever the config file says: that choice
overrides any `font-family`, `font-family-bold`, `font-family-italic`, and
`font-family-bold-italic` in `~/.config/docket-app/ghostty.conf`. The built-in
face is not an installed font. Ghostty compiles JetBrains Mono and the Symbols
Nerd Font into libghostty (`src/font/embedded.zig`) and adds them after every
configured family as fallbacks (`SharedGridSet.collection`), without
registering them with CoreText, so no family name selects them. Docket.app
therefore writes `font-family = ""` (and the same for the three styled keys),
which empties Ghostty's lists so that fallback becomes the face. The picker also
offers “From the Ghostty config”, which writes nothing and leaves the families to
the config file, and every installed fixed-pitch family. An installed family resets
those four lists too, then sets `font-family`, so its bold and italic come from its
own styles. Because the embedded symbols font is always the fallback, any family
still draws Docket's Nerd Font glyphs.

A change applies immediately to the open terminal and keeps its zoom. The choice is
stored as `docketTerminalFont`: absent is the built-in font, `ghostty-config`
defers to the config, and any other value names a family:

```bash
defaults write io.github.sglavoie.docket docketTerminalFont -string "Menlo"
defaults write io.github.sglavoie.docket docketTerminalFont -string ghostty-config
defaults delete io.github.sglavoie.docket docketTerminalFont   # the built-in font
```

### Terminal theme

Ghostty's standard themes ship inside the app, so `theme = Catppuccin Mocha`
works without Ghostty.app installed; `build-ghosttykit.sh` vendors them, with
Ghostty's terminfo and shell integration, into `Vendor/resources/`.
Settings › Terminal › Theme lists those bundled themes and themes in
`~/.config/ghostty/themes`. A selected theme applies immediately to the open
terminal and is saved as `docketTerminalTheme`; the optional Dark Mode choice
follows macOS appearance. “From the Ghostty config” removes the override. Theme
changes keep the terminal's current zoom, and Docket repaints its palette for
the new background as described under [Appearance](#appearance).

### Drag and drop

Dropping files on the window types their paths, shell-escaped and
space-separated, as a paste would; URLs and text drop as text.

### Menus

Edit has Undo, Redo, Cut, Copy, Paste, and Select All. Undo, Redo, and Cut are
for the text fields in Settings and quick capture; in the terminal their keys
pass through, and Docket's own `u` and `Ctrl+R` undo and redo. Copy copies a
terminal selection (Shift+drag selects). View names the zoom keys (Actual Size,
Zoom In, Zoom Out) and Enter Full Screen (`Ctrl+Cmd+F`); Window has Minimize,
Zoom, Close, and Bring All to Front; and Help has Docket Specification, which
opens SPEC.md on GitHub, and Report an Issue…. The app menu also carries Quick
Capture… and Relaunch Docket.

## Layout

```text
project.yml                 XcodeGen spec
GHOSTTY_VERSION             pinned Ghostty tag, Zig version, Zig checksums
XCODEGEN_VERSION            pinned XcodeGen version and checksum
patches/ghostty/            patches applied to the pinned tag (see its README)
scripts/                    build-ghosttykit, generate-project, build-app,
                            test-app, sign, install-app, package-release, make-icons
DocketApp/App/              AppDelegate, menus, lifecycle
DocketApp/Ghostty/          runtime glue, config, surface view (ported, trimmed)
DocketApp/Shell/            window, status item, hotkeys, preferences, quick capture,
                            Docket resolution, docket add / ls runs
DocketApp/Resources/        Info.plist, entitlements, docket-launch, Assets.xcassets
DocketShare/                Share extension: hands what was shared to quick capture
Shared/                     docket:// links, capture lines, the share hand-off;
                            compiled into the app and the extension
DocketAppTests/             XCTest
```

## Upgrading Ghostty

The embedding API in `ghostty.h` is not stable, so the app pins one tag and
the Swift under `DocketApp/Ghostty/` is a trimmed port of that tag's macOS app.
Staying pinned is fine; upgrade deliberately:

1. **Bump the pin.** In `GHOSTTY_VERSION`, set `GHOSTTY_TAG`, set
   `ZIG_VERSION` to the tag's `build.zig.zon` `minimum_zig_version`, and copy
   both `ZIG_SHA256_*` values from `ziglang.org/download/index.json` for that
   release.
2. **Re-check the patches.** Read `patches/ghostty/README.md`: delete each patch
   the new tag already carries and refresh any that no longer apply.
3. **Rebuild.** `GHOSTTYKIT_FORCE=1 just mac-ghosttykit`, then `just mac-app`.
   Compile errors here are API churn; fix them in `DocketApp/Ghostty/`.
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
   into the matching file under `DocketApp/Ghostty/`; a new or renamed
   `ghostty_*` function or enum case in `ghostty.h` is where to look first.
5. **Test.** `just mac-test`, then `just mac-install`.
6. **Rerun the key checklist** in the installed app, side by side with
   Ghostty.app on the same keys:
   - `Ctrl+M` toggles Docket's menu bar and is not Enter.
   - `Ctrl+I` is not `Tab`; `Shift+Enter` and `Ctrl+J` do what they do in
     Ghostty.app.
   - `é`, `ç`, and dead-key `^` then `e` type correctly into capture (`a`).
   - Drag-select and `Cmd+C`, then `Cmd+V`, round-trip text.
   - Scrolling, the cursor shape, and a paste confirmation behave as in
     Ghostty.app.
   - The global shortcut toggles Docket from another app; closing and reopening
     keeps the session; `q` follows the exit preference.
   - Each global shortcut fires once with Docket in front, and once from the open
     status menu, whose items show them.

Desktop navigation refusals appear briefly beneath the terminal without taking
keyboard focus. They explain what to close, save, or cancel before trying again.
Full-screen show/hide requests preserve the last requested visibility through
transitions; a window whose display disappeared returns within a remaining
display’s visible bounds.

The `macOS desktop shell` CI job builds and runs `just mac-test` on every pull
request and push to main. Its dependency cache is keyed by architecture, Xcode
and SDK versions, the Ghostty/Zig and XcodeGen pins, and build patches/scripts.
XCTest result bundles, including failures, are uploaded as `macos-results`.
Locally, set `DOCKET_MACOS_RESULTS` to keep a result bundle for each test run.
