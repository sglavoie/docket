# Presentation grammar

What the Outliner draws around the task list: the title row and its workspace
pulse, the menu bar, contextual guidance, the status strip and the advisories it
carries, the conflict browser, how the cursor row is marked, and the colors a
tag wears.

## Common actions

| Goal | Keys | Example |
| --- | --- | --- |
| Find a shortcut | `?`, then `/` | Search `defer` to find scheduling controls |
| Explain a narrow view | `Q` | Remove one restriction with its numbered key |
| Open the menus | `F10` | Task → Set Up Check-Ins |
| Keep the menu bar visible | `M` | Press again to hide it |
| Reread feedback | Help → Recent Messages | Recover the full text of an expired message |

Help puts capture, search, Peek, and editing together under Everyday, followed
by Plan, the Doing/Done Status actions, and Recovery. These actions through
Undo fit on the first screen at 60×20.
Other statuses, task details, hierarchy, and selection follow. Search also matches those group names. The label
**Explain Current View** opens the same `Q` readout.

## Find a shortcut

In `?` help, `/` searches within the selected category tab. Each search word
may match the key, description, category, or an alias of the same entry, so a
category and an action can be searched together. Every word must match that
entry; words never combine across different entries. While searching,
each tab shows the number of matches selecting it would reveal, including its
own case-insensitive fallback. If that category has
no matches but others do, the empty view names the category and counts the
matches available elsewhere. Press `Tab` to show all matches with the same
query, including while typing. This explicitly selects All; search never
switches categories on its own. Help measures shortcut widths per section or
task group and uses two columns when each description has at least 24 columns
of room. Search
highlighting follows matches across wrapped description lines. When a search
matches an alias, that alias stays beside the description and wraps as needed
so the reason for the match remains readable.
An exact key, description, or alias match appears first under `Exact matches`;
the other results retain their category order. Clearing the search restores the
ordinary grouped reference, and category tabs still constrain both kinds of match.

Help remembers its query, category, and scroll position separately for each
surface until you quit Docket. Press `?` to close a filtered reference and reopen
it later in the same place; `Backspace` clears the query and returns to All.
Available actions refresh on every opening. If a category has disappeared,
Help returns to All, and shorter results or a resized window clamp the scroll.

## Read the title and current restrictions

The title names the surface, scope, and mode. On the right, the workspace pulse
summarizes nonzero counts, for example `workspace: inbox 3 · today 2 · overdue 1`.
Wide windows also show the modes reached by `Tab` and `Shift+Tab`. Ranked search
replaces the mode strip with `Search · all modes`: results span the displayed
project scope, including tasks outside the mode you started in. Its context row
summarizes active filters and the admitted task population; `Q` gives details.

A second row appears while the view has restrictions or a selection. It names
the active lens, search, filters, sort, Focus, and section zoom. If they do not
all fit, `+2 more · Q details` reports the hidden segments. Press `Q` for the
complete, scrollable explanation and numbered controls to remove restrictions.
See [search and restrictions](search-and-lenses.md#reading-and-peeling-the-narrowing).

## Open a menu

Press `F10` to open the menus. Press `M` to keep the menu bar visible or hide it.
The bar contains Task, View, Go, Find, and Help. Opening a menu temporarily shows
a hidden bar; closing it restores the previous setting.

Task begins with capture, then groups everyday actions under **Plan**,
**Status**, and **Details**. Today and Next apply the same planning actions as
`wt` and `wn`. **Set Status…** opens the existing state picker, where you can
choose Doing or Done; **Backlog (clear plan)** performs `wb`. Details includes
priority, due/defer/check-in scheduling, tags/context, pinning, and project moves,
followed by editing and supporting material. Group headings are skipped by the
cursor and disappear when none of their actions apply.

Use left/right or `h`/`l` to switch menus, up/down or `j`/`k` to select an entry,
and `Enter` to run it. `Esc` or `F10` closes the menu. Menus overlay the current
view and preserve its scroll position, selection, and Peek. Tall menus scroll.
Direct shortcuts remain usable while a menu is open.

Capture, Edit, Today, Next, Set Status, Backlog, Undo, and Redo keep their
positions at the start of Task. An unavailable everyday action stays visible
in muted text. Focus it to read the reason below the list; Enter leaves the
menu open. Specialist actions that do not apply are omitted. The Task menu offers **Add Checklist Item**
(`c`) when the cursor names a task or one of its checklist items. Modes and
toggles carry a checkmark — the
active mode, whether the bar is kept — so a menu reads the state back instead
of only offering to change it, and every row spells the key it is bound to.
Those keys keep working while the menu is up: pressing one the open menu does
not list closes the menu and does what the key does, so nothing is trapped
behind a dropdown.

The Task menu also offers Undo and Redo with the next recorded action in their
labels, such as `Undo: Set Priority: Ship release`. These describe the same
workspace-wide history that `u` and `Ctrl+R` restore, including after a restart.
The descriptions refresh each time the menu opens; reading them changes nothing.
An action with no available history remains visible with “Nothing to undo” or
“Nothing to redo” when focused.

Entries with no shortcut are available only through the menus. View → Clear All
Folds opens every folded section, project group, and task subtree, reporting
`All folds cleared` or `Nothing is folded`. View → Advisories toggles ambient
workspace notices. Task → Set Up Check-Ins schedules the tasks named by a
recent check-in reminder.

Menu-bar visibility is saved on this machine, as are the theme and advisory
setting. These preferences do not follow a workspace to another computer.

### Terminal keyboard compatibility

`F10` opens the menu in every supported keyboard mode. Feedback uses `F10`
until the terminal confirms enhanced keyboard support, then uses `Ctrl+M`.
Without that support, `Ctrl+M` arrives as Enter and `Ctrl+I` as Tab.
Use `Alt+→` to move forward in navigation history and `Alt+↑` to focus the
parent. `Ctrl+I` and `Shift+Enter` remain enhanced-keyboard aliases for those
operations. To insert a newline in capture, comments, or the text editor,
use `Alt+Enter`; `Shift+Enter` remains an alias. Multiline paste works in capture
too. The menu keeps every operation available by arrow keys and Enter.

Docket.app can follow macOS appearance for the current session without changing
the saved terminal theme.

## Read workload and feedback

Today mode compares open work with the workspace's 30-day completion pace,
for example `9 open · median 3 done/day (30d)`. It shows no comparison until
there is completion history, and hides optional information in narrow windows.

The footer describes the current interaction. Press a chord prefix such as `w`,
`f`, or `g` to see its next keys. `w` separates Plan from Status; `?` lists all
shortcuts, including those a narrow footer cannot show. Pickers and editors
show their own controls, and closing them restores the outline's shortcuts.

The `Esc` entry names what the next press does, not what you are in. Standing in
a Focus inside a lens it reads `Esc leave Focus`, because that is the layer the
key takes first; once the Focus is gone it reads `Esc leave lens`. Open Peek and
one entry covers both keys that close it, `b/Esc close Peek`; when the paging
hints wrap that footer, an entry that would sit alone on the second row is
dropped rather than given the row. Start a range and
it reads `Esc freeze range`, since freezing is what that press does and clearing
is the one after. Search typed under a lens reads `Esc restore lens` rather than
`Esc clear search`, because the lens is what comes back. The empty-state
sentences use the same words — `Press Esc to unzoom`, `Press Esc to leave
Focus` — and name the same layer: an empty Focus taken inside a zoomed section
says `leave Focus`, and an open Peek over an empty view says `close Peek`, so
the footer and the explanation never disagree.

In capture and its expanded form, the footer says `Esc keep draft`;
`Ctrl+X` explicitly asks to discard. Other editors retain their existing
confirmation on close. Wherever that confirmation applies, the entry answers
for your draft. An untouched composer,
form, comment box, or editor reads `Esc cancel` and leaves at once. Type
something and it becomes `Esc discard`, and the press asks first: `Y discard ·
N/Esc keep editing`. Declining keeps every character and leaves you in the
editor, which is why that question is worded apart from the `y confirm ·
n/Esc cancel` used to confirm a deletion.

Pending chord hints dim both the key and label when an action would currently
do nothing or be refused: for example, conflicts when none are outstanding,
an already-selected mode or priority, an unavailable workflow transition, or
clearing empty filters or sort settings. Emphasis is recomputed on every render;
dimmed shortcuts remain usable. Toggles and Recent's refresh stay prominent.

Feedback appears above the controls, so an error never hides the keys needed
to correct it. Messages can reduce the room available to the list or editor;
the active field and its recovery controls remain visible.

Ordinary feedback lasts two to six seconds, depending on its length; undo
feedback lasts six seconds. Warnings allow six to twelve seconds, based on text
length and wrapped rows when published. Warnings and errors wrap across up to
three rows so the reason and recovery action can be read together. Longer
diagnostics point to Help → Recent Messages for their full text.
A mode change clears transient view feedback.
A save failure with a retained draft stays visible until that field is edited
or the draft is saved or discarded. Moving the cursor, waiting, or opening Help
does not dismiss it. The selection count stays in the second header row.

### Reread a message

`F10` → Help → Recent Messages opens the latest feedback in a read-only,
scrollable viewer, even after it has expired or a mode change has cleared it.
The viewer keeps the latest 20 messages for this session, including operation
errors and transient view feedback. Left visits older messages and Right returns
to newer ones; `Message 2 of 8` identifies your position, with 1 the newest.
Each entry shows its original local time (`14:32`), including the date when it
is from another day, and whether it is feedback or an error. Adding details
preserves that time. Up/down scroll the displayed
message; browsing away and back restores its scroll position, clamped to the
current window size. `y` copies the message text in full; `Y` copies all retained
messages newest first, with full local timestamps (including UTC offsets) and
message kinds. `Esc` returns to the outline. Ambient advisories and copy notices
do not enter the history.
Press `/` to filter messages by text or kind, case-insensitively. Matching
characters are highlighted in the message and its kind; copied text stays plain.
`Enter` leaves
the input so Left/Right can browse just the matches. `Esc` clears the filter and
restores the message and scroll position you started from; while typing, another
`Esc` leaves the input. `Y` still copies the whole retained history, including
messages outside the filter. While filtered, its hint names the full count
(`Y copy all 20`), and successful copying reports how many copied messages were
hidden by the filter. Searching changes neither timestamps nor messages.
After accepting a nonempty search with `Enter`, `Ctrl+Y` copies only its matching
messages, newest first, with the same full timestamps and kinds as `Y`. The hint
appears only when there are matches, and the result reports how many were copied.
Details added to an existing message update that entry. Recovery hints in the
recalled text are historical text, not controls in this viewer.

### Workspace advisories

Unresolved conflicts and projects that failed to load appear as persistent
advisories. The conflict notice names `gc` to review them; it disappears after
the last conflict is resolved. Use `docket conflict list` from a shell.

View → Advisories saves whether these notices are shown on this machine.
`docket --no-advisories` hides them for one launch. Hiding notices changes no data
and does not suppress operation errors, the `Q` explanation, or the error when
every project failed to load.

### While Docket is in the background

Docket names its window after the workspace pulse — `Docket · inbox 2 · today 3 ·
overdue 1`, nonzero counts only — so a terminal tab or title bar reads it
without switching to Docket; a view that names its own title keeps it, and quitting
clears it.

In a terminal that reports focus, Docket notes the workspace on disk when it loses
focus and compares on return. If another machine's sync, or another Docket,
changed it meanwhile, Docket reloads the catalog, planning, and every project, and
says `Reloaded: the workspace changed on disk`. It waits while you are inside a
picker, an editor, a visual selection, a ranked search, or another view, and
reloads as soon as you leave the last protected interaction, so nothing you are
acting on is renumbered under you. The selected task and scroll position are
preserved where still available, and a refresh does not replace save or undo
feedback. A failed read stays pending for the normal timer to retry. A reload
writes nothing.

`docket --notify` also sends a desktop notification (OSC 777) when work arrives
while the session runs: a check-in whose day comes, a timed deadline that
passes, or a deferral that ends. One arrival names its task; several are
counted, `2 check-ins · 1 deferral ended`. It reports only what crossed over
since the last minute's tick, so launching reports nothing already standing and
waking from sleep reports the gap once. A date-only deadline is not an arrival:
its day is the Today lane's and the overdue count's. It is opt-in because a
terminal that honors the escape shows it whether or not Docket is in view.

`docket --control PATH` lets a host such as Docket.app request capture and task actions
through the running session. Those actions use the same validation and undo as
the keyboard. A host may also search every task with the `/` grammar, whatever
the view is showing, which is how Docket.app's quick find works; list that same
open work for Spotlight; and narrow the counts and reminders it reports to some
projects while a macOS Focus is on, which leaves this window untouched. Read-only
completion, preview, search, and index requests remain available during editing;
mutations are refused while a protected interaction is open. With
`--headless` too, the same session runs with no terminal at all, answering
only that socket: Docket.app runs one after you quit with `q`, so its counts,
widget, sync, notifications, and Shortcuts keep working while no window is
open. It takes the same per-workspace lock as any session, so it never runs
beside one, and it quits by itself if the host that started it goes away.
See [Docket.app](../../macos/README.md#talking-to-docket) for host behavior and
[the architecture](../architecture.md) for the control protocol.

## Resolve a conflict

`gc` opens the conflict browser in the TUI and desktop app. It lists recorded
bundles newest first, followed by sync conflicts. `Enter` inspects the selected
conflict. Sync conflict details lead with what differs, the devices that wrote
each version, and the other files keeping that version would resolve. Full file
contents and diagnostic identifiers follow that explanation. `d` toggles a Git-style diff against the current content (recorded
remote content for project bundles). `b` switches to or from the base comparison. Project bundles offer
`l` to keep local and `r` to keep the current remote; sync conflicts use `[`/`]`
to select a version, `v` to keep it, and `c` to keep the current content. `e`
opens the in-app merge editor where text replacement is supported. `Ctrl+S`
reviews the merged result, and `Enter` confirms a resolution. Related files
and retry requirements are shown before confirmation. Errors retain the draft.

The pane scrolls with the ordinary list keys. `Esc` backs out one level at a
time and asks before discarding edits. With no unresolved conflicts, `gc`
says so in the strip instead of opening an empty overlay. See
[conflict resolution](storage.md) for the complete workflow.

## Undo and return

After a commit, feedback offers `u undo` while that transaction is still the
newest history entry. A relocation can also offer `Ctrl+O origin`; `Alt+→`
returns to the relocated task. Recovery keys remain visible in narrow windows.

Undo and redo follow a restored single task when it is visible, offering
`Ctrl+O origin` to return. If the task is hidden, the message names its destination
without changing the view. Bulk and project undo preserve the current selection
and position. After restarting Docket, undo still restores data but does not move
the cursor.

## Read cursor and context markers

The Outliner marks its cursor with a bright column at the far left. A selected
task also carries a subtle background band across the full row, so the eye can
follow a long title and its indicators without losing the selection. The tree
connectors, state and priority badges, deadline, project label, and search
highlights retain their own foreground colors over that band. Headers,
checklist items, and notes keep their margin mark and trailing fill.

The bar reverses the fold arrow on a section header and the `✓` on a task in a
visual selection. A cursor flash brightens the gutter and the task's band.
Other lists—the pickers, tag and parent selectors, notes, and project hub—use
whole-row inversion.

A list scrolled into the middle of a lane says which lane it is showing. When
the lane's header is above the window, the first list row becomes a pinned
context row naming the lane, the project the window opens in, and the lane's
count:

```text
▾  Waiting › [release-catalog]  (212) ──────────────────────
```

This pinned context row cannot be selected. Moving up scrolls toward the real
header; `gg` and `N` can still reach it. It is absent when the real header is
visible, in flat search results, and in single-project views. In a grouped
mode without lane headers, such as Inbox, it names the project whose heading
has scrolled away. Project names use neutral text, with identity color on their
brackets and tree connectors, leaving task titles and attention cues prominent.

## Read tag colors

Tags keep a color derived from their name, adjusted for the current theme.
For example, `#billing` keeps the same chip color wherever tags are shown.
Colors are a mnemonic: unrelated tags may share one, and color never determines
a task's behavior.

A row inside `#`, `f#`, or `F#` is an offer rather than a label, so it shows you
the color it is about to paint. Most of those rows paint nothing else and simply
wear their own chip: the descriptive tag rows, and the tag rows that `#` collects
under `CONTEXTS` because the same value is also in use as a context. A tag that does
something louder shows that instead — a row under `SEMANTIC` wears the color the
six-name vocabulary reserves for it, and a `parked` project row wears the warning
badge it hangs on the project. So the same tag can read as one color in a picker
and another as a chip; when they disagree, the picker is naming what the row is
about rather than repainting the chip. Two rows stay plain on purpose: the
`##name` radio rows in `#`, which set a field rather than name a tag, and `F#`'s
`Untagged` row, which names the projects carrying no tag at all. `fc`'s rows are
plain for the same reason: a context is a field, not a chip. A Select
Project row is the one place a tag is named without its color, because its tags
share the one secondary line that carries the counts beside them.

The semantic tags `bug`, `question`, `epic`, `story`, `feature`, and `idea` also
tint the task's state glyph, in that precedence order when several apply.
The task title stays ordinary text.

A task row shows no chips at all. Its columns go to the title, and Peek names
the whole tag set on one `Tags:` line, in an order that does not depend on how
you typed them: descriptive tags first in alphabetical order, then the semantic
ones, also alphabetical. A task tagged `ux`, `bug`, and `billing` reads
`#billing #ux #bug` however it was captured — the six semantic names are a
vocabulary every task shares, so the tags you cannot guess lead. Nothing is
dropped and nothing is counted away, because Peek wraps rather than clips: the
order decides reading sequence, not survival.
