# /// script
# requires-python = ">=3.10"
# dependencies = ["rich==14.2.0"]
# ///
"""Render the README's exported ANSI frame as an SVG, preserving terminal styles.

Usage: uv run scripts/render_terminal_svg.py INPUT.ansi OUTPUT.svg
"""

import io
import re
import sys
from pathlib import Path

from rich.console import Console
from rich.terminal_theme import TerminalTheme
from rich.text import Text


def main():
    source, destination = map(Path, sys.argv[1:])
    screen = Text.from_ansi(source.read_text())
    console = Console(
        file=io.StringIO(), record=True, width=100, color_system="truecolor"
    )
    console.print(screen, end="", soft_wrap=True)
    # Match tt's dark palette; explicit ANSI colors retain their original values.
    palette = TerminalTheme(
        (26, 27, 38), (192, 202, 245),
        [(26, 27, 38), (247, 118, 142), (158, 206, 106), (224, 175, 104),
         (122, 162, 247), (187, 154, 247), (125, 207, 255), (192, 202, 245)],
    )
    destination.parent.mkdir(parents=True, exist_ok=True)
    svg = console.export_svg(title="tt — Terminal Tasks", theme=palette)
    # Keep the checked-in image self-contained; use the viewer's monospace font.
    svg = re.sub(r"@font-face\s*\{[^}]*\}", "", svg)
    destination.write_text(svg)


if __name__ == "__main__":
    main()
