"""Mac task capture and editing through the trial terminal."""

import os
import signal
import time
from .core import (
    Trial,
    fail,
    note,
)


def drive_tui(trial, keys, binary=None):
    """Runs the Mac's trial tt in a pseudo-terminal, as a person at the
    keyboard would, sends each key string in turn, then Ctrl+C."""
    import pty
    import select
    import struct
    import fcntl
    import termios
    env = {**os.environ, "HOME": trial.home, "TERM": "xterm-256color"}
    pid, fd = pty.fork()
    if pid == 0:
        os.execve(binary or os.path.join(trial.dir, "bin", "tt"),
                  [binary or "tt", "--data-dir", trial.working], env)
    fcntl.ioctl(fd, termios.TIOCSWINSZ, struct.pack("HHHH", 40, 140, 0, 0))

    def drain(seconds):
        end = time.time() + seconds
        while time.time() < end:
            ready, _, _ = select.select([fd], [], [], 0.1)
            if ready:
                try:
                    os.read(fd, 65536)
                except OSError:
                    return

    drain(3.0)
    for key in list(keys) + ["\x03"]:
        os.write(fd, key.encode())
        drain(1.5)
    try:
        os.kill(pid, signal.SIGTERM)
    except ProcessLookupError:
        pass
    os.waitpid(pid, 0)
    os.close(fd)


def mac_capture(trial, line, binary=None):
    """Captures one line on the Mac (`a`, the line, Enter) and checks that its
    title now appears exactly once more."""
    title = line.split(" &", 1)[0]
    before = trial.marker_counts([title])[title]
    if before == 1:
        # A rerun of the same step after an interruption reuses its marker
        # rather than adding a second copy.
        note(f"Mac: {title!r} already exists; reusing it")
        return
    if before > 1:
        fail(f"the Mac already holds {before} copies of {title!r}: rerun the scenario with the next --attempt")
    drive_tui(trial, ["a", line, "\r"], binary)
    after = trial.marker_counts([title])[title]
    if after != before + 1:
        fail(f"the Mac capture of {title!r} did not land: {before} -> {after} copies")
    note(f"Mac: captured {title!r}")


def mac_rename(trial, old, new, binary=None):
    """Renames a task on the Mac: `/` search, Enter onto the result, `e` to
    edit inline, clear the line, type the new title, Enter."""
    drive_tui(trial, ["/", old, "\r", "e", "\x15", new, "\r"], binary)
    counts = trial.marker_counts([old, new])
    if counts[new] != 1 or counts[old] != 0:
        fail(f"the Mac rename of {old!r} to {new!r} did not land: {counts}")
    note(f"Mac: renamed {old!r} to {new!r}")


def cmd_mac_capture(args):
    """mac-capture LINE: one Mac capture in the trial, verified by count."""
    if not args:
        fail("usage: mac-capture LINE")
    mac_capture(Trial.current(), " ".join(args))


# Interaction.
