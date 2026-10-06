[← Mobile contracts](README.md)

# Compatibility fixture contract, version 1

A compatibility fixture is a committed desktop workspace that the Go engine,
the Swift bridge tests, and the sync tests all open. Its bytes are produced by
the live writers, never typed by hand. The code at the fixture's commit could
therefore have published exactly those bytes. The fixtures do not describe the
format; they are the compatibility baseline.

## Layout

```text
internal/compat/testdata/workspaces/
├── <name>.fixture.json   # manifest
└── <name>/               # a portable workspace root
    ├── .tt/workspace.json
    ├── .tt/planning.json
    ├── .tt/lenses.json
    ├── .tt/global-notes/*.md
    └── projects/<slug>/{tasks.md,notes/*.md,.attachments/…}
```

A fixture workspace holds portable artifacts only: no `machine.json`,
`.tt/transactions/`, `.tt/undo/`, `.tt/backups/`, `.tt/conflicts/`, `*.bak`, or
`*.lock`. These are the same exclusions that apply to
[sync](sync-envelope.md#what-is-never-exchanged).

## Manifest

```json tt:fixture-manifest
{
  "format": "tt-compat-fixture",
  "format_version": 1,
  "name": "representative",
  "workspace": "representative",
  "description": "Desktop workspace exercising relationships, planning, check-ins, rich records, lenses, and archived and parked projects.",
  "schemas": { "domain_envelope": 1, "catalog": 21, "task_markdown": 19 },
  "expect": { "projects": 5, "archived_projects": 1, "parked_projects": 1, "tasks": 15, "today": 2, "next": 2, "lenses": 2, "project_notes": 1, "global_notes": 1, "attachment_files": 1 },
  "coverage": ["relationship.parent_child", "planning.today", "project.archived"]
}
```

| Field | Rule |
| --- | --- |
| `format`, `format_version` | Always `tt-compat-fixture` and `1`. |
| `name` | Equals the manifest's file name without `.fixture.json`. |
| `workspace` | The directory beside the manifest that holds the workspace. |
| `description` | Required and not blank. |
| `schemas` | Must equal the live `config.CurrentDomainSchemaVersion`, `config.CurrentConfigVersion`, and `storage.CurrentTaskSchemaVersion`. A schema bump that leaves a fixture behind fails until the fixture is regenerated. |
| `expect` | An exact census taken through the live readers. Any count that differs fails. |
| `coverage` | Keys from the vocabulary below. Each key has a probe, and the probe must pass against what the live readers produced. A key with no probe is refused. |

Decoding is strict: an unknown manifest field fails.

## What a fixture must survive

`internal/compat` runs these checks against every manifest:

1. **Live readers.** `config.LoadRuntimeState` reads the domains. Each cataloged
   project (archived ones included) goes through `storage.LoadProjectWithDocument`,
   notes go through `storage.ListNotes`/`ListNotesFrom`, and lenses go through
   the strict lens decoder. `workspace.Loader` builds a snapshot, which must
   report no diagnostics.
2. **Audit and canonical bytes.** Every project passes `models.AuditProject`,
   and `storage.WriteProjectMarkdown` reproduces every `tasks.md` byte for byte.
3. **References resolve.** Every `depends` locator and every Today/Next locator
   names a task the snapshot holds, under the project it claims.
4. **Desktop open is a no-op.** Opening the workspace through
   `Manager.LoadRuntimeState` runs recovery, migration classification, and
   domain bootstrap. None of these may add, remove, or change a portable file.
5. **Coverage probes.** One subtest runs per declared key.

At least one fixture must declare each key that `requiredFixtureCoverage` lists.

## Coverage vocabulary

| Key | Holds when the live readers produce… |
| --- | --- |
| `relationship.parent_child` | a task whose parent is resolved |
| `relationship.nested_depth_3` | a task at depth 2 or deeper (a grandchild) |
| `relationship.dependency_cross_project` | a `depends` locator naming another project |
| `relationship.dependency_reciprocal` | cross-project dependencies in both directions between two projects |
| `relationship.checklist_origin` | a promoted child whose `checklist-origin` names its parent |
| `planning.today` | a non-empty Today |
| `planning.next` | a non-empty Next after parked filtering |
| `planning.parked_next_hidden` | a stored Next entry that `NextOrder` hides because its project is parked |
| `checkin.repeating` | a check-in with a repeat |
| `checkin.one_off` | a check-in without a repeat |
| `checkin.waiting` | a Waiting task carrying a check-in |
| `checkin.reviewed_terminal` | a terminal task that keeps `reviewed` and has no check-in |
| `lifecycle.all_states` | all eight lifecycle states |
| `schedule.due_date` | a deadline |
| `schedule.due_time` | a deadline with a `due_time` |
| `schedule.deferred` | a pending deferral with a non-zero defer count |
| `rich.description` | a multi-line description |
| `rich.resume_note` | a resume note |
| `rich.task_comments` | more than one task comment |
| `rich.project_comments` | a multi-line project comment |
| `rich.checklist` | a checklist with both done and open items |
| `rich.attachment_file` | a file attachment whose bytes exist and are non-empty |
| `rich.attachment_url` | a URL attachment |
| `rich.attachment_code_ref` | a code-reference attachment with a line number |
| `rich.project_note` | a parsed project note with a body |
| `rich.global_note` | a parsed global note with a body |
| `rich.tags_and_context` | a task with several tags and a context |
| `rich.pinned` | a pinned task with its pin instant |
| `rich.review_flag` | a task with the review flag set |
| `lens.scoped` | a lens scoped to an existing project, with a query |
| `lens.unscoped` | a lens that does not own scope |
| `project.archived` | an archived project with tasks and an archive instant |
| `project.archived_check_in` | a check-in inside an archived project |
| `project.parked` | a parked project that is absent from the task workspace but still available |
| `project.system_unfiled` | a system Unfiled project holding tasks |
| `project.tags` | a project with several descriptive tags |

## Regenerating

```bash
UPDATE_COMPAT_FIXTURES=1 go test ./internal/compat/ -run TestRegenerateRepresentativeWorkspaceFixture
```

The generator builds the workspace through `Manager.LoadOrCreateDomains`,
`CommitCatalog`, `CommitPlanning`, `CommitLensDefinitions`,
`WriteProjectMarkdown`, and `RenderNoteContent`, then copies the portable
artifacts only. Project, task, comment, and lens ids are fixed. Regenerating
does change the workspace id and the bootstrap Unfiled project's timestamps,
so commit the result as a reviewed diff. When adding a fixture, add its
generator beside this one, add its manifest, and add a probe for any new
coverage key in the same change.
