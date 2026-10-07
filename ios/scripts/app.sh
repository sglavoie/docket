#!/usr/bin/env sh
#
# Drive the iPhone app on a simulator from the command line, so an agent can
# build, launch, look at, and repeat a workflow without opening Xcode.
#
#   ios/scripts/app.sh build
#   ios/scripts/app.sh run [fixture] [appearance] [text-size] [device]
#   ios/scripts/app.sh reset
#   ios/scripts/app.sh screenshot NAME
#   ios/scripts/app.sh export
#   ios/scripts/app.sh stop
#
# build       Generates the project and builds tt.app for the simulator.
#             Writes RESULTS/build/{TTMobile-build.xcresult,xcodebuild.log,
#             summary.json} and prints the app's path.
# run         Builds, boots the simulator, installs the app, and launches it
#             over a fresh isolated root seeded from a fixture:
#               fixture     representative (default) or empty (onboarding)
#               appearance  light (default) or dark, set on the simulator
#               text-size   a `simctl ui content_size` name, default large;
#                           accessibility-extra-extra-extra-large is the
#                           largest Dynamic Type size
#               device      default or small (see simulator.sh)
#             The app never reads its real container: TT_UITEST_ROOT points it
#             at RUN/root, and it writes its semantic state to RUN/state.json.
#             Waits until that state reports the workspace open.
# reset       Terminates the app, reseeds RUN/root from the same fixture, and
#             relaunches with the same options, so a workflow repeats from the
#             same start.
# screenshot  Saves RUN/screenshots/NAME.png and prints its path.
# export      Prints the app's semantic state (schema tt-mobile-state/v1, see
#             ios/TTMobile/App/AutomationState.swift) and writes it, with the
#             app's recent unified log as NDJSON and a summary of the session,
#             under RUN/export/.
# stop        Terminates the app.
#
# Environment:
#   TT_IOS_RESULTS          where build results go (default ios/.cache/results)
#   TT_IOS_RUN              the run session directory (default ios/.cache/run)
#   TT_IOS_CACHE            cache root (default ios/.cache, gitignored)
#   TT_IOS_SIMULATOR, TT_IOS_SMALL_SIMULATOR   see simulator.sh

set -eu

script_dir=$(CDPATH="" cd -- "$(dirname -- "$0")" && pwd)
ios_dir=$(dirname -- "$script_dir")
repo_root=$(dirname -- "$ios_dir")
cache_root="${TT_IOS_CACHE:-$ios_dir/.cache}"
results="${TT_IOS_RESULTS:-$cache_root/results}"
run_dir="${TT_IOS_RUN:-$cache_root/run}"
bundle_id=io.github.sglavoie.tt.mobile
derived="$cache_root/AppDerivedData"
app_path="$derived/Build/Products/Debug-iphonesimulator/tt.app"

die() {
	echo "app.sh: $*" >&2
	exit 1
}

absolute() {
	mkdir -p "$1"
	CDPATH="" cd -- "$1" && pwd
}

build() {
	out=$(absolute "$results/build")
	"$script_dir/generate-project.sh"
	rm -rf "$out/TTMobile-build.xcresult"
	echo "Building tt.app for the simulator..." >&2
	status=0
	xcodebuild build \
		-project "$ios_dir/TTMobile.xcodeproj" \
		-scheme TTMobile \
		-destination "generic/platform=iOS Simulator" \
		-derivedDataPath "$derived" \
		-resultBundlePath "$out/TTMobile-build.xcresult" \
		CODE_SIGNING_ALLOWED=NO \
		>"$out/xcodebuild.log" 2>&1 || status=$?
	# The structured diagnostics: the build's result, errors, and warnings
	# as the result bundle records them.
	xcrun xcresulttool get build-results --path "$out/TTMobile-build.xcresult" >"$out/summary.json" 2>/dev/null || true
	if [ "$status" -ne 0 ]; then
		grep -E 'error:' "$out/xcodebuild.log" >&2 || tail -40 "$out/xcodebuild.log" >&2
		die "xcodebuild build failed; log at $out/xcodebuild.log, diagnostics at $out/summary.json"
	fi
	[ -d "$app_path" ] || die "the build left no app at $app_path"
	echo "$app_path"
}

# session_get KEY reads one value the last run recorded.
session_get() {
	[ -f "$run_dir/session.env" ] || die "no app session; run \`just ios-run\` first"
	sed -n "s/^$1=//p" "$run_dir/session.env"
}

seed() {
	fixture=$1 root=$2
	rm -rf "$root"
	mkdir -p "$root/shared"
	case "$fixture" in
	representative)
		mkdir -p "$root/workspaces"
		cp -R "$repo_root/internal/compat/testdata/workspaces/representative" "$root/workspaces/representative"
		# The fixture's one wall-clock deferral would make Today and the
		# outline depend on the day the run happens; without it they don't.
		atlas="$root/workspaces/representative/projects/atlas-launch/tasks.md"
		sed 's/, deferred: 2026-10-07T09:00:00Z//' "$atlas" >"$atlas.seed" && mv "$atlas.seed" "$atlas"
		printf representative >"$root/selected-workspace"
		;;
	empty) ;;
	*) die "unknown fixture $fixture (representative or empty)" ;;
	esac
}

launch() {
	udid=$(session_get udid)
	root=$(session_get root)
	fixture=$(session_get fixture)
	xcrun simctl terminate "$udid" "$bundle_id" >/dev/null 2>&1 || true
	seed "$fixture" "$root"
	rm -f "$run_dir/state.json"
	SIMCTL_CHILD_TT_UITEST_ROOT="$root" \
		SIMCTL_CHILD_TT_UITEST_DEVICE_ID=automation-phone \
		SIMCTL_CHILD_TT_UITEST_SHARED_CONTAINER="$root/shared" \
		SIMCTL_CHILD_TT_UITEST_STATE_EXPORT="$run_dir/state.json" \
		xcrun simctl launch "$udid" "$bundle_id" >/dev/null
	want=ready
	[ "$fixture" = empty ] && want=onboarding
	tries=0
	until [ -f "$run_dir/state.json" ] && grep -q "\"phase\" : \"$want\"" "$run_dir/state.json"; do
		tries=$((tries + 1))
		[ "$tries" -le 120 ] || die "the app never reported phase $want; see $run_dir/state.json"
		sleep 0.5
	done
	# Let the first lists land in the export before reporting the launch.
	if [ "$want" = ready ]; then
		tries=0
		until grep -q '"lists"' "$run_dir/state.json"; do
			tries=$((tries + 1))
			[ "$tries" -le 60 ] || break
			sleep 0.5
		done
	fi
	echo "tt is running on $udid over $root ($fixture); state at $run_dir/state.json" >&2
}

run() {
	fixture=${1:-representative} appearance=${2:-light} size=${3:-large} device=${4:-default}
	case "$appearance" in light | dark) ;; *) die "appearance is light or dark, not $appearance" ;; esac
	build >/dev/null
	udid=$("$script_dir/simulator.sh" "$device")
	xcrun simctl boot "$udid" >/dev/null 2>&1 || true
	xcrun simctl bootstatus "$udid" -b >/dev/null
	xcrun simctl ui "$udid" appearance "$appearance"
	xcrun simctl ui "$udid" content_size "$size" || die "unknown text size $size"
	xcrun simctl install "$udid" "$app_path"
	mkdir -p "$run_dir"
	run_dir=$(absolute "$run_dir")
	cat >"$run_dir/session.env" <<EOF
udid=$udid
device=$device
fixture=$fixture
appearance=$appearance
text_size=$size
root=$run_dir/root
EOF
	launch
}

export_state() {
	udid=$(session_get udid)
	[ -f "$run_dir/state.json" ] || die "the app has written no state yet"
	out="$run_dir/export"
	mkdir -p "$out"
	# A change the app is still settling is written within a quarter second.
	sleep 1
	cp "$run_dir/state.json" "$out/state.json"
	xcrun simctl spawn "$udid" log show --last 5m --style ndjson \
		--predicate "subsystem BEGINSWITH \"$bundle_id\" OR process == \"tt\"" >"$out/log.ndjson" 2>/dev/null || true
	name=$(xcrun simctl list devices | awk -v id="$udid" 'index($0, id) { sub(/^ +/, ""); sub(/ \(.*/, ""); print; exit }')
	cat >"$out/session.json" <<EOF
{
  "simulator": "$udid",
  "device_name": "$name",
  "device": "$(session_get device)",
  "fixture": "$(session_get fixture)",
  "appearance": "$(session_get appearance)",
  "text_size": "$(session_get text_size)",
  "root": "$(session_get root)",
  "state": "$out/state.json",
  "log": "$out/log.ndjson"
}
EOF
	cat "$out/state.json"
}

command=${1:-}
[ -n "$command" ] || die "usage: $0 build|run|reset|screenshot|export|stop"
shift
case "$command" in
build) build ;;
run) run "$@" ;;
reset) launch ;;
screenshot)
	[ "$#" -eq 1 ] || die "usage: $0 screenshot NAME"
	mkdir -p "$run_dir/screenshots"
	xcrun simctl io "$(session_get udid)" screenshot "$run_dir/screenshots/$1.png" >/dev/null 2>&1 ||
		die "the simulator took no screenshot"
	echo "$run_dir/screenshots/$1.png"
	;;
export) export_state ;;
stop) xcrun simctl terminate "$(session_get udid)" "$bundle_id" ;;
*) die "unknown command $command" ;;
esac
