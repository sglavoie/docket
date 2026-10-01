[← Architecture index](../architecture.md)

## Workspace location and cloud consent

Workspace selection precedence is explicit `--data-dir`, `TT_DATA_DIR`, the
machine-local active-workspace pointer, then the local default. A previously
existing iCloud workspace is preserved for compatibility, but new cloud use is
chosen through `tt workspace move`.

Move acquires TUI process locks for source and destination, recovers both commit
roots, inventories and hash-binds the source, copies without overwriting a
divergent destination, validates the result, and updates the active-workspace
pointer last. The source remains as a backup. `--force` skips only the
confirmation prompt; it never bypasses process locks, commit locks, or
validation.

## Backup and restore

Back up the complete selected data directory, including `.tt/`, project notes,
and attachment binaries. Stop active `tt` processes for a plain filesystem copy,
or use a snapshot mechanism that captures the directory atomically. Keep an
independent backup outside the same sync provider when provider deletion or
account loss is in scope.

Test a restored copy with:

```bash
tt --data-dir <restore> doctor
tt --data-dir <restore> project export <project>
```

Machine state is optional for canonical recovery; tasks, notes, attachments,
portable domains, and migration snapshots are not.

## Maintenance CLI and platform support

The public CLI contains the TUI launcher, `doctor`, `conflict`, `workspace`,
`completion`, and `project export`, plus help and version output.
`project export` names one project, or takes `--all` to write every cataloged
project, archived included, as `<slug>.md` into `--dir` (default `exports/`,
created 0700); `--file -` prints one export to stdout instead. `--all` refuses a
slug and `--file`, and naming nothing is refused as "project slug or ID required".
Each file destination passes the migration-snapshot guard on its own. `project
export`, `conflict show`, and `conflict resolve` complete slugs and short
conflict IDs from a direct catalog and conflict-store read, never
`LoadCLIWorkspace`, so a Tab press recovers and creates nothing.
`workspace location` also prints the logs, backups, exports, and conflicts
paths.
`main.run` exits with the code of any error implementing `ExitCode() int` and 1
for every other error; `doctor`'s `doctorIssuesError` is the one such error, 2
for "issues found". `doctor` loads every project through
`LoadProjectForRepair`, the one reader that admits the two defects `--fix`
repairs — a duplicated short ID and a missing parent — keeping both in the model
so their own checks report them, while validating everything else exactly as the
strict reader does; every other surface keeps the strict `LoadProject`.
`doctor --fix` reloads each project it repairs through that reader and commits
through `CommitProjectRepair` against the raw revision it read, then runs
`collectDoctorResults` again from a fresh workspace load, so the exit reflects
the state after fixing.
`doctor` adds one informational census of its own: `checkDependencyCycles`
prints each ring of soft references as `reference cycle: a -> b -> a`. It is
never an issue and never a fix target, because a ring deadlocks nothing a soft
reference could gate; it is reported only because one hop is all a row shows.
`project export` carries the resume note as a labelled `**Resume:**` paragraph
above the description, escaped through `escapeExportInlineText`: a note is one
line collected through a prompt that never offered markup, so leaving its
punctuation raw would let it reformat a paragraph its author did not write,
while the description stays verbatim because it is Markdown its author did
write. It also writes a task's outgoing references as a `**Dependencies:**`
list, labelled through `exportReferenceLabeller`: a resolvable target reads as
project, `(#ShortID)`, title, and `(state)` — the same parenthesized form Peek
uses, so the two never have to be reconciled — and anything else keeps its
durable `project-id:task-id` locator, so an export degrades rather than drops.
The state is the whole of what an export says about a target: there is no
transitive walk behind it, because a document holding one project cannot assert
facts about work it does not contain. Its label
population is every non-archived project plus the archived ones, because an
exported document is not a live view and naming archived work beats emitting a
bare locator. Computed dependents are never exported: they are an artifact of
what happened to be loaded, and freezing them into a document would state as
durable something that is not. Global flags
are `--data-dir` and `--no-color`; `--no-advisories` and `--notify` are the
launcher's own, declared on the root command rather than persistently, because
each configures a TUI session: no maintenance command has ambient rows to hide
or runs long enough to see work arrive. It has no task
mutation or undo/redo command.

There are no release artifacts. Distribution is `go install
github.com/sglavoie/tt@latest` alone: no prebuilt binaries, no package-manager
recipes, and no cross-compilation matrix, so the module's own `go` directive is
the only platform gate the toolchain enforces. `tt` is developed and used daily
on macOS; the full suite also runs on Linux in CI on every push, so Linux is
exercised but not daily-driven; Windows is not supported. Because no ldflags
reach a `go install` build, `resolveBuildInfo` reads `debug.ReadBuildInfo` when
the compiled-in version is still the `dev` placeholder, taking the module
version and, for a source-checkout build, the `vcs.revision` and `vcs.time`
stamps. The justfile's ldflags still win whenever they are set. Core use does
not require external AI services or provider integrations.

## Verification

```bash
./scripts/check_docs.sh
./scripts/run_tui_scenarios.sh
just ci
```

The scenario script replays every committed TUI JSON workflow and the focused
snapshot-first migration and interrupted-recovery scenarios. It selects a subset
of the ordinary suite, which `just ci` runs in full.
