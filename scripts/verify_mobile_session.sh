#!/usr/bin/env bash
# Focused verification for the native iPhone app and offline sync plan.
#
#   bash scripts/verify_mobile_session.sh <session>
#
# Each session adds its own case below. A case passes only on behavioral
# evidence: every required suite must exist, match at least one test, run
# without a skip, and report each named test (and named subtest) as passed.
# Building or listing something is never enough on its own.

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root"

work_dir=$(mktemp -d)
trap 'rm -rf "$work_dir"' EXIT

fail() {
	echo "verify_mobile_session: $*" >&2
	exit 1
}

note() {
	echo "verify_mobile_session: $*"
}

# require_files PATH... fails unless every path exists and is non-empty.
require_files() {
	local path
	for path in "$@"; do
		[ -s "$path" ] || fail "required artifact missing or empty: $path"
	done
}

# run_go_suite LABEL PACKAGE RUN_REGEX REQUIRED_TEST...
#
# Runs one Go test selection verbosely and holds it to the evidence rules:
# the command must succeed, must match tests, must skip none, and must report
# every REQUIRED_TEST (a top-level name or a Test/subtest path) as passed. The
# verbose log is left at $work_dir/LABEL.log for later evidence checks.
run_go_suite() {
	local label=$1 package=$2 pattern=$3
	shift 3
	local log="$work_dir/$label.log"
	note "running $label: go test -count=1 -v -run '$pattern' $package"
	if ! go test -count=1 -v -run "$pattern" "$package" >"$log" 2>&1; then
		cat "$log" >&2
		fail "$label: go test failed"
	fi
	if grep -q 'no tests to run' "$log"; then
		cat "$log" >&2
		fail "$label: pattern '$pattern' matched no tests in $package"
	fi
	if grep -Eq -- '--- SKIP:' "$log"; then
		grep -E -- '--- SKIP:' "$log" >&2
		fail "$label: a required test was skipped"
	fi
	local passed
	passed=$(grep -Ec -- '--- PASS:' "$log" || true)
	[ "$passed" -gt 0 ] || fail "$label: zero tests passed"
	local test
	for test in "$@"; do
		require_passed "$label" "$test"
	done
	note "$label: $passed passing tests and subtests"
}

# require_passed LABEL TEST fails unless the suite log records TEST as passed.
require_passed() {
	local label=$1 test=$2
	local log="$work_dir/$label.log"
	grep -Fq -- "--- PASS: $test (" "$log" || fail "$label: missing required passing test $test"
}

# fixture_coverage_keys MANIFEST prints the coverage keys a fixture manifest
# declares, one per line.
fixture_coverage_keys() {
	sed -n '/"coverage": \[/,/\]/p' "$1" | grep -Eo '"[a-z_0-9]+\.[a-z_0-9]+"' | tr -d '"'
}

verify_session_1() {
	local docs=(
		docs/mobile/README.md
		docs/mobile/bridge-contract.md
		docs/mobile/sync-envelope.md
		docs/mobile/fixture-contract.md
		docs/mobile/write-path-inventory.md
	)
	require_files "${docs[@]}" \
		internal/compat/testdata/write_paths.tsv \
		internal/compat/testdata/workspaces/representative.fixture.json \
		internal/compat/testdata/workspaces/representative/.tt/workspace.json \
		internal/compat/testdata/workspaces/representative/.tt/planning.json \
		internal/compat/testdata/workspaces/representative/.tt/lenses.json

	if grep -q 'UNASSIGNED' internal/compat/testdata/write_paths.tsv; then
		fail "write-path inventory has unclassified rows"
	fi

	run_go_suite compat ./internal/compat/ '^TestCompat' \
		TestCompatFixtureManifestsDeclareLiveSchemas \
		TestCompatRepresentativeWorkspaceLoadsThroughLiveReaders \
		TestCompatRepresentativeWorkspaceLoadsThroughLiveReaders/representative \
		TestCompatFixtureCoverageSurvivesLiveParsing \
		TestCompatDesktopOpenLeavesFixtureUntouched \
		TestCompatDesktopOpenLeavesFixtureUntouched/representative \
		TestCompatExternalMarkdownEditIsObservable \
		TestCompatWritePathInventoryCoversEveryDurableWrite \
		TestCompatCommitEntryPointsAreDocumented \
		TestCompatContractDocumentsAreWellFormed \
		TestCompatFixtureVocabularyMatchesProbes

	# Every coverage key the fixture declares must have run as its own passing
	# subtest, so a probe silently dropped from the loop is missing evidence.
	local manifest key count=0
	for manifest in internal/compat/testdata/workspaces/*.fixture.json; do
		local name
		name=$(basename "$manifest" .fixture.json)
		while IFS= read -r key; do
			require_passed compat "TestCompatFixtureCoverageSurvivesLiveParsing/$name/$key"
			count=$((count + 1))
		done < <(fixture_coverage_keys "$manifest")
	done
	[ "$count" -gt 0 ] || fail "no fixture coverage keys were found to verify"
	note "fixture coverage: $count declared keys passed"

	# The raw-save audits are what keep the commit-journal rows outside
	# CommitService confined to the test-double path the inventory claims.
	run_go_suite storage-audit ./internal/storage/ \
		'^(TestMutationCallsiteInventoryBaseline|TestRawProjectSaveCallsitesAreAllowlisted|TestCurrentTaskSchema.*)$' \
		TestMutationCallsiteInventoryBaseline \
		TestRawProjectSaveCallsitesAreAllowlisted \
		TestCurrentTaskSchemaRoundTripCarriesEveryPersistedField

	note "session 1 verified"
}

verify_session_2() {
	require_files \
		internal/engine/config.go \
		internal/engine/runtime.go \
		internal/engine/repository.go \
		internal/engine/planning.go \
		internal/engine/query.go \
		internal/models/query_census.go \
		internal/models/query_sort.go

	# The headless runtime: capture, planning, editing, completion, and reopen
	# with no frontend import, explicit configuration, and query evaluation.
	run_go_suite engine ./internal/engine/ '^TestRuntime' \
		TestRuntimeHeadlessCapturePlanEditCompleteReopen \
		TestRuntimeOpenRequiresExplicitConfiguration \
		TestRuntimeBindsTheConfiguredDevice \
		TestRuntimeImportsNoFrontend \
		TestRuntimeQueryAnswersPopulationCensusAndModes \
		TestRuntimeQueryAnswersPopulationCensusAndModes/all \
		TestRuntimeQueryAnswersPopulationCensusAndModes/census \
		TestRuntimeQueryAnswersPopulationCensusAndModes/population \
		TestRuntimeQueryAnswersPopulationCensusAndModes/modes \
		TestRuntimeQueryAnswersPopulationCensusAndModes/scope_and_sort

	# The canonical query evaluator the runtime and the Outliner share.
	run_go_suite models-query ./internal/models/ \
		'^(TestQueryAdmitsInPopulationAnswersCensusFields|TestCompareTasksBySortFieldOrdersEachField|TestQuery.*)$' \
		TestQueryAdmitsInPopulationAnswersCensusFields \
		TestCompareTasksBySortFieldOrdersEachField

	# Desktop startup composes through the runtime: recovery before load, and
	# the write-path and raw-save audits that follow the repository's move.
	run_go_suite tui-composition ./internal/tui/ \
		'^(TestNewAppModel.*|TestProjectPopulationCallersAreClassified|TestRawArchivePredicatesAreClassified)$' \
		TestNewAppModelRecoversTransactionsBeforeLoadingWorkspace \
		TestNewAppModelBlocksOnRecoveryQuarantine \
		TestProjectPopulationCallersAreClassified \
		TestRawArchivePredicatesAreClassified
	run_go_suite storage-audit ./internal/storage/ \
		'^(TestMutationCallsiteInventoryBaseline|TestRawProjectSaveCallsitesAreAllowlisted)$' \
		TestMutationCallsiteInventoryBaseline \
		TestRawProjectSaveCallsitesAreAllowlisted
	run_go_suite compat-inventory ./internal/compat/ \
		'^TestCompatWritePathInventoryCoversEveryDurableWrite$' \
		TestCompatWritePathInventoryCoversEveryDurableWrite

	# Every committed desktop scenario replays through the runtime-composed
	# shell, each as its own passing subtest, so a skipped or dropped scenario is
	# missing evidence rather than a quiet success.
	run_go_suite tui-scenarios ./internal/tui/ \
		'^Test(CommittedScenarioInventoryCoversFinalProduct|ExampleScenariosPass)$' \
		TestCommittedScenarioInventoryCoversFinalProduct \
		TestExampleScenariosPass
	local scenario count=0
	for scenario in docs/tui-testing/scenarios/*.json; do
		require_passed tui-scenarios "TestExampleScenariosPass/$(basename "$scenario")"
		count=$((count + 1))
	done
	[ "$count" -gt 0 ] || fail "no committed scenarios were found to verify"
	note "desktop scenarios: $count passed through the shared runtime"

	# The existing aggregate, unchanged, as the desktop regression gate.
	note "running scripts/run_tui_scenarios.sh"
	if ! ./scripts/run_tui_scenarios.sh >"$work_dir/run_tui_scenarios.log" 2>&1; then
		cat "$work_dir/run_tui_scenarios.log" >&2
		fail "scripts/run_tui_scenarios.sh failed"
	fi

	note "session 2 verified"
}

session=${1:-}
case "$session" in
1) verify_session_1 ;;
2) verify_session_2 ;;
"") fail "usage: verify_mobile_session.sh <session>" ;;
*) fail "no verifier defined for session $session" ;;
esac
