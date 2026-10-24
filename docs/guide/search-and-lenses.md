<a id="filters-search-lenses-and-sorting"></a>

# Find and organize work

Press `/`, type words from a task, and press `Enter`. Search looks across the
current project scope, including work outside the mode you started in. `Esc`
returns to the view you searched from.

## Everyday finding and filtering

| Goal | Keys or example |
| --- | --- |
| Find a task | `/`, type `login`, `Enter` |
| Find completed work | Search `login ?completed` |
| Find deferred work | Search `login ?deferred` |
| Search one project | Choose it with `P`, then `/` |
| Keep only certain task rows | `f` |
| Include parked or archived projects | `F` outside ranked search |
| See hidden tasks | `fH` outside ranked search |
| Explain missing work | `Q` |
| Save this view | `\` → Save current view |
| Change order | `o` |

## Filters and search

The header says **Search · all modes** while ranked search is open. The project
name or scope beside it still limits the results. Active filters remain in
effect; their count appears beside the query, with `Q` for details.

Large searches briefly show **Searching…** while results are prepared. Keep
typing to refine the query, or press `Esc` to return immediately. Superseded
results do not replace the latest query or reopen a search you closed.

By default search includes open tasks and excludes deferred work. The context
row names that population when space permits; `Q` keeps the full explanation.
Add `?completed` or `?deferred` to admit those tasks. An `Also matches` message
names matching tasks that are still hidden.

### Hidden tasks

`wh` hides the cursor task, or the selection, from every view: every mode,
`/` search, lens results, the workspace pulse, Review, and Today's pace. Its
children go with it. Hiding changes nothing else about the task — state,
plans, pin, review flag, deadline, and check-in stay as they were — and one `u`
takes it back. Nothing you type in a capture or edit line can hide a task, and
editing a hidden task keeps it hidden.

`fH` narrows the current mode and scope to hidden tasks, including children
hidden through a parent. `Q` names it `+hidden:only`, a lens saved with it
reopens it, and it needs a fresh load, so the ranked list refuses it: press
`or` first. Peek marks a hidden task `(hidden)`, and `wh` under `fH` unhides
it. A task hidden only because its parent is hidden comes back when the parent
does.

Use `or` to switch from ranked search to searching within the current mode's
lanes. Press it again to return to ranked results. `Esc` restores the mode,
filters, and position you had before searching.

### Read search results

Title matches come before matches in descriptions and other details. A quiet
label such as `description` or `comment` explains a result whose title does not
match. Press `b` to read its details, or `Enter` to focus the task.

Examples:

| Query | Finds |
| --- | --- |
| `login #auth` | Matching work tagged `auth` |
| `$waiting` | Waiting tasks |
| `!1 @overdue` | High-priority overdue work |
| `?review` | Flagged tasks and tasks whose check-in is due |
| `login -#backend` | Matching work without the `backend` tag |

Search supports fuzzy matching and multiple words. For exact matching rules,
smart case, supported tokens, and sorting within results, see the
[search reference](search-reference.md#read-search-results).

### Search inside a picker

Press `/` to edit a picker's search. Up/down move through results while letters
enter text. `Enter` accepts the search; `Esc` clears it, then leaves editing.
The footer names the controls for the picker you are using.

To reuse a previous task query, close suggestions with `Esc`, then use up/down
in the search box. `Ctrl+R` opens searchable query history. The last 50 submitted
queries are kept on this machine; drafts are not. See
[search history and completion](search-reference.md#read-search-results).

### Reading and peeling the narrowing

If work seems missing, press `Q`. It lists the scope, mode, query, filters, and
other active restrictions. Press a row's numbered key to remove that restriction
or `x` to remove all peelable restrictions. Read the effect before removing a
saved lens or changing project scope.

`f` filters task rows. `F` changes which projects load, so it is unavailable in
ranked search. For example, `Fp` includes parked projects and `Fa` includes
archived ones. Press `Fx` to clear project filters. See the
[filter tables](search-reference.md#filters-and-search) and
[restriction details](search-reference.md#reading-and-peeling-the-narrowing).

### Renaming a tag or a context

Open `f#` for tags or `fc` for contexts, select the spelling, and press `e`.
Enter the replacement and review the confirmation. Renaming to an existing
value merges them. See [rename scope and recovery](search-reference.md#renaming-a-tag-or-a-context).

### Named lenses

A lens saves a useful combination of mode, query, filters, sort, and optional
project scope. A lens saved under `fH` keeps `+hidden:only`, so it reopens on
hidden work. To save one, arrange the view, press `\`, select **Save current
view**, and name it. Reopen it from the same picker with `Enter` or its numbered
shortcut. **Add built-in view** offers ready-made views such as Waiting and
Needs Review.

A `*` beside an active lens means the view differs from its saved settings.
`Q` shows what changed. To save those changes, open the lens picker, select the
lens, and use `Space` → **Replace from current view**. Review the change summary;
lens edits are not part of task undo history.

`Esc` leaves a lens. Searching while one is active temporarily suspends it;
`Esc` restores it. Saved lenses travel with the workspace, while each machine
remembers its own active lens. See [lens management and restart behavior](search-reference.md#named-lenses).

## Sorting

| Keys | Order |
| --- | --- |
| `o@` | Due date |
| `o!` | Priority |
| `oc` / `ou` | Created / updated |
| `ot` | Title |
| `ox` | Restore the mode's order |
| `or` | Switch between ranked search and the mode's lanes |

Repeat a field key to reverse its order. Today, Next, Due, and Recent keep their
own ordering outside ranked search. See [sorting rules](search-reference.md#sorting)
for group order, tie-breaking, and cleanup workflows.
