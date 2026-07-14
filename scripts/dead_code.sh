#!/usr/bin/env sh

set -eu

output=$(mktemp)
trap 'rm -f "$output"' EXIT HUP INT TERM

if ! deadcode -test ./... >"$output" 2>&1; then
	echo "deadcode failed:" >&2
	cat "$output" >&2
	exit 1
fi

if [ -s "$output" ]; then
	echo "dead code detected:" >&2
	cat "$output" >&2
	exit 1
fi

echo "No dead code found"
