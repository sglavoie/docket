#!/usr/bin/env sh

# Screenshot the everyday screens for a taste pass: build the current tree,
# seed a fictional workspace through the real interface, and write one PNG per
# screen. The workspace lives in a temporary directory and never touches
# ~/.docket. Requires vhs (brew install vhs).
#
#   ./scripts/shots.sh [output-dir]    # default: ./shots
#
# Compare a run against the last one side by side; the screens are the review,
# not a test, so nothing here fails on a visual change.

set -eu

out=${1:-shots}
command -v vhs >/dev/null 2>&1 || { echo "shots: vhs is required (brew install vhs)" >&2; exit 1; }

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
go build -o "$work/docket" .
mkdir -p "$out"
out=$(cd "$out" && pwd)

# vhs reads paths relative to where it runs, and refuses absolute ones in some
# commands, so the tape runs from the work directory.
cat >"$work/shots.tape" <<'TAPE'
Output shots.gif
Set Width 1400
Set Height 820
Set FontSize 15
Set Shell bash
Set TypingSpeed 5ms
Hide
Type "./docket --data-dir ./ws"
Enter
Sleep 2s
Screenshot welcome.png

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
Show

Type "gg"
Sleep 800ms
Screenshot all.png
Down
Down
Down
Type "b"
Sleep 800ms
Screenshot peek.png
Type "b"
Type "gt"
Sleep 1s
Screenshot today.png
Type "gd"
Sleep 1s
Screenshot now.png
Type "f"
Sleep 800ms
Screenshot focus.png
Escape
Sleep 600ms
Type "?"
Sleep 1s
Screenshot help.png
Escape
Sleep 600ms
Type "M"
Sleep 800ms
Screenshot menu.png
Escape
TAPE

(cd "$work" && vhs shots.tape >/dev/null)
cp "$work"/*.png "$out"/
echo "Wrote $(ls "$work"/*.png | wc -l | tr -d ' ') screens to $out"
