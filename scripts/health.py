#!/usr/bin/env python3

from __future__ import annotations

import argparse
import subprocess
import sys
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parent.parent


def run(command: list[str]) -> int:
    completed = subprocess.run(command, cwd=REPO_ROOT)
    return completed.returncode


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--test", action="store_true")
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args()

    selected = False

    if args.check or not args.test:
        selected = True
        if run(["./scripts/check.sh"]) != 0:
            return 1

    if args.test:
        selected = True
        if run(["go", "test", "."]) != 0:
            return 1

    if not selected:
        parser.print_usage(sys.stderr)
        return 2

    return 0


if __name__ == "__main__":
    raise SystemExit(main())