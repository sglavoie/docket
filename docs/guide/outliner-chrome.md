# Presentation grammar

What the Outliner draws around the task list: the title row and its workspace
pulse, the menu bar, contextual guidance, the status strip and the advisories it
carries, the conflict browser, how the cursor row is marked, and the colors a
tag wears.

## Common actions

| Goal | Keys | Example |
| --- | --- | --- |
| Find a shortcut | `?`, then `/` | Search `defer` to find scheduling controls |
| Find and run an action | `?`, `/`, type, `Enter` | Search `tags` to open the selected task's tag picker |
| Explain a narrow view | `Q` | Remove one restriction with its numbered key |
| Recover search matches | `Ctrl+G` | Show withheld finished or deferred matches, or explain the view |
| Open the menus | `M` or `F10` | Task → Set Up Check-Ins |
| Keep the menu bar visible | View → Menu Bar | Choose it again to hide the bar |
| Reread feedback | Help → Recent Messages | Recover the full text of an expired message |
| Inspect and reverse changes | Edit → Change History | Read before/after details, then undo or redo one change |

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
switches categories on its own. Every shortcut group shares one grid, measured
from the widest key and the widest description on screen: three columns when
each can hold that widest description whole, otherwise two when each
description has at least 24 columns of room, otherwise one. Grammar never
takes more than two columns. One blank line separates groups. Search
highlighting follows matches across wrapped description lines. When a search
matches an alias, that alias stays beside the description and wraps as needed
so the reason for the match remains readable.
An exact key, description, or alias match appears first; the other results
retain their category order. Clearing the search restores the ordinary grouped
reference, and category tabs still constrain both kinds of match.

In Outliner help, search results form a selectable list. Use up/down to choose
an entry and `Enter` to run it, including while typing. Its shortcut stays beside
the action. Exact matches lead, followed by literal words in action names,
then aliases and other fuzzy matches; ties keep category order.
The target line names the task, project, or selection; a bulk
selection includes its hidden count. An unavailable action explains what it
needs and keeps help open when you press `Enter`. Grammar entries are reference
text and stay in help too.
Search also includes the existing task and project menu actions, such as
**Set Check-In**, **Delete**, and **Archive Project**. Their menu route appears
beside the name; destructive actions retain their confirmation.

For example, select a task, press `?`, then `/`, type `defer`, and press
`Enter` to open its existing scheduling picker. Choosing a value uses the same
save and undo as the shortcut. Press `Esc` to finish typing without running
anything; then `?` closes help, `/` edits the query again, and `j`/`k` also choose
results. Another `Esc` clears the filter. If the target changes while help is
open, the action asks you to close help and choose it again.

Help on other surfaces remains a reference: `Enter` applies its search filter,
and exact results are grouped under `Exact matches`.

Help remembers its query, category, scroll position, and selected search result separately for each
surface until you quit Docket. Press `?` to close a filtered reference and reopen
it later in the same place; `Backspace` clears the query and returns to All.
Actions, targets, and availability refresh on every opening. If a category has disappeared,
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

In All, Agenda, and Today, while something is in Doing, a Now card stands between the
header and the lanes, marked by an accent bar. It names the task that entered
Doing most recently, its project, and how long it has been in Doing, such as
`website · 12m in Doing`. A second row shows its resume note, and a third its
checklist or subtask meter and `+2 more in Doing` when other work is active
too. The card follows the view's scope and filters, steps aside in the other
modes, in search, under a lens or a Focus, and in a terminal under 24 rows, and
gives a narrow window's room to the title before the age. It takes no keys or
clicks; the same task still sits in its Doing lane below.

`gd` (Go → Now) opens the **Now screen**: the list is put down and one task in
Doing fills the frame — the cursor's task when it is in Doing, else the Now
card's — with how long it has been in hand, its project, `Left off:` and its
resume note, and its checklist with the next open step marked `→`. `d`
completes it, `p` pauses it, `r` writes its resume note, `c` adds a checklist
step, and `Esc` returns to the list; each is the row's own operation and its
own undo entry. `f` starts a 25-minute focus session on the task: the screen and
the Now card above the lanes read `◷ focus 18m left` with a meter of the time
spent, and when it runs out the footer says `Focus done · 25m on <title> · take
a break` with the success sweep, plus a desktop notification on a `--notify`
run. `f` again stops it. A session writes nothing and lasts only while Docket
runs; finishing or pausing the task first ends it without a word. When the task leaves Doing the screen closes by itself, so
finishing lands on the list. From a mode that does not draw the task, such as
Inbox or Today, `gd` steps to All to open it, and `Esc` steps back to the mode
you came from. A visual selection, nothing in Doing, or a search or lens that
hides the task refuses it.

## Open a menu

Press `M` or `F10` to open the menus; press it again or `Esc` to close them.
Choose **View → Menu Bar** to keep the menu bar visible or hide it.
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
recent check-in reminder. Task → Note Where I Left Off writes the resume note
of the task that most recently left Doing in this session, wherever the cursor
is now.

View → Motion turns off the brief animations: the row a task
leaves behind, the mode badge lighting, workspace pulse counts lighting when they
change, the undo countdown on the divider, the Today sweep, the list gliding on a
page jump, a fold rolling open, the mark a moved or captured task lights where
it lands, and a new mode or screen easing in. View → Nerd Font
Glyphs draws the state, priority, date, checkbox, notice, pin, attachment, and
description symbols from a Nerd Font, such as Someday's thought bubble in place
of `¿`; it needs a terminal font that carries them. Every glyph keeps its
one-cell width, so no row moves when you switch.

View → Theme chooses how Docket is painted: **Dark** and **Light**, which `T`
toggles between, three curated palettes — **Ember**, a warm dark; **Fjord**, a
cool dark; **Sepia**, a warm light — and **Terminal Colors**, which asks the
terminal for its background, foreground, and sixteen ANSI colors and paints
Docket in them. Every theme is also in the `:` palette; type `theme`. A terminal
that does not answer within a moment leaves Dark on screen and says so; the
choice is kept, and the next launch asks again. From any palette theme, `T`
goes to Light or Dark by whichever the palette is not.

Menu-bar visibility is saved on this machine, as are the theme, advisory,
motion, and glyph settings. These preferences do not follow a workspace to
another computer. Launching with `DOCKET_GLYPHS=nerd` or `DOCKET_GLYPHS=unicode`
chooses the glyph set regardless of the saved setting; under it, the View row
still switches the set, for that session only, and saves nothing. Command-line
output such as `project export` always uses plain Unicode.

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
the saved terminal theme. It starts the session that way too, with `docket
--appearance dark|light`, and its terminal on that theme's background, so a
launch never shows the other theme first; the flag saves nothing. It also follows the system's Reduce Motion setting,
and its own Settings › General › Motion: while Reduce Motion is on or that
setting is off, the session draws no motion whatever View → Motion says, and
the saved setting is unchanged.

## Read workload and feedback

Today mode says what is open and what is already done, against your own 30-day
record: `9 open · 3 done today · usually 2/day`, or `best day in 30 days` once
today beats every earlier day. If most days finish nothing, the usual rate is
taken over the days that did (`usually 2 on active days`). It shows nothing until
there is completion history, and hides optional information in narrow windows.

The footer describes the current interaction. Press a chord prefix such as `w`,
`f`, or `g` to see its next keys. `w` separates Plan from Status; `?` lists all
shortcuts, including those a narrow footer cannot show. Pickers and editors
show their own controls, and closing them restores the outline's shortcuts.

The task-row footer learns with you. Once you have pressed one of its keys ten
times on this machine, such as `b` for Peek, the footer stops naming it and the
hints behind it move up into the room. `? help` always stays, and `?`, `:`, and
the menus still list every key. View → Reset Footer Hints brings every hint
back. Chord, picker, and editor footers are never shortened this way, since they
name what the moment in front of you takes.

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
feedback lasts six seconds, and the divider above it drains from right to left
as that window closes, so you can see how long `u` still answers. Warnings allow six to twelve seconds, based on text
length and wrapped rows when published. Warnings and errors wrap across up to
three rows so the reason and recovery action can be read together. Longer
diagnostics point to Help → Recent Messages for their full text.
A mode change clears transient view feedback.

Finishing a task says so plainly, with the day's tally: `✓ Done: Write the
release notes · 3 done today · u undo` (the first one of the day reads `first
done today`). Capturing names where the task landed before what it was, so a
long title is what gets shortened: `Captured to website · Inbox: Draft the
release notes · gl open · u undo`, or `Captured 3 tasks to website` for a
multiline capture. `gl open` is the way to the task you just saved, for a
detail you forgot: the cursor moves onto it, or, when this view does not draw
it, the view changes to its project in All the way `g~` does, and `Esc` brings
back the view and the task you were reading. `gl` keeps answering for the last
single-task capture of the session after the toast is gone; the toast leaves the
ending off when the cursor is already on the new task. Change History keeps each change's own record, such as `Change Status`
or `Add Task`, and so does the `Undid:` line an undo answers with.

Finishing the last task planned for Today adds `Today is clear` to the
completion's feedback, ahead of `u undo`, and the divider sweeps once in the
success color. When the days before it finished something too, the note
counts the run: `Today is clear · 4-day streak`. It is said once a day;
replanning Today's work away is not finishing it and says nothing. Triaging the last Inbox task, whether you start
it, plan it, file it, or finish it, adds `Inbox zero` the same way, once a day.
An undo, a redo, or a sync that empties the Inbox is not triage and says
nothing.

The first launch of each day opens with a greeting that names the one move the
day asks for first, since the pulse already counts the rest: overdue work, then
an untriaged Inbox, then the plan, such as `Good morning · start with the 1
overdue (Task → Start the Day)` or `Good morning · 2 in Inbox to sort (I
triage)`; after noon the overdue move names `7 Due` instead. Like any notice,
it leaves the footer after its reading time, and the pulse keeps the counts. The first greeting of a week also recaps the week before, such as
`last week 23 done, most on Tuesday`; a week with nothing done adds nothing.
The day it greeted is remembered on this machine, so later launches
that day stay quiet, and a key pressed before the workspace finishes loading
skips it. Quitting leaves one line in the terminal, such as `docket: 4 done
today · 2 still in Today`, or nothing when the day has nothing to report.

Docket draws a little motion where an action needs following. A task that
leaves the view, such as one you just finished, stays in its place for a moment
before the rows close up: a finished task first shows its check landing on the
row as it stood, then is struck through and fades. A task you moved is never
struck: it names where it went, such as `→ Next`, `→ Waiting`, or `→ Website`,
then fades. A canceled or deleted task is struck through and fades. The next key retires it at once. A mode switch lands its badge highlighted for a
moment and lights the `‹` or `›` it traveled through. Switching modes with
`Tab`, `Shift+Tab`, a digit, or the `g` chord, and opening or leaving a screen
such as the project hub, Help, comments, or attachments, eases the new rows in
from the background over about a sixth of a second while the title and the
footer stay put; nothing moves. A workspace count in the
title that just changed, such as `done` after you finish a task, lights for a
moment and settles back: green when Done rises, amber when Overdue rises, and
the accent color otherwise. A page jump — `Ctrl+D`, `Ctrl+U`, `PgDn`, `PgUp`,
`gg`, `G`, or the `z` chord — glides the list through two in-between windows
before it settles, in well under a tenth of a second; Peek's own scrolling does
not glide. A fold you open with `l`, `Enter`, `s`, `p`, `t`, or a click on its
arrow rolls its rows out over two quick frames instead of appearing at once;
closing one closes up the same way: the rows it held stay open as
blank space for two frames while the rows below slide up into it. A task you move with `J`, `K`, `Ctrl+J`, `Ctrl+K`,
`]`, `[`, `>`, or `%` lights the cursor's mark in the accent where it lands, on
a brighter band at first and then on the ordinary one, for about half a second,
and each further move lights it again; a move that takes the task out of view,
such as `%` from a scoped project, shows the departing row instead. A task you
capture lights its own mark in the accent wherever it lands, then dims; the
cursor stays where you were reading, and no fold opens to show the new task.
When the view does not draw it, the status strip says why, such as `Deferred
until …`; the first capture into an empty view puts the cursor on it. Opening
Peek rolls its document out under the divider over two quick frames; the
dock stands at its full height from the first, so nothing above it moves, and
moving the cursor under an open dock swaps the document in place. Capture's
panel rises into its rows: the line you type on is there at once, and the
rows above it ease in from the background, bottom first. A workspace count
that jumps by more than one, as a sync or a bulk change can make it, counts
through a few in-between values before it lights, whenever the number keeps
its width. Any key,
click, or wheel step finishes whatever is still animating, and a click always
lands on the rows the list is settling on. Tests and the harness run without this
motion.

In a terminal that reports focus, the cursor rests while the window is in the
background: its mark turns muted and the row loses its band, so a glance says
the keyboard is elsewhere. A count that changes while you are away, after a sync
or a reload, lights when you return rather than settling where nobody saw it.
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
clears it. While Today holds open work, Docket also sets the terminal's progress
indicator (OSC 9;4, drawn in the tab by Ghostty, Windows Terminal, and others) to the
day's completions against what Today still holds, so it fills as Today empties
and turns to its warning color while anything is overdue. Terminals without
support ignore it.

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

### Launching Docket twice

One session holds a workspace at a time. On macOS, typing `docket` while a
session already holds the workspace switches to that session instead: Docket
brings the application it runs in to the front — your terminal, or Docket.app
for a session Docket.app hosts, windowed or not — says `Docket is already open
(PID 75819) — switched to Ghostty.`, and exits successfully. The terminal is
also asked to raise the window that session is drawn in, which a terminal
implementing the xterm window operations honors and any other ignores.
Switching brings the application forward, not necessarily the tab or split the
session sits in. Where no application owns the session — inside tmux or
screen, over SSH — or on other systems, the launch refuses as before, `docket
TUI already running (PID 75819, started 2026-11-05T18:49:30Z)`, and exits with
status 1. A launch with `--control` or `--headless` never switches, so a host
always hears the refusal.

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
highlights retain their own foreground colors over that band, and its title
is the one bold title on the screen, so the task you are on is found before
the band is read. Headers,
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
That tint is why such a glyph still draws inside its own lifecycle section,
where an untinted one is left to the section header.
The task title stays ordinary text.

A task row shows no chips at all. Its columns go to the title, and Peek names
the whole tag set on one `Tags:` line, in an order that does not depend on how
you typed them: descriptive tags first in alphabetical order, then the semantic
ones, also alphabetical. A task tagged `ux`, `bug`, and `billing` reads
`#billing #ux #bug` however it was captured — the six semantic names are a
vocabulary every task shares, so the tags you cannot guess lead. Nothing is
dropped and nothing is counted away, because Peek wraps rather than clips: the
order decides reading sequence, not survival.
