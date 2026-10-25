#!/usr/bin/env bash
#
# Run the real Mac/iPhone device qualification and write it as evidence the
# session-18 verifier validates. A person drives the iPhone and the Mac's trial
# Docket; this script builds and installs the signed app, enrolls a disposable
# copy of the representative fixture, prompts for each step, and records what
# both devices report after it. docs/mobile/personal-rollout.md is the guide.
#
#   ios/scripts/record-device-qualification.sh start
#   ios/scripts/record-device-qualification.sh scenario NAME
#       NAME: airplane_mode simultaneous_edits termination delayed_delivery
#             revoked_access large_attachments upgrade
#   ios/scripts/record-device-qualification.sh day          # once a day, at least 7 days
#   ios/scripts/record-device-qualification.sh procedure backup|recovery|troubleshooting
#   ios/scripts/record-device-qualification.sh status
#   ios/scripts/record-device-qualification.sh finish       # writes the evidence
#
# Environment:
#   DOCKET_IOS_DEVELOPMENT_TEAM        signing team (start, scenario upgrade)
#   DOCKET_IOS_DEVICE                  iPhone UDID (default: the first physical iPhone)
#   DOCKET_QUALIFICATION_ROOT          trial directories (default ~/.tt-qualification)
#   DOCKET_QUALIFICATION_EXCHANGE      exchange parent (default iCloud Drive/tt-qualification)
#   DOCKET_QUALIFICATION_UPGRADE_FROM  commit of the earlier build (scenario upgrade)
#   DOCKET_QUALIFICATION_EVIDENCE      output (default docs/mobile/evidence/session-18-qualification.json)
#
# The evidence is bound to the sources: changing the app, the bridge, or Docket
# during a trial makes it stale, so finish the trial on one tree.

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$repo_root"
[ "$(uname -s)" = Darwin ] || { echo "record-device-qualification: needs macOS" >&2; exit 1; }
exec python3 -I "$repo_root/ios/scripts/device_qualification.py" "$@"
