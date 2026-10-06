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

session=${1:-}
case "$session" in
1) verify_session_1 ;;
2) verify_session_2 ;;
3) verify_session_3 ;;
"") fail "usage: verify_mobile_session.sh <session>" ;;
*) fail "no verifier defined for session $session" ;;
esac
