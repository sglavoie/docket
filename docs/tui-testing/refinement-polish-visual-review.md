# Refinement polish review

The refinement keeps existing workflows while reducing structural emphasis,
putting common Help actions first, and reserving more rows for the outline.

## Frame coverage

`TestPolishVisualReviewFrames` exports 64 ANSI frames: dark and light palettes,
four terminal sizes (60×20, 80×24, 100×35, 120×40), and eight states:
compact Peek, populated Peek, expanded Peek, actions over Peek, Help, capture
validation, expanded Schedule, and expanded Extras with adoption/dependencies.
Each frame checks terminal bounds, exact height, and unchanged content after
ANSI color removal. Help also checks that its footer occupies the final row.

Reproduce the frame export from the repository root:

```sh
TT_TERMINAL_REVIEW_DIR="$PWD/macos/.cache/refinement-review" \
  go test ./internal/tui -run '^TestPolishVisualReviewFrames$' -count=1
```

The review inspected ANSI renders of the narrow and wide layouts. Project names
retain their identity colors without bold weight, structural headings use text
rather than filled badges, and relationship markers remain readable without
competing with deadlines. Compact Peek puts status and resume before supporting
metadata; its position row and paging controls expose the rest. Help opens with
capture, editing, planning, Doing/Done, and undo/redo. Expanded relation values
wrap at minimum width; the form advertises discard for a populated draft.

The committed interaction scenarios cover paging, resizing, focus restoration,
footer ownership, and metadata below the compact window. Form integration tests
cover retained deferral, adoption, dependencies, insertion order, invalid edits,
clearing fields, refused saves, and a single undo/redo transaction.

## Native rendering limitation

The opt-in macOS `TerminalVisualReviewTests` launch was attempted, but
ScreenCaptureKit returned error −3801 because Screen Recording access was denied.
The ANSI review does not certify native Ghostty glyph rasterization or Metal
output. No permission settings were changed. Generated local frames are under
`macos/.cache/refinement-review/`; they are review artifacts, not repository data.
