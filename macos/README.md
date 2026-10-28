# Docket.app

A standalone macOS app that runs `docket` in an embedded Ghostty terminal: its own
Dock icon, a menu-bar icon, and a global shortcut that shows or hides it. It
spawns the ordinary `docket` binary on a pty; nothing here changes the Go program,
and none of it runs under `just ci`.

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

A release installs it with Homebrew: `brew install --cask sglavoie/tap/docket`
puts the notarized, universal Docket.app in `/Applications` and links the
`docket` it bundles as `docket` and `dkt`. The release workflow's `macos-app`
job builds that app with `macos/scripts/package-release.sh` and writes the cask
with `scripts/homebrew_cask.sh`. The rest of this section builds it from a clone.

```bash
just mac-install        # Release build, bundled docket, ad-hoc signed, into /Applications
```

That is the whole install from a clean clone. It runs `just mac-app Release`,
which runs `just build` (so the bundled `docket` carries the same version ldflags
as `just install`'s), builds the app, copies `docket` to
`Docket.app/Contents/Resources/bin/docket`, signs, and then copies
`macos/build/Docket.app` to `/Applications`. It refuses while Docket.app is running,
since replacing the bundle would end its session: quit it from the menu-bar
icon first. It also ends a running widget extension, which would otherwise keep
drawing the old widget; macOS starts the new one when the widget next draws.
`DOCKET_APP_INSTALL_DIR` installs elsewhere.

The other recipes:

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

The first executable of: the Settings override (`docketBinaryPath`), the
bundled `Contents/Resources/bin/docket`, `~/.local/bin/docket`, then `PATH`. A Release
build always bundles one, so an installed app runs the `docket` it was built with,
however recent a `just install` build is; point the override at
`~/.local/bin/docket` to run a dev build instead. A Debug build bundles none and
falls through to `just install`'s copy. Settings › Advanced names the binary
running, its version, and whether `~/.local/bin/docket` is older or newer than the
bundled copy, since the older of the two refuses a workspace the newer one has
migrated. A
binary choice takes effect when Docket next starts, and Settings offers Relaunch Docket
while the running one is another binary than the next would be.

If no Docket is found at launch, a window says where it looked and offers Open
Settings…, Try Again, and Quit; Try Again searches again without restarting
the app, and Return chooses it. When the terminal itself cannot start, the
window names the config file and lists what Ghostty found wrong in it, and
Return opens Settings; it never quits. A Dock click or switching to Docket.app brings that window back while no
Docket runs.

### Environment

A Finder-launched app inherits almost no environment, so Docket.app asks your
login shell, interactively (`$SHELL -l -i`), for `PATH`, `EDITOR`, `VISUAL`,
`DOCKET_EDITOR`, and `DOCKET_DATA_DIR`, and passes those to Docket: `Ctrl+E` opens the
editor your terminal would, and `DOCKET_DATA_DIR` picks the same workspace. A
shell that cannot run interactively within five seconds is asked again as a
plain login shell.

Settings lists the five, each showing the shell's value in gray. Typing a
value gives Docket that instead, and clearing the field returns to the shell's.
They are stored together as the `docketEnvironment` dictionary default:

```bash
defaults write io.github.sglavoie.docket docketEnvironment -dict EDITOR hx
defaults delete io.github.sglavoie.docket docketEnvironment   # use the shell's for all
```

Docket reads its environment when it starts, so an edit reaches the next Docket:
Settings offers Relaunch Docket while the running one has the old values. A
`PATH` override also searches for `docket` itself, which happens before each session.

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
`just mac-install` is a new app to macOS and asks again. The first launch after
switching from ad-hoc asks once more.

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
The only entitlement is the App Group the widget reads the pulse from,
`MH33GYKDAW.io.github.sglavoie.docket`, which `sign.sh` signs into both the app
(`DocketApp/Resources/Docket.entitlements`) and the sandboxed widget extension
(`DocketWidgets/DocketWidgets.entitlements`), the extension first. The group is
prefixed with the team ID, so a Developer ID or Apple Development signature from
that team owns it without a provisioning profile; an ad-hoc build runs, but
macOS may not load its widget or may refuse it the container. Nothing else is
needed: the app only spawns `docket`, and Ghostty's own entitlements exist for
the arbitrary shells Ghostty.app hosts.

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

Clicking the menu-bar icon shows or hides Docket; right-click or Control-click
opens its menu: Show/Hide, Quick Capture…, Find Task…, Relaunch Docket, Settings… (`Cmd+,`),
Launch at Login, About Docket, Docket Guide, Quit (`Cmd+Q`). Show/Hide, Quick
Capture…, and Find Task… show their global shortcuts while those are registered,
as Quick Capture… and Find Task… do in the app menu; a shortcut that is off, or
that another app holds, is not shown. The Dock icon's menu has the same pulse and agenda items, then
Show/Hide, Quick Capture…, Find Task…, and Relaunch Docket.
Once Docket has reported its pulse, the menu leads with one item per count still
owed — `Inbox 2`, `Today 3`, `Overdue 1` — leaving out any count at zero and
all of them when all three are. Choosing one shows Docket in that mode, as its mode
jump would: Inbox, Today, or Due for Overdue. Under the counts, an Overdue and a
Today heading list the tasks behind them, overdue first by oldest deadline, then
Today by priority, each with its project and deadline beside the title (`Send
report  Work · Yesterday`). Each opens a submenu: Show in Docket puts the cursor
on it, as a notification does, and Done, Defer 1 Hour, and Acknowledge (offered
only where `R` would accept it) act on it without bringing Docket forward, as
that notification's buttons do; a refusal arrives as a notification. The menu holds eight; the rest are one `N more…` item showing
Due while overdue work is among them, else Today. A task overdue and planned for
today is listed once, as overdue. A Docket that is busy, because
something is open in it, or too old to know the request is still shown, in the
mode it was in. The icon's tooltip is Docket's window title, the pulse as one line
(`docket · inbox 2 · today 3 · overdue 1`). VoiceOver names the icon Docket, reads that
line, without the title, as its value, and says what it does: click to show or
hide Docket, Control-click for the menu. The count items read as they are shown.
Closing the window hides it and keeps the session; the next show returns to
it. Hiding leaves full screen first, and the next show enters it again; it
hides the app only when no other Docket window, such as Settings or About, is
open. Showing brings the window to the Space you are on rather than switching
to the Space it was last on; a full-screen window keeps its own Space. The window resizes in whole cells
and never below 80×24 of them, following the cell size as the font zooms. Settings also chooses whether Docket appears in the Dock and menu bar, the
Dock only, or the menu bar only (`docketActivationPolicy`: `regular`, `dock`, or
`accessory`). With the Dock only, there is no menu-bar icon; its commands stay
in the app menu and the Dock icon's menu. With the menu bar only, the overdue
count the Dock icon would carry sits beside the menu-bar icon instead.

When `docket` exits cleanly (`q`), the window hides and the next show starts a
fresh `docket`, or, if Settings says so, the app quits (`docketExitBehavior`:
`relaunch` or `quit`). A failed exit or a crash always stays on screen with
its status and a Relaunch button. One while the window is hidden leaves it
hidden: the Dock icon bounces once, or with no Dock icon a "Docket stopped"
notification says so when notifications are on, and the banner is waiting the
next time the window is shown. About Docket names the running Docket's version above the
Ghostty build.

### Docket in the background

After a clean exit that hides the window, Docket.app starts the same Docket again
with no window: `docket --headless`, on the window's own control socket. It keeps
the menu-bar and Dock counts, the agenda, the widget, sync, notifications, and
the Shortcuts actions current until you show Docket, which ends it and starts a
fresh one in the window, since only one Docket may hold a workspace at a time.
A request that arrives while one is handing over to the other waits for the
window's. Quitting Docket.app ends it; one whose host disappeared without
ending it, after a crash, quits by itself within seconds. Settings › General
turns it off (`docketBackgroundSession`), shown while Docket hides on exit:

```bash
defaults write io.github.sglavoie.docket docketBackgroundSession -bool false
```

With it off, or with a Docket too old to list `--headless`, nothing runs between
`q` and the next show. A background Docket that ends on its own, as one refused
because a Docket started in another terminal holds the workspace, is not
restarted until something asks Docket for an answer.

### Talking to Docket

Docket.app starts Docket with `--control`, naming a socket in the app's temporary
directory, and keeps one connection to it for the session. Over it Docket reports
its pulse, the tasks behind its Today and Overdue counts, the work that arrives
and the work still to come, and takes the requests below, each
answered exactly as the key in Docket would be: the same commit, the same undo
entry, and a refusal while you are mid-way through something in Docket rather than
a change made under you. Docket.app passes the flag only to a Docket whose `--help`
lists it, since an older one would refuse it and never start; with such a Docket,
the quick-capture panel names the binary and how to update it instead of
sending anything, the notification actions are off, and notifications fall back
to `--notify`. The binary and its flags are read again before every session,
off the main thread, so Relaunch Docket picks up an updated Docket without quitting the
app: quick capture, the notification actions, and the line Settings shows for
the running binary all follow the Docket that just started. If no Docket can be found at
that moment, the last one that was keeps running; only a launch that never found
one shows the error instead of a window.

Settings keeps the running session's version separate from the executable now
on disk. Reinstalling at the same path still offers **Relaunch Docket**, naming the
version available after relaunch. With a current Docket, the running version comes
from the process itself; older builds use the version recorded before launch.

Quitting Docket.app asks Docket to quit first, the way `q` does, so it saves its view
and filter state. The app ends once Docket has, or after three seconds. If Docket is
holding unsaved content or still saving, the window comes forward with a
"Quit with unfinished work?" alert naming the draft and offering Keep Editing
or Quit Anyway. A quick-capture draft is protected even while its panel is
hidden. Search, selection, and unchanged pickers do not require confirmation. With no socket, or a Docket too old to know the request, Docket.app quits at
once and Docket gets a hangup, as a closed terminal would give it.

### Quick capture

`Ctrl+Opt+Cmd+A` by default (after Docket's own `a`) opens a capture field over
whatever app is in front, without bringing Docket forward. Type a line in the
composer's grammar — `#tag`, `@date`, `%project`, `$today`, `!1`, which the
empty field shows as its placeholder while the line under it names the keys —
and Enter
sends it to Docket, which captures it into Unfiled's Inbox unless the line says
otherwise and offers `u undo` the next time you look. A line Docket refuses stays in
the field with Docket's reason under it. Typing a sigil lists what Docket's composer
would offer there — projects after `%`, tags after `#`, dates after `@`, and so
on — read from the running Docket as you type: ↑/↓ choose, Tab accepts, and Enter
accepts too unless what you typed already stands on its own, as a new tag does.
Esc closes the list first; with no list, Esc or clicking elsewhere puts the
field away; the hint spells this as `Esc keep draft`. Text is saved locally as you edit, so restarting the app or recovering
from a crash restores the unfinished draft without submitting it. Deleting the
text removes its recovery copy. A failed recovery write keeps the text in the
field and reports the problem; another edit retries the write.
Reopening an unfinished capture in the same session restores its text and caret or
selection, so typing resumes where you left off; Select All still replaces the
whole draft. A successful capture clears the draft and its selection. With suggestions
closed, `Ctrl+N` captures and keeps the field open for another task; Enter keeps
its usual automatic dismissal. A refused capture preserves the draft for either
key, and a pending capture cannot be submitted twice. Quick capture saves its draft and a submission identity together before sending it to Docket.
After five seconds without an answer, the panel stops waiting and keeps the
saved draft. Press Enter to retry safely: Docket returns the original outcome if
it already committed, including after a restart or an undo, rather than creating
the tasks again. A new capture after a confirmed success receives a new identity,
even when its text is identical.

The saved submission is bound to its workspace. Reopening Docket.app restores an
unresolved draft; switching workspaces refuses to resend it into the wrong one.
The file lives at `~/Library/Application Support/io.github.sglavoie.docket/pending-capture.json`
and is removed after a successful reply. A failure to save this file prevents
transmission. A damaged saved file is reported rather than overwritten.
An older Docket without safe capture support is refused with update instructions.

The five-second reply timer starts after transmission, so startup can use the
control connection's 30-second window. Esc or clicking elsewhere stops waiting
and keeps the draft. A late success settles every retry of that submission and
can be undone with `Cmd+Z`; it never clears a different draft. Undo retains its
existing timeout warning: an unanswered undo may still complete, so check Docket
before trying it again. Reopening the panel cancels a pending automatic dismissal.
VoiceOver names the field Quick capture and the list
Suggestions, and announces each status line. The panel always opens centred
on the screen under the pointer and cannot be dragged. The
menu-bar and Dock menus also carry Quick Capture. If Docket had
exited, the capture starts it again first, in the background when it can and
otherwise in the window; a completion never does.

`Cmd+Z` in the empty field takes back the last capture through Docket's own undo,
the one `u` would make, and the line under the field says what Docket answered.
Only that most recent capture: a second `Cmd+Z` does not reach the ones before
it, which `u` in Docket still can. Docket
refuses it once something newer has changed in Docket (`Something newer was
changed in Docket; undo it there`), or while something is open in Docket, and the
capture stays. With text in the field, `Cmd+Z` undoes the typing instead. So
it serves a `Ctrl+N` capture and a reopened panel; an undone capture is not
offered again, and an undo never starts Docket.

A live preview under the field names the project, plan (when present), status,
and resolved deadline, including its weekday and any time. It also shows
priority and the resolved deferral date and time when supplied. An unfinished
task with the same title in the destination project adds the terminal composer's
`Matching title` advisory, including its state and parent; this never blocks a
save. It reads the whole
draft through the running Docket's composer, so moving the caret to complete a
token does not change the preview's meaning. Invalid drafts hide the preview;
Enter reports the refusal, keeps the text, and selects the offending token
when the parser can locate it. Preview requests wait for a brief pause in typing;
"Previewing…" appears only when the response takes longer. Refusals wrap in full and the
panel grows to fit them. A preview never starts Docket, and is simply absent with
an older Docket that does not support it. Reopening the panel cancels a previous
capture's pending automatic dismissal.

The shortcut is set in Settings beside the show/hide one, and stored as
`docketCaptureHotKey`, spelled like `docketHotKey`:

```bash
defaults write io.github.sglavoie.docket docketCaptureHotKey 'ctrl+opt+n'
defaults write io.github.sglavoie.docket docketCaptureHotKey ''    # off
```

### Quick find

`Ctrl+Opt+Cmd+/` by default (after Docket's own `/`) opens a find field over
whatever app is in front, without bringing Docket forward. Type a query in
Docket's `/` grammar — words match titles, then descriptions, checklists,
comments, tags, and the rest in the ranked list's order, and `#tag`, `%project`,
`$doing`, `@today`, `!1`, or `?blocked` narrow the answer — and the running Docket
ranks every task in the workspace against it, best match first, whatever view
it is showing: its scope, filters, Focus, and lens do not narrow the answer, and
nothing in Docket moves. As in Docket, finished and deferred work answers only
a query that asks for it (`?completed`, `$done`, `@@`), parked and archived
projects stay out, and hidden work answers only `+hidden:only`. Each row names
the task, then its reference (`work#12`), project, ancestors, deadline, its
state unless Backlog, and the field that matched when it was not the title. A
query Docket cannot honor, such as `$ready`, says why in red and keeps the list.
With the field empty, the list is the Overdue and Today agenda the status menu
shows.

↑/↓ choose a task. Enter, or a double-click, shows it in Docket with the cursor
on it, as a notification does. `Cmd+D` marks it Done, `Cmd+L` defers it an
hour, and `Cmd+R` acknowledges it, as the agenda's submenu does, without
bringing Docket forward; the line under the list says what Docket answered, and
the list is asked for again, so a task that left the answer leaves it. Esc, or
clicking elsewhere, puts the panel away; the query stays, selected, so the next
open can repeat it or be typed over. A Docket that had exited starts again in
the background to answer, never in the window, and one too old to search says
how to update it.

The shortcut is set in Settings beside the other two, and stored as
`docketFindHotKey`, spelled like `docketHotKey`:

```bash
defaults write io.github.sglavoie.docket docketFindHotKey 'ctrl+opt+f'
defaults write io.github.sglavoie.docket docketFindHotKey ''    # off
```

### Links, Services, and Shortcuts

Other apps reach Docket.app three ways, all answered by the running app and none
by a second copy of Docket.

`docket://` links show something or fill quick capture, and nothing more, so a web
page opening one can never commit, complete, or delete work unseen:

| Link | Does |
| --- | --- |
| `docket://` or `docket://show` | Shows Docket. |
| `docket://capture?text=…` | Opens quick capture holding the text; without `text`, opens it as it was. |
| `docket://view/inbox`, `/today`, `/due` (or `/overdue`) | Shows Docket in that mode, as a status-menu count does. |
| `docket://view/done` | Shows All with the filters `$canceled $done ?completed`, replacing any others. |
| `docket://task/PROJECT-ID/TASK-ID` | Shows that task, by the stored IDs in `tasks.md`. |

Encode spaces as `%20`: a `+` stays a `+`, as the grammar reads it. A link that
shows Docket brings the app forward the way a Dock click does, so one clicked on
a widget over the revealed desktop puts the windows back rather than changing
Docket behind them. A link that
arrives while Docket is still starting waits for it, and one Docket refuses (a
mode change while something is open, say) is said in Docket's window.

**Capture to Docket**, in any app's Services menu (and the right-click menu),
opens quick capture holding the selected text, collapsed to one line, for you to
read the preview and press Enter. A selection that is only a web address becomes
` &https://…` with the caret before it, so the title is what you type first and
the address is attached. `docket://capture` does the same with its text. Neither
replaces an unsaved draft: the panel opens on the draft you had and says so.
macOS lists the service once Docket.app has run from `/Applications`; System
Settings › Keyboard › Keyboard Shortcuts › Services turns it off or gives it a key.

**Shortcuts**, Spotlight, and Siri get these actions; Capture Task, Show
Docket, Count Docket Tasks, and Get Docket Agenda are also offered as App
Shortcuts ("Capture a task in Docket"):

- **Capture Task** commits its text through the composer's grammar without
  opening anything, each line its own task in one undo entry, and returns what
  Docket said; a refusal is the action's error. It starts Docket if it had
  exited, in the background when it can.
  Unlike quick capture it keeps no recovery copy, so a run that fails is simply
  reported, and running a shortcut twice captures twice.
- **Show Docket** brings Docket forward, in Inbox, Today, or Overdue when chosen.
- **Count Docket Tasks** returns the Inbox, Today, Overdue, or Done Today count
  from the live pulse. A Docket that had exited starts again in the background
  to answer it.
- **Get Docket Agenda** returns the tasks the status menu lists — Overdue,
  Today, or both, at most 20 — as Docket Task values carrying the title,
  project, deadline, priority, and whether it is overdue.
- **Find Docket Tasks** returns the tasks a query finds, as quick find lists
  them — best match first, 20 unless set, at most 200 — as the same Docket Task
  values. It is also an App Shortcut ("Find tasks in Docket").
- **Show Docket Task** brings Docket forward with the cursor on one Docket Task.
- **Mark Docket Task Done**, **Defer Docket Task** (by a number of minutes,
  60 unless set), and **Acknowledge Docket Task** act on one Docket Task as `wd`,
  a deferral, and `R` would, each its own undo entry, and return what Docket
  said; a refusal is the action's error. A task can be picked from the agenda,
  found by typing in the picker, which searches every task as quick find does,
  or passed from Get Docket Agenda or Find Docket Tasks, and a saved shortcut
  keeps naming it by its stored IDs after it leaves the agenda.

The task actions, like Count Docket Tasks, Get Docket Agenda, and Find Docket
Tasks, start a Docket
that had exited in the background, never in the window, and fail only when
none can run that way: with the background turned off in Settings, or a Docket
too old for it.

### Widget

The Docket widget (desktop and Notification Center, small, medium, or large) shows the
pulse's Inbox, Today, Overdue, and Done counts. Docket.app writes each pulse to
`pulse.json` in the App Group container and asks for a redraw, so the widget
never reads the workspace: it shows what the app last heard and how long ago.
At local midnight the counts describe a day that is over, and it says to open
Docket instead. The small widget opens Due while anything is overdue and Today
otherwise; the medium and large ones open each count's mode — for Done, All
filtered to `$canceled $done ?completed` — and have a Capture button, all
through the links above. The large one adds the agenda the status menu lists:
up to six overdue and Today tasks, each with its project, deadline, and
priority, opening Docket on that task, and a `N more in Docket` line for the
rest. Each row has buttons for Done, Defer 1 Hour, and, where `R` would
accept it, Acknowledge. The sandboxed widget cannot reach Docket's control
socket, so a button writes its request to `widget-actions/` in the App Group
container and posts a Darwin notification; Docket.app takes the request, sends
it to Docket as the status menu's act would, and reports a refusal as a
notification. The widget leaves a Done or deferred task out, and its count,
at once. With Docket in the background, a request is taken as soon as it is
made. One Docket.app has not taken within a minute, because Docket.app was not
running, is dropped rather than run later, and the task is listed again. Docket sends that list beside the pulse, so a Docket too old to send it
leaves the large widget saying to update it.

### Notifications and the Dock badge

A check-in whose day comes, a timed deadline that passes, or a deferral that
ends becomes a Notification Center alert. Docket reports what is due over the
next eight days, and Docket.app books each one with macOS for its moment, so the
alert appears on time even if Docket.app has quit since, or the Mac was asleep;
a newer report withdraws what no longer applies, and the arrival Docket then
reports live is not shown a second time. What a booking says is what Docket
reported last: a task finished elsewhere after Docket.app quit may still be
announced, and its buttons then say Docket refused. Work that arrives with no
booking, as a check-in set for today, is reported live. Up to three at once are
one alert per task: clicking it shows Docket with the cursor on that task, and its
buttons act without leaving the app you are in — Acknowledge, offered only where
`R` would accept it, Done, which commits it as `wd` would, and Defer 1 Hour; a
button pressed while no Docket runs starts one in the background to answer it.
More arrive as one summary that brings Docket forward. macOS asks for
permission the first time there is one to book or show, and none is shown while
Docket is already in front. Settings turns them off (`docketNotifications`) at
once, withdrawing every booking. If
macOS has notifications for Docket.app turned off, Settings says so under the
toggle and offers Open Notification Settings.

The Dock icon carries the overdue count from Docket's pulse, and none at zero, or
the menu-bar icon does when Docket has no Dock icon (`docketDockBadge`, also in
Settings):

```bash
defaults write io.github.sglavoie.docket docketNotifications -bool false
defaults write io.github.sglavoie.docket docketDockBadge -bool false
```

A bell while Docket.app is in the background bounces the Dock icon once rather
than beeping. For an enrolled workspace, Docket exchanges with mobile automatically
at launch, on focus, and every five seconds while its session runs, including
with the window hidden. Incoming changes refresh the Outliner after an active
edit, picker, selection, or child view is closed. An unavailable exchange is
retried automatically; the status strip reports failures and pending changes.
Cloud delivery may take longer. Quitting Docket.app stops automatic sync; closing
the window keeps it running, and so does `q` while Docket keeps running in the
background. See `docs/guide/storage.md` for enrollment and sync status.

### Terminal config

The terminal reads `~/.config/docket-app/ghostty.conf`, in Ghostty's own config
syntax, and not Ghostty.app's config. To share that one, write:

```text
config-file = ~/.config/ghostty/config
```

Settings has Open and Reload for it; Reload says "Reloaded." or lists what
Ghostty found wrong in the file. The app always overrides `command`,
`wait-after-command`, and `confirm-close-surface`, since it owns the process
Docket runs in.

### Pointer

Docket answers clicks and the wheel itself; what each one does is in the
guide's [Pointer section](../docs/guide/outliner-keys.md#pointer). The app
adds three things:

- **Shift+drag selects text.** While Docket has the mouse, a plain drag is a
  click to Docket, so hold Shift to select and copy as in any terminal. This is
  Ghostty's default (`mouse-shift-capture = false`), and the app leaves it
  alone.
- **Clicking an inactive window only brings it forward.** That click and its
  release never reach Docket, so it moves no cursor and opens no menu; the
  same holds for a right-click. Click again once the window is in front.
- **Scrolling speed is Ghostty's.** A trackpad sends one wheel step per row of
  finger travel, momentum included, and a mouse wheel notch sends three. To
  change either, set `mouse-scroll-multiplier` in the terminal config, e.g.
  `mouse-scroll-multiplier = precision:0.5,discrete:2`; the app sets none.

To turn the pointer off, use View → Mouse in Docket's own `M` menu bar; it is
saved for this machine, so Docket.app keeps it across launches. With it off, a
plain drag selects and the wheel moves the cursor, as Ghostty's alternate
scroll turns it into arrow keys.

### Drag and drop

Dropping files on the window types their paths, shell-escaped and
space-separated, as a paste would; URLs and text drop as text. Type `&` in
capture and drop a file to attach it, or drop one into the Attachments
panel's add prompt.

### Window background

Docket announces its theme background to the terminal (OSC 11), and the title bar
takes that color, with light or dark title text to match, so the window reads
as one surface. Before Docket sets it, the title bar uses the Ghostty config's
`background`.

With Settings › General › Match macOS appearance on (`docketFollowAppearance`, the
default), Docket.app tells the running Docket to use its light or dark theme when Docket
starts and again whenever macOS changes appearance. Docket uses that theme for the
session only, so a Docket started from a terminal keeps the theme it saved, and
`T` still toggles until the next change. An older Docket ignores the request.

### Menus

Edit has Undo, Redo, Cut, Copy, Paste, Select All, and Find. In the terminal,
`Cmd+Z` and `Shift+Cmd+Z` undo and redo the focused draft; while browsing,
they use Docket’s workspace history. The menu names the available action and
disables it when unavailable. `Cmd+F` opens Outliner search. Native fields
in Settings and quick capture retain AppKit editing; Cut remains a native
text-field action. View names the
zoom keys (Actual Size, Zoom In, Zoom Out) and Enter Full Screen
(`Ctrl+Cmd+F`); Window has Minimize, Zoom, Close, and Bring All to Front; and
Help has Docket Guide, which opens the user guide on GitHub, and Report an Issue…,
which opens the repository's issues.
The app menu also carries Relaunch Docket. Peek's attachment links (OSC 8) open
with `Cmd`-click.

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

Ghostty's standard themes ship inside the app, so `theme = Catppuccin Mocha`
works without Ghostty.app installed; `build-ghosttykit.sh` vendors them, with
Ghostty's terminfo and shell integration, into `Vendor/resources/`.
Settings › Terminal › Theme lists those bundled themes and themes in
`~/.config/ghostty/themes`. A selected theme applies immediately to the open
terminal and is saved as `docketTerminalTheme`; the optional Dark Mode choice follows
macOS appearance. “From the Ghostty config” removes the override. Theme changes
keep the terminal's current zoom. Docket.app also sends the selected theme's
resolved colors to the running Docket process, which repaints its task colors,
selections, and background for that session. Clearing the selection restores
Docket's saved theme or the “Match macOS appearance” setting. A Docket started outside
the app keeps its own saved theme.

## Layout

```text
project.yml                 XcodeGen spec
GHOSTTY_VERSION             pinned Ghostty tag, Zig version, Zig checksums
XCODEGEN_VERSION            pinned XcodeGen version and checksum
patches/ghostty/            patches applied to the pinned tag (see its README)
scripts/                    build-ghosttykit, generate-project, build-app,
                            test-app, sign, install-app, make-icons
DocketApp/App/                  AppDelegate, menus, lifecycle
DocketApp/Ghostty/              runtime glue, config, surface view (ported, trimmed)
DocketApp/Shell/                window, status item, hotkey, preferences, Docket resolution
DocketApp/Resources/            Info.plist, entitlements, docket-launch, Assets.xcassets
DocketWidgets/                  the WidgetKit extension, its Info.plist and entitlements
Shared/                         link routes and the pulse snapshot, in the app and the widget
DocketAppTests/                 XCTest
NOTICE-ghostty              upstream MIT notices for the ported sources and themes
```

The icons are drawn by `scripts/make-icons.swift` and committed: rerun
`swift macos/scripts/make-icons.swift` after changing the artwork there,
never edit the PNGs by hand. `StatusIcon` is a template image, so macOS tints
it for light, dark, and highlighted menu bars.

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
