#!/usr/bin/env sh

# Regenerate the README animation from TestReadmeDemo, or with --check fail when
# the committed docs/images/demo.svg no longer matches what the app draws.
# Requires uv for the SVG renderer.

set -eu

image=docs/images/demo.svg
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

DOCKET_README_SCREENSHOT_DIR="$work" go test ./internal/tui -run '^TestReadmeDemo$' -count=1 >/dev/null
uv run -q scripts/render_terminal_svg.py --animate "$work/demo.json" "$work/demo.svg"

if [ "${1:-}" = "--check" ]; then
	if ! cmp -s "$work/demo.svg" "$image"; then
		echo "$image is stale: run 'just readme-media' and commit the result." >&2
		exit 1
	fi
	echo "$image is current."
else
	mkdir -p "$(dirname "$image")"
	cp "$work/demo.svg" "$image"
	echo "Wrote $image."
fi
