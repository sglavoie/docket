#!/usr/bin/env sh

set -eu

if [ "$#" -gt 1 ]; then
	echo "usage: $0 [scenario.json]" >&2
	exit 2
fi

if [ "$#" -eq 1 ]; then
	TT_TUI_SCENARIO=$1 go test -count=1 ./internal/tui -run '^TestExampleScenariosPass$'
	exit
fi

go test -count=1 ./internal/tui \
	-run '^Test(CommittedScenarioInventoryCoversFinalProduct|ExampleScenariosPass)$'

go test -count=1 ./internal/config \
	-run '^TestManager(ActivatesSnapshotFirstV5MigrationAndIsIdempotent|V5MigrationCorruptInputRefusesBeforeSnapshot|V5MigrationRejectsFutureTaskSchemaBeforeSnapshot|V5MigrationRecoversPreparedActivationBeforeInspection)$'
