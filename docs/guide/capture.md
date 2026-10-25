<a id="capture-and-edit-grammar"></a>

# Capture and edit

Press `a`, type a title, and press `Enter`. Check the destination above the draft
before saving. With no project in scope, capture goes to Unfiled / Inbox.
The draft stays at the bottom of the terminal. Previews, completions, validation
messages, and keyboard hints appear above it, so adding metadata keeps the typing
line in place. Multiline drafts grow upward and scroll when they fill the panel.

## Everyday capture

| Goal | Keys or example |
| --- | --- |
| Capture one task | `a`, type, `Enter` |
| Capture a child | `A` on its parent |
| Capture several tasks | Paste one task per line, then `Enter` |
| Save and keep capturing | `Ctrl+N` after closing suggestions with `Esc` |
| Edit a task's title or metadata | `e` on the task |
| Edit its description | `E` |
| Undo a save | `u` in the Outliner |

Start with a title and add only the metadata you need:

```text
Fix login redirect %web @tomorrow $today
```

This creates a task in `web`, due tomorrow and planned for Today. Its status
is Backlog; `wg` starts it later. The preview shows the exact date and destination
before you save. `%web` requires a project with that slug; a missing project can
be created with the capture after confirmation.

### Choose the destination project or parent

Use `%project` in the draft or `Ctrl+P` to choose a project. Inside a project,
plain capture uses that project. On a project header it uses the named project;
a task cursor in the global list does not change the default destination.

Inside Focus, `a` creates a child of the focused task. `A` creates a child of
the selected task. The panel names the parent. See
[routing and nesting exceptions](capture-reference.md#choose-the-destination-project-or-parent).

## Capture grammar

### A worked example

Build a draft one instruction at a time:

| Draft | Adds |
| --- | --- |
| `Fix login redirect` | The title |
| `Fix login redirect %web` | Project `web` |
| `Fix login redirect %web @friday` | A deadline on the next Friday |
| `Fix login redirect %web @friday $today` | A place in today's plan |

A weekday means its next occurrence, never today. Read the preview's calendar
date before saving. [Date rules and literal text](capture-reference.md#capture-grammar)
cover escaping and supported expressions.

### Sigil reference

| Token | Meaning |
| --- | --- |
| `!1` | High priority (`!2` Medium, `!3` Low) |
| `#auth` | Tag |
| `##office` | Context |
| `@tomorrow` | Deadline |
| `@@+2h` | Defer for two hours; moves work to Backlog and clears its plan |
| `$today` / `$next` | Plan membership |
| `$doing` / `$waiting` | Status |
| `*pin` / `=review` | Pin / review flag |
| `%web` | Destination project |
| `>12` / `<12` / `~web:4` | Parent / adopt a child / dependency reference |
| `&https://example.com` / `;Call Dana;` | Attachment / comment |

Some tokens behave differently when editing existing work. Use the
[complete syntax reference](capture-reference.md#sigil-reference) for clearing
values, literal tokens, files with spaces, and relationship rules.

### Complete a token or look up syntax

Type a sigil to see suggestions. Move the caret into an earlier token to complete
it there. After closing suggestions with `Esc`, press `?` for syntax help at the
caret. Closing help returns to the same draft.

### Expand and edit a capture

Press `Ctrl+F` to expand a valid single-task draft into the full form. Use `Tab`
and `Shift+Tab` between fields, `Shift+←` and `Shift+→` between tabs, and `Ctrl+S`
to save. Basics holds flags and context; Schedule holds dates and check-ins;
Extras holds relationships, attachments, and comments.

Multiline batches stay in compact capture. See
[expanded editing](capture-reference.md#expand-and-edit-a-capture) for each field.

### Capture several tasks in succession

Paste one task per line to save a batch together, or use `Ctrl+N` to save and
start another draft. Each successful save is one undo entry. A refusal keeps the
whole draft. Compact capture accepts up to 4,000 characters; an oversized paste
is refused without changing existing text.

### Correct a refused capture

The cursor moves to the first invalid token. Correct it and press `Enter` again.
For a batch, `Alt+N` and `Alt+P` visit the next and previous problems. Nothing is
saved until every line is valid. `Esc` closes capture and keeps its draft;
`Ctrl+X` asks before discarding it.

### Close and resume a draft

In compact capture and its expanded form, `Esc` keeps the draft on this machine.
Open capture from the same context to resume it, including after restarting Docket.
The text, caret, selected project, and expanded fields are retained. A child
capture stays tied to its original parent. Closing an open suggestion list or
field picker first returns to the editor.

Drafts are saved after a brief pause in editing, when you close capture, and
before submitting. A crash can lose only the newest edits that had not yet
reached that pause. If a recovery write fails, capture keeps the text in memory
and reports the problem; fix it and edit or close again to retry.
`Ctrl+X` opens the explicit discard confirmation; `N` or `Esc` keeps editing.
A successful capture removes its recovery copy.

If a previous save was interrupted, the recovered draft explains that the task
may already exist and prevents resubmission. Check the task list before discarding
that recovery copy. Recovery files live under
`~/.docket/runtime/<workspace-id>/capture-drafts/` and do not sync.

[Validation and recovery details](capture-reference.md#correct-a-refused-capture)
cover the expanded form and scheduling errors.

## Workflow and hierarchy

Today and Next are plans; status says where work stands. Planning a task does
not start it. Use `wg` to start and `wd` to complete; completing clears its plan.
A parent cannot finish while a direct child remains open. Tasks nest within one
project, up to six levels. See [workflow rules](capture-reference.md#workflow-and-hierarchy).

## Outline editing

Press `O` to edit the current project's tree in `$EDITOR`. Indentation sets
parentage. Keep each existing task's `[[tt:...]]` marker to edit it in place;
removing a line deletes that task after confirmation. A new unmarked line creates
a task. Read the [outline reference](capture-reference.md#outline-editing) before
using it for bulk changes.
