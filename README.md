# tt — Terminal Tasks

`tt` is a local-first task manager for the terminal. One Bubble Tea Outliner is
the whole product: you capture, triage, plan, and finish work in a single
keyboard-driven outline, and readable Markdown on your own disk stays the
canonical store. Nothing is hidden in a database you cannot open.

There is no server, no sync service, and no web interface. There is also no
`tt add`: the command line is deliberately limited to diagnostics, conflict
recovery, workspace relocation, project export, and shell completion. Run `tt`
and the interface is the product.

## What using it looks like

Press `a`, type one line, press `Enter`:

```text
a  fix login redirect !1 @friday #auth %web $today
```

Captured on Friday 28 August 2026, that lands in the Today lane of project
`web` as:

```text
 □ ◉ fix login redirect  ⊡ Sep 4
```

- `!1` set High priority, drawn as the `◉` badge after the state badge.
- `@friday` resolved to the *next* Friday, never today, so the due chip reads
  `⊡ Sep 4`.
- `#auth` set the tag, minting it if it did not exist. Rows draw no tags, so
  it shows in Peek (`b`), not on the row.
- `%web` routed the task into the `web` project instead of wherever the cursor
  was. It names a project that already exists — capture refuses an unknown
  slug — so on a fresh workspace create `web` first with `P` then `a`.
- `$today` planned it for today.

The one non-obvious part is that last line: `$today` adds Today membership and
leaves the task's lifecycle state at **Backlog**, which is why the row wears the
Backlog `□` badge rather than a Today one. Today and Next are planning
collections, not states — see the mental model below.

## Mental model

Four ideas carry the whole product.

**Eight lifecycle states**, and a task is in exactly one of them:

```text
inbox  backlog  doing  waiting  paused  someday  done  canceled
```

**Today and Next are not states.** They are project-qualified planning
collections a task holds membership in, so a task planned for today still
answers `$backlog`. They are mutually exclusive, and finishing a task clears
both.

**Projects hold tasks, and a parent holds children.** Parents are project-local
and hierarchy depth is capped at six. Capture with no project in view routes to
the protected `Unfiled` project, which cannot be deleted or parked.

**Seven modes are seven questions.** `Tab` cycles exactly All → Today → Inbox →
Next → Backlog → Due → Recent, and `Shift+Tab` reverses it; the `g` chord jumps
straight to one. All is the home view, and it lays its sections out in this
order:

```text
Pinned, Today, Doing, Review, Inbox, Next, Paused, Backlog, Waiting, Someday
```

## Install and start

```bash
go install github.com/sglavoie/tt@latest
tt
```

That needs Go 1.25.13 or newer and `$(go env GOPATH)/bin` — usually `~/go/bin` —
on your `PATH`. The floor is the first Go release without a known vulnerability
`tt` actually reaches; a newer toolchain is fetched automatically unless you
have pinned `GOTOOLCHAIN=local`. To build from a clone instead:

```bash
go build -o tt .
./tt
```

There are no prebuilt binaries and no package-manager recipes for the terminal
program; `go install` is its distribution path.

On macOS, [tt.app](macos/README.md) runs the same program in its own window and
adds a menu-bar shortcut, quick capture, and notifications. From a clone, run
`just mac-install` to build and install it; this requires the full Xcode app.

A new workspace starts locally in `~/.tt/`. Use `--data-dir`, `TT_DATA_DIR`, or
`tt workspace move` to select a different location. Moving into iCloud Drive or
another synced directory is an explicit choice; an existing selected iCloud
workspace is preserved.

A launch starts in Active Projects, All mode, then reapplies the saved view of
any lens you left active on this machine: its mode, query, filters, sort, and
project scope. Selection and navigation are never restored. Press `Esc` to leave
a restored lens and return to Active Projects, All.

Active Projects is the base task workspace: every project you have not tagged
`parked`. Press `a` to capture the first task. At Active Projects scope, capture
routes to the protected Unfiled project with Inbox lifecycle; in a single-project
scope, it routes to that project.

## Platform support

`tt` is developed and used daily on macOS. The full test suite also runs on
Linux in CI on every push, so Linux is exercised but not daily-driven. Windows
is not supported.

## Your data

There is no telemetry, no analysis service, no account, and no network call
anywhere in the program. Everything `tt` knows lives under `~/.tt/` — a JSON
catalog plus one readable Markdown file per project — on a machine you control.
You can read, grep, edit, and back the whole thing up with ordinary tools.

## Before you upgrade

Migration is snapshot-first and one-way. Opening a workspace with a newer `tt`
publishes an immutable archive of the old bytes and then activates the new
schema; there is no down-migration, and an older build refuses a migrated
workspace rather than converting it back.

So before a version jump, keep your own copy of `~/.tt/`. If anything looks
wrong afterwards, run `tt doctor` for read-only integrity and
migration-snapshot diagnostics, and `tt doctor --fix` for the explicitly
supported repairs. [`docs/guide/storage.md`](docs/guide/storage.md) documents
the archive names and the manual recovery path.

## Daily loop

The Outliner is the only primary task presentation:

1. `a` captures one or more tasks.
2. `gi` moves to Inbox for triage.
3. `w` opens the workflow chord over the ten destinations the `$destination`
   sigil names.
4. `wb`, `wn`, and `wt` organize work; `wg` starts it and `wd` completes it.
5. Direct metadata keys handle priority, state, dates, tags, and pin.
6. `/` searches for a task.
7. `Space` opens the occasional task tools, Delete among them, which asks
   before it removes anything.
8. `u` undoes the last change and `Ctrl+R` redoes it.

Together, these operations form one capture, triage, plan, execute, and
complete loop.

## Keys at a glance

One key is one act, on every surface. `Space` is reserved for the destructive
and the rare.

| Keys | Act |
| --- | --- |
| `Tab` `g` `Enter` `Esc` | cycle modes · go to a mode · zoom in · unwind one rung |
| `a` `A` `e` `E` `Ctrl+E` | capture · child · inline edit · description · external editor |
| `w` | the workflow chord: `wi` `wb` `wn` `wt` `wg` `ww` `wp` `wm` `wd` `wc` |
| `!` `@` `#` `*` | priority · due, defer, and check-in · tags and context · pin |
| `-` `+` | step the deadline one day earlier · one day later |
| `[` `]` `>` `<` `%` `~` | outdent · indent · set parent · adopt children · move project · dependencies |
| `J`/`K` `Ctrl+J`/`Ctrl+K` | reorder one step · reorder to the far end |
| `/` `f` `F` `o` | search · row facets · which projects load · sort |
| `P` `Ctrl+P` `\` | project scope · pinned projects · named lenses |
| `v` `V` `Ctrl+A` | select one · select a range · select every visible row |
| `&` `;` `b` `O` | attachments · comments · Peek · outline-edit the project |
| `y`/`Y` `u`/`Ctrl+R` `Ctrl+N` `T` | copy · undo and redo · Notes · theme |
| `Space` `?` `q`/`Ctrl+C` | menu · the generated key reference for this surface · quit |

`?` is always the honest answer: it is generated from the same operation
descriptors that dispatch the keys, for whichever surface you are on.
[`docs/guide/outliner-keys.md`](docs/guide/outliner-keys.md) has the full table.

## Maintenance CLI

```text
tt [--no-advisories] [--notify] [--control PATH]
tt doctor [--fix] [--json]
tt conflict list [--json]
tt conflict show <id> [--json]
tt conflict resolve <id> --use local|remote|file [--file PATH]
tt workspace [location]
tt workspace move <destination> [--force]
tt completion bash|zsh|fish
tt project export <slug-or-id> [--file PATH|-]
tt project export --all [--dir DIR]
tt --version
```

Global flags are `--data-dir` and `--no-color`. Run `tt help <command>` for
per-command syntax.

## Where to read more

Behavior lives in [`docs/guide/`](docs/guide/README.md), written to be looked
things up in rather than read straight through:

- [`docs/guide/outliner.md`](docs/guide/outliner.md) — the index to the four
  Outliner files: the modes, the chrome around the list, the full key table and
  task actions, and row indicators, checklists, Peek, and multi-select.
- [`docs/guide/capture.md`](docs/guide/capture.md) — every capture and edit
  sigil, the workflow and hierarchy rules, and the `O` outline buffer.
- [`docs/guide/search-and-lenses.md`](docs/guide/search-and-lenses.md) — the `f`
  row facets, the `F` population facets, slash search, named lenses, sorting.
- [`docs/guide/projects-and-notes.md`](docs/guide/projects-and-notes.md) —
  project scope, Select Project, project tags including `parked`, All Notes.
- [`docs/guide/storage.md`](docs/guide/storage.md) — the durable layout, the undo
  ledger, conflict recovery, and the migration chain up to the schema-21
  catalog and its schema-19 task files.
- [`docs/guide/retired-keys.md`](docs/guide/retired-keys.md) — every retired key,
  per surface, and what answers it now.

Mechanism — data boundaries, transaction mechanics, migration internals,
platform support — is [`docs/architecture.md`](docs/architecture.md).
Contributor invariants are [`CLAUDE.md`](CLAUDE.md), and the TUI scenario
harness is [`docs/tui-testing/README.md`](docs/tui-testing/README.md).

## Development

```bash
just test
./scripts/check_docs.sh
./scripts/run_tui_scenarios.sh
just ci
```

The scenario script replays the committed final-product TUI workflows plus the
snapshot-first migration and interrupted-activation recovery scenarios. It
re-selects tests `just test` and `just ci` already run, for a faster loop while
authoring one.

## License

MIT — see [`LICENSE`](LICENSE).
