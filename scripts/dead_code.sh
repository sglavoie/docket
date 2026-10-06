#!/usr/bin/env sh

set -eu

production_output=$(mktemp)
actionable_output=$(mktemp)
test_output=$(mktemp)
import_output=$(mktemp)
declaration_output=$(mktemp)
trap 'rm -f "$production_output" "$actionable_output" "$test_output" "$import_output" "$declaration_output"' EXIT HUP INT TERM

if ! deadcode ./... >"$production_output" 2>&1; then
	echo "production deadcode analysis failed:" >&2
	cat "$production_output" >&2
	exit 1
fi

# internal/models/query*.go is the canonical query value, built ahead of the
# grammar that compiles into it. deadcode still reports NewQuery,
# AllQueryModes, AllQuerySortFields, and ParseQuery as unreached from
# production; delete this pattern once internal/textgrammar imports the query.
#
# internal/engine's explicit machine and query evaluation serve headless
# frontends; the desktop opens as the process machine and queries through the
# Outliner. They are exercised by internal/engine's own tests and become
# reachable when the mobile bridge calls them; delete this pattern then.
grep -v '^internal/testutil/' "$production_output" |
	grep -v '^internal/models/query' |
	grep -Ev '^internal/engine/[a-z_]+\.go:[0-9]+:[0-9]+: unreachable func: (ExplicitMachine|QueryItem\.Locator|Runtime\.(Query|queryPopulation|Now|Refresh)|opensPopulation|planningRanks|modeAdmits|modeLead|planningTieRank)$' >"$actionable_output" || true
if [ -s "$actionable_output" ]; then
	echo "production dead code detected outside internal/testutil:" >&2
	cat "$actionable_output" >&2
	exit 1
fi

if git grep -n 'github.com/sglavoie/tt/internal/testutil' -- '*.go' ':!*_test.go' >"$import_output"; then
	echo "production source imports internal/testutil:" >&2
	cat "$import_output" >&2
	exit 1
fi

if ! deadcode -test ./... >"$test_output" 2>&1; then
	echo "test-inclusive deadcode analysis failed:" >&2
	cat "$test_output" >&2
	exit 1
fi

if [ -s "$test_output" ]; then
	echo "dead code detected in the test-inclusive graph:" >&2
	cat "$test_output" >&2
	exit 1
fi

if ! go run ./scripts/deadcodeaudit ./... >"$declaration_output" 2>&1; then
	echo "production declaration/field/message dead code detected:" >&2
	cat "$declaration_output" >&2
	exit 1
fi

echo "No actionable production or test-inclusive dead code found"
