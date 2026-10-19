#!/usr/bin/env sh
# run/start/resume OUTPUT; status OUTPUT. No device, account, or prompts.
set -eu
exec python3 -I "$(dirname "$0")/simulator_qualification.py" "$@"
