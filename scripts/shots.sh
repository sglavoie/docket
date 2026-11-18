#!/usr/bin/env sh

# Screenshot the everyday screens for a taste pass: build the current tree,
# seed a fictional outline with tools/seed, drive the real binary through
# vhs, and write one PNG per screen. The database lives in a temporary
# directory, never at the path DOCKET_DB names. Requires vhs
# (brew install vhs).
#
#   ./scripts/shots.sh [output-dir] [dark|light] [wide|minimum]
#
# Defaults: ./shots dark wide. The terminal's background is the theme's, and
# Docket picks its palette from that background, as it does in any
# terminal. minimum is the narrowest pane with no sidebar beside it.
#
# Compare a run against the last one side by side; the screens are the review,
# not a test, so nothing here fails on a visual change.

set -eu

out=${1:-shots}
appearance=${2:-dark}
size=${3:-wide}
case "$appearance" in
	dark) theme='{"name":"docket-dark","background":"#181c22","foreground":"#dce1e7","cursor":"#dce1e7","black":"#181c22","red":"#ef8787","green":"#9ece6a","yellow":"#e6bc64","blue":"#7aa2f7","magenta":"#bb9af7","cyan":"#69c7d3","white":"#dce1e7","brightBlack":"#929ead","brightRed":"#ef8787","brightGreen":"#9ece6a","brightYellow":"#e6bc64","brightBlue":"#7aa2f7","brightMagenta":"#bb9af7","brightCyan":"#69c7d3","brightWhite":"#ffffff"}' ;;
	light) theme='{"name":"docket-light","background":"#fafafa","foreground":"#232d38","cursor":"#232d38","black":"#232d38","red":"#b42828","green":"#3c7a1e","yellow":"#825600","blue":"#2a5bc4","magenta":"#7a3cb4","cyan":"#00697a","white":"#fafafa","brightBlack":"#5b6977","brightRed":"#b42828","brightGreen":"#3c7a1e","brightYellow":"#825600","brightBlue":"#2a5bc4","brightMagenta":"#7a3cb4","brightCyan":"#00697a","brightWhite":"#ffffff"}' ;;
	*) echo "shots: appearance must be dark or light" >&2; exit 2 ;;
esac
case "$size" in
	wide) width=1400; height=820; padding=40 ;;
	minimum) width=640; height=420; padding=20 ;;
	*) echo "shots: size must be wide or minimum" >&2; exit 2 ;;
esac
command -v vhs >/dev/null 2>&1 || { echo "shots: vhs is required (brew install vhs)" >&2; exit 1; }

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
go build -o "$work/docket" .
go run ./tools/seed -db "$work/shots.db" -tasks 400 -projects 12 >/dev/null
mkdir -p "$out"
out=$(cd "$out" && pwd)

# vhs drives a browser terminal that paints well behind the keys, and takes a
# screenshot only on a later frame, so each step sleeps both before and after
# its screenshot.
# vhs reads paths relative to where it runs, so the tape runs from the work
# directory and its screenshots are moved out afterwards.
cat >"$work/shots.tape" <<TAPE
Output shots.gif
Set Width $width
Set Height $height
Set Padding $padding
Set FontSize 15
Set Framerate 10
Set Shell bash
Set TypingSpeed 5ms
Set Theme $theme
Hide
Type "stty size > terminal-grid.txt; COLORTERM=truecolor DOCKET_DB=./shots.db ./docket"
TAPE
cat >>"$work/shots.tape" <<'TAPE'
Enter
Sleep 2s
# A launch may restore sidebar focus; Esc there returns to the pane.
Escape
Sleep 1500ms
Show
Screenshot launch.png
Sleep 1s

Tab
Sleep 800ms
Type "G"
Sleep 800ms
Screenshot sidebar.png
Sleep 1s
Enter
Sleep 1500ms
Screenshot project.png
Sleep 1s

Type "ggjVjj"
Sleep 1500ms
Screenshot selection.png
Sleep 1s
Escape
Sleep 800ms

Type "/#home"
Sleep 1500ms
Screenshot filter.png
Sleep 1s
Enter
Sleep 800ms
Escape
Sleep 800ms

Type "a"
Sleep 800ms
Type "Call the bank about the card @fri #errands !"
Sleep 1500ms
Screenshot capture.png
Sleep 1s
# Esc keeps the typed row and ends capture.
Escape
Sleep 1500ms

Tab
Type "ggj"
Sleep 800ms
Enter
Sleep 1500ms
Screenshot today.png
Sleep 1s

Tab
Type "ggjj"
Sleep 800ms
Enter
Sleep 1500ms
Screenshot waiting.png
Sleep 1s

Type ":review"
Enter
Sleep 1500ms
Screenshot review.png
Sleep 1s
Escape
Sleep 800ms

Type ":help"
Enter
Sleep 1500ms
Screenshot help.png
Sleep 1s
Escape
Sleep 800ms
Hide
Ctrl+C
Sleep 500ms
TAPE

(cd "$work" && vhs shots.tape >/dev/null 2>&1) || { echo "shots: vhs failed" >&2; exit 1; }
cp "$work"/*.png "$out"/
echo "$(cat "$work/terminal-grid.txt") (rows cols): wrote $(ls "$work"/*.png | wc -l | tr -d ' ') screens to $out"
