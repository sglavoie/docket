# Wow pass: everyday terminal screens

This review closes the six-session terminal presentation pass on 2026-11-14,
over base revision `4ced6793`. The fixture is fictional, captured through the
real interface in a temporary workspace. It covers dark and light appearances,
the roomy everyday view, and the minimum 60×20 terminal.

## Before and after

| Problem | Before | After and evidence |
| --- | --- | --- |
| Two title columns in All | Tasks below a project header started before merged singleton tasks. | Root identities occupy one bounded column before the shared badges and title. All, Today, Recent, and ranked search keep their respective columns; single-project preview rows remain compact. |
| Flat siblings looked like subtasks | Roots wore `├──` / `└──` and project-colored continuation lines. | Roots have no connectors. Muted connectors belong to actual descendants and checklist items. All's three checklist rows and Select Project's root rows show the distinction. |
| Floating lane headings | Tinted labels began partway across the row. | The fold arrow and lane name lead at the left; the rule follows the label and count. All and Recent use the same treatment. |
| Too many competing colors | Lane fills, colored tree lines, identities, priorities, and deadlines all competed. | Lane names and active tabs are bold neutral text, with Today alone accented; counts and rules are muted. Only `%` carries project identity color. Dates and priorities retain their attention cues. |
| Doing read as an alarm | The ordinary Doing glyph was red. | Ordinary Doing uses the shared accent in both themes. The screenshot fixture's login task deliberately retains `#bug`, whose semantic red still wins. `TestDoingStateUsesAccentColor` separately verifies the untagged glyph in both themes. |
| Now had a small clock and meter | `just started` and an eight-cell focus bar undersold the screen. | Roomy Now has five-row elapsed digits; focus has remaining minutes and a meter across the reading column. The resume note and three next steps remain visible. Short frames use text time when the block would crowd out task content. The compact Now card keeps its small meter. |
| All lacked completion feedback | Spare space below its rows stayed empty after finishing work. | One completion produces a muted `Done today · 1` and seven-day sparkline below a blank separator. It disappears when the list needs the space. Today's richer progress, week-ahead, and resume blocks retain their own layout. |
| Labels repeated or obscured meaning | The Now card was unlabeled, lifted counts read `+1 in Today`, and Peek repeated state and priority in `Row:`. | The card starts `Now:` when the full title fits, counts read `1 more in Today`, and Peek omits the redundant legend. Help still explains the glyphs. |

Before evidence is retained outside the repository: session 1's
`/tmp/docket-session1-before/all.png` records the old columns, connectors,
headings, colors, and lifted labels; session 3's
`/tmp/docket-session3-before/{now,focus}.png` records the old clock; session 4's
`/tmp/docket-session4-before/all.png` records the empty space; session 5's
`/tmp/docket-session5-before/{all,peek}.png` records the unlabeled card and
redundant legend. These are historical captures, not a claim that every old
theme and size was recaptured. The fixture gained resume/checklist content in
session 3 and a completion in session 4, so those additions are distinguished
from rendering changes.

## Reproduce

Run from the repository root, serially, with `vhs` installed:

```sh
./scripts/shots.sh /tmp/docket-wow-dark-wide dark wide
./scripts/shots.sh /tmp/docket-wow-light-wide light wide
./scripts/shots.sh /tmp/docket-wow-dark-minimum dark minimum
./scripts/shots.sh /tmp/docket-wow-light-minimum light minimum
```

Each run builds the current tree and captures 16 PNGs: welcome, All, Peek,
Today, Now, focus, Help, menu, All with progress, ranked search, Recent,
Select Project, Inbox triage, Start the Day, Close the Day, and Day closed.
`--appearance` selects the app's built-in palette without saving a preference.
VHS runs its browser headlessly; no desktop window or physical-device trial is
part of this review.

The script retains `shots.tape` and `terminal-grid.txt` beside the images.
The latter comes from `stty size` in the same PTY immediately before launch.
The minimum preset is 630×380 pixels with no padding at font size 15, and
refuses a result other than `20 60`. This is an observed cell grid, not an
inference from pixel dimensions. Font availability can change that grid on
another host; the refusal should be investigated rather than bypassed.

Escape and screenshot commands have settling intervals. Without them a
following key can become Alt input, or a screenshot can show part of the next
frame. Early captures exhibiting those artifacts were rejected and recaptured.
Welcome is captured while recording is visible, before hidden fixture setup.

## Review findings

The guide audit corrected residual descriptions of whole project labels being
colored, the old project-header indentation, and a slug-only shared column.
Select Project now explicitly documents connector-free roots. README's general
outline description and CLAUDE.md's updated presentation invariants remain
accurate; no new contributor rule was needed. README media was regenerated and
remained byte-identical.

The roomy captures are 1400×820 pixels with a measured 125×38 cell grid.
Both palettes keep project identities, task titles, dates, and selected rows
legible. Select Project's preview has aligned badges and no root connectors;
ranked search keeps context beside the project without shifting the title.
Recent shares the quiet lane headings and gives actual checklist descendants
their connectors. Triage, Start the Day, and Close the Day keep their title,
position, and controls readable above the host; the Day closed summary shows
the same completion total as All and Today.

At 60×20 the long All and Recent lists use a scrollbar, leaving 59 usable row
columns, so root identities correctly stay inline. All's completion line is
absent because the rows fill the window. Today has fewer rows and keeps its
progress and week-ahead canvas. Select Project omits its preview; the task list
and controls remain readable. Help wraps its category tabs and the menu scrolls
its own entries. Peek pages the remaining detail without a repeated legend.

Both Now block clocks fit this particular 60×20 fixture after quiet feedback
is cleared. The full title, resume note, and three steps survive; this capture
does not demonstrate the text-clock fallback. The existing
`now-screen-clock` scenario and Now-screen unit tests cover tighter content and
feedback budgets. Triage and day-card metadata shorten with visible ellipses;
their task titles and controls survive. Day closed drops the 26-week grid
whole at the floor, retaining the completion, weekly total, and send-off.

All 64 final PNGs were visually inspected. Each directory below contains all
16 screens, its tape, and the measured PTY dimensions:

| Appearance | Pixels | Cells | Evidence directory |
| --- | --- | --- | --- |
| Dark | 1400×820 | 125×38 | `/tmp/docket-session6-dark-wide-final` |
| Light | 1400×820 | 125×38 | `/tmp/docket-session6-light-wide-final` |
| Dark | 630×380 | 60×20 | `/tmp/docket-session6-dark-minimum-final` |
| Light | 630×380 | 60×20 | `/tmp/docket-session6-light-minimum-final` |

No remaining inconsistency among the eight presentation problems required a
production rendering change. This session changes the capture procedure,
documentation, and source organization required by repository fitness checks.
The screenshot filenames describe states, not assertions:
`all-progress.png` at the floor intentionally contains no progress line.

## Validation

Focused checks passed for Select Project's tree rows, Doing's accent in both
palettes, neutral lane headings, the filtered Peek legend, root title alignment
across folds and widths, and Now's text fallback under short content budgets.
`./scripts/check_docs.sh` passed, `just readme-media` regenerated the unchanged
animation, and shell syntax and whitespace checks passed. The targeted
`now-screen-clock` scenario supplements the pixels with resize and
`no_layout_issues` assertions; its result is retained in the session handoff.

The ASL runner owns the authoritative completion checks: the review document,
`./scripts/readme_media.sh --check`, and `just ci` (including race tests,
coverage, budgets, lint, module metadata, dead code, and build). Those are the
release gate, not a conclusion inferred from screenshots. The worker does not
duplicate that entire final gate; the runner records its result after this
implementation handoff.

The first runner replay passed the behavioral suites but found two files above
the 500-line limit and a shared fixture in a test-bearing file. Peek legend
tests now live in a separate test file, and the fixture lives in the existing
helper-only file. An initial canvas split exceeded Outliner's 86-production-file
ceiling on the second replay. The final organization keeps canvas functions in
`outliner_empty_state.go` and moves two row-inspection helpers into the existing
`outliner_rows.go`. Both file-size and package-file limits are satisfied without
changing function bodies or relaxing guards. The complete root race suite and
focused Outliner race tests passed after this consolidation. Existing visual
evidence remains valid.

Lint also identified four import-grouping errors and a direct error comparison
in the Now-feedback test. Imports were formatted and that assertion now uses
`errors.Is`. Its focused race test passed, and TUI-wide lint reported zero
issues. Outliner lint also passed after the final consolidation. Authoritative
full CI replay remains the runner's responsibility.

## Deliberately retained

- Today keeps its richer progress and week-ahead canvas. All gets only the
  quiet completion line; launch still opens Active Projects, All.
- Semantic task tags still override state glyph colors, including `#bug`.
  Overdue, priority, review, and project-sigil colors keep their meanings.
- Ranked search remains a flat list with its own identity width. Select Project
  uses compact single-project rows and hides the preview below its two-pane
  minimum; neither is forced into All's cross-project spacing.
- Peek's workflow value chips retain their tints. Only lane/tab treatments and
  redundant legend entries changed; this is not a Peek-card redesign.
- Now keeps its existing minute refresh cadence. No seconds timer, new theme,
  or additional animation was introduced.
- Capture, the expanded form, native macOS/iPhone presentation, and the
  intentional three-row chord guidance remain outside this pass.

VHS pixels demonstrate the terminal renderer on this host, not Ghostty native
window chrome, physical iPhone behavior, or iCloud delivery. Scratch evidence
under `/tmp` is ephemeral; the checked-in tape generator and ordinary scenario
and geometry tests are the reproducible record.
