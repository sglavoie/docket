#!/usr/bin/env bash
#
# Print the fingerprint of the Swift sources a recorded reminder delivery run
# is built from: the app, its reminder delivery UI test, the reminder,
# engine-adapter, and transport packages it links, and the project spec. The
# Go engine is pinned separately by the bridge fingerprint
# (ios/scripts/build-bridge.sh --fingerprint), so recorded evidence names
# exactly the builds it exercised.
#
#   ios/scripts/reminder-evidence-fingerprint.sh

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$repo_root"

{
	printf '%s\n' apple/TTSync/Package.swift ios/TTEngine/Package.swift ios/TTReminders/Package.swift ios/project.yml \
		ios/TTMobileUITests/ReminderDeliveryUITests.swift
	find apple/TTSync/Sources ios/TTMobile ios/TTEngine/Sources ios/TTReminders/Sources -type f ! -name .DS_Store
} | LC_ALL=C sort | while IFS= read -r path; do
	printf '%s\t%s\n' "$path" "$(shasum -a 256 <"$path" | cut -d' ' -f1)"
done | shasum -a 256 | cut -d' ' -f1
