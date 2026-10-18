# Repository instructions

Read `CLAUDE.md` for the repository's contributor rules and invariants.
Read `~/.codex/memories/sglavoie_tt.md`, when available, for project preferences.

## UI testing

Before running UI tests that may open windows or steal focus, ask the user
for permission and wait for an explicit answer. Prefer running these tests
over SSH on the MacBook Pro. Use this machine only when the laptop is
unavailable, and ask the user before running them locally. Permission already
granted for the same testing in the current session need not be requested again.
