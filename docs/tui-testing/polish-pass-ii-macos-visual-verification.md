# Polish pass II: macOS visual verification

Live checks of the tt.app changes from polish pass II sessions 7–10, run by
hand in the real app. Each "Session N visual verification: PASS" line below is
written only when every check of that session was performed and passed. A check
that was skipped, blocked, or failed withholds the line, unless the operator
explicitly waived it. The one waiver is named under session 9.

## Setup

- Revision: `5d290d01` on `main`, clean tree. `tt --version`:
  `v0.0.0-20261014185926-5d290d014d96 (commit: 5d290d01, built: 2026-10-14T19:02:14Z)`.
- Machine: macOS 27.0.1 (26A434), Xcode 27.0.
- Build: `just build` and `just mac-app` (Debug) gave
  `macos/build/tt.app` and `./tt`.
- Workspace: a new, empty throwaway at `~/tt-asl-11.2-throwaway`, passed to tt
  as `DOCKET_DATA_DIR` through the app's `ttEnvironment` default. tt created it on
  its first start.
- Defaults: the app's defaults domain (since renamed to `io.github.sglavoie.docket`) was exported before testing and restored
  exactly afterwards. Testing set `ttBinaryPath` to the missing
  `/tmp/asl-11.2-missing/tt`, and Settings then moved it to `./tt`.
- Date: 2026-10-14, 14:02–14:38 local.
- Who: the person at the Mac clicked and read each check. The agent ran the
  setup and recorded process, socket, and defaults evidence from the shell.

## Session 7: launch-error window and Settings tabs

| Action | Observed |
| --- | --- |
| Launch `macos/build/tt.app` with `ttBinaryPath` set to a missing path | PASS: the error window names `/tmp/asl-11.2-missing/tt` and offers Open Settings…, Try Again, and Quit |
| Dock click, then switch away and back | PASS: the error window comes back |
| Open Settings… | PASS: "tt Settings" has General / Shortcuts / Terminal / Advanced tabs, each laid out without clipping, and the window resizes per tab. Each tab holds the expected controls |
| Advanced: set the tt binary to `./tt` | PASS: the note reads "Takes effect when tt next starts" |
| Try Again on the error window | PASS: the error window closes and tt starts on the throwaway workspace |
| Menus | PASS: "Settings…" is in both the app menu and the menu-bar icon menu |

Evidence: after Try Again, `macos/build/tt.app` (pid 38871) was running
`/Users/sglavoie/1_dev_projects/sglavoie_tt/tt --control …tt-app-38871.sock`.
`~/tt-asl-11.2-throwaway` held a freshly created `.tt/` and `projects/`, and
the `ttBinaryPath` default read the `./tt` path entered in Settings.

Session 7 visual verification: PASS

## Session 8: shortcut refusals

| Action | Observed |
| --- | --- |
| Record ⌃⌥⌘A (quick capture's shortcut) in the show/hide recorder | PASS: "⌃⌥⌘A already opens quick capture. Choose another shortcut."; ⌃⌥⌘A still opens capture |
| Record show/hide back to ⌃⌥⌘T | PASS: the note clears and ⌃⌥⌘T shows and hides tt |
| Record ⌃⌥⌘T in the quick-capture recorder, then back to ⌃⌥⌘A | PASS: "⌃⌥⌘T already shows and hides tt. Choose another shortcut.", then the note clears |
| Press ⌘W in a recorder | PASS: "⌘W is tt's Close shortcut. Choose another shortcut."; Settings stays open |
| Press ⌘Q in a recorder | PASS: "⌘Q is tt's Quit tt shortcut. Choose another shortcut."; the app does not quit |

Evidence: the person at the Mac reported that the note wording matched. After
the test, `ttHotKey` and `ttCaptureHotKey`, absent before testing, held
`ctrl+opt+cmd+t` and `ctrl+opt+cmd+a`. That is the trace the recorders leave,
and the app (pid 38871) was still running.

Session 8 visual verification: PASS

## Session 9: sizing, hidden crash, hiding, accessibility

| Action | Observed |
| --- | --- |
| Shrink the terminal window from its corner | PASS: it stops at about 80×24 cells and tt draws normally |
| Hide with ⌃⌥⌘T, `kill -SEGV` tt's pid, then ⌃⌥⌘T | PASS: the window stays hidden through the crash; on show it carries the "tt stopped" banner, and Relaunch tt brings tt back |
| Open Settings, front the terminal, press ⌃⌥⌘T | PASS: only the terminal hides and Settings stays on screen |
| Enter full screen, then ⌃⌥⌘T, then ⌃⌥⌘T | PASS: the window leaves full screen and then hides; it shows again as a normal window |
| VoiceOver on the menu-bar status item | NOT PERFORMED, waived by operator: declined twice and never run, so it is not a pass |

Evidence: tt pid 43245 was gone after the SIGSEGV, and a new
`tt --control …tt-app-38871.sock` (pid 1172, started 14:13:47) ran under the
same app pid 38871. A first `kill` attempt hit nothing: its `pgrep -f` pattern
also matched an `asl ask` process whose argument quoted that pattern. The
signal that counted went to tt's exact pid.

The status-item VoiceOver check is waived, not passed. On 2026-10-14 the
operator amended this repair session's contract to waive it and say it does not
withhold this line. Every other session 9 check was performed and passed.

Session 9 visual verification: PASS

## Session 10: Command-Q

| Action | Observed |
| --- | --- |
| Capture open with a draft, ⌘Q | PASS: tt comes to the front with the "tt is busy." alert, offering Quit Anyway and Cancel |
| Cancel | PASS: the app stays and the capture draft is intact |
| ⌘Q again, Quit Anyway | PASS: the app quits |
| Relaunch, plain Outliner, ⌘Q | PASS: the app quits at once with no alert |

Evidence: a watcher sampled tt.app and tt processes, plus the control socket,
every 0.5 s. Quit Anyway ended pid 38871 and its tt by 14:37:42. The relaunch
(app pid 55600, then tt and its socket `tt-app-55600.sock`) began at
14:37:59–14:38:01. The clean ⌘Q left no process and no socket by 14:38:07.

Session 10 visual verification: PASS
