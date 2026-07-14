#!/usr/bin/env sh

set -eu

go run ./scripts/gen-api-ref > cmd/cli-reference.md
go test -count=1 . -run 'TestREADME|TestCLIReferenceContractStaysCurrent|TestArchitecture'
