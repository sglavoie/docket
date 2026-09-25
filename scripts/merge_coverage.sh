#!/usr/bin/env sh
# Merge Go text coverage profiles into one, summing each block's count, so a
# block that several processes all reported is counted once rather than once
# per profile by check_coverage.sh.
#
# usage: merge_coverage.sh OUTPUT INPUT...

set -eu

if [ "$#" -lt 2 ]; then
	echo "usage: $0 OUTPUT INPUT..." >&2
	exit 2
fi

output=$1
shift

awk '
	/^mode:/ {
		if (mode == "") mode = $0
		else if ($0 != mode) { print "mixed coverage modes: " mode " and " $0 > "/dev/stderr"; exit 1 }
		next
	}
	{
		key = $1 " " $2
		if (!(key in count)) order[++n] = key
		count[key] += $3
	}
	END {
		if (mode == "") { print "no coverage mode line in inputs" > "/dev/stderr"; exit 1 }
		print mode
		for (i = 1; i <= n; i++) print order[i], count[order[i]]
	}
' "$@" >"$output"
