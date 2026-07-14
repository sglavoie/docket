#!/usr/bin/env sh

set -eu

profile=${1:-coverage.out}
overall_threshold=$(tr -d '[:space:]' < .coverage-threshold)

if [ ! -f "$profile" ]; then
	echo "coverage profile not found: $profile" >&2
	exit 1
fi

coverage_for_prefix() {
	prefix=$1
	awk -v prefix="$prefix" '
		NR == 1 { next }
		index($1, prefix) == 1 {
			total += $2
			if ($3 > 0) covered += $2
		}
		END {
			if (total == 0) exit 2
			printf "%.4f", covered * 100 / total
		}
	' "$profile"
}

check_threshold() {
	label=$1
	actual=$2
	threshold=$3
	echo "$label coverage: ${actual}% (minimum ${threshold}%)"
	if ! awk -v actual="$actual" -v threshold="$threshold" 'BEGIN { exit !(actual + 0 >= threshold + 0) }'; then
		echo "$label coverage ${actual}% is below ${threshold}%" >&2
		exit 1
	fi
}

overall=$(coverage_for_prefix "github.com/sglavoie/tt/")
check_threshold "overall" "$overall" "$overall_threshold"

while read -r prefix threshold; do
	case "$prefix" in
		""|'#'*) continue ;;
	esac
	package_coverage=$(coverage_for_prefix "$prefix")
	check_threshold "$prefix" "$package_coverage" "$threshold"
done < .coverage-packages
