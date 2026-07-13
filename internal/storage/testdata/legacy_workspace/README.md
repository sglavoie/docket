# Legacy workspace fixtures (sanitized)

Synthetic, hand-authored fixtures that reproduce the **pre-migration** task
Markdown schema: `status`, `section`, `stage`, and `active` metadata fields plus
custom project sections. They exist so migration sessions (WorkflowState model,
Markdown v2, application-layer commits) have a stable, reviewable input without
copying any live task content into the repository.

Rules for these fixtures:

- Never paste real task titles, descriptions, IDs, or personal data here. Every
  value below is invented and safe to publish.
- Each task deliberately exercises one branch of the legacy → WorkflowState
  migration precedence documented in the program's `context.md`.
- Treat these files as golden inputs: update them only alongside a deliberate
  change to the legacy parser or the migration mapping.
- These stay authored in the **v1** schema on purpose: they are the pre-migration
  *input*. As of session 33.7.6.2 the storage layer writes **v2** unconditionally,
  so saving any of these tasks re-serializes them as a single `state:` field and
  drops the retired `section`/`stage`/`active` keys (`active` migrates to the
  Focus Queue; custom stages are dropped). The loader still fills the legacy
  `Status`/`Section`/`Stage`/`Active` shims on parse, which is what
  `legacy_fixtures_test.go` and the migration-audit capture rely on — so the
  fixture files themselves are unchanged even though their round-trip output is
  now v2.

`project-alpha.tasks.md` maps to the migration precedence branches as follows:

| Task title            | Legacy signal                     | Expected WorkflowState |
| --------------------- | --------------------------------- | ---------------------- |
| Dropped experiment    | `status: canceled`                | `canceled`             |
| Shipped changelog     | `status: done`                    | `done`                 |
| Completed via section | `section: Completed`              | `done`                 |
| Awaiting sign-off     | `status: review`                  | `review`              |
| Wiring the parser     | `status: in_progress`             | `in_progress`          |
| Blocked on vendor     | `status: blocked`                 | `waiting`              |
| Waiting on teammate   | `section: Waiting`                | `waiting`              |
| Parked for now        | `status: standby`                 | `paused`               |
| Someday backlog       | `status: maybe`                   | `someday`              |
| Fresh capture         | `section: Inbox`                  | `inbox`                |
| Planned for today     | `section: Next`                   | `next`                 |
| Flagged active task   | `active: true`                    | `ready` + Focus Queue  |
| Custom staged idea    | `stage: Brainstorm` (custom stage)| `ready` (stage dropped)|
