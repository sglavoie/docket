"""Terminal and ASL mailbox prompts for the person running a trial."""

import json
import os
import re
from .core import (
    fail,
    run,
)


# With TT_QUALIFICATION_MAILBOX set to an ASL plan directory, every prompt is
# asked through that plan's mailbox (asl ask --wait) and the person's reply is
# the answer; otherwise the prompts read the terminal.
MAILBOX = os.environ.get("TT_QUALIFICATION_MAILBOX")
MAILBOX_WAIT_ROUNDS = 12  # 30 minutes each
_pending = []


def say(text):
    print("\n" + text.strip() + "\n")
    if MAILBOX:
        _pending.append(text.strip())


def human(question):
    """One answer from the person."""
    if not MAILBOX:
        return input(question + " ")
    body = "\n\n".join(_pending + [question])
    _pending.clear()
    argv = ["asl", "ask", MAILBOX, body, "--wait", "--wait-timeout", "1800", "--json"]
    for _ in range(MAILBOX_WAIT_ROUNDS):
        result = json.loads(run(argv, check=False).stdout or "{}")
        if result.get("reply"):
            return result["reply"].get("text", "")
        if result.get("resolution"):
            fail(f"the mailbox question was resolved without an answer: {result['resolution']}")
        if not result.get("timed_out") or not result.get("question_id"):
            fail(f"asl ask failed: {result}")
        argv = ["asl", "ask", MAILBOX, "--question-id", result["question_id"], "--wait", "--wait-timeout", "1800", "--json"]
    fail("no answer arrived through the mailbox")


def wait_enter(prompt="Press Enter when done."):
    answer = human(prompt if not MAILBOX else "Reply 'done' when you have finished these steps.")
    if MAILBOX and not re.match(r"\s*(done|yes|y|ok|okay|finished|completed)\b", answer, re.I):
        # Only an explicit completion counts; a refusal or a question must
        # never be recorded as steps performed.
        fail(f"the person did not confirm the steps: {answer!r}")


def ask_yes(question):
    while True:
        answer = human(question + " [y/n]").strip().lower()
        if answer.startswith("y"):
            return True
        if answer.startswith("n"):
            return False
