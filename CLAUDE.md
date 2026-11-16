# CLAUDE.md

Docket v2 is a from-scratch rewrite on an unrelated history. Nothing here
shares code or rules with the `main` branch.

[`SPEC.md`](SPEC.md) is the contract: the six rules, the model, the keys, and
the milestone order. Read it before changing behaviour, and change it in the
same commit as any behaviour it describes.

```bash
just build   # bin/docket
just test
just check   # fmt, vet, test
just run     # against ./dev.db, never the real database; seeds it if absent
just seed    # replace ./dev.db with ~6,000 tasks in 48 projects
```

## Layers

- `internal/outline`: the tree and its three operations. No SQL, no terminal.
- `internal/sigil`: the typed line and the fields it stands for, both ways.
- `internal/store`: SQLite. Appends to the log and updates state in one
  transaction.
- `internal/tui`: view state and keys. It changes the tree only through
  operations, so everything it does is undoable and logged.

## Rules for changes

- A new stored value is a new field name, not a schema change.
- A new key needs a free slot in the thirty-two-key budget in `SPEC.md`.
- Never add a confirmation, a dialog, or a second route to an existing act.
- Keep this file short. Detail belongs in `SPEC.md` or in a test.
