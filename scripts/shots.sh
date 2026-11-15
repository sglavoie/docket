#!/usr/bin/env sh

# Screenshot the everyday screens for a taste pass: build the current tree,
# seed a fictional workspace through the real interface, and write one PNG per
# screen. The workspace lives in a temporary directory and never touches
# ~/.docket. Requires vhs (brew install vhs).
#
#   ./scripts/shots.sh [output-dir] [dark|light] [wide|minimum]
# Defaults: ./shots dark wide. Captures run in VHS's headless browser.
# minimum verifies the actual PTY is 60 columns by 20 rows, not just a PNG size.
#
# Compare a run against the last one side by side; the screens are the review,
# not a test, so nothing here fails on a visual change.

set -eu

out=${1:-shots}
appearance=${2:-dark}
size=${3:-wide}
case "$appearance" in
	dark | light) ;;
	*) echo "shots: appearance must be dark or light" >&2; exit 2 ;;
esac
case "$size" in
	wide) width=1400; height=820; padding=60 ;;
	minimum) width=630; height=380; padding=0 ;;
	*) echo "shots: size must be wide or minimum" >&2; exit 2 ;;
esac
command -v vhs >/dev/null 2>&1 || { echo "shots: vhs is required (brew install vhs)" >&2; exit 1; }

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
go build -o "$work/docket" .
mkdir -p "$out"
out=$(cd "$out" && pwd)

# vhs reads paths relative to where it runs, and refuses absolute ones in some
# commands, so the tape runs from the work directory.
cat >"$work/shots.tape" <<TAPE
Output shots.gif
Set Width $width
Set Height $height
Set Padding $padding
Set FontSize 15
Set Framerate 10
Set Shell bash
Set TypingSpeed 5ms
Hide
Type "stty size > terminal-grid.txt; ./docket --data-dir ./ws --appearance $appearance"
TAPE
cat >>"$work/shots.tape" <<'TAPE'
Enter
Sleep 2s
Show
Screenshot welcome.png
Sleep 500ms
Hide

Type "a"
Sleep 300ms
Type "Draft the release notes %website !1 #docs $today "
Enter
Sleep 400ms
Type "y"
Sleep 600ms
Type "a"
Sleep 300ms
Type "Fix login redirect %website @tomorrow #bug $doing "
Enter
Sleep 600ms
Type "a"
Sleep 300ms
Type "Review analytics dashboard %website $next "
Enter
Sleep 600ms
Type "a"
Sleep 300ms
Type "Call the bank !1 @today #errands $today "
Enter
Sleep 600ms
Type "a"
Sleep 300ms
Type "Order new keyboard @yesterday !2 "
Enter
Sleep 600ms
Type "a"
Sleep 300ms
Type "Renew the passport @fri #errands "
Enter
Sleep 600ms
Type "a"
Sleep 300ms
Type "Plan offsite agenda %team ##desk $waiting "
Enter
Sleep 400ms
Type "y"
Sleep 600ms
Type "a"
Sleep 300ms
Type "Book venue for offsite %team @nov20 $next "
Enter
Sleep 600ms
Type "a"
Sleep 300ms
Type "Ship dark mode toggle %website !2 $next #feature "
Enter
Sleep 600ms
Type "a"
Sleep 300ms
Type "Read the Bubble Tea docs $backlog "
Enter
Sleep 1s
# Give the Now reading a concrete place to resume and three next steps.
Type "gd"
Sleep 400ms
Type "r"
Sleep 300ms
Type "Reproduce with an expired token"
Enter
Sleep 500ms
Type "c"
Sleep 300ms
Type "Reproduce the redirect loop"
Enter
Sleep 500ms
Type "c"
Sleep 300ms
Type "Add a regression test"
Enter
Sleep 500ms
Type "c"
Sleep 300ms
Type "Ship the fix"
Enter
Sleep 500ms
Escape
Sleep 400ms
Show

Type "gg"
Sleep 800ms
Screenshot all.png
Sleep 500ms
Down
Down
Down
Type "b"
Sleep 800ms
Screenshot peek.png
Sleep 500ms
Type "b"
Type "gt"
Sleep 1s
Screenshot today.png
Sleep 500ms
Type "gd"
Sleep 1s
Screenshot now.png
Sleep 500ms
Type "f"
Sleep 800ms
Screenshot focus.png
Sleep 500ms
Escape
Sleep 600ms
Type "?"
Sleep 1s
Screenshot help.png
Sleep 500ms
Escape
Sleep 600ms
Type "M"
Sleep 800ms
Screenshot menu.png
Sleep 500ms
Escape
Sleep 400ms
Type "a"
Sleep 300ms
Type "Finish weekly review $done "
Enter
Sleep 800ms
Type "ga"
Sleep 800ms
Screenshot all-progress.png
Sleep 500ms
Type "/offsite"
Enter
Sleep 800ms
Screenshot search.png
Sleep 500ms
Escape
Sleep 400ms
Type "gr"
Sleep 800ms
Screenshot recent.png
Sleep 500ms
Type "P"
Sleep 600ms
Type "G"
Sleep 800ms
Screenshot select-project.png
Sleep 500ms
Escape
Sleep 400ms
Type "gi"
Sleep 400ms
Type "I"
Sleep 800ms
Screenshot triage.png
Sleep 500ms
Escape
Sleep 400ms
Type "gm"
Sleep 800ms
Screenshot start-day.png
Sleep 500ms
Escape
Sleep 400ms
Escape
Sleep 400ms
Type "ge"
Sleep 800ms
Screenshot close-day.png
Sleep 500ms
Escape
Sleep 800ms
Screenshot day-closed.png
Sleep 500ms
TAPE

(cd "$work" && vhs shots.tape >/dev/null)
if [ "$size" = minimum ] && [ "$(cat "$work/terminal-grid.txt")" != "20 60" ]; then
	echo "shots: expected PTY grid 20 60, got $(cat "$work/terminal-grid.txt")" >&2
	exit 1
fi
cp "$work"/*.png "$out"/
cp "$work/shots.tape" "$work/terminal-grid.txt" "$out"/
echo "Wrote $(ls "$work"/*.png | wc -l | tr -d ' ') screens to $out"
