#!/usr/bin/env sh
# Read-only code quality checks: nothing here rewrites a file.

set -eu

unformatted=$(gofmt -l $(git ls-files '*.go'))
if [ -n "$unformatted" ]; then
	echo "gofmt reports unformatted files (run 'just fmt'):" >&2
	echo "$unformatted" >&2
	exit 1
fi

unimported=$(goimports -l -local "github.com/sglavoie/docket" $(git ls-files '*.go'))
if [ -n "$unimported" ]; then
	echo "goimports reports misordered imports (run 'just fmt'):" >&2
	echo "$unimported" >&2
	exit 1
fi

go vet ./...
golangci-lint run
./scripts/dead_code.sh
