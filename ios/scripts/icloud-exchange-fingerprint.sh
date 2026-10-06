#!/usr/bin/env bash
#
# Print the fingerprint of the Swift sources a real iCloud exchange run is
# built from: the shared transport, the app, its device test, the engine
# adapter, and the project spec. The Go side is pinned separately, by the
# bridge fingerprint (ios/scripts/build-bridge.sh --fingerprint) for the
# iPhone and by the reproducible bridgehost hash for the Mac, so recorded
# evidence names exactly the builds it exercised.
#
#   ios/scripts/icloud-exchange-fingerprint.sh

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$repo_root"

{
	printf '%s\n' apple/TTSync/Package.swift ios/TTEngine/Package.swift ios/project.yml
	find apple/TTSync/Sources ios/TTMobile ios/TTMobileDeviceTests ios/TTEngine/Sources -type f ! -name .DS_Store
} | LC_ALL=C sort | while IFS= read -r path; do
	printf '%s\t%s\n' "$path" "$(shasum -a 256 <"$path" | cut -d' ' -f1)"
done | shasum -a 256 | cut -d' ' -f1
