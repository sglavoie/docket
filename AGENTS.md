# Repository instructions

Read `CLAUDE.md` for the repository's contributor rules and invariants.
Read `~/.codex/memories/sglavoie_tt.md`, when available, for project preferences.

## UI testing

The user authorized autonomous simulator testing over SSH on the MacBook Pro
on 2026-10-19. Routine implementation checks there need no per-run confirmation.
For other UI tests that may open windows or steal focus, ask the user and wait
for an explicit answer. Use this machine only when the laptop is unavailable,
and ask before running UI tests locally. Laptop unavailability does not
authorize local testing. Reuse permission already granted for the same work.

Use `just ios-qualify` for unattended iPhone implementation checks. Run it
over SSH on the MacBook Pro; `start`, `status`, and `resume` support long runs.
Do not start a human-guided physical-device trial unless the user explicitly
requests one. Simulator evidence does not certify real iCloud delivery.
