# Synthetic v4 migration corpus

This directory is the public, hand-authored input corpus for the schema-v5
simplification migration. All names, UUIDs, task text, dates, paths, and file
contents are invented. Nothing was copied from a user workspace.

The corpus intentionally mixes healthy v4 artifacts with shapes that the v5
migration must repair, report, snapshot without importing, or reject:

- `workspace/` is a complete split-domain workspace with active and archived
  task Markdown, project and global notes, attachment binaries, project
  blockers, a protected Inbox, an archived project, and Sandbox/Internal
  projects.
- `machine/` represents machine-local state kept outside the portable
  workspace. The runtime UUID path is preserved in the fixture layout.
- `variants/` carries independent theme inputs needed to exercise both theme
  conversion branches.
- `unsupported/` carries corrupt, partial, future-schema, or otherwise
  unsupported inputs. These are never treated as loadable workspace data.
- `expected-audit.json` is the exhaustive migration-rule inventory. Each rule
  names its input artifact and its expected import, repair, report, snapshot,
  or failure outcome.
- `expected-archive-reintegration-report.json` is the stable repair report for
  merging the mixed active/archive task corpus into one active task set.

Task Markdown stays in the current schema-v2 storage format because that is the
task format written by the v4 product. Split-domain payloads carry config schema
version 4 inside version-1 envelopes. Future migration tests should copy the
corpus to a temporary directory before executing any code that writes.
