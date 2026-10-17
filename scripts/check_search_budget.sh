#!/usr/bin/env sh
set -eu

cd "$(dirname "$0")/.."
report=$(mktemp "${TMPDIR:-/tmp}/tt-search-budget.XXXXXX")
trap 'rm -f "$report"' EXIT HUP INT TERM

go version
go test ./internal/tui/outliner -run '^$' \
  -bench '^BenchmarkLiveSearchTyping/tasks=10000$/projects=20$/notes=true$' \
  -benchtime=1x -count=3 > "$report"
go test ./internal/tui/outliner -run '^$' \
  -bench '^BenchmarkLiveSearchContent/tasks=(1000|10000)$/query=(release|handoff|zzzzzzz)$' \
  -benchtime=1x -count=3 >> "$report"
go test ./internal/tui/outliner -run '^$' \
  -bench '^BenchmarkLiveSearchColdUniqueContent/tasks=10000$/query=(release|handoff|zzzzzzz)$' \
  -benchtime=1x -count=3 >> "$report"
go test ./internal/tui/outliner -run '^$' -bench '^BenchmarkLiveSearchResponsive$' \
  -benchtime=1x -count=3 >> "$report"
cat "$report"
go run ./scripts/searchbudget "$@" < "$report"
