# Palette captures

`dark.txt` and `light.txt` are actual `tmux capture-pane -p -e -N` captures
of `DOCKET_DB=./dev.db ./bin/docket`, built with `just build`, in a 110 × 34
pane. Each contains a project, Today, a filter in context, a selection and
sidebar focus. `-N` preserves trailing spaces. `preview.html` renders all
five views side by side; `preview.png` renders the project and selection.
Preview backgrounds are #181c22 and #fafafa; the application leaves the
terminal's ordinary background unchanged.

The detached tmux session used `COLORTERM=truecolor`, `TERM=xterm-256color`
and no `NO_COLOR`. To exercise both background replies without an app
setting, the capture driver sent OSC 11 replies to the application's input:
`ESC ] 11 ; rgb:1818/1c1c/2222 ESC \\` and
`ESC ] 11 ; rgb:fafa/fafa/fafa ESC \\`.
No application theme override or configuration was introduced.

The project is `docket-rewrite`; the context filter is `#work`. The selection
is made with `ggjjVjj`; `Esc Tab` then shows sidebar focus. Blank indent
columns keep the dense outline legible without additional guide strokes.

## Terminal capability evidence

Inspected the versions pinned in go.mod, rather than relying on another
Bubble Tea release's API:

- Bubble Tea v2.1.1 `color.go`: `RequestBackgroundColor` emits the request;
  `BackgroundColorMsg.IsDark` classifies the answer. `tea.go` handles the
  request with the terminal's background query, detects the color profile
  with `colorprofile.Detect`, and supplies it to the renderer.
- Bubble Tea `cursed_renderer.go` gives that profile to Ultraviolet's
  `TerminalRenderer`. The pinned Ultraviolet revision's `terminal_renderer.go`
  calls `ConvertStyle` for each cell; `cell.go` converts foreground and
  background to ANSI/ANSI256, and removes colors for the Ascii profile.
- colorprofile v0.4.3 `env.go` detects `NO_COLOR` (parsed as a boolean, e.g.
  `NO_COLOR=1`) before terminal capability upgrades. `profile.go` converts
  RGB to indexed colors. Consequently no application fallback is needed.

The original inherited environment had `NO_COLOR` enabled: a first tmux
launch produced glyphs and bold attributes with no RGB escapes. The saved
palette captures explicitly clear that variable.
