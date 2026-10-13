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

Preferences lists the five, each showing the shell's value in gray. Typing a
value gives tt that instead, and clearing the field returns to the shell's.
They are stored together as the `ttEnvironment` dictionary default:

```bash
defaults write io.github.sglavoie.tt ttEnvironment -dict EDITOR hx
defaults delete io.github.sglavoie.tt ttEnvironment   # use the shell's for all
```

tt reads its environment when it starts, so an edit reaches the next tt:
Preferences offers Relaunch tt while the running one has the old values. A
`PATH` override also searches for `tt` itself, which happens when tt.app starts.

### Signing and notarization

`scripts/sign.sh` signs the bundled `tt` and then the app. By default it uses
the first Apple Development identity in your keychain (Xcode › Settings ›
Accounts creates one), else signs ad-hoc; either way the app runs on the Mac
that built it, and Gatekeeper (`spctl`) rejects it anywhere else.
`TT_CODESIGN_IDENTITY=-` forces ad-hoc.

The certificate matters for privacy prompts. tt.app's login-shell probe runs
your shell startup files, so a `~/.zshrc` that reads from `~/Documents` makes
macOS ask whether tt.app may access Documents. A certificate-signed app keeps
that answer across rebuilds; an ad-hoc one is identified by its hash, so every
`just mac-install` is a new app to macOS and asks again. The first launch after
switching from ad-hoc asks once more.

To sign for distribution:

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
opens its menu: Show/Hide, Quick Capture…, Relaunch tt, Preferences…, Launch at
Login, Quit. Show/Hide and Quick Capture… show their global shortcuts while
those are registered, as Quick Capture… does in the app menu; a shortcut that is
off, or that another app holds, is not shown. The Dock icon's menu has Quick
Capture… and Relaunch tt.
Once tt has reported its pulse, the menu leads with it as one line that runs
nothing (`Inbox 2 · Today 3 · Overdue 1`), leaving out any count at zero and the
whole line when all three are. The icon's tooltip is tt's window title, that
same pulse (`tt · inbox 2 · today 3 · overdue 1`).
Closing the window hides it and keeps the session; the next show returns to
it. Preferences also chooses whether tt appears in the Dock and menu bar, the
Dock only, or the menu bar only (`ttActivationPolicy`: `regular`, `dock`, or
`accessory`). With the Dock only, there is no menu-bar icon; its commands stay
in the app menu and the Dock icon's menu.

When `tt` exits cleanly (`q`), the window hides and the next show starts a
fresh `tt`, or, if Preferences says so, the app quits (`ttExitBehavior`:
`relaunch` or `quit`). A failed exit or a crash always stays on screen with
its status and a Relaunch button.

### Talking to tt

tt.app starts tt with `--control`, naming a socket in the app's temporary
directory, and keeps one connection to it for the session. Over it tt reports
its pulse and the work that arrives, and takes the requests below, each
answered exactly as the key in tt would be: the same commit, the same undo
entry, and a refusal while you are mid-way through something in tt rather than
a change made under you. tt.app passes the flag only to a tt whose `--help`
lists it, since an older one would refuse it and never start; with such a tt,
the quick-capture panel names the binary and how to update it instead of
sending anything, the notification actions are off, and notifications fall back
to `--notify`. The binary and its flags are read when tt.app starts, so an
updated tt is picked up by quitting and reopening the app, not by Relaunch tt.

### Quick capture

`Ctrl+Opt+Cmd+A` by default (after tt's own `a`) opens a capture field over
whatever app is in front, without bringing tt forward. Type a line in the
composer's grammar — `#tag`, `@date`, `%project`, `$state`, `!1`, which the
empty field shows as its placeholder while the line under it names the keys —
and Enter
sends it to tt, which captures it into Unfiled's Inbox unless the line says
otherwise and offers `u undo` the next time you look. A line tt refuses stays in
the field with tt's reason under it. Typing a sigil lists what tt's composer
would offer there — projects after `%`, tags after `#`, dates after `@`, and so
on — read from the running tt as you type: ↑/↓ choose, Tab accepts, and Enter
accepts too unless what you typed already stands on its own, as a new tag does.
Esc closes the list first; with no list, Esc or clicking elsewhere puts the
field away. Reopening an unfinished capture restores its text and caret or
selection, so typing resumes where you left off; Select All still replaces the
whole draft. A successful capture clears the draft and its selection. With suggestions
closed, `Ctrl+N` captures and keeps the field open for another task; Enter keeps
its usual automatic dismissal. A refused capture preserves the draft for either
key, and a pending capture cannot be submitted twice. The
menu-bar and Dock menus also carry Quick Capture. If tt had
exited, the capture starts it again first; a completion never does.

A live preview under the field names the project, plan (when present), status,
and resolved deadline, including its weekday and any time. It also shows
priority and the resolved deferral date and time when supplied. An unfinished
task with the same title in the destination project adds the terminal composer's
`Matching title` advisory, including its state and parent; this never blocks a
save. It reads the whole
draft through the running tt's composer, so moving the caret to complete a
token does not change the preview's meaning. Invalid drafts hide the preview;
Enter reports the refusal and keeps the text. Refusals wrap in full and the
panel grows to fit them. A preview never starts tt, and is simply absent with
an older tt that does not support it. Reopening the panel cancels a previous
capture's pending automatic dismissal.

The shortcut is set in Preferences beside the show/hide one, and stored as
`ttCaptureHotKey`, spelled like `ttHotKey`:

```bash
defaults write io.github.sglavoie.tt ttCaptureHotKey 'ctrl+opt+n'
defaults write io.github.sglavoie.tt ttCaptureHotKey ''    # off
```

### Notifications and the Dock badge

A check-in whose day comes, a timed deadline that passes, or a deferral that
ends while tt runs becomes a Notification Center alert. Up to three at once are
one alert per task: clicking it shows tt with the cursor on that task, and its
buttons act without leaving the app you are in — Acknowledge, offered only where
`R` would accept it, and Defer 1 Hour. More arrive as one summary that brings tt
forward. macOS asks for permission on the first one, and none is shown while tt
is already in front. Preferences turns them off (`ttNotifications`), at once.

The Dock icon carries the overdue count from tt's pulse, and none at zero
(`ttDockBadge`, also in Preferences):

```bash
defaults write io.github.sglavoie.tt ttNotifications -bool false
defaults write io.github.sglavoie.tt ttDockBadge -bool false
```

A bell while tt.app is in the background bounces the Dock icon once rather
than beeping. For an enrolled workspace, tt exchanges with mobile automatically
at launch, on focus, and every five seconds while its session runs, including
with the window hidden. Incoming changes refresh the Outliner after an active
edit, picker, selection, or child view is closed. An unavailable exchange is
retried automatically; the status strip reports failures and pending changes.
Cloud delivery may take longer. Quit stops automatic sync; closing the window
keeps it running. See `docs/guide/storage.md` for enrollment and sync status.

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

### Menus

Edit has Copy, Paste, and Select All, and nothing else: tt owns every other
edit, undo included, through its own keys. View names the zoom keys (Actual
Size, Zoom In, Zoom Out) and Enter Full Screen (`Ctrl+Cmd+F`); Window has
Minimize, Zoom, Close, and Bring All to Front; and Help › tt Guide opens the
user guide on GitHub.
The app menu also carries Relaunch tt. Peek's attachment links (OSC 8) open
with `Cmd`-click.

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
   - Each global shortcut fires once with tt in front, and once from the open
     status menu, whose items show them.
