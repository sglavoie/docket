# Capture and edit grammar

The sigils every capture and edit surface speaks, the workflow and hierarchy
rules they commit against, and the `O` outline buffer that edits a whole
project at once.

## Capture grammar

One nonblank line creates one task. Multiple lines are fully parsed and
validated before one atomic transaction is committed.

Creating work in Doing or Waiting without a check-in interrupts nothing. The
footer says so once the creation commits — `No check-in: <task> · menu: Set up
check-ins`, or `No check-in: N tasks` when one capture committed several — and
the route it names is the keyless **Set up check-ins** menu entry, not a key,
because the new task may not be under the cursor. A creation that already
supplies a schedule is never counted. When create-and-open is used, the content
editor keeps its own screen and the note follows the edit it commits. The same
note follows committed line or outline edits that enter either state, and it
rides whatever message that path was already showing rather than replacing it.
Scheduling stays your own act, through `@`, the `Space` menu's **Set Check-In**,
or that workspace entry, and each save is a separate undoable transaction.

```text
!1-3  #tag  ##context  @date  @@defer  $destination  *pin  =review  %project  >parent  <child  ~task  &link  ;comment;
```

- `!1` through `!3` set High, Medium, or Low priority.
- `#tag` creates and sets a lowercase ASCII kebab tag on the fly. Input is
  canonicalized (`#Needs / Review` becomes `needs-review`). A tag decides
  nothing about the Review lane; `=review` does.
- `##context` sets one optional task-level context, normalized by the same rules
  as a tag. It is distinct from the retired v4 project-level context. `##0`
  clears the value on the edit surfaces only; outline editing clears it by
  omission. Therefore `"0"` is permanently reserved as a context name, unlike
  the otherwise legal clearing/reference forms `!0`, `@0`, and `>0`. Expanding
  the line into the full form carries the value onto the Basics tab's Context
  row, where you can keep it, change it, or empty the field to create the task
  without one.
- `@date` accepts the supported date expressions and stores a local calendar
  date.
- `@@defer` hides the task until an instant rather than a day, so it also
  accepts the `+Nh` hour offsets a due date refuses. Deferring carries the same
  lifecycle effect the picker's defer section does: the task moves to Backlog, loses any
  Today or Next membership, and a terminal task refuses outright. `@@0` clears
  the deferral on the edit surfaces; the outline buffer has no deferral channel
  and rejects the sigil by name.
- `$destination` accepts `inbox`, `backlog`, `today`, `next`, `doing`,
  `waiting`, `paused`, `someday`, `done`, or `canceled`. Today and Next create
  Backlog tasks with an explicit planning membership.
- `*pin` pins the task, exactly as the `*` key does on an existing row. Only
  `pin` and `unpin` are recognized, and any other value stays in the title, so
  `Refactor *args handling` keeps its `*args`.
- `=review` flags the task for the Review lane. It is the same shape as the pin:
  only `review` and `unreview` are recognized, so `a=b` stays prose, and
  `=unreview` clears the flag on the edit and search surfaces alone — a new task
  starts unflagged, so capture refuses it by name. The `=` key toggles the same
  flag on an existing row, and the `f=` filter facet narrows by it.
- `%project` overrides scope routing.
- `>parent` resolves inside the selected project after project routing.
- `<child` adopts a task that already exists, and may be repeated. `>parent`
  stays singleton because a task has one parent, while a task can gain many
  children at once.
- `~task` records a soft dependency on another task and may be repeated. Unlike
  `>parent` and `<child` it crosses project boundaries, so it takes either a
  bare reference resolved in the effective project (`~1`, `~Login`) or a
  project-qualified one (`~project-slug:1`). The reference is informational: it
  is stored, shown, and searchable, and nothing about the task's readiness,
  ordering, or eligibility to complete follows from it, so a task with an open
  dependency still starts and still completes. The edit surfaces add `-~task`,
  which removes one reference, and `~0`, which clears them all; capture accepts
  neither, because a task being created has no set to subtract from. Search
  takes `-~task` too, where removing means dropping those tasks from the result
  rather than editing anything, and refuses `~0` for the same reason capture
  does. On an
  existing row the `~` key opens Set Dependencies over the same set, with the
  stored references already checked: checking one adds it and unchecking one
  removes it, exactly as `~task` and `-~task` do in text.
- `&link` attaches something to the new task and may be repeated. It takes an
  `http://` or `https://` URL, a `path:line` code reference such as
  `&internal/app/add.go:19`, or a local file path, which is copied into the
  task's attachments in the very same transaction that creates the task, so one
  undo takes back the task and its bytes together. A path holding spaces is
  written the way a shell takes it — `&'~/notes/Q3 plan.pdf'`, `&"~/notes/Q3
  plan.pdf"`, or `&~/notes/Q3\ plan.pdf` — and the quoting is stripped before
  the value is classified, so a path pasted from a file manager lands whole
  instead of splitting at its first space. Anything else stays in the title, so
  `Plan the R&D offsite`, `Q&A with legal`, and `&10:30 standup` are ordinary
  prose, and so is a quoted phrase naming nothing attachable, as in `Ask &"why"
  before the demo`. Two files on one line whose names would collide in the
  attachments directory are refused by name rather than silently overwritten.
  The sigil is capture-only: the outline buffer and search both reject it and say
  where attachments do belong. Expanding the line into the full form instead
  carries the value onto that form's Extras tab, which is where the expanded
  composer attaches.
- `;comment;` writes a comment onto the new task and may be repeated. Both
  delimiters are required and the opening one has to follow a space, which is
  what keeps ordinary punctuation ordinary: `Fix a; then b` and an unterminated
  `Ship ;note` are prose, while `Call Dana ;ask about the SSO deadline;` creates
  the task with that comment already on it, in the same transaction. Everything
  between the delimiters is literal text, so `Fix login ;check the #auth flow;`
  files one comment mentioning `#auth` and creates no tag at all. It is
  capture-only on the same terms as `&link`, and the panel the refusals name is
  the Comments one.

The syntax guide teaches the sigils in a different order from the one the parser
peels them in. `;comment;` has to be recognized first, or a sigil written inside
a comment would be claimed by its own rule, and `&link` before `#tag`, or
`#work&http://x/y` would leave a stray `&` that reads as an attachment. Both are
parsing details and neither leads the guide.

Both structural sigils name a task by exact short ID or by a unique
case-insensitive title prefix, and both resolve in the effective project — the
one `%project` selected, not the one the cursor was in. `<child` is therefore
never a way to pull a task across a project boundary: a reference that names a
task outside the effective project is rejected, and adopting a task never moves
it or its subtree between projects.

Adoption is additive. Children you do not mention keep their parent, an adopted
task keeps its own subtree, and every task already in the project keeps its
place: an adoption re-nests a task, it does not resequence the project. The one
move you can still see is the standing rule that finished work sinks below open
work, so adopting a finished task under an open one lifts it out of the finished
block, and reopening a finished parent lifts it and its subtree with it.
A `$done` or `$canceled` line cannot adopt unfinished work: capture and the edit
grammar both refuse it and name the open task, because a finished parent above an
open child is the one shape the completion rule forbids. Adopting work that is
already finished is fine. A task that is already done and gains an open child
some other way is reopened rather than left in that shape.
There is no clear form: `<0` is an ordinary reference that resolves to nothing
and is reported as such, and a task is detached by giving it a new `>parent` or
`>0` instead. Aliases that name the same task collapse to one adoption, keeping
the first occurrence, so `<2 <Login` adopts task 2 once. A task cannot be both
`>parent` and `<child` of the same line, and across a multiline capture two
lines cannot adopt the same task; both are reported before anything is written.

`^type`, priorities `!4`–`!5`, `$ready`, and `$in_progress` are
rejected with replacement guidance. The tilde no longer names recurrence, which
stays retired: `~weekly` is now an ordinary dependency reference, so it is
refused as a task that cannot be found rather than as a retired sigil. Other
aliases such as `priority:`, `due:`, `state:`, and `proj:` are literal title
text, not task attributes. Root capture
defaults to Inbox. A new child defaults to Backlog under its selected parent.

Capture opens as a panel across the bottom of the screen rather than a view that
replaces what you were looking at. The outline stays above it, live and scrolled
where you left it, so the row you were on — and its Peek dock, if one was open —
is still readable while you type. The panel leads with a divider naming the act
(`── Capture ─`), states the effective route on one row, and holds the input.
A blank capture is exactly those three rows; the panel then grows a row at a
time as the draft, the preview of what `Enter` would create, a suggestion list,
or a refusal needs one, and shrinks again as they go away, up to a ceiling of
fourteen rows and never below the four the outline keeps. The route row reads
`Project: Client Alpha · Status: Inbox`, and when
the draft joins a planning collection it says so in its own half —
`Project: Client Alpha · Plan: Today · Status: Backlog`, because `$today` is a
plan and the status it implies is one the word does not spell. A draft with no
`$destination` and no `%project` reads whatever it would default to, a draft
whose lines disagree about an axis reads `Mixed` for that axis alone, and the
preview repeats the pair under the same two words. Both wear the colors the
Outliner already uses: the project its own color, the plan and the status their
section badges, the preview's state glyph the badge the new row will carry,
and each tag its chip color; `Mixed`
and `Unavailable` read muted. Autocomplete and `?` syntax help borrow
those rows while they are open, and the guide takes the whole screen for as long
as it is up. `Ctrl+F` expands the same draft into the full form, which does
replace the outline; `Esc` from either hands the screen back with the cursor,
the folds, and the Peek dock exactly as capture found them.

Once the panel is at its ceiling — or on a short terminal, where the ceiling is
whatever the outline can spare — it keeps its shape and shortens the preview
instead of its chrome, and a refusal keeps its rows ahead of the draft's, so the
message that says what went wrong is never the thing that scrolls away. Rather
than show the first few labelled groups and silently drop the
rest, it falls back to one wrapped run — `Panel capture · □ $inbox · !2 · #panel`
— so nothing the parser recognized goes unnamed. At the 60×20 floor the outline
keeps enough rows to show the row the cursor is on; capture never hides it.

The expanded composer keeps deadlines and check-ins separate on its Schedule
tab. Due remains the deadline field and is the only date that can infer Today.
Next check-in and Repeat both open the shared check-in editor, with the same
date parsing, repeat choices, custom positive-day validation, and explicit
Clear behavior used by Set Check-In on an existing task. A schedule chosen there
travels through expansion and the task-creation transaction as one paired value;
a check-in due today does not add the new task to Today.

Every field on every tab is reachable at any supported height. When a tab holds
more than the terminal can show — a long checklist, a dozen attachments, a
comments list — the form scrolls instead of cutting: `Tab` and `Shift+Tab` move
the focus as usual and the window follows it, and the last row reads `3 above · 1
below` so what is off screen is named rather than lost. A tab that already fits
shows no such row. A refused save keeps its message on screen and moves the focus
to the field it names, so the two stay readable together, and a taller terminal
simply gives the rows back.

Every field is reachable at any supported width too. The expanded composer opens
wherever the rest of `tt` opens — down to the same 60-column minimum every other
surface accepts — rather than replacing itself with a "terminal too small"
notice on anything under 68 columns. A narrow terminal shortens what a row can
spare and nothing else: the grey key hints beside a focused row go before its
value does, and a value too long for the row is shortened there and read whole by
pressing `Enter` on it, which opens it in its own editor. The nested pickers —
priority, state, flags, due date, the check-in editor, and the checklist,
subtask, attachment, and comment editors — take the whole screen while they are
open, so at every width they keep a complete frame and a complete list; `Esc`
hands the form back exactly as it was.

`a` at base scope captures into Unfiled: a task cursor never lends capture its
project. A project header row is the one exception, because it names a project
explicitly rather than incidentally, so `a` there captures into that project.
The route row names whichever destination it resolved — `Project: Unfiled` or
`Project: Client Alpha` — so the two never look alike, and `%project` still
overrides both. The divider above it says `Capture` in either case: the
destination is stated once, on the row `Ctrl+P` rewrites. `A` on a project
header row keeps refusing: a project has no task to parent a child under.

Inside a task Focus, `a` captures a direct child of the focused task and the
divider names the parent, as `── Capture → child of Launch platform`, since no
half of the route row can carry one; the new task shows up in the focused
projection immediately. Focus wins over a project header row
under the cursor. A focus root already at the nesting cap degrades to plain
capture into its project, where the divider reads `Capture` and the route row
names the project, rather than refusing. `A` is
unchanged and still means a child of the cursor task, one level deeper.

Exact case-insensitive `bug`, `question`, `epic`, `story`, `feature`, and `idea`
tags name a kind of work. They wear their own chip on a row like any other tag
and leave the title and the state glyph alone; the order above is the order they are listed in.
Namespaced values such as
`type:bug` and partial matches remain ordinary tags and add no prefix or marker
icon. Both tag pickers group those six under a `SEMANTIC` section above the rest:
`#` Set Tags always offers all six with a one-line summary each, whether or not
any task carries them, while `f#` Filter by Tags stays usage-derived and lists
only the ones in use.

Task contexts are their own filter axis on `fc`, so `f#` narrows tags alone and
carries no `CONTEXTS` section at all. In both tag pickers `s`, `c`, and `t`
jump the cursor to the first `SEMANTIC`, `CONTEXTS`, and `TAGS` row. They name a
kind of value rather than a place on screen, so in `#` they still reach a row
the `APPLIED` section borrowed, and a section emptied by that promotion answers
with the promoted row instead of refusing. `f#` promotes nothing, so its jumps
are unchanged. A hint line inside the overlay names the sections that picker
actually has — which is what drops `c` from the `f#` hint. `CONTEXTS` therefore means one thing
only: in `#` those are the `##name` radio rows that write the field, listed above
the tag rows the section also collects.


## Workflow and hierarchy

The eight persisted workflow states are:

```text
inbox  backlog  doing  waiting  paused  someday  done  canceled
```

Today and Next are separate project-qualified planning collections, not
lifecycle states. Today and Next are mutually exclusive, and terminal
transitions remove both references. Waiting is an explicit lifecycle state.
Done and Canceled are rejected while a direct child remains nonterminal.

Parents are project-local and hierarchy depth is capped at six.


## Outline editing


`O` writes the cursor's project out as one indented outline, opens it in
`$EDITOR`, and applies the whole tree when you save and quit. Indentation is
parentage: two spaces per level, up to the usual depth of six. Every line
otherwise speaks the ordinary capture grammar, so `!2`, `#tag`, `##context`, `@friday`,
`$backlog`, `*pin`, `=review`, and `~task` all work, while `>parent` and `<child` are rejected
because the indentation already said it, `@@defer` is rejected because the
buffer carries no deferral at all, and `&link` and `;comment;` are rejected
because attaching and commenting are creation-time acts the buffer could only
duplicate. The dump prints the values a task
currently holds, so a pinned task arrives carrying `*pin` and deleting that
token unpins it; there is no `*unpin` here. `=review` dumps and clears the same
way, for the same reason. Dependencies dump the same way, as
one `~` token each in the shortest form that still addresses the target: a bare
short ID inside the dumped project, a slug-qualified `~radar:7` in another
loaded one, and the durable `~project-id:task-id` locator when this workspace
cannot address it at all, which is what carries a reference to archived or
deleted work through the round trip untouched. Every stored reference is
printed, so deleting a token is how you remove one — the line you save is the
complete set.

```text
# Sonar
- Ship the release [[tt:4f2a9c11]] $doing !1
  - Cut the changelog [[tt:9b31de07]] @friday
  - Tag the build
- Draft the migration notes $backlog #docs
```

The `[[tt:...]]` markers identify tasks that already exist. Keep one and the
task is updated in place, wherever you moved it; drop it, or write a new line
without one, and a new task is created. Delete a line and the task is deleted:
the buffer is the project's whole tree, so absence is how you remove work and
there is no `$delete` sigil. Before anything is applied, `tt` names the tasks
that would go and the attachments that would go with them and asks you to
confirm. Declining cancels the entire apply, edits included, and keeps the file
so you can reopen it. An emptied buffer is the same rule at its limit and
deletes every task in the project; it is refused only when it would delete
nothing at all, which is the truncated-file case.

The buffer holds the project's entire task tree rather than the rows currently
on screen, which is why `O` refuses in Today, Next, Due, and Recent: those modes
order rows by something other than the tree, so the indentation you would be
editing is not the parentage you would be saving. It also refuses while a visual
selection is open, since a selection names rows and the outline names a project.

Nothing is written until the whole buffer validates. If any line is wrong you
get every problem at once, numbered by line, and the file is kept so you can
reopen it, fix the named lines, and run `O` again. A successful apply is one
undoable transaction.
