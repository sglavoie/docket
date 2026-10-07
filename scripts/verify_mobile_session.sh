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

# require_slice_platform LABEL BINARY ARCHS PLATFORM fails unless the archive
# carries exactly ARCHS and every object in it was built for Mach-O PLATFORM
# (2 is iOS, 7 the iOS simulator).
require_slice_platform() {
	local label=$1 binary=$2 archs=$3 platform=$4
	local actual arch platforms
	actual=$(lipo -archs "$binary" | tr ' ' '\n' | sort | xargs)
	[ "$actual" = "$archs" ] || fail "$label slice carries architectures '$actual', want '$archs'"
	for arch in $archs; do
		platforms=$(otool -arch "$arch" -l "$binary" | awk '/LC_BUILD_VERSION/ { want = 1; next } want && $1 == "platform" { print $2; want = 0 }' | sort -u | xargs)
		[ "$platforms" = "$platform" ] || fail "$label $arch objects target platforms '$platforms', want '$platform'"
	done
}

# require_xcresult_passed BUNDLE TEST... holds an xcodebuild result bundle to
# the evidence rules: it passed, ran tests, failed and skipped none, and records
# each named test case as passed.
require_xcresult_passed() {
	local bundle=$1
	shift
	local summary="$work_dir/xcresult-summary.json" tests="$work_dir/xcresult-tests.json"
	xcrun xcresulttool get test-results summary --path "$bundle" >"$summary" ||
		fail "cannot read the result bundle summary at $bundle"
	xcrun xcresulttool get test-results tests --path "$bundle" >"$tests" ||
		fail "cannot read the result bundle tests at $bundle"
	local result passed failed skipped
	result=$(plutil -extract result raw -o - "$summary")
	passed=$(plutil -extract passedTests raw -o - "$summary")
	failed=$(plutil -extract failedTests raw -o - "$summary")
	skipped=$(plutil -extract skippedTests raw -o - "$summary")
	[ "$result" = Passed ] || fail "simulator suite result is $result"
	[ "$failed" -eq 0 ] || fail "simulator suite failed $failed tests"
	[ "$skipped" -eq 0 ] || fail "simulator suite skipped $skipped tests"
	[ "$passed" -gt 0 ] || fail "simulator suite passed zero tests"
	local test
	for test in "$@"; do
		python3 -I - "$tests" "$test" <<'PY' || fail "simulator suite does not record $test as passed"
import json, sys

def cases(node):
    if node.get("nodeType") == "Test Case":
        yield node
    for child in node.get("children", []):
        yield from cases(child)

with open(sys.argv[1]) as handle:
    tree = json.load(handle)
wanted = sys.argv[2]
found = [case for root in tree["testNodes"] for case in cases(root) if case.get("name") == wanted]
sys.exit(0 if found and all(case.get("result") == "Passed" for case in found) else 1)
PY
	done
	note "simulator suite: $passed passed, 0 failed, 0 skipped"
}

verify_session_3() {
	[ "$(uname -s)" = Darwin ] || fail "session 3 builds and runs the iOS simulator suite on macOS only"
	require_files \
		internal/bridge/protocol.go \
		internal/bridge/bridge.go \
		internal/bridge/ops.go \
		mobile/ttbridge/ttbridge.go \
		mobile/bridgehost/main.go \
		ios/project.yml \
		ios/scripts/build-bridge.sh \
		ios/scripts/generate-project.sh \
		ios/scripts/test-engine.sh \
		ios/TTEngine/Package.swift \
		ios/TTEngine/Sources/TTEngine/TTEngine.swift \
		ios/TTEngine/Sources/TTEngine/BridgeModels.swift \
		ios/TTEngine/Tests/TTEngineTests/BridgeSimulatorTests.swift \
		ios/TTMobile/App/TTMobileApp.swift
	grep -Eq '^[[:space:]]+golang.org/x/mobile/cmd/gomobile$' go.mod || fail "go.mod pins no gomobile tool"
	grep -Eq '^[[:space:]]+golang.org/x/mobile/cmd/gobind$' go.mod || fail "go.mod pins no gobind tool"

	# The protocol in Go: envelope refusals, the open/capture/complete/reopen
	# cycle with replay and stale preconditions, refusing migration, and the
	# two-function binding surface.
	run_go_suite bridge ./internal/bridge/ \
		'^(TestBridgeRefusesMalformedAndMisaddressedRequests|TestBridgeCapturesCompletesAndReopens|TestBridgeOpenRefusesToMigrate|TestBridgeCreatesAFreshWorkspace|TestSimulatorEvidenceShapeMatchesTheSwiftRecord)$' \
		TestBridgeRefusesMalformedAndMisaddressedRequests \
		TestBridgeRefusesMalformedAndMisaddressedRequests/reserved_op \
		TestBridgeRefusesMalformedAndMisaddressedRequests/workspace_mismatch \
		TestBridgeCapturesCompletesAndReopens \
		TestBridgeCapturesCompletesAndReopens/stale_expectation_commits_nothing \
		TestBridgeCapturesCompletesAndReopens/replay_returns_the_first_result \
		TestBridgeCapturesCompletesAndReopens/queries \
		TestBridgeOpenRefusesToMigrate/older \
		TestBridgeOpenRefusesToMigrate/newer \
		TestBridgeCreatesAFreshWorkspace \
		TestSimulatorEvidenceShapeMatchesTheSwiftRecord
	run_go_suite engine-refuse-migration ./internal/engine/ \
		'^(TestRuntimeOpenRefusesMigrationWhenAsked|TestRuntimeOpenRequiresExplicitConfiguration)$' \
		TestRuntimeOpenRefusesMigrationWhenAsked \
		TestRuntimeOpenRequiresExplicitConfiguration
	run_go_suite mobile-binding ./mobile/... '^Test' \
		TestBindingExportsExactlyTheContractSurface \
		TestBindingAnswersThroughTheSharedBridge \
		TestServeAnswersEachRequestLine

	# Both slices from the pinned generator, published only with the
	# fingerprint the current sources carry, and reused while it holds.
	note "building TTBridgeCore.xcframework"
	ios/scripts/build-bridge.sh >"$work_dir/build-bridge.log" 2>&1 || {
		cat "$work_dir/build-bridge.log" >&2
		fail "ios/scripts/build-bridge.sh failed"
	}
	local framework=ios/TTEngine/Frameworks/TTBridgeCore.xcframework
	local stamp=ios/TTEngine/Frameworks/TTBridgeCore.fingerprint
	require_files "$framework/Info.plist" "$stamp" \
		"$framework/ios-arm64/TTBridgeCore.framework/TTBridgeCore" \
		"$framework/ios-arm64_x86_64-simulator/TTBridgeCore.framework/TTBridgeCore"
	[ "$(cat "$stamp")" = "$(ios/scripts/build-bridge.sh --fingerprint)" ] ||
		fail "the published framework's fingerprint is not the current sources'"
	ios/scripts/build-bridge.sh >"$work_dir/build-bridge-again.log" 2>&1 ||
		fail "rerunning ios/scripts/build-bridge.sh failed"
	grep -q 'is current' "$work_dir/build-bridge-again.log" ||
		fail "an unchanged bridge was rebuilt rather than reused: $(cat "$work_dir/build-bridge-again.log")"
	require_slice_platform device "$framework/ios-arm64/TTBridgeCore.framework/TTBridgeCore" arm64 2
	require_slice_platform simulator "$framework/ios-arm64_x86_64-simulator/TTBridgeCore.framework/TTBridgeCore" "arm64 x86_64" 7
	note "XCFramework: device and simulator slices at fingerprint $(cat "$stamp")"

	# The generated app links each slice: built for a simulator and, unsigned,
	# for a device.
	note "generating ios/TTMobile.xcodeproj"
	ios/scripts/generate-project.sh >"$work_dir/generate-project.log" 2>&1 || {
		cat "$work_dir/generate-project.log" >&2
		fail "ios/scripts/generate-project.sh failed"
	}
	require_files ios/TTMobile.xcodeproj/project.pbxproj
	local destination products
	for destination in 'iOS Simulator:iphonesimulator' 'iOS:iphoneos'; do
		products="$work_dir/app-derived/Build/Products/Debug-${destination#*:}/tt.app"
		note "building TTMobile for generic/platform=${destination%%:*}"
		xcodebuild build -project ios/TTMobile.xcodeproj -scheme TTMobile \
			-destination "generic/platform=${destination%%:*}" \
			-derivedDataPath "$work_dir/app-derived" CODE_SIGNING_ALLOWED=NO \
			>"$work_dir/app-${destination#*:}.log" 2>&1 || {
			tail -40 "$work_dir/app-${destination#*:}.log" >&2
			fail "TTMobile does not build for ${destination%%:*}"
		}
		[ -d "$products" ] || fail "no tt.app was produced for ${destination%%:*}"
		# A Debug build moves the app's code into tt.debug.dylib; read both.
		nm -arch arm64 "$products"/tt "$products"/tt.debug.dylib >"$work_dir/app-symbols.txt" 2>/dev/null || true
		grep -q '_TtbridgeCall$' "$work_dir/app-symbols.txt" ||
			fail "tt.app for ${destination%%:*} does not link the Go bridge"
	done

	# The simulator suite over the real engine, then Go reading what it wrote.
	note "running the TTEngine simulator suite"
	ios/scripts/test-engine.sh "$work_dir/ios" >"$work_dir/test-engine.log" 2>&1 || {
		cat "$work_dir/test-engine.log" >&2
		fail "ios/scripts/test-engine.sh failed"
	}
	require_xcresult_passed "$work_dir/ios/TTEngine.xcresult" \
		'testLinkedEngineSpeaksTheModelsBridgeVersion()' \
		'testEngineRefusalsArriveAsTypedErrors()' \
		'testOpenFixtureCaptureCompleteReopen()'
	require_files "$work_dir/ios/evidence/evidence.json"
	TT_BRIDGE_SIMULATOR_EVIDENCE="$work_dir/ios/evidence/evidence.json" \
		run_go_suite simulator-evidence ./internal/bridge/ '^TestBridgeSimulatorEvidence$' \
		TestBridgeSimulatorEvidence \
		TestBridgeSimulatorEvidence/fixture_tasks_survive \
		TestBridgeSimulatorEvidence/other_project_files_untouched \
		TestBridgeSimulatorEvidence/go_reopen_agrees

	note "session 3 verified"
}

# require_crash_prefixes LABEL TEST MIN requires TEST to have run as passing
# crash-injection subtests at no fewer than MIN distinct points, including the
# undurable manifest (-1), the prepared manifest (00), and a first applied
# operation (01), so a fixture that quietly stopped injecting is missing
# evidence.
require_crash_prefixes() {
	local label=$1 test=$2 min=$3
	local prefix count
	for prefix in -1 00 01; do
		require_passed "$label" "$test/crash_after_$prefix"
	done
	count=$(grep -Fc -- "--- PASS: $test/crash_after_" "$work_dir/$label.log" || true)
	[ "$count" -ge "$min" ] || fail "$label: $test ran $count crash points, want at least $min"
	note "$label: $test survived $count crash points"
}

verify_session_4() {
	require_files \
		internal/synclog/manifest.go \
		internal/synclog/history.go \
		internal/synclog/plan_local.go \
		internal/synclog/plan_incoming.go \
		internal/storage/commit_sync.go \
		internal/storage/sync_history_test.go \
		internal/engine/sync.go

	# The record format: content addressing, explicit effects, and which paths
	# are exchanged at all.
	run_go_suite synclog ./internal/synclog/ '^TestSync' \
		TestSyncManifestContentAddressIsCanonical \
		TestSyncManifestRefusesIncompleteEffects \
		TestSyncClassifyPaths

	# The commit boundary: every entry point stopped at every point of its
	# journal transaction lands the edit and its outgoing record together or not
	# at all; incoming transactions are applied and acknowledged together; every
	# refusal acknowledges nothing; ancestry and outbox tracking hold.
	local mutations=(commit_multi artifact_transaction config_artifacts ops_transaction_rename undo)
	local refusals=(held_capability held_schema sequence_gap missing_parent missing_blob concurrent_local_edit foreign_workspace tampered own_transaction)
	local required=(
		TestSyncLocalCommitAndOutboxAreAtomic
		TestSyncLocalRecordsNameProjectsAndAncestry
		TestSyncLocalCommitRefusesUnclassifiedPaths
		TestSyncPublicationConfirmationDrainsTheOutbox
		TestSyncIncomingApplyAndReceiptAreAtomic
		TestSyncIncomingRefusalsAcknowledgeNothing
		TestSyncIncomingKeepsCausalOrderAndConcurrentHeads
	)
	local name
	for name in "${mutations[@]}"; do
		required+=("TestSyncLocalCommitAndOutboxAreAtomic/$name")
	done
	for name in "${refusals[@]}"; do
		required+=("TestSyncIncomingRefusalsAcknowledgeNothing/$name")
	done
	run_go_suite storage-sync ./internal/storage/ '^TestSync' "${required[@]}"
	for name in "${mutations[@]}"; do
		require_crash_prefixes storage-sync "TestSyncLocalCommitAndOutboxAreAtomic/$name" 6
	done
	require_crash_prefixes storage-sync TestSyncIncomingApplyAndReceiptAreAtomic 6

	# The runtime records its own mutations and a second device applies them.
	run_go_suite engine-sync ./internal/engine/ '^TestRuntimeRecordsSyncHistoryAndAppliesItElsewhere$' \
		TestRuntimeRecordsSyncHistoryAndAppliesItElsewhere

	# The journal and ledger guarantees the history rides on, unchanged.
	run_go_suite journal ./internal/transaction/ '^TestRecover_' \
		TestRecover_RollsForwardPreparedWrite \
		TestRecover_RollsForwardMoveAndDelete \
		TestRecover_SweepsOrphanStaging \
		TestRecover_ResumesAfterMidApplyCrash
	run_go_suite storage-ledger ./internal/storage/ \
		'^(TestLedgerRestoreCrashRecoveryConvergesAtEveryOperation|TestUndoConcurrentCommitSerializesLedgerRestore|TestCommitResultTransactionIDSharedByCommitMulti|TestMutationCallsiteInventoryBaseline|TestRawProjectSaveCallsitesAreAllowlisted)$' \
		TestLedgerRestoreCrashRecoveryConvergesAtEveryOperation \
		TestUndoConcurrentCommitSerializesLedgerRestore \
		TestCommitResultTransactionIDSharedByCommitMulti \
		TestMutationCallsiteInventoryBaseline \
		TestRawProjectSaveCallsitesAreAllowlisted
	run_go_suite compat-inventory ./internal/compat/ \
		'^(TestCompatWritePathInventoryCoversEveryDurableWrite|TestCompatCommitEntryPointsAreDocumented|TestCompatContractDocumentsAreWellFormed)$' \
		TestCompatWritePathInventoryCoversEveryDurableWrite \
		TestCompatCommitEntryPointsAreDocumented \
		TestCompatContractDocumentsAreWellFormed

	note "session 4 verified"
}

verify_session_5() {
	require_files \
		internal/synclog/graph.go \
		internal/synclog/reconcile.go \
		internal/storage/project_reconcile.go \
		internal/config/sync_reconcile.go \
		internal/syncmerge/merger.go \
		internal/syncmerge/reconcile_test.go \
		internal/syncmerge/seeded_test.go

	# Deterministic replicas: devices and genesis observers receiving the same
	# transactions in forward, reversed, and shuffled orders must hold identical
	# valid content and identical conflict records, with every competing
	# version recoverable. Each required scenario is its own named test.
	local required=(
		TestSyncReconcileConvergesAcrossDeliveryOrders
		TestSyncReconcileConvergesAcrossDeliveryOrders/independent_edits
		TestSyncReconcileConvergesAcrossDeliveryOrders/competing_edits
		TestSyncReconcilePlanningOrder
		TestSyncReconcilePlanningOrder/incompatible_reorders_keep_base
		TestSyncReconcileDeleteEdit
		TestSyncReconcileDeleteEdit/task
		TestSyncReconcileDeleteEdit/project
		TestSyncReconcileDeleteEdit/project_uncontested
		TestSyncReconcileProjectMoves
		TestSyncReconcileProjectMoves/cross_project_task_move
		TestSyncReconcileHierarchyInvariants
		TestSyncReconcileHierarchyInvariants/cycle
		TestSyncReconcileHierarchyInvariants/depth
		TestSyncReconcileHierarchyInvariants/deleted_parent_gains_child
		TestSyncReconcileShortIDAllocation
		TestSyncReconcileMultipleCommonAncestors
		TestSyncReconcileSimultaneousResolutions
		TestSyncReconcileSimultaneousResolutions/agree
		TestSyncReconcileSimultaneousResolutions/keep_current
		TestSyncReconcileSimultaneousResolutions/disagree
		TestSyncReconcileSeededHistoriesConverge
	)
	local seed
	for seed in 1 2 3 4; do
		required+=("TestSyncReconcileSeededHistoriesConverge/seed_$seed")
	done
	run_go_suite reconcile ./internal/syncmerge/ '^TestSyncReconcile' "${required[@]}"

	# The seeded histories must actually be concurrent: at least one must merge
	# several heads and at least one must leave a conflict to agree on.
	grep -Eq 'transactions, [1-9][0-9]* merging several heads' "$work_dir/reconcile.log" ||
		fail "reconcile: no seeded history merged concurrent heads"
	grep -Eq 'merging several heads, [1-9][0-9]* open conflicts' "$work_dir/reconcile.log" ||
		fail "reconcile: no seeded history produced a conflict"

	# The merge rules underneath, and the fast-forward and refusal guarantees
	# session 4 established, unchanged by reconciliation.
	run_go_suite merge-units ./internal/storage/ '^(TestReconcileProjectDocuments|TestMerge_)' \
		TestReconcileProjectDocumentsAgreesWithTwoWayMerge \
		TestReconcileProjectDocumentsAlwaysYieldsValidDocuments
	run_go_suite config-reconcile ./internal/config/ '^TestReconcile(Catalog|Planning)' \
		TestReconcileCatalogArtifactsKeepsSlugsUnique \
		TestReconcileCatalogArtifactsKeepsChangedProjectOverDeletion \
		TestReconcilePlanningArtifactsMergesMembership
	run_go_suite storage-sync ./internal/storage/ '^TestSync' \
		TestSyncIncomingApplyAndReceiptAreAtomic \
		TestSyncIncomingRefusalsAcknowledgeNothing \
		TestSyncIncomingKeepsCausalOrderAndConcurrentHeads
	run_go_suite engine-sync ./internal/engine/ '^TestRuntimeRecordsSyncHistoryAndAppliesItElsewhere$' \
		TestRuntimeRecordsSyncHistoryAndAppliesItElsewhere

	note "session 5 verified"
}

verify_session_6() {
	require_files \
		internal/synclog/observe.go \
		internal/storage/commit_sync.go \
		internal/config/sync_validate.go \
		internal/syncmerge/merger.go \
		internal/syncmerge/rich_test.go

	# Every rich artifact round-trips through sync to every device and to
	# observers that start from genesis in several delivery orders; valid
	# external edits are imported as changes and invalid ones are held intact
	# and unpublished; undo and redo are device-local, revision-checked new
	# transactions that never restore over another device's work.
	local required=(
		TestSyncRichArtifactsRoundTrip
		TestSyncExternalEdits
		TestSyncUndo
	)
	local name
	for name in notes comments checklists dependencies lenses lenses_domain_save attachments \
		concurrent_rich_edits mobile_edit_preserves_unexposed_fields; do
		required+=("TestSyncRichArtifactsRoundTrip/$name")
	done
	for name in valid_markdown_edit_is_published invalid_markdown_edit_is_held invalid_domain_edits_are_held \
		external_files_are_published domain_save_is_observed_before_incoming runtime_open_imports_external_edit; do
		required+=("TestSyncExternalEdits/$name")
	done
	for name in undo_and_redo_publish_new_transactions remote_edit_refuses_stale_undo remote_edit_refuses_stale_redo \
		unrecorded_change_refuses_undo undo_history_is_device_local; do
		required+=("TestSyncUndo/$name")
	done
	run_go_suite rich ./internal/syncmerge/ '^TestSync(RichArtifactsRoundTrip|ExternalEdits|Undo)$' "${required[@]}"

	# The ledger's own revision checks and grouped atomicity, which the sync
	# undo rides on, unchanged.
	run_go_suite storage-ledger ./internal/storage/ \
		'^(TestLedgerRestore.*|TestWorkspaceLedger.*|TestUndo.*|TestRedo.*)$' \
		TestLedgerRestoreAtomicRevisionDrift \
		TestLedgerRestoreAtomicPreflightRejectsMissingBlob \
		TestLedgerRestoreCrashRecoveryConvergesAtEveryOperation \
		TestUndoConcurrentCommitSerializesLedgerRestore \
		TestUndoRedoConcurrentCallersSerializeStackMovement

	# Session 4 and 5 guarantees hold with indexing and observation in front of
	# every apply.
	run_go_suite storage-sync ./internal/storage/ '^TestSync' \
		TestSyncLocalCommitAndOutboxAreAtomic \
		TestSyncIncomingApplyAndReceiptAreAtomic \
		TestSyncIncomingRefusalsAcknowledgeNothing \
		TestSyncIncomingKeepsCausalOrderAndConcurrentHeads
	run_go_suite reconcile ./internal/syncmerge/ '^TestSyncReconcile(ConvergesAcrossDeliveryOrders|DeleteEdit|SimultaneousResolutions)$' \
		TestSyncReconcileConvergesAcrossDeliveryOrders \
		TestSyncReconcileDeleteEdit \
		TestSyncReconcileSimultaneousResolutions
	run_go_suite compat-inventory ./internal/compat/ \
		'^(TestCompatWritePathInventoryCoversEveryDurableWrite|TestCompatCommitEntryPointsAreDocumented|TestCompatContractDocumentsAreWellFormed|TestCompatExternalMarkdownEditIsObservable)$' \
		TestCompatWritePathInventoryCoversEveryDurableWrite \
		TestCompatCommitEntryPointsAreDocumented \
		TestCompatContractDocumentsAreWellFormed \
		TestCompatExternalMarkdownEditIsObservable

	note "session 6 verified"
}

# fault_stat_total LABEL STAT sums STAT over every seeded summary line the
# suite logged ("seed N: ... STAT=COUNT ...").
fault_stat_total() {
	local label=$1 stat=$2
	grep -E '^[[:space:]]+sync_fault_test\.go:[0-9]+: seed [0-9]+: ' "$work_dir/$label.log" |
		grep -Eo "(^| )$stat=[0-9]+" | cut -d= -f2 | awk '{ total += $1 } END { print total + 0 }'
}

verify_session_7() {
	require_files \
		internal/storage/export_test.go \
		internal/storage/sync_fault_harness_test.go \
		internal/storage/sync_fault_test.go \
		internal/synclog/reconcile.go

	# The deterministic multi-replica harness: each named fault on its own, the
	# seeded properties with every fault at once, exact replay of a seed, and a
	# failure that names its seed, a replay command, and its trace.
	local scenarios=(delayed_delivery duplicated_delivery reordered_delivery incomplete_delivery
		corrupted_delivery missing_ancestors prolonged_offline three_device_concurrency
		conflicting_resolutions ordinary_edit_keeps_conflict_open crash_restart)
	local required=(
		TestSyncFaultScenarios
		TestSyncFaultPropertiesConverge
		TestSyncFaultSeedsReplayDeterministically
		TestSyncFaultSeedsReplayDeterministically/run_1
		TestSyncFaultSeedsReplayDeterministically/run_2
		TestSyncFaultFailuresReportSeedAndTrace
	)
	local name seed
	for name in "${scenarios[@]}"; do
		required+=("TestSyncFaultScenarios/$name")
	done
	for seed in 1 2 3 4 5 6; do
		required+=("TestSyncFaultPropertiesConverge/seed_$seed")
	done
	run_go_suite fault ./internal/storage/ '^TestSyncFault' "${required[@]}"

	# The seeded runs must actually have been hostile: every fault fired and
	# every guarded outcome was reached somewhere across the seeds.
	local stat total
	for stat in delayed sent_duplicate applied_out_of_order refused_truncated refused_corrupted \
		blob_lagged blob_corrupt_reads waiting_for_blob waiting_for_ancestor crashes restarts \
		offline_toggles repeat_ignored live_user_writes preserved_in_conflict; do
		total=$(fault_stat_total fault "$stat")
		[ "$total" -gt 0 ] || fail "fault: no seeded run exercised $stat"
	done
	note "fault: seeded runs crashed $(fault_stat_total fault crashes) commits and preserved $(fault_stat_total fault live_user_writes) unresolved user writes"

	# A seed replays exactly in a fresh process: the same summary, transaction
	# for transaction and fault for fault.
	TT_SYNC_FAULT_SEED=3 run_go_suite fault-replay ./internal/storage/ '^TestSyncFaultPropertiesConverge$' \
		TestSyncFaultPropertiesConverge/seed_3
	local first replayed
	first=$(grep -Eo 'seed 3: .*' "$work_dir/fault.log")
	replayed=$(grep -Eo 'seed 3: .*' "$work_dir/fault-replay.log")
	[ -n "$first" ] || fail "fault: seed 3 logged no summary"
	[ "$first" = "$replayed" ] || fail "fault: replaying seed 3 diverged: '$first' vs '$replayed'"
	if grep -Fq -- '--- PASS: TestSyncFaultPropertiesConverge/seed_1 (' "$work_dir/fault-replay.log"; then
		fail "fault: TT_SYNC_FAULT_SEED did not select one seed"
	fi

	# Conflicts now stay open until a resolution; the reconciliation and rich
	# artifact suites hold their guarantees under that rule, and the commit
	# boundary's crash atomicity is unchanged by the shared crash seam.
	run_go_suite reconcile ./internal/syncmerge/ \
		'^TestSync(Reconcile(ConvergesAcrossDeliveryOrders|DeleteEdit|MultipleCommonAncestors|SimultaneousResolutions|SeededHistoriesConverge)|RichArtifactsRoundTrip|Undo)$' \
		TestSyncReconcileConvergesAcrossDeliveryOrders \
		TestSyncReconcileDeleteEdit \
		TestSyncReconcileMultipleCommonAncestors \
		TestSyncReconcileSimultaneousResolutions/agree \
		TestSyncReconcileSimultaneousResolutions/keep_current \
		TestSyncReconcileSimultaneousResolutions/disagree \
		TestSyncReconcileSeededHistoriesConverge \
		TestSyncRichArtifactsRoundTrip \
		TestSyncUndo
	run_go_suite storage-sync ./internal/storage/ '^TestSync(Local|Incoming)' \
		TestSyncLocalCommitAndOutboxAreAtomic \
		TestSyncIncomingApplyAndReceiptAreAtomic \
		TestSyncIncomingRefusalsAcknowledgeNothing \
		TestSyncIncomingKeepsCausalOrderAndConcurrentHeads
	require_crash_prefixes storage-sync TestSyncIncomingApplyAndReceiptAreAtomic 6
	run_go_suite compat-docs ./internal/compat/ '^TestCompatContractDocumentsAreWellFormed$' \
		TestCompatContractDocumentsAreWellFormed

	note "session 7 verified"
}

# run_swift_suite LABEL PACKAGE_DIR REQUIRED_TEST...
#
# Runs one SwiftPM test suite and holds it to the same evidence rules as
# run_go_suite: it must succeed, execute tests, skip none, fail none, and
# report every REQUIRED_TEST (Class/method) as passed.
run_swift_suite() {
	local label=$1 package=$2
	shift 2
	local log="$work_dir/$label.log"
	note "running $label: swift test${TT_SWIFT_FILTER:+ --filter $TT_SWIFT_FILTER} in $package"
	if ! (cd "$package" && swift test ${TT_SWIFT_FILTER:+--filter "$TT_SWIFT_FILTER"}) >"$log" 2>&1; then
		grep -E 'error:|failed' "$log" >&2 || tail -40 "$log" >&2
		fail "$label: swift test failed"
	fi
	if grep -Eq "Test Case .* skipped|with [1-9][0-9]* tests? skipped" "$log"; then
		grep -E 'skipped' "$log" >&2
		fail "$label: a required test was skipped"
	fi
	grep -Eq 'Executed [1-9][0-9]* tests?, with 0 failures' "$log" || fail "$label: no tests executed, or some failed"
	local test
	for test in "$@"; do
		grep -Fq -- "Test Case '-[${test%%/*} ${test##*/}]' passed" "$log" ||
			fail "$label: missing required passing test $test"
	done
	note "$label: $(grep -Ec "Test Case '.*' passed" "$log") passing tests"
}

verify_session_8() {
	[ "$(uname -s)" = Darwin ] || fail "session 8 runs the Apple transport suites and checks real iCloud evidence on macOS only"
	require_files \
		internal/syncexchange/exchange.go \
		internal/syncexchange/import.go \
		internal/bridge/sync_ops.go \
		apple/TTSync/Package.swift \
		apple/TTSync/Sources/TTSync/ExchangeTransport.swift \
		apple/TTSync/Sources/TTSync/CoordinatedFolderProvider.swift \
		apple/TTSync/Sources/TTSync/FolderAccess.swift \
		apple/TTSync/Sources/TTSync/TransferState.swift \
		apple/TTSync/Sources/TTSyncContract/ExchangeContract.swift \
		apple/TTSync/Sources/tt-sync-helper/main.swift \
		apple/TTSync/Tests/TTSyncTests/ExchangeContractTests.swift \
		ios/TTMobileDeviceTests/ICloudExchangeDeviceTests.swift \
		ios/scripts/record-icloud-exchange.sh \
		ios/scripts/icloud-exchange-fingerprint.sh

	# The engine half: the reserved sync ops are served, export lists every
	# file, and an import applies only verified content and reports missing
	# content as unavailable without touching the workspace.
	run_go_suite bridge-sync ./internal/bridge/ \
		'^(TestBridgeSyncExchangeImportsOnlyVerifiedContent|TestBridgeRefusesMalformedAndMisaddressedRequests)$' \
		TestBridgeSyncExchangeImportsOnlyVerifiedContent \
		TestBridgeSyncExchangeImportsOnlyVerifiedContent/manifest_without_content_waits \
		TestBridgeSyncExchangeImportsOnlyVerifiedContent/damaged_content_waits \
		TestBridgeSyncExchangeImportsOnlyVerifiedContent/verified_content_imports_once \
		TestBridgeSyncExchangeImportsOnlyVerifiedContent/misnamed_manifest_is_invalid \
		TestBridgeSyncExchangeImportsOnlyVerifiedContent/acknowledge_empties_the_outbox \
		TestBridgeSyncExchangeImportsOnlyVerifiedContent/missing_attachment_is_unavailable_not_deleted \
		TestBridgeRefusesMalformedAndMisaddressedRequests/reserved_op
	run_go_suite exchange-paths ./internal/syncexchange/ '^(TestManifestPaths|TestRequiredBlobs)' \
		TestManifestPathsRoundTripAndRefuseOtherNames \
		TestRequiredBlobsNamesEveryBaseAndNewDigestOnce

	# The Apple transport against the fake iCloud provider and a real
	# coordinated folder, each device a real bridgehost engine.
	local bridgehost="$work_dir/bridgehost"
	go build -trimpath -o "$bridgehost" ./mobile/bridgehost || fail "building bridgehost failed"
	local class=TTSyncTests.ExchangeContractTests
	TT_BRIDGEHOST="$bridgehost" run_swift_suite transport apple/TTSync \
		"$class/testContractRoundTripOverFakeICloud" \
		"$class/testAccountUnavailabilityKeepsLocalWorkAndQueues" \
		"$class/testExpiredFolderAccessIsRenewedOrReported" \
		"$class/testFailedTransfersRetryWithBackoff" \
		"$class/testRestartResumesTheDurableUploadQueue" \
		"$class/testMissingContentIsUnavailableAndNeverDeletes" \
		"$class/testForeignExchangeIsRefused" \
		"$class/testContractRoundTripOverACoordinatedFolder" \
		"$class/testBookmarkAccessFollowsAMoveAndReportsLoss" \
		"$class/testCoordinatedListingReportsNestedFilesAndNothingForAMissingFolder"

	# The same contract over real iCloud Drive between this Mac and a physical
	# iPhone, recorded by ios/scripts/record-icloud-exchange.sh and held to the
	# builds the current tree produces.
	local source_fp bridge_fp bridgehost_sha
	source_fp=$(ios/scripts/icloud-exchange-fingerprint.sh) || fail "fingerprinting the Swift sources failed"
	bridge_fp=$(ios/scripts/build-bridge.sh --fingerprint) || fail "fingerprinting the bridge failed"
	bridgehost_sha=$(shasum -a 256 "$bridgehost" | cut -d' ' -f1)
	require_files docs/mobile/evidence/session-8-icloud-exchange.json
	TT_ICLOUD_SOURCE_FINGERPRINT="$source_fp" TT_IOS_BRIDGE_FINGERPRINT="$bridge_fp" TT_BRIDGEHOST_SHA256="$bridgehost_sha" \
		run_go_suite icloud-evidence ./internal/syncexchange/ '^TestRecordedICloudExchangeEvidence$' \
		TestRecordedICloudExchangeEvidence
	note "real iCloud exchange: run $(grep -Eo '"run_id": "[^"]+"' docs/mobile/evidence/session-8-icloud-exchange.json | head -1)"

	note "session 8 verified"
}

# enrollment_old_binary_ref is the newest commit that predates the enrolled
# catalog schema: the parent of the commit that introduced it, or HEAD while
# that change is still uncommitted. Its tt reads every workspace this one mints
# without enrolling, so its refusal of a working copy proves the enrolled
# marker rather than some other schema bump.
enrollment_old_binary_ref() {
	local introduced
	introduced=$(git log -1 --format=%H -S'SyncEnrolledConfigVersion = ' -- internal/config/config_core.go 2>/dev/null || true)
	if [ -n "$introduced" ]; then
		git rev-parse "$introduced^"
	else
		git rev-parse HEAD
	fi
}

verify_session_9() {
	require_files \
		internal/config/sync_enroll.go \
		internal/config/sync_enroll_test.go \
		internal/engine/enroll.go \
		internal/engine/enroll_test.go \
		internal/synclog/held.go \
		internal/syncexchange/folder.go \
		internal/tui/conflict/sync_conflicts.go \
		cmd/sync.go \
		cmd/sync_test.go \
		scripts/cross_machine_gate.sh

	# Enrollment through the shared runtime: the snapshot and the local working
	# copy, the genesis and its publication, a device rebuilt from the published
	# history alone, restarts, interruptions at every engine step, external
	# edits, an incompatible peer that holds and names the upgrade, unsafe
	# placements, and workspaces never enrolled.
	local required=(
		TestSyncEnrollment
		TestSyncEnrollmentRefusesUnsafePlacement
	)
	local name
	for name in enrolls_from_a_snapshot_into_a_local_working_copy workspaces_never_enrolled_keep_their_behavior \
		a_second_device_joins_from_the_published_history restart_keeps_history_and_exchanges_new_work \
		an_interrupted_enrollment_resumes an_interrupted_enrollment_resumes/genesis an_interrupted_enrollment_resumes/exchange \
		external_edits_are_published_and_invalid_ones_held an_incompatible_peer_holds_and_names_the_upgrade; do
		required+=("TestSyncEnrollment/$name")
	done
	for name in working_copy_in_icloud working_copy_in_original exchange_in_working_copy exchange_in_original \
		working_copy_not_empty original_is_not_workspace; do
		required+=("TestSyncEnrollmentRefusesUnsafePlacement/$name")
	done
	run_go_suite enroll ./internal/engine/ '^TestSyncEnrollment' "${required[@]}"

	# The copy phase interrupted at each durable step, and the enrolled schema
	# and sync block held to each other.
	required=(TestSyncEnrollInterruptedMigration TestSyncEnrolledCatalogSchema)
	for name in snapshot staged published; do
		required+=("TestSyncEnrollInterruptedMigration/$name")
	done
	for name in current_without_sync enrolled_with_sync enrolled_without_sync sync_at_current_schema \
		unknown_exchange newer_than_supported; do
		required+=("TestSyncEnrolledCatalogSchema/$name")
	done
	run_go_suite enroll-config ./internal/config/ '^TestSyncEnroll' "${required[@]}"

	# The maintenance commands and the Outliner's status and conflict access.
	run_go_suite sync-cli ./cmd/ '^(TestSyncCommands|TestPublicCommandTreeIsMaintenanceOnly)$' \
		TestSyncCommands \
		TestSyncCommands/enroll_status_exchange_doctor \
		TestSyncCommands/a_workspace_never_enrolled_is_told_how \
		TestSyncCommands/enroll_refuses_unconfirmed_and_an_icloud_working_copy \
		TestPublicCommandTreeIsMaintenanceOnly
	run_go_suite sync-tui ./internal/tui/ '^(TestSyncStatusReachesTheStripAndTheConflictBrowser|TestSyncAdvisoryTextNamesEachStateApart)$' \
		TestSyncStatusReachesTheStripAndTheConflictBrowser \
		TestSyncAdvisoryTextNamesEachStateApart \
		TestSyncAdvisoryTextNamesEachStateApart/held
	run_go_suite sync-browser ./internal/tui/conflict/ '^TestConflictBrowser' \
		TestConflictBrowserListsSyncConflicts

	# Earlier guarantees hold with the genesis, the typed holds, and the
	# genesis-started history in place.
	run_go_suite storage-sync ./internal/storage/ '^TestSync(Local|Incoming)' \
		TestSyncLocalCommitAndOutboxAreAtomic \
		TestSyncIncomingRefusalsAcknowledgeNothing \
		TestSyncIncomingRefusalsAcknowledgeNothing/held_schema
	run_go_suite compat-inventory ./internal/compat/ \
		'^(TestCompatWritePathInventoryCoversEveryDurableWrite|TestCompatCommitEntryPointsAreDocumented|TestCompatContractDocumentsAreWellFormed)$' \
		TestCompatWritePathInventoryCoversEveryDurableWrite \
		TestCompatCommitEntryPointsAreDocumented \
		TestCompatContractDocumentsAreWellFormed

	# Real processes: the whole cross-machine gate with both older binaries
	# pinned, so no old-binary case may skip. Case 3 and case 6 use the
	# schema-19 binary; case 12 uses the newest tt from before enrollment and
	# must refuse the enrolled working copy byte-identically.
	local enroll_ref
	enroll_ref=$(enrollment_old_binary_ref) || fail "resolving the pre-enrollment ref failed"
	local gate_log="$work_dir/cross-machine.log"
	note "running the cross-machine gate (old binary 08c0cb08, pre-enrollment binary $enroll_ref)"
	if ! TT_OLD_BINARY_REF=08c0cb08fec10154e6d2c9455e32d4c4ffa5f72d TT_ENROLL_OLD_BINARY_REF="$enroll_ref" \
		sh scripts/cross_machine_gate.sh >"$gate_log" 2>&1; then
		cat "$gate_log" >&2
		fail "the cross-machine gate failed"
	fi
	if grep -q '^SKIP:' "$gate_log"; then
		grep '^SKIP:' "$gate_log" >&2
		fail "the cross-machine gate skipped a required case"
	fi
	grep -q '^cross-machine gate passed\.$' "$gate_log" || fail "the cross-machine gate did not report a clean pass"
	local expected
	for expected in \
		"refuses a newer catalog and leaves it byte-identical" \
		"migrates, carries a resume note and project comments" \
		"a tt from before enrollment refuses the enrolled working copy byte-identically and still reads the original"; do
		grep -Fq "PASS: " "$gate_log" && grep -F "PASS: " "$gate_log" | grep -Fq "$expected" ||
			fail "cross-machine gate: missing required pass: $expected"
	done
	note "cross-machine gate: $(grep -c '^PASS:' "$gate_log") cases passed"

	note "session 9 verified"
}

verify_session_10() {
	[ "$(uname -s)" = Darwin ] || fail "session 10 runs the iPhone browsing UI tests on an iOS simulator, on macOS only"
	require_files \
		internal/bridge/browse_ops.go \
		internal/bridge/browse_test.go \
		apple/TTSync/Sources/TTSync/JoiningSyncEngine.swift \
		apple/TTSync/Tests/TTSyncTests/JoinTests.swift \
		ios/scripts/test-ui.sh \
		ios/TTMobile/App/OnboardingView.swift \
		ios/TTMobile/App/MainTabView.swift \
		ios/TTMobile/App/BrowseView.swift \
		ios/TTMobile/App/TaskListView.swift \
		ios/TTMobile/App/TaskDetailView.swift \
		ios/TTMobile/App/StateViews.swift \
		ios/TTMobileUITests/BrowsingUITests.swift
	grep -q 'TTMobileUITests' ios/project.yml || fail "ios/project.yml declares no UI test target"

	# The engine half: every mode, lenses, the search box and its refusal,
	# sorting, hierarchy, dependencies and an unavailable reference, and the
	# onboarding join from a staged inbox.
	run_go_suite bridge-browse ./internal/bridge/ \
		'^(TestBridgeBrowsesASeededWorkspace|TestBridgeNamesAnUnavailableDependency|TestBridgeJoinsFromAStagedInbox)$' \
		TestBridgeBrowsesASeededWorkspace/modes \
		TestBridgeBrowsesASeededWorkspace/sort \
		TestBridgeBrowsesASeededWorkspace/lenses_listed \
		TestBridgeBrowsesASeededWorkspace/lens_reopens_its_query \
		TestBridgeBrowsesASeededWorkspace/lens_narrowed_by_search \
		TestBridgeBrowsesASeededWorkspace/lens_owns_its_axes \
		TestBridgeBrowsesASeededWorkspace/search_text \
		TestBridgeBrowsesASeededWorkspace/search_sigils \
		TestBridgeBrowsesASeededWorkspace/search_refuses_by_name \
		TestBridgeBrowsesASeededWorkspace/hierarchy \
		TestBridgeBrowsesASeededWorkspace/dependencies \
		TestBridgeNamesAnUnavailableDependency \
		TestBridgeJoinsFromAStagedInbox/publishing_device_is_refused \
		TestBridgeJoinsFromAStagedInbox/empty_inbox_delivers_nothing \
		TestBridgeJoinsFromAStagedInbox/joined_copy_browses \
		TestBridgeJoinsFromAStagedInbox/joined_copy_is_not_joined_twice

	# The iPhone shell on a simulator against the gomobile-bound engine: seeded
	# workspaces browsed offline, onboarding (create and join), workspace
	# access, sync status, and the loading, empty, unavailable, and error
	# states.
	note "running the TTMobile UI suite"
	ios/scripts/test-ui.sh "$work_dir/ui" >"$work_dir/test-ui.log" 2>&1 || {
		cat "$work_dir/test-ui.log" >&2
		fail "ios/scripts/test-ui.sh failed"
	}
	require_xcresult_passed "$work_dir/ui/TTMobileUITests.xcresult" \
		'testSeededTodayInboxAndBrowseModes()' \
		'testProjectsOutlineAndRelations()' \
		'testLensesSearchFiltersAndSorting()' \
		'testOpeningShowsLoadingThenContent()' \
		'testOnboardingStartsAnEmptyWorkspaceAndCaptures()' \
		'testUnavailableWorkspaceAndReference()' \
		'testErrorStates()' \
		'testJoinFromAnExchangeFolder()'

	# The join through the shared Apple transport, each device a bridgehost
	# engine, over the exchange the UI suite's desktop enrollment published.
	local bridgehost="$work_dir/bridgehost"
	go build -trimpath -o "$bridgehost" ./mobile/bridgehost || fail "building bridgehost failed"
	require_files "$work_dir/ui/exchange/tt-sync/v1/workspace.json"
	local class=TTSyncTests.JoinTests
	TT_BRIDGEHOST="$bridgehost" TT_ENROLLED_EXCHANGE="$work_dir/ui/exchange" run_swift_suite transport-join apple/TTSync \
		"$class/testJoinBuildsAWorkingCopyFromTheExchange" \
		"$class/testJoinRefusesAFolderWithoutAnExchange" \
		"$class/testFailedJoinLeavesNoWorkingCopy"

	note "session 10 verified"
}

verify_session_11() {
	[ "$(uname -s)" = Darwin ] || fail "session 11 runs the iPhone capture and editing UI tests on an iOS simulator, on macOS only"
	require_files \
		internal/capturetext/parse.go \
		internal/capturetext/requests.go \
		internal/bridge/capture_ops.go \
		internal/bridge/capture_edit_test.go \
		internal/mobileparity/parity_test.go \
		ios/TTMobile/App/CaptureView.swift \
		ios/TTMobile/App/TaskEditView.swift \
		ios/TTMobile/App/DraftStore.swift \
		ios/TTMobileUITests/CaptureEditingUITests.swift

	# The capture grammar now reaches the phone: the bound package must link
	# the parser without the terminal stack it moved out of.
	local frontend
	frontend=$(go list -deps ./mobile/ttbridge | grep -E '^(github.com/sglavoie/tt/internal/tui|charm.land/|github.com/charmbracelet/)' || true)
	[ -z "$frontend" ] || fail "the mobile binding links frontend packages: $frontend"
	go list -deps ./mobile/ttbridge | grep -qx 'github.com/sglavoie/tt/internal/capturetext' ||
		fail "the mobile binding does not link the shared capture parser"

	# Desktop and phone over two copies of one workspace: the composer's
	# reading, the inline editor's grammar, the resume prompt, and the workflow
	# chord against the bridge's capture, edit, plan, and transition ops. Each
	# scenario must leave byte-identical task Markdown and planning state, and
	# each refusal must refuse on both sides and write nothing.
	run_go_suite capture-parity ./internal/mobileparity/ '^TestDesktopAndMobileCaptureAndEditAlike$' \
		TestDesktopAndMobileCaptureAndEditAlike/shorthand_capture \
		TestDesktopAndMobileCaptureAndEditAlike/shorthand_preview_matches_the_capture \
		TestDesktopAndMobileCaptureAndEditAlike/shorthand_refusal_names_the_line_and_commits_nothing \
		TestDesktopAndMobileCaptureAndEditAlike/structured_capture \
		TestDesktopAndMobileCaptureAndEditAlike/daily_edit \
		TestDesktopAndMobileCaptureAndEditAlike/deferral_routes_to_backlog \
		TestDesktopAndMobileCaptureAndEditAlike/today_next_exclusivity \
		TestDesktopAndMobileCaptureAndEditAlike/completion_clears_planning \
		TestDesktopAndMobileCaptureAndEditAlike/lifecycle_refusals

	# The bridge contract of those ops: envelopes, refusal details, preview,
	# replay, planning membership on the task read, and the device time zone
	# the engine reads due dates and Today in.
	run_go_suite bridge-capture ./internal/bridge/ '^(TestBridgeCapturesAndEditsThroughTheSharedEngine|TestBridgeAdoptsTheDeviceTimeZone)$' \
		TestBridgeAdoptsTheDeviceTimeZone \
		TestBridgeCapturesAndEditsThroughTheSharedEngine/grammar_capture_is_one_transaction \
		TestBridgeCapturesAndEditsThroughTheSharedEngine/grammar_refusal_names_every_line \
		TestBridgeCapturesAndEditsThroughTheSharedEngine/text_stands_alone \
		TestBridgeCapturesAndEditsThroughTheSharedEngine/structured_capture_keeps_a_literal_title \
		TestBridgeCapturesAndEditsThroughTheSharedEngine/preview_reads_without_writing \
		TestBridgeCapturesAndEditsThroughTheSharedEngine/edit_applies_named_fields_once \
		TestBridgeCapturesAndEditsThroughTheSharedEngine/plan_keeps_today_and_next_exclusive \
		TestBridgeCapturesAndEditsThroughTheSharedEngine/lifecycle_refusals_explain_themselves

	# The shared engine's half: the resume note rides capture and the line
	# edit through the one gate, and the desktop composer still reads text
	# through the request builder the bridge now shares.
	run_go_suite app-resume ./internal/app/ '^TestResumeNoteRidesCaptureAndTheLineEdit$' \
		TestResumeNoteRidesCaptureAndTheLineEdit
	run_go_suite composer-capture ./internal/tui/taskform/ '^TestParseCaptureTextReadsLinesAsTheComposerDoes$' \
		TestParseCaptureTextReadsLinesAsTheComposerDoes
	run_go_suite capture-grammar ./internal/capturetext/ '^(TestParseDate|TestResolveParent|TestResolveProject|TestStructuralReferencesShareMatchingAndNameTheirRelationship)$' \
		TestParseDate TestResolveParent TestResolveProject TestStructuralReferencesShareMatchingAndNameTheirRelationship

	# The iPhone on a simulator against the gomobile-bound engine: shorthand
	# capture with its preview, a structured capture, the structured edit, plan
	# and status, explained refusals, and drafts kept across a refused save and
	# a killed app, each checked against the seeded workspace's files.
	note "running the TTMobile capture and editing UI suite"
	TT_UITEST_ONLY=CaptureEditingUITests ios/scripts/test-ui.sh "$work_dir/ui" >"$work_dir/test-ui.log" 2>&1 || {
		cat "$work_dir/test-ui.log" >&2
		fail "ios/scripts/test-ui.sh failed"
	}
	require_xcresult_passed "$work_dir/ui/TTMobileUITests.xcresult" \
		'testShorthandCaptureWithLivePreview()' \
		'testRefusedCaptureKeepsItsDraftAcrossARelaunch()' \
		'testStructuredCaptureCarriesEveryField()' \
		'testDailyEditSavesOnceAndKeepsARefusedDraft()' \
		'testPlanStatusAndLifecycleRefusals()'

	note "session 11 verified"
}

verify_session_12() {
	[ "$(uname -s)" = Darwin ] || fail "session 12 runs the iPhone organizing UI tests on an iOS simulator, on macOS only"
	require_files \
		internal/engine/projects.go \
		internal/bridge/organize_ops.go \
		internal/bridge/organize_test.go \
		internal/bridge/organize_sync_test.go \
		internal/mobileparity/organize_parity_test.go \
		ios/TTMobile/App/OrganizeSections.swift \
		ios/TTMobile/App/ProjectSettingsView.swift \
		ios/TTMobileUITests/OrganizingUITests.swift

	# The desktop's project hub commits through the engine's one project
	# lifecycle, the same functions the bridge serves.
	local delegate
	for delegate in \
		'engine.CreateProject(:internal/tui/projecthub/project_creator.go' \
		'engine.EditProject(:internal/tui/projecthub/project_editor.go' \
		'engine.DeleteProject(:internal/tui/handler_project.go'; do
		grep -Fq "${delegate%%:*}" "${delegate#*:}" || fail "${delegate#*:} does not commit through ${delegate%%:*}"
	done

	# One device: checklists and their item identity, promotion, a repeating
	# check-in and its acknowledgement, subtasks and the depth limit, blocked
	# parent completion, moves carrying references, selection actions, the
	# project lifecycle, and the confirmed delete.
	run_go_suite bridge-organize ./internal/bridge/ '^TestBridgeOrganizesThroughTheSharedEngine$' \
		TestBridgeOrganizesThroughTheSharedEngine/checklist_lifecycle \
		TestBridgeOrganizesThroughTheSharedEngine/checklist_stale_selection_is_refused \
		TestBridgeOrganizesThroughTheSharedEngine/checklist_promotion \
		TestBridgeOrganizesThroughTheSharedEngine/recurring_check_in \
		TestBridgeOrganizesThroughTheSharedEngine/subtasks_and_hierarchy \
		TestBridgeOrganizesThroughTheSharedEngine/hierarchy_limit \
		TestBridgeOrganizesThroughTheSharedEngine/blocked_parent_completion \
		TestBridgeOrganizesThroughTheSharedEngine/move_keeps_references \
		TestBridgeOrganizesThroughTheSharedEngine/bulk_actions \
		TestBridgeOrganizesThroughTheSharedEngine/project_lifecycle \
		TestBridgeOrganizesThroughTheSharedEngine/guarded_project_delete

	# Two devices through the exchange, the phone over the bridge and the
	# desktop through its own application calls: concurrent checklist edits,
	# a recurring check-in acknowledged on either side, a move against a
	# concurrent reference, a merge that would pass the depth limit, a parent
	# finished while the other device added a subtask, and a delete confirmed
	# before the other device's work arrived. Both copies must converge.
	run_go_suite bridge-organize-sync ./internal/bridge/ '^TestBridgeOrganizingConvergesAcrossDevices$' \
		TestBridgeOrganizingConvergesAcrossDevices/concurrent_checklist_edits \
		TestBridgeOrganizingConvergesAcrossDevices/recurring_check_in \
		TestBridgeOrganizingConvergesAcrossDevices/move_related_references \
		TestBridgeOrganizingConvergesAcrossDevices/hierarchy_limit \
		TestBridgeOrganizingConvergesAcrossDevices/blocked_parent_completion \
		TestBridgeOrganizingConvergesAcrossDevices/destructive_confirmation

	# Desktop and phone over two copies: the same acts leave the same task
	# Markdown, planning, and catalog.
	run_go_suite organize-parity ./internal/mobileparity/ '^TestDesktopAndMobileOrganizeAlike$' \
		TestDesktopAndMobileOrganizeAlike/checklist \
		TestDesktopAndMobileOrganizeAlike/check_in \
		TestDesktopAndMobileOrganizeAlike/hierarchy \
		TestDesktopAndMobileOrganizeAlike/moves \
		TestDesktopAndMobileOrganizeAlike/bulk \
		TestDesktopAndMobileOrganizeAlike/projects

	# The engine rules the ops lean on: the frozen-checklist promotion guard
	# and its depth limit, and the desktop project hub over the shared
	# lifecycle.
	run_go_suite app-promotion ./internal/app/ '^TestPromoteChecklistItem' \
		TestPromoteChecklistItemRefusesChangedChecklist \
		TestPromoteChecklistItemRefusesAtMaximumDepth \
		TestPromoteChecklistItemRefusesUnreachableOwner
	run_go_suite project-hub ./internal/tui/projecthub/ '.'

	# The iPhone on a simulator against the gomobile-bound engine, each
	# outcome read back from the seeded workspace's files.
	note "running the TTMobile organizing UI suite"
	TT_UITEST_ONLY=OrganizingUITests ios/scripts/test-ui.sh "$work_dir/ui" >"$work_dir/test-ui.log" 2>&1 || {
		cat "$work_dir/test-ui.log" >&2
		fail "ios/scripts/test-ui.sh failed"
	}
	require_xcresult_passed "$work_dir/ui/TTMobileUITests.xcresult" \
		'testChecklistEditingAndPromotion()' \
		'testRecurringCheckInScheduleAndAcknowledge()' \
		'testSubtasksHierarchyLimitAndBlockedCompletion()' \
		'testMoveToProjectCarriesItsReferences()' \
		'testBulkActionsOverASelection()' \
		'testProjectLifecycleAndConfirmedDelete()'

	note "session 12 verified"
}

verify_session_13() {
	[ "$(uname -s)" = Darwin ] || fail "session 13 runs the iPhone rich-content UI tests on an iOS simulator, on macOS only"
	require_files \
		internal/bridge/rich_ops.go \
		internal/bridge/rich_test.go \
		internal/bridge/rich_sync_test.go \
		internal/storage/project_merge_conflict_value_test.go \
		ios/TTEngine/Sources/TTEngine/RichContent.swift \
		ios/TTMobile/App/AttachmentImporter.swift \
		ios/TTMobile/App/RichSections.swift \
		ios/TTMobile/App/NotesViews.swift \
		ios/TTMobileUITests/RichContentUITests.swift

	# The rich-content ops are served, not reserved.
	local served
	for served in query.notes query.note command.comment command.note command.attachment; do
		grep -Fq "\"$served\":" internal/bridge/bridge.go || fail "the bridge does not serve $served"
	done

	# One device: comments addressed by what was read, notes guarded by their
	# revision, attachments staged under runtime_dir and committed with their
	# metadata, with no temporary path recorded, a truncated or vanished
	# staging copy refused, a resumed import answering present, unavailable
	# bytes reported, and everything surviving a termination and reopen.
	run_go_suite bridge-rich ./internal/bridge/ '^TestBridgeKeepsRichContentThroughTheSharedEngine$' \
		TestBridgeKeepsRichContentThroughTheSharedEngine/task_comments \
		TestBridgeKeepsRichContentThroughTheSharedEngine/project_comments \
		TestBridgeKeepsRichContentThroughTheSharedEngine/notes_lifecycle \
		TestBridgeKeepsRichContentThroughTheSharedEngine/stale_note_is_refused \
		TestBridgeKeepsRichContentThroughTheSharedEngine/attachment_import \
		TestBridgeKeepsRichContentThroughTheSharedEngine/attachment_kinds_and_removal \
		TestBridgeKeepsRichContentThroughTheSharedEngine/unavailable_attachment \
		TestBridgeKeepsRichContentThroughTheSharedEngine/interrupted_import \
		TestBridgeKeepsRichContentThroughTheSharedEngine/termination_and_reopen

	# Two devices through the exchange: offline work on both sides crosses
	# byte for byte, concurrent edits of one note, one comment, or one
	# attachment name keep every side's bytes recoverable on both devices, and
	# an attachment whose bytes have not arrived applies nothing.
	run_go_suite bridge-rich-sync ./internal/bridge/ '^TestBridgeRichContentConvergesAcrossDevices$' \
		TestBridgeRichContentConvergesAcrossDevices/offline_edits_cross_byte_identical \
		TestBridgeRichContentConvergesAcrossDevices/conflicting_note_edits \
		TestBridgeRichContentConvergesAcrossDevices/conflicting_comment_edits \
		TestBridgeRichContentConvergesAcrossDevices/conflicting_attachment_imports \
		TestBridgeRichContentConvergesAcrossDevices/attachment_unavailable_until_its_bytes_arrive

	# The merge records the same conflict on every device: a comment conflict
	# spells values, never addresses.
	run_go_suite merge-conflict-text ./internal/storage/ '^TestMergeConflictsSpellValuesNotAddresses$' \
		TestMergeConflictsSpellValuesNotAddresses
	run_go_suite app-import ./internal/app/ '^TestImportAttachment' \
		TestImportAttachmentRecordsNoTemporaryPath \
		TestImportAttachmentCommitsNothingWhenRefused

	# The transport's explicit download asks for exactly the content waited
	# on and retries a backed-off transfer now.
	local bridgehost="$work_dir/bridgehost"
	go build -trimpath -o "$bridgehost" ./mobile/bridgehost || fail "building bridgehost failed"
	TT_BRIDGEHOST="$bridgehost" TT_SWIFT_FILTER='ExchangeContractTests/testExplicitDownloadRequestsUnavailableContentNow' \
		run_swift_suite transport-download apple/TTSync \
		TTSyncTests.ExchangeContractTests/testExplicitDownloadRequestsUnavailableContentNow

	# The iPhone on a simulator against the gomobile-bound engine, offline,
	# each outcome read back from the seeded workspace's files.
	note "running the TTMobile rich-content UI suite"
	TT_UITEST_ONLY=RichContentUITests ios/scripts/test-ui.sh "$work_dir/ui" >"$work_dir/test-ui.log" 2>&1 || {
		cat "$work_dir/test-ui.log" >&2
		fail "ios/scripts/test-ui.sh failed"
	}
	require_xcresult_passed "$work_dir/ui/TTMobileUITests.xcresult" \
		'testTaskCommentsEditRefuseAndKeepADraftAcrossARelaunch()' \
		'testNotesCreateEditMoveAndKeepAStaleEditAsACopy()' \
		'testAttachmentImportResumesAfterTerminationAndShares()' \
		'testAttachmentKindsReportTheirAvailability()' \
		'testProjectCommentsAndProjectNotes()'

	note "session 13 verified"
}

verify_session_14() {
	[ "$(uname -s)" = Darwin ] || fail "session 14 runs the iPhone conflict and recovery UI tests on an iOS simulator, on macOS only"
	require_files \
		internal/engine/conflicts.go \
		internal/bridge/conflict_ops.go \
		internal/bridge/conflict_sync_test.go \
		cmd/sync_conflicts.go \
		cmd/sync_conflicts_test.go \
		ios/TTEngine/Sources/TTEngine/Conflicts.swift \
		ios/TTMobile/App/ConflictViews.swift \
		ios/TTMobileUITests/ConflictRecoveryUITests.swift

	# The conflict ops are served, not reserved.
	local served
	for served in conflict.list conflict.show conflict.resolve sync.diagnostics; do
		grep -Fq "\"$served\":" internal/bridge/bridge.go || fail "the bridge does not serve $served"
	done

	# One engine behind both interfaces: the phone through the bridge and the
	# desktop through the call tt sync resolve makes. Base and competing
	# versions with device and change context; keeping a version, the current
	# bytes, or replacement text; coupled changes resolved in one transaction;
	# convergence once the resolution travels; a stale resolution refused
	# without writing; a resolution racing an edit or another resolution
	# keeping every variant; local content during a transport failure and a
	# retry; and a content-free diagnostics export.
	run_go_suite bridge-conflicts ./internal/bridge/ '^TestBridgeResolvesSyncConflictsFromEitherDevice$' \
		TestBridgeResolvesSyncConflictsFromEitherDevice/inspect_base_and_competing_versions \
		TestBridgeResolvesSyncConflictsFromEitherDevice/resolve_from_phone_converges \
		TestBridgeResolvesSyncConflictsFromEitherDevice/resolve_from_desktop_converges \
		TestBridgeResolvesSyncConflictsFromEitherDevice/replacement_text \
		TestBridgeResolvesSyncConflictsFromEitherDevice/coupled_changes_resolve_together \
		TestBridgeResolvesSyncConflictsFromEitherDevice/stale_resolution_is_refused \
		TestBridgeResolvesSyncConflictsFromEitherDevice/resolution_racing_an_edit_keeps_both \
		TestBridgeResolvesSyncConflictsFromEitherDevice/competing_resolutions_keep_both \
		TestBridgeResolvesSyncConflictsFromEitherDevice/transport_failure_keeps_local_content \
		TestBridgeResolvesSyncConflictsFromEitherDevice/diagnostics_export

	# The desktop's commands, between two real desktop devices sharing an
	# exchange folder, and the read-only gc browser over the same views.
	run_go_suite cli-conflicts ./cmd/ '^(TestSyncConflictCommands|TestPublicCommandTreeIsMaintenanceOnly)$' \
		TestSyncConflictCommands \
		TestPublicCommandTreeIsMaintenanceOnly
	run_go_suite tui-conflicts ./internal/tui/ '^TestSyncStatusReachesTheStripAndTheConflictBrowser$' \
		TestSyncStatusReachesTheStripAndTheConflictBrowser
	run_go_suite tui-conflict-browser ./internal/tui/conflict/ '^TestConflictBrowserListsSyncConflicts$' \
		TestConflictBrowserListsSyncConflicts
	# Simultaneous resolutions stay convergent, and the new commit entry point
	# is inventoried.
	run_go_suite simultaneous-resolutions ./internal/syncmerge/ '^TestSyncReconcileSimultaneousResolutions$' \
		TestSyncReconcileSimultaneousResolutions/agree \
		TestSyncReconcileSimultaneousResolutions/keep_current
	run_go_suite write-inventory ./internal/compat/ '^(TestCompatCommitEntryPointsAreDocumented|TestCompatWritePathInventoryCoversEveryDurableWrite)$' \
		TestCompatCommitEntryPointsAreDocumented \
		TestCompatWritePathInventoryCoversEveryDurableWrite

	# The iPhone on a simulator against the gomobile-bound engine: it joins
	# an exchange where two Macs left a coupled conflict, inspects and resolves
	# it, writes replacement text, survives a stale resolution, and keeps
	# working through a lost exchange folder with a retry and a diagnostics
	# export.
	note "running the TTMobile conflict and recovery UI suite"
	TT_UITEST_ONLY=ConflictRecoveryUITests ios/scripts/test-ui.sh "$work_dir/ui" >"$work_dir/test-ui.log" 2>&1 || {
		cat "$work_dir/test-ui.log" >&2
		fail "ios/scripts/test-ui.sh failed"
	}
	require_xcresult_passed "$work_dir/ui/TTMobileUITests.xcresult" \
		'testKeepingAVersionResolvesTheWholeChangeAndPublishesIt()' \
		'testReplacementTextAndAStaleResolutionKeepEveryEdit()' \
		'testTransportFailureKeepsLocalContentRetriesAndExportsDiagnostics()'

	# Every replica converges on the phone's resolution: both Macs apply what
	# the phone published and end with its bytes and nothing open.
	local phone_root conflict="$work_dir/ui/conflict" tt="$work_dir/ui/enroll/tt" who path
	phone_root=$(find "$work_dir/ui/work" -maxdepth 1 -type d -name '*testKeepingAVersion*' | head -1)
	[ -n "$phone_root" ] && [ -d "$phone_root/exchange" ] || fail "the phone's resolution test left no exchange"
	for who in studio laptop; do
		HOME="$conflict/$who-home" "$tt" --data-dir "$conflict/$who" sync exchange --folder "$phone_root/exchange" \
			>"$work_dir/$who-exchange.log" 2>&1 || {
			cat "$work_dir/$who-exchange.log" >&2
			fail "the $who Mac could not apply the phone's resolution"
		}
		HOME="$conflict/$who-home" "$tt" --data-dir "$conflict/$who" sync conflicts --json >"$work_dir/$who-conflicts.json" ||
			fail "the $who Mac could not list its conflicts"
		python3 -I -c 'import json, sys; sys.exit(0 if json.load(open(sys.argv[1]))["conflicts"] == [] else 1)' \
			"$work_dir/$who-conflicts.json" || {
			cat "$work_dir/$who-conflicts.json" >&2
			fail "the $who Mac still has open conflicts after the phone's resolution"
		}
		for path in projects/atlas-launch/tasks.md projects/atlas-launch/notes/2026-09-03-090000_0a1b2c3d-release-runbook.md; do
			cmp -s "$conflict/$who/$path" "$phone_root/workspaces/joined/$path" ||
				fail "the $who Mac's $path differs from the phone's after the resolution"
		done
	done
	grep -Fq '## Task: Ship beta build (laptop)' "$conflict/studio/projects/atlas-launch/tasks.md" ||
		fail "the studio Mac does not hold the version the phone kept"
	note "both Macs converged on the phone's resolution"

	note "session 14 verified"
}

session=${1:-}
verify_session_15() {
	[ "$(uname -s)" = Darwin ] || fail "session 15 runs the reminder scheduling suite and checks real iPhone evidence on macOS only"
	require_files \
		internal/engine/reminders.go \
		internal/bridge/reminder_ops.go \
		internal/bridge/reminders_test.go \
		internal/bridge/reminder_evidence_test.go \
		ios/TTEngine/Sources/TTEngine/Reminders.swift \
		ios/TTReminders/Package.swift \
		ios/TTReminders/Sources/TTReminders/ReminderScheduler.swift \
		ios/TTReminders/Sources/TTReminders/ReminderReconciler.swift \
		ios/TTReminders/Sources/TTReminders/ReminderActions.swift \
		ios/TTReminders/Sources/TTReminders/SystemNotificationCenter.swift \
		ios/TTReminders/Tests/TTRemindersTests/FakeNotificationCenter.swift \
		ios/TTMobile/App/ReminderModel.swift \
		ios/TTMobile/App/ReminderSettingsView.swift \
		ios/TTMobileUITests/ReminderDeliveryUITests.swift \
		ios/scripts/record-reminder-evidence.sh \
		ios/scripts/reminder-evidence-fingerprint.sh
	grep -Fq '"query.reminders":' internal/bridge/bridge.go || fail "the bridge does not serve query.reminders"

	# The engine plans under the shared attention rules: which tasks raise a
	# reminder and when, wall-clock reminders replanned in a new zone while
	# deferrals keep their instant, the actions' commands reflected in the
	# next plan, another device's changes once imported, parked and archived
	# projects silent, and the bounds and refusals.
	run_go_suite bridge-reminders ./internal/bridge/ '^TestBridgePlansReminders$' \
		TestBridgePlansReminders/attention_rules \
		TestBridgePlansReminders/time_zone_change \
		TestBridgePlansReminders/actions_replan \
		TestBridgePlansReminders/incoming_changes_replan \
		TestBridgePlansReminders/parked_and_archived_raise_nothing \
		TestBridgePlansReminders/limit_and_refusals

	# Scheduling and actions against a fake notification center: explicit
	# trigger zones, replace/keep/cancel reconciliation, opt-in, a harmless
	# permission denial, time-zone and incoming-change replanning, a failed
	# plan, a switched workspace, and every action routed to its command.
	local scheduling=TTRemindersTests.ReminderSchedulingTests actions=TTRemindersTests.ReminderActionTests
	run_swift_suite reminders ios/TTReminders \
		"$scheduling/testSchedulesThePlanWithExplicitZones" \
		"$scheduling/testReconcileReplacesMovedRemovesStaleAndKeepsUnchanged" \
		"$scheduling/testTimeZoneChangeReplansWallClockReminders" \
		"$scheduling/testIncomingChangesReconcile" \
		"$scheduling/testRemindersAreOptInAndNeverPromptUntilTurnedOn" \
		"$scheduling/testPermissionDenialIsHarmless" \
		"$scheduling/testPlanFailureLeavesPendingRemindersAlone" \
		"$scheduling/testClosedOrSwitchedWorkspaceRemovesItsReminders" \
		"$scheduling/testAFailedAddIsReportedAndTheRestScheduled" \
		"$actions/testAcknowledgeRoutesToTheEngine" \
		"$actions/testDeferActionsUseTheEngineDeferralGrammar" \
		"$actions/testOpeningShowsTheTaskAndWritesNothing" \
		"$actions/testAnotherWorkspacesReminderIsRefused" \
		"$actions/testAnEngineRefusalIsRecordedNotThrown" \
		"$actions/testCategoriesOfferOnlyWhatTheReminderAllows"

	# Real delivery and actions on a physical iPhone, recorded by
	# ios/scripts/record-reminder-evidence.sh and held to the builds the
	# current tree produces. Simulator runs do not count.
	local source_fp bridge_fp
	source_fp=$(ios/scripts/reminder-evidence-fingerprint.sh) || fail "fingerprinting the Swift sources failed"
	bridge_fp=$(ios/scripts/build-bridge.sh --fingerprint) || fail "fingerprinting the bridge failed"
	require_files docs/mobile/evidence/session-15-reminders.json
	TT_REMINDER_SOURCE_FINGERPRINT="$source_fp" TT_IOS_BRIDGE_FINGERPRINT="$bridge_fp" \
		run_go_suite reminder-evidence ./internal/bridge/ '^TestRecordedReminderDeviceEvidence$' \
		TestRecordedReminderDeviceEvidence
	note "real iPhone reminders: run $(grep -Eo '"run_id": "[^"]+"' docs/mobile/evidence/session-15-reminders.json | head -1)"

	note "session 15 verified"
}

verify_session_16() {
	[ "$(uname -s)" = Darwin ] || fail "session 16 runs the App Group package and the share and widget UI tests on macOS only"
	require_files \
		internal/bridge/capture_inbox_ops.go \
		internal/bridge/capture_inbox_test.go \
		internal/bridge/widget_ops.go \
		internal/app/capture_task_id_test.go \
		docs/mobile/capture-inbox.md \
		ios/TTShared/Package.swift \
		ios/TTShared/Sources/TTShared/CaptureInbox.swift \
		ios/TTShared/Sources/TTShared/WidgetSnapshot.swift \
		ios/TTShared/Sources/TTShared/DeepLink.swift \
		ios/TTShared/Sources/TTShared/SharedItems.swift \
		ios/TTShared/Tests/TTSharedTests/CaptureImportContractTests.swift \
		ios/TTEngine/Sources/TTEngine/Captures.swift \
		ios/TTShare/ShareViewController.swift \
		ios/TTWidgets/TTWidgets.swift \
		ios/TTMobile/App/CaptureInboxModel.swift \
		ios/TTMobile/App/PendingCapturesSection.swift \
		ios/TTMobile/App/Shortcuts.swift \
		ios/TTMobileUITests/ShareCaptureUITests.swift

	local served
	for served in capture.import query.widget_snapshot; do
		grep -Fq "\"$served\":" internal/bridge/bridge.go || fail "the bridge does not serve $served"
	done

	# The engine imports each saved capture exactly once: duplicate delivery,
	# a termination after the commit, between files, and after the receipt,
	# an interrupted removal, an extension terminated before it published and
	# its retry, a refusal kept pending with its reason, a damaged file, and
	# another workspace's capture left alone. The widget snapshot answers the
	# app's own Today and Inbox, each row linking to its task, for the local
	# day it is true for.
	run_go_suite capture-inbox ./internal/bridge/ '^(TestBridgeImportsSavedCapturesOnce|TestBridgeAnswersAWidgetSnapshot)$' \
		TestBridgeImportsSavedCapturesOnce/text_link_and_file_import \
		TestBridgeImportsSavedCapturesOnce/duplicate_delivery \
		TestBridgeImportsSavedCapturesOnce/termination_after_commit \
		TestBridgeImportsSavedCapturesOnce/termination_between_files \
		TestBridgeImportsSavedCapturesOnce/termination_after_receipt \
		TestBridgeImportsSavedCapturesOnce/interrupted_removal_is_finished \
		TestBridgeImportsSavedCapturesOnce/extension_termination_and_retry \
		TestBridgeImportsSavedCapturesOnce/refused_capture_stays_pending \
		TestBridgeImportsSavedCapturesOnce/damaged_file_imports_nothing \
		TestBridgeImportsSavedCapturesOnce/other_workspace_is_left_alone \
		TestBridgeAnswersAWidgetSnapshot
	run_go_suite capture-task-id ./internal/app/ '^TestCaptureWithTaskIDTakesThatIdentityOnce$' \
		TestCaptureWithTaskIDTakesThatIdentityOnce
	run_go_suite write-inventory ./internal/compat/ '^TestCompatWritePathInventoryCoversEveryDurableWrite$' \
		TestCompatWritePathInventoryCoversEveryDurableWrite

	# The App Group package: the extension's writer publishes once and whole,
	# survives its own termination and a retry, reads a share sheet's words,
	# links, and files, and judges a widget snapshot's freshness; then the
	# same writer's captures imported by the real engine in separate
	# processes, and the engine's snapshot read back with its links.
	local bridgehost="$work_dir/bridgehost"
	go build -trimpath -o "$bridgehost" ./mobile/bridgehost || fail "building bridgehost failed"
	local inbox=TTSharedTests.CaptureInboxTests share=TTSharedTests.SharedItemsTests widget=TTSharedTests.WidgetAndLinkTests contract=TTSharedTests.CaptureImportContractTests
	TT_BRIDGEHOST="$bridgehost" TT_CAPTURE_FIXTURE="$repo_root/internal/compat/testdata/workspaces/representative" \
		run_swift_suite shared ios/TTShared \
		"$inbox/testSavePublishesTheEnvelopeAndVerifiableFiles" \
		"$inbox/testATerminatedSaveLeavesNothingPendingAndItsRetryPublishesOnce" \
		"$inbox/testDeliveringTheSameCaptureTwiceKeepsOneEntry" \
		"$inbox/testPendingCapturesShowTheirRefusalUntilRevised" \
		"$inbox/testFilenamesStayInsideTheirEntry" \
		"$share/testTextLinksAndFilesAreReadAndFilesCopied" \
		"$share/testADraftIsProposedFromALinkOrAFileAlone" \
		"$widget/testFreshnessFollowsAgeAndTheDayItDescribes" \
		"$widget/testTheTimelineTurnsAtStalenessAndMidnight" \
		"$widget/testTheStoreRoundTripsAndSkipsAnUnchangedWrite" \
		"$widget/testDeepLinksRoundTripAndRefuseWhatTheyDoNotName" \
		"$contract/testCapturesThisPackageWritesImportOnceAcrossDeliveriesAndRestarts" \
		"$contract/testARefusedCaptureStaysPendingWithTheEnginesReason" \
		"$contract/testTheEnginesWidgetSnapshotDecodesAndLinksToItsTasks"

	# The iPhone on a simulator against the gomobile-bound engine: saved
	# captures shown as pending, imported once across relaunches and a second
	# delivery, a refusal corrected in the app; the widget snapshot following
	# the workspace with rows that open their tasks; and links to views,
	# capture, and another workspace's task.
	note "running the TTMobile share and widget UI suite"
	TT_UITEST_ONLY=ShareCaptureUITests ios/scripts/test-ui.sh "$work_dir/ui" >"$work_dir/test-ui.log" 2>&1 || {
		cat "$work_dir/test-ui.log" >&2
		fail "ios/scripts/test-ui.sh failed"
	}
	require_xcresult_passed "$work_dir/ui/TTMobileUITests.xcresult" \
		'testSavedCapturesStayPendingUntilImportedAndLandOnce()' \
		'testTheWidgetSnapshotFollowsTheWorkspaceAndItsRowsOpenTheirTasks()' \
		'testLinksOpenViewsAndCaptureAndRefuseAnotherWorkspace()'

	# The app that suite ran embeds the share extension and the widgets,
	# declares the tt scheme and its shortcuts, and neither extension links
	# the engine, so neither can rewrite the workspace.
	local app="${TT_IOS_CACHE:-ios/.cache}/UIDerivedData/Build/Products/Debug-iphonesimulator/tt.app"
	[ -d "$app" ] || fail "no simulator app at $app"
	[ "$(plutil -extract NSExtension.NSExtensionPointIdentifier raw "$app/PlugIns/TTShare.appex/Info.plist")" = com.apple.share-services ] ||
		fail "the app embeds no share extension"
	[ "$(plutil -extract NSExtension.NSExtensionPointIdentifier raw "$app/PlugIns/TTWidgets.appex/Info.plist")" = com.apple.widgetkit-extension ] ||
		fail "the app embeds no widget extension"
	[ "$(plutil -extract CFBundleURLTypes.0.CFBundleURLSchemes.0 raw "$app/Info.plist")" = tt ] || fail "the app does not declare the tt scheme"
	grep -rqF CaptureTaskIntent "$app/Metadata.appintents" || fail "the app declares no capture shortcut"
	# The app itself links the engine, which is what makes the check below
	# able to see a link at all.
	local linked="" binary
	for binary in "$app/tt" "$app/tt.debug.dylib"; do
		[ -f "$binary" ] && linked="$linked$(otool -L "$binary")"
	done
	printf '%s' "$linked" | grep -q 'TTEngine' || fail "cannot see the app's own link to the engine, so the extension check proves nothing"
	for binary in "$app"/PlugIns/TTShare.appex/* "$app"/PlugIns/TTWidgets.appex/*; do
		[ -f "$binary" ] && file "$binary" | grep -q Mach-O || continue
		if otool -L "$binary" | grep -qE 'TTEngine|TTBridgeCore'; then
			fail "$(basename "$(dirname "$binary")") links the engine"
		fi
	done
	# PlugIns also holds the hosted device test bundle, which links the engine
	# on purpose; only the two extensions are held to this.
	if find "$app/PlugIns/TTShare.appex" "$app/PlugIns/TTWidgets.appex" -name 'TTBridgeCore*' | grep -q .; then
		fail "an extension embeds the engine framework"
	fi
	note "the share extension and widgets are embedded and link no engine"

	note "session 16 verified"
}

# require_suite_summary SUMMARY SUITE... holds the summary ios/scripts/
# test-suites.sh wrote to the evidence rules: each named suite is listed as
# passed, executed tests, and failed and skipped none.
require_suite_summary() {
	local summary=$1
	shift
	local suite
	for suite in "$@"; do
		python3 -I - "$summary" "$suite" <<'PY' || fail "suite $suite is missing from $summary, or did not pass whole"
import json, sys
suites = {entry["suite"]: entry for entry in json.load(open(sys.argv[1]))["suites"]}
entry = suites.get(sys.argv[2])
ok = entry and entry["status"] == "passed" and entry["executed"] > 0 and entry["failed"] == 0 and entry["skipped"] == 0
if ok:
    print(f"verify_mobile_session: suite {sys.argv[2]}: {entry['executed']} tests passed")
sys.exit(0 if ok else 1)
PY
	done
}

# require_xctest_passed LOG CLASS/METHOD... requires each XCTest case passed in
# a swift test log.
require_xctest_passed() {
	local log=$1
	shift
	local test
	for test in "$@"; do
		grep -Fq -- "Test Case '-[${test%%/*} ${test##*/}]' passed" "$log" || fail "missing required passing test $test in $log"
	done
}

# require_ui_screenshots RESULTS NAME... requires the UI suite's exported
# attachments to hold a screenshot for each NAME the tests attached.
require_ui_screenshots() {
	local results=$1
	shift
	local name
	for name in "$@"; do
		python3 -I - "$results/attachments/manifest.json" "$results/attachments" "$name" <<'PY' || fail "no exported screenshot $name in $results/attachments"
import json, os, sys
manifest, directory, wanted = sys.argv[1:4]
found = [a for test in json.load(open(manifest)) for a in test["attachments"]
         if a["suggestedHumanReadableName"].startswith(wanted + "_")
         and os.path.getsize(os.path.join(directory, a["exportedFileName"])) > 0]
sys.exit(0 if found else 1)
PY
	done
}

# state_field FILE PATH prints one value of a tt-mobile-state/v1 document.
state_field() {
	python3 -I - "$1" "$2" <<'PY'
import json, sys
node = json.load(open(sys.argv[1]))
for key in sys.argv[2].split("."):
    node = node[key]
print(json.dumps(node, sort_keys=True) if isinstance(node, (list, dict)) else node)
PY
}

# state_lists FILE prints the Today and Inbox rows a state export names,
# without the snapshot's own clock, so two exports of one state compare equal.
state_lists() {
	python3 -I - "$1" <<'PY'
import json, sys
state = json.load(open(sys.argv[1]))
lists = state["lists"]
print(json.dumps({"today": lists["today"], "inbox": lists["inbox"], "drafts": state["drafts"],
                  "workspace": state["workspace"]["name"]}, sort_keys=True))
PY
}

verify_session_17() {
	[ "$(uname -s)" = Darwin ] || fail "session 17 drives the iOS simulator recipes on macOS only"
	require_files \
		ios/scripts/app.sh \
		ios/scripts/simulator.sh \
		ios/scripts/test-suites.sh \
		ios/scripts/test-ui.sh \
		ios/TTMobile/App/AutomationState.swift \
		ios/TTMobileUITests/AccessibilityUITests.swift \
		ios/README.md \
		.github/workflows/ci.yml
	command -v just >/dev/null || fail "just is required to exercise the recipes"

	# The public recipes exist and are documented.
	local recipe recipes
	recipes=$(just --summary)
	for recipe in ios-build ios-test ios-ui-test ios-run ios-reset ios-screenshot ios-export ios-stop; do
		printf ' %s ' "$recipes" | grep -q " $recipe " || fail "the justfile has no $recipe recipe"
		grep -Fq "just $recipe" ios/README.md || fail "ios/README.md does not document just $recipe"
	done

	# CI: the Go gate still runs on Linux and macOS, and a separate macOS job
	# runs the simulator suites through those recipes, keeps their results,
	# and does not repeat the Go gate.
	ruby -ryaml -e '
		jobs = YAML.load_file(".github/workflows/ci.yml").fetch("jobs")
		go = jobs.fetch("ci")
		abort "the Go job lost a platform" unless go.dig("strategy", "matrix", "os").to_a.sort == %w[macos-latest ubuntu-latest]
		abort "the Go job no longer runs scripts/ci.sh" unless go["steps"].any? { |s| s["run"].to_s.include?("./scripts/ci.sh") }
		ios = jobs.fetch("ios") { abort "no ios job" }
		abort "the ios job does not run on macOS" unless ios["runs-on"].to_s.start_with?("macos")
		runs = ios["steps"].map { |s| s["run"].to_s }
		["just ios-build", "just ios-test all", "just ios-ui-test", "just ios-ui-test AccessibilityUITests small"].each do |want|
			abort "the ios job does not run #{want}" unless runs.any? { |r| r.strip == want }
		end
		abort "the ios job repeats the Go gate" if runs.any? { |r| r.include?("scripts/ci.sh") || r.include?("go test") }
		upload = ios["steps"].find { |s| s["uses"].to_s.start_with?("actions/upload-artifact") }
		abort "the ios job keeps no results" unless upload && upload["if"].to_s.include?("always()")
	' || fail "the CI configuration does not run the simulator suites as required"
	note "CI: Go on Linux and macOS, plus the iOS simulator job"

	export TT_IOS_RESULTS="$work_dir/results" TT_IOS_RUN="$work_dir/run"

	# Build: the app, its result bundle, and the structured build diagnostics.
	note "running just ios-build"
	just ios-build >"$work_dir/ios-build.log" 2>&1 || { tail -40 "$work_dir/ios-build.log" >&2; fail "just ios-build failed"; }
	require_files "$TT_IOS_RESULTS/build/summary.json"
	[ -d "$TT_IOS_RESULTS/build/TTMobile-build.xcresult" ] || fail "the build left no result bundle"
	[ "$(plutil -extract status raw -o - "$TT_IOS_RESULTS/build/summary.json")" = succeeded ] || fail "the build diagnostics do not report success"
	[ "$(plutil -extract errorCount raw -o - "$TT_IOS_RESULTS/build/summary.json")" -eq 0 ] || fail "the build diagnostics report errors"

	# The four Swift suites, each run and reported on its own.
	note "running just ios-test all"
	just ios-test all >"$work_dir/ios-test.log" 2>&1 || { tail -40 "$work_dir/ios-test.log" >&2; fail "just ios-test all failed"; }
	local suites="$TT_IOS_RESULTS/all"
	require_suite_summary "$suites/summary.json" unit-shared unit-reminders bridge sync
	require_xctest_passed "$suites/unit-shared.log" \
		TTSharedTests.CaptureInboxTests/testSavePublishesTheEnvelopeAndVerifiableFiles \
		TTSharedTests.CaptureImportContractTests/testCapturesThisPackageWritesImportOnceAcrossDeliveriesAndRestarts
	require_xctest_passed "$suites/unit-reminders.log" \
		TTRemindersTests.ReminderSchedulingTests/testSchedulesThePlanWithExplicitZones \
		TTRemindersTests.ReminderActionTests/testAcknowledgeRoutesToTheEngine
	require_xctest_passed "$suites/sync.log" \
		TTSyncTests.ExchangeContractTests/testContractRoundTripOverFakeICloud \
		TTSyncTests.ExchangeContractTests/testContractRoundTripOverACoordinatedFolder \
		TTSyncTests.JoinTests/testJoinBuildsAWorkingCopyFromTheExchange
	require_xcresult_passed "$suites/bridge/TTEngine.xcresult" \
		'testLinkedEngineSpeaksTheModelsBridgeVersion()' \
		'testOpenFixtureCaptureCompleteReopen()'

	# Accessibility on the default simulator, then again on a small one.
	local a11y_tests=(
		'testTheLargestTextSizeKeepsTheMainWorkflowsWhole()'
		'testVoiceOverLabelsNameEveryControl()'
		'testDarkModeDrawsEveryMainScreenReadably()'
		'testASmallScreenKeepsEveryMainActionReachable()'
		'testAHardwareKeyboardDrivesTabsAndCapture()'
		'testDraftsComeBackAfterARelaunchAtTheLargestTextSizeInDarkMode()'
	)
	local device results
	for device in default small; do
		results="$TT_IOS_RESULTS/ui-$device"
		note "running just ios-ui-test AccessibilityUITests $device"
		just ios-ui-test AccessibilityUITests "$device" >"$work_dir/ios-ui-test-$device.log" 2>&1 || {
			tail -60 "$work_dir/ios-ui-test-$device.log" >&2
			fail "just ios-ui-test AccessibilityUITests $device failed"
		}
		require_files "$results/summary.json" "$results/tests.json" "$results/attachments/manifest.json"
		require_xcresult_passed "$results/TTMobileUITests.xcresult" "${a11y_tests[@]}"
		require_ui_screenshots "$results" today-largest-text detail-largest-text capture-largest-text \
			sync-voiceover-labels today-dark detail-dark capture-dark browse-dark \
			today-small-screen capture-small-screen capture-keyboard-restored \
			capture-restored-dark-largest edit-restored-dark-largest
	done
	# The small run really was on a small screen.
	local width
	width=$(python3 -I - "$TT_IOS_RESULTS/ui-small/attachments" <<'PY'
import json, os, re, sys
directory = sys.argv[1]
for test in json.load(open(os.path.join(directory, "manifest.json"))):
    for a in test["attachments"]:
        if a["suggestedHumanReadableName"].startswith("window size_"):
            print(re.search(r"window (\d+) x", open(os.path.join(directory, a["exportedFileName"])).read()).group(1))
            sys.exit(0)
sys.exit(1)
PY
) || fail "the small run recorded no window size"
	[ "$width" -le 375 ] || fail "the small-screen run drew a $width-point window"
	note "small screen: a $width-point window"

	# Launch, look, export, reset, and repeat from the command line.
	note "running just ios-run"
	just ios-run >"$work_dir/ios-run.log" 2>&1 || { tail -40 "$work_dir/ios-run.log" >&2; fail "just ios-run failed"; }
	local state="$TT_IOS_RUN/state.json" first="$work_dir/first-state.json" second="$work_dir/second-state.json"
	[ "$(state_field "$state" phase)" = ready ] || fail "ios-run did not leave the workspace open"
	just ios-screenshot launch >/dev/null 2>&1 || fail "just ios-screenshot failed"
	file "$TT_IOS_RUN/screenshots/launch.png" | grep -q 'PNG image' || fail "the screenshot is not a PNG"
	just ios-export >"$first" 2>"$work_dir/ios-export.log" || { cat "$work_dir/ios-export.log" >&2; fail "just ios-export failed"; }
	require_files "$TT_IOS_RUN/export/state.json" "$TT_IOS_RUN/export/session.json" "$TT_IOS_RUN/export/log.ndjson"
	[ "$(state_field "$first" schema)" = tt-mobile-state/v1 ] || fail "the export is not tt-mobile-state/v1"
	[ "$(state_field "$first" environment.color_scheme)" = light ] || fail "the default run is not light"
	[ "$(state_field "$first" screen.tab)" = today ] || fail "the launch does not show Today"
	state_lists "$first" | grep -q '"title": "Ship beta build"' || fail "the export does not name Today's rows"
	[ "$(state_field "$first" lists.inbox.count)" -gt 0 ] || fail "the export names no Inbox rows"

	note "running just ios-reset"
	just ios-reset >"$work_dir/ios-reset.log" 2>&1 || { cat "$work_dir/ios-reset.log" >&2; fail "just ios-reset failed"; }
	just ios-export >"$second" 2>>"$work_dir/ios-export.log" || fail "just ios-export after the reset failed"
	[ "$(state_lists "$first")" = "$(state_lists "$second")" ] || {
		diff <(state_lists "$first") <(state_lists "$second") >&2 || true
		fail "a reset run does not start from the same state"
	}
	note "a reset relaunch repeats the same start"

	note "running just ios-run representative dark accessibility-extra-extra-extra-large"
	just ios-run representative dark accessibility-extra-extra-extra-large >"$work_dir/ios-run-dark.log" 2>&1 ||
		{ tail -40 "$work_dir/ios-run-dark.log" >&2; fail "the dark, largest-text run failed"; }
	just ios-export >"$second" 2>>"$work_dir/ios-export.log" || fail "just ios-export of the dark run failed"
	[ "$(state_field "$second" environment.color_scheme)" = dark ] || fail "the dark run was not drawn dark"
	[ "$(state_field "$second" environment.dynamic_type)" = accessibility5 ] || fail "the largest-text run was not drawn at accessibility5"
	just ios-screenshot dark-largest >/dev/null 2>&1 || fail "just ios-screenshot of the dark run failed"

	note "running just ios-run empty"
	just ios-run empty >"$work_dir/ios-run-empty.log" 2>&1 || { tail -40 "$work_dir/ios-run-empty.log" >&2; fail "the empty-fixture run failed"; }
	[ "$(state_field "$TT_IOS_RUN/state.json" phase)" = onboarding ] || fail "the empty fixture did not start at onboarding"
	just ios-stop >/dev/null 2>&1 || fail "just ios-stop failed"
	# Leave the simulator at its ordinary appearance and size.
	local udid
	udid=$(sed -n 's/^udid=//p' "$TT_IOS_RUN/session.env")
	xcrun simctl ui "$udid" appearance light >/dev/null 2>&1 || true
	xcrun simctl ui "$udid" content_size large >/dev/null 2>&1 || true

	note "session 17 verified"
}

case "$session" in
1) verify_session_1 ;;
2) verify_session_2 ;;
3) verify_session_3 ;;
4) verify_session_4 ;;
5) verify_session_5 ;;
6) verify_session_6 ;;
7) verify_session_7 ;;
8) verify_session_8 ;;
9) verify_session_9 ;;
10) verify_session_10 ;;
11) verify_session_11 ;;
12) verify_session_12 ;;
13) verify_session_13 ;;
14) verify_session_14 ;;
15) verify_session_15 ;;
16) verify_session_16 ;;
17) verify_session_17 ;;
"") fail "usage: verify_mobile_session.sh <session>" ;;
*) fail "no verifier defined for session $session" ;;
esac
