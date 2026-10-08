#!/usr/bin/env bash
#
# Print the fingerprint of every Swift source and project input the signed
# iPhone app is built from: the app, its share and widget extensions, the
# shared, reminder, and engine packages, the sync transport, the project spec,
# and the signing configuration. The Go side is pinned separately, by the
# bridge fingerprint (ios/scripts/build-bridge.sh --fingerprint) for the
# iPhone and by the reproducible tt hash for the Mac, so a device
# qualification names exactly the builds it exercised.
#
#   ios/scripts/qualification-fingerprint.sh

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$repo_root"

{
	printf '%s\n' ios/project.yml apple/TTSync/Package.swift ios/TTEngine/Package.swift \
		ios/TTShared/Package.swift ios/TTReminders/Package.swift
	find ios/TTMobile ios/TTShare ios/TTWidgets ios/Config \
		apple/TTSync/Sources ios/TTEngine/Sources ios/TTShared/Sources ios/TTReminders/Sources \
		-type f ! -name .DS_Store
} | LC_ALL=C sort | while IFS= read -r path; do
	printf '%s\t%s\n' "$path" "$(shasum -a 256 <"$path" | cut -d' ' -f1)"
done | shasum -a 256 | cut -d' ' -f1
