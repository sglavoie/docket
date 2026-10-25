"""Guided real-device qualification of the Mac and iPhone sync, recorded as
evidence the session-18 verifier validates.

Run through ios/scripts/record-device-qualification.sh, which documents the
commands. Everything happens in one disposable trial: a copy of the
representative fixture enrolled under its own HOME, a working copy under the
trial directory, an exchange folder of its own in iCloud Drive, and on the
iPhone a joined workspace beside any other. No real workspace is read or
written.

A person performs the iPhone steps and the Mac edits in the trial's own Docket;
this script prompts for them, exchanges the Mac, reads the iPhone's exported
diagnostics over the cable, and records raw observations: both devices'
synced-file digests, outstanding work, open conflicts, transport condition,
the build each device ran, and how many times each scenario's marker task
appears on the Mac. The verifier recomputes every verdict from those
observations; nothing here is trusted as a bare "passed".
"""

import os
import sys

# The shell launcher uses python3 -I, which excludes the script directory.
# Add only this trusted directory so the recorder's sibling package is found.
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from qualification.builds import cmd_reinstall
from qualification.commands import (
    cmd_day, cmd_enrollment, cmd_finish, cmd_start, cmd_status,
)
from qualification.core import fail
from qualification.mac import cmd_mac_capture
from qualification.procedures import cmd_procedure
from qualification.scenarios import cmd_scenario


COMMANDS = {"start": cmd_start, "reinstall": cmd_reinstall, "enrollment": cmd_enrollment, "mac-capture": cmd_mac_capture, "scenario": cmd_scenario, "day": cmd_day, "procedure": cmd_procedure,
            "status": cmd_status, "finish": cmd_finish}

if __name__ == "__main__":
    if len(sys.argv) < 2 or sys.argv[1] not in COMMANDS:
        fail("usage: record-device-qualification.sh start|enrollment|scenario NAME|day|procedure NAME|status|finish")
    COMMANDS[sys.argv[1]](sys.argv[2:])
