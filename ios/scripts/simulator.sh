#!/usr/bin/env sh
#
# Print the UDID of the simulator the iOS recipes run on.
#
#   ios/scripts/simulator.sh [default|small]
#
# default   DOCKET_IOS_SIMULATOR when set, else the first available iPhone on an
#           iOS 18 or later runtime.
# small     DOCKET_IOS_SMALL_SIMULATOR when set, else a simulator named
#           "docket-small" (an iPhone SE (3rd generation), 375 x 667 points, the
#           narrowest iPhone the newest runtime still creates), made on the
#           newest iOS runtime the first time it is asked for.

set -eu

kind=${1:-default}

case "$kind" in
default)
	if [ -n "${DOCKET_IOS_SIMULATOR:-}" ]; then
		echo "$DOCKET_IOS_SIMULATOR"
		exit 0
	fi
	simulator=$(xcrun simctl list devices available | awk '
		/^-- iOS [0-9]+/ { split($3, version, "."); eligible = (version[1] + 0 >= 18); next }
		/^-- / { eligible = 0; next }
		eligible && /iPhone/ && match($0, /\([0-9A-F-]{36}\)/) {
			print substr($0, RSTART + 1, RLENGTH - 2); exit
		}')
	if [ -z "$simulator" ]; then
		echo "no available iPhone simulator on iOS 18 or later; set DOCKET_IOS_SIMULATOR" >&2
		exit 1
	fi
	echo "$simulator"
	;;
small)
	if [ -n "${DOCKET_IOS_SMALL_SIMULATOR:-}" ]; then
		echo "$DOCKET_IOS_SMALL_SIMULATOR"
		exit 0
	fi
	existing=$(xcrun simctl list devices available | awk '
		/^    docket-small \(/ && match($0, /\([0-9A-F-]{36}\)/) { print substr($0, RSTART + 1, RLENGTH - 2); exit }')
	if [ -n "$existing" ]; then
		echo "$existing"
		exit 0
	fi
	runtime=$(xcrun simctl list runtimes available | awk '
		/^iOS [0-9]+/ && match($0, /com\.apple\.CoreSimulator\.SimRuntime\.iOS-[0-9-]+/) {
			split($2, version, "."); if (version[1] + 0 >= 18) id = substr($0, RSTART, RLENGTH)
		}
		END { print id }')
	if [ -z "$runtime" ]; then
		echo "no iOS 18 or later simulator runtime to make the small-screen simulator on" >&2
		exit 1
	fi
	for device in iPhone-SE-3rd-generation iPhone-13-mini; do
		if created=$(xcrun simctl create docket-small "com.apple.CoreSimulator.SimDeviceType.$device" "$runtime" 2>/dev/null); then
			echo "$created"
			exit 0
		fi
	done
	echo "the $runtime runtime makes no small iPhone; set DOCKET_IOS_SMALL_SIMULATOR" >&2
	exit 1
	;;
*)
	echo "usage: $0 [default|small]" >&2
	exit 2
	;;
esac
