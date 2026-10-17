# /// script
# requires-python = ">=3.10"
# dependencies = ["rich==14.2.0"]
# ///
"""Build an ordered, offline replay of exported transition-review frames.

Usage: uv run scripts/render_transition_review.py FRAME_DIRECTORY
Native Ghostty PNGs are used when present; otherwise frames are rendered from
ANSI to SVG and explicitly labelled as such. No screen-capture access is needed
to inspect the synthetic rendering, and it is not native pixel evidence.
"""

import io
import json
import re
import sys
from pathlib import Path

from rich.console import Console
from rich.terminal_theme import TerminalTheme
from rich.text import Text


def main():
    root = Path(sys.argv[1]).resolve()
    groups = {}
    pattern = re.compile(
        r"(dark|light)-(\d+)x(\d+)-transition-(.+)-(\d+x\d+)-(\d+)-(.+)"
    )
    for source in sorted(root.glob("*-transition-*.ansi")):
        match = pattern.fullmatch(source.stem)
        if not match:
            raise ValueError(f"Unrecognized frame name: {source.name}")
        palette, columns, rows, sequence, initial_size, step, label = match.groups()
        png = source.with_suffix(".png")
        native = png.exists()
        destination = png if native else source.with_suffix(".svg")
        if not native:
            dark = palette == "dark"
            background = (26, 27, 38) if dark else (250, 250, 250)
            foreground = (192, 202, 245) if dark else (56, 58, 66)
            theme = TerminalTheme(background, foreground, [background, foreground] * 4)
            console = Console(file=io.StringIO(), record=True, width=int(columns), color_system="truecolor")
            console.print(Text.from_ansi(source.read_text()), end="", soft_wrap=True)
            svg = console.export_svg(title=f"tt · {columns}×{rows} · {label}", theme=theme)
            svg = re.sub(r"@font-face\s*\{[^}]*\}", "", svg)
            # Rich supplies a viewBox only. Give <img> its real intrinsic size
            # so the browser's default SVG dimensions do not shrink the replay.
            svg = re.sub(
                r'viewBox="0 0 ([\d.]+) ([\d.]+)"',
                lambda match: f'{match[0]} width="{match[1]}" height="{match[2]}"',
                svg,
                count=1,
            )
            destination.write_text(svg)
        key = f"{palette} · {initial_size} · {sequence}"
        groups.setdefault(key, []).append({
            "step": int(step), "label": label, "size": f"{columns}×{rows}",
            "file": destination.name, "source": "Ghostty pixels" if native else "ANSI rendering (not native pixels)",
        })
    if not groups:
        raise ValueError("No transition frames found")
    for frames in groups.values():
        frames.sort(key=lambda frame: frame["step"])
    data = json.dumps(groups, ensure_ascii=True).replace("<", "\\u003c")
    page = """<!doctype html>
<html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width">
<title>tt transition review</title>
<style>
body{margin:24px;background:#20222a;color:#eee;font:16px system-ui}
header{position:sticky;top:0;background:#20222a;padding:12px 0;z-index:1}
select,button,input{font:inherit;margin:4px;padding:6px}
button{cursor:pointer} #stage{overflow:auto;min-height:65vh}
img{display:block;max-width:none} #detail{margin:10px 4px;color:#cbd0dd}
</style>
<header><h1>tt transition review</h1>
<p>Follow the selected row, reading passage, and active draft across each sequence.
Left/right arrow keys step through frames. Images share one scale and top-left origin.</p>
<label>Sequence <select id="sequence"></select></label>
<button id="previous" aria-label="Previous frame">←</button>
<button id="play">Play</button><button id="next" aria-label="Next frame">→</button>
<label>Scale <select id="scale"><option value="0.5">50%</option><option value="0.75" selected>75%</option><option value="1">100%</option></select></label>
<div id="detail" aria-live="polite"></div>
<label>Frame <input type="range" id="frame" min="0" value="0"></label>
</header><div id="stage"><img id="image" alt="Terminal frame"></div>
<script>
const groups=DATA, selector=document.getElementById('sequence'), slider=document.getElementById('frame');
const picture=document.getElementById('image'), scale=document.getElementById('scale');
let frames=[], index=0, timer=null;
for(const name of Object.keys(groups)){const option=document.createElement('option');option.textContent=name;selector.append(option);}
function resize(){picture.style.width=(picture.naturalWidth*Number(scale.value))+'px';}
function show(){const item=frames[index];slider.value=index;picture.src=item.file;picture.alt=item.label+' at '+item.size;
document.getElementById('detail').textContent=(index+1)+' / '+frames.length+' · '+item.label+' · '+item.size+' · '+item.source;}
function stop(){clearInterval(timer);timer=null;document.getElementById('play').textContent='Play';}
function move(delta){index=(index+delta+frames.length)%frames.length;show();}
selector.onchange=()=>{stop();frames=groups[selector.value];index=0;slider.max=frames.length-1;show();};
slider.oninput=()=>{stop();index=Number(slider.value);show();};
document.getElementById('previous').onclick=()=>{stop();move(-1);};
document.getElementById('next').onclick=()=>{stop();move(1);};
document.getElementById('play').onclick=()=>{if(timer){stop();return;}timer=setInterval(()=>move(1),1000);document.getElementById('play').textContent='Pause';};
document.onkeydown=event=>{if(event.target.matches('select,input'))return;if(event.key==='ArrowRight'||event.key==='ArrowLeft'){event.preventDefault();stop();move(event.key==='ArrowRight'?1:-1);}};
picture.onload=resize;scale.onchange=resize;selector.onchange();
</script></html>""".replace("DATA", data)
    (root / "index.html").write_text(page)
    print(f"{sum(map(len, groups.values()))} frames in {len(groups)} sequences: {root / 'index.html'}")


if __name__ == "__main__":
    main()
