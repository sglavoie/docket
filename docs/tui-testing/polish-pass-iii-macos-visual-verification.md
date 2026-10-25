# Polish pass III: macOS visual verification

Live checks of the tt.app changes from polish pass III sessions 9–13, run by
hand in the real app, and the integrated TUI frame review that closed the pass.
Each "Session N visual verification: PASS" line below is written only when every
check of that session was performed and passed. A check that was skipped,
blocked, or failed withholds the line, unless the operator explicitly waived it.
The one waiver is named under session 13, and because it is that session's only
check, session 13 carries no PASS line.

## Setup

- Revision: `acef2560` on `main`. The only uncommitted change was the README
  sentence recorded under session 11.
- Binaries: `just mac-install` built the Release app and installed it to
  `/Applications/tt.app`, with `tt` bundled at
  `v0.0.0-20261015043038-acef2560763c (commit: acef2560, built: 2026-10-15T04:45:30Z)`.
  `just install` then put the same commit at `~/.local/bin/tt`
  (`built: 2026-10-15T16:25:35Z`). That copy is newer, so the app ran it, as
  the README's "Which tt runs" rule says.
- Machine: macOS 27.0.1 (26A434), Xcode 27.0 (27A266a).
- Workspace: a new, empty throwaway at `~/tt-asl-p3-throwaway`, given to tt as
  `DOCKET_DATA_DIR` through the app's `ttEnvironment` default. tt created it on its
  first start.
- Defaults: the session changed only `ttEnvironment`. Afterwards it held its
  original three entries again (`EDITOR`, `DOCKET_EDITOR`, `VISUAL`), and
  `ttFollowAppearance`, absent before the checks and written by the Settings
  toggle, was deleted again. The exported backup was lost when the Mac
  rebooted, since it was in `/tmp`. The restore therefore used the domain as
  printed before testing.
- Date: 2026-10-15, 10:35–11:10 local.
- Who: the person at the Mac clicked and read each check. The agent ran the
  setup and recorded process, workspace, and defaults evidence from the shell.

### First attempt at check 1, discarded

The first appearance check, asked on 2026-10-14, came back FAIL. The app on
screen was not the one under test. A Debug build at
`macos/DerivedData/Build/Products/Debug/tt.app` had started at login (pid
2694). It ran a pre-feature `~/.local/bin/tt` at `7b053da4`, which answers the
appearance request as an unknown operation. With the operator's approval, the
agent quit the Debug app, pointed the tt Login Item at `/Applications/tt.app`,
updated `~/.local/bin/tt`, and launched the installed app. `ps` then showed
`/Applications/tt.app/Contents/MacOS/tt` (pid 99627) running
`~/.local/bin/tt --control …tt-app-99627.sock` (pid 111) with
`DOCKET_DATA_DIR=/Users/sglavoie/tt-asl-p3-throwaway`. Every check below ran
against that pair.

## Session 9: following the macOS appearance

| Action | Observed |
| --- | --- |
| Show tt on the empty throwaway workspace | PASS: "No tasks yet" |
| `Match macOS appearance` on; switch macOS Dark to Light | PASS: tt's colors turn light with no key press |
| Switch macOS back to Dark | PASS: tt turns dark again |
| Toggle off; switch macOS to Light | PASS: tt stays dark |
| Switch macOS back to Dark, toggle on again | PASS |

Evidence: `ttFollowAppearance` read `1` after the toggle round trip, and the
running tt was `acef2560`, the first commit to answer the appearance request.

Session 9 visual verification: PASS

## Session 10: status-menu count items

| Action | Observed |
| --- | --- |
| Open the menu-bar icon's menu on the empty workspace | PASS: no count items lead the menu |
| Capture `Count inbox task`, `Count today task $today`, `Count overdue task @2026-10-13`; reopen the menu | PASS: one item per nonzero count leads the menu |
| Choose Overdue, then Today, then Inbox | PASS: tt comes forward in Due, Today, and Inbox mode respectively |
| Choose a count item while a search is open in tt | PASS: tt comes forward and stays where it was |

Evidence: after the captures, Unfiled held `Count inbox task` and
`Count overdue task` in Inbox (the second due `2026-10-13`) and
`Count today task` in Backlog with a Today planning reference. That gives
Inbox 2, Today 1, Overdue 1. The operator reported PASS without spelling out
the items.

Session 10 visual verification: PASS

## Session 11: Cmd+Z in quick capture

| Action | Observed |
| --- | --- |
| Capture with `Ctrl+N`, then `Cmd+Z` in the empty field | PASS: the field's line reports tt's `Undid:` reply and the task leaves tt |
| `Cmd+Z` again in the empty field | PASS: nothing earlier is undone |
| Type text, then `Cmd+Z` | PASS: the typing is undone in the field and tt is untouched |
| Capture, change something in tt (`=`), then `Cmd+Z` in the empty field | PASS: refused with "Something newer was changed in tt; undo it there", and the capture stays |

The operator reported one observation: "The undo stack only works for the last
item from the quick capture though. Rest is good." That is decision D11 (only
the most recent capture, and only while it is still tt's newest undo entry), so
it is recorded as designed, not as a failure. `macos/README.md` now says so
plainly: "Only that most recent capture: a second `Cmd+Z` does not reach the
ones before it, which `u` in tt still can."

Evidence: the throwaway workspace ends with the captures the operator kept
(`Undo me first`, `Undo me now`). It holds no task from the capture the
operator undid, and the three count tasks are untouched.

Session 11 visual verification: PASS

## Session 12: Done from a notification

| Action | Observed |
| --- | --- |
| Capture `Notify done test @2026-10-15 11:04`, then bring another app to the front | PASS: a tt notification arrives for the task when the deadline passes |
| Notification buttons | PASS: Done is offered beside Defer 1 Hour |
| Click Done without opening tt | PASS: tt marks the task Done |

Evidence: `Notify done test` was stored with `due: 2026-10-15`,
`due_time: 11:04`, and after the button `state: done`,
`completed: 2026-10-15T11:06:57.6825-06:00`. The operator reported PASS without
listing every button.

Session 12 visual verification: PASS

## Session 13: VoiceOver on the status item

| Action | Observed |
| --- | --- |
| VoiceOver on the menu-bar status item: label, pulse value, hint, count items | NOT PERFORMED, waived by the user |

The user waived it on 2026-10-15 in their own words: "let's skip that one, I
don't care about VoiceOver, assume it's working". It is not a pass. The unit
test `StatusItemTests.testStatusButtonIsLabeledForVoiceOver` still covers the
label, the hint, and a value that follows the pulse.

Session 13 visual verification: NOT PERFORMED (waived by the user)

## Integrated TUI review

Every state sessions 1–8 changed was re-rendered at 60×20, 80×24, 100×35, and
120×40, in the dark and light palettes and in the monochrome projection of
each, and read as plain text. That is 13 states, 312 files. The states were the
description editor with a draft, the task `Space` sheet alone and over an open
Peek, the inline editor, the capture create-project offer and its decline, the
live ranked search, the applied search, the query readout, Peek under a search,
Peek on a task, and `?` help. Every frame fit its terminal in display cells,
and monochrome kept all of its content.

No two sessions interfered with each other. Two things were observed and left
alone, because neither comes from combining this pass's sessions:

- `?` help leaves three blank rows under its footer at every size. A render of
  `7b053da4`, before this pass, shows the same three rows.
- The anchored `Space` sheet, opened with Peek showing, covers the top of the
  Peek dock at 100×35. A modal covers rows rather than displacing them, so this
  is the anchoring rule working as written.

`just ci`, `just cross-machine` (two old-binary cases skipped for want of a
tag), `just mac-test`, and `just mac-install` passed at `acef2560`.
