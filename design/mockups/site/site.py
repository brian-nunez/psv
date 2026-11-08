"""PSV GitHub Pages landing page: desktop (1440) and phone (375) frames.

Example data is read from testdata/examples/*/report.json (read only). Text heights are
measured with figcraft-cli before the final build, so wrapped copy never overlaps.

    cd design/mockups/site && python3 site.py
"""
import json
import math
import os
import re
import subprocess
import sys

sys.dont_write_bytecode = True
HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.dirname(HERE))
from fcgen import B, shadow, lin, LH  # noqa: E402

ROOT = os.path.abspath(os.path.join(HERE, "..", "..", ".."))
CLI = os.path.expanduser("~/.local/bin/figcraft-cli")

# Daylight tokens (from the approved brief)
BG, SURF, LINE, SOFT = "#F7F6F3", "#FFFFFF", "#E8E5DF", "#F1EFEA"
INK, MUTED, FAINT = "#0F0F12", "#6B6A73", "#A09FA8"
ACCENT, LUM, RED, PASS = "#2346FF", "#E39B0B", "#E5384A", "#12A150"
LUM_BG, LUM_FG = "#FDF3DC", "#9A6400"
RED_BG, RED_FG = "#FDECEE", "#C21F33"   # text darkened from #E5384A for 4.5:1 at 12px
PASS_BG, PASS_FG = "#E2F5E9", "#0B7A3B"
MONO = "Menlo"
CODE_BG, CODE_HEAD = "#16161C", "#1E1E25"
CODE_FG, CODE_OUT, CODE_DIM, CODE_CMT = "#EEEDEA", "#C8C7CF", "#8B8A94", "#9A99A3"
MONO_ADV = 0.603  # Menlo advance width, em


# ---------------------------------------------------------------------------
# Text measurement: pass 1 records every wrapped text, figcraft measures them,
# pass 2 lays out with real heights.

class Measure:
    def __init__(self):
        self.cache = {}
        self.miss = {}

    @staticmethod
    def key(s, size, width, weight, font, lh, ls):
        return json.dumps([s, size, width, weight, font, lh, ls])

    def get(self, s, size, width=None, weight="Regular", font="Inter", lh=None, ls=None):
        k = self.key(s, size, width, weight, font, lh, ls)
        if k in self.cache:
            return self.cache[k]
        self.miss[k] = (s, size, width, weight, font, lh, ls)
        per = size * (MONO_ADV if font == MONO else 0.55)
        line = size * (float(lh[:-1]) / 100 if isinstance(lh, str) else LH)
        if width is None:
            return (len(s) * per, line * (s.count("\n") + 1))
        n = sum(max(1, math.ceil(len(p) * per / width)) for p in s.split("\n"))
        return (width, n * line)

    def resolve(self):
        if not self.miss:
            return False
        keys = list(self.miss)
        cmds = [{"command": "frame.create", "params": {"x": 0, "y": 0, "width": 2000, "height": 2000,
                                                        "props": {"fills": []}}}]
        for k in keys:
            s, size, width, weight, font, lh, ls = self.miss[k]
            props = {"fontSize": size, "fontFamily": font, "fontStyle": weight}
            if lh is not None:
                props["lineHeight"] = lh
            if ls is not None:
                props["letterSpacing"] = ls
            p = {"text": s, "x": 0, "y": 0, "parent": 2, "props": props}
            if width is not None:
                p["width"] = width
            cmds.append({"command": "text.create", "params": p})
        for i in range(len(keys)):
            cmds.append({"command": "node.get", "params": {"id": 3 + i}})
        path = os.path.join(HERE, ".measure.script.json")
        with open(path, "w") as f:
            json.dump(cmds, f)
        res = subprocess.run([CLI, "run", path], capture_output=True, text=True)
        os.remove(path)
        out = [json.loads(l) for l in res.stdout.splitlines() if l.startswith("{")]
        gets = [r for r in out if r["command"] == "node.get"]
        for k, r in zip(keys, gets):
            self.cache[k] = (r["result"]["width"], r["result"]["height"])
        self.miss = {}
        return True


M = Measure()


def tx(b, p, x, y, s, size=14, color=INK, weight="Regular", width=None, lh=None, ls=None,
       font="Inter", align=None, tnum=False, upper=False):
    """Create text and return its measured (width, height)."""
    b.text(p, x, y, s, size, color, weight, font=font, width=width, align=align, ls=ls, lh=lh,
           tnum=tnum, upper=upper)
    return M.get(s.upper() if upper else s, size, width, weight, font, lh, ls)


def tw(s, size, weight="Regular", font="Inter", ls=None):
    return M.get(s, size, None, weight, font, None, ls)[0]


def th(s, size, width, weight="Regular", lh=None, font="Inter"):
    return M.get(s, size, width, weight, font, lh, None)[1]


# ---------------------------------------------------------------------------
# Data

EXAMPLES = ["electric-soldier-porygon-strobes", "colors-epilepsy-warning", "psychedelic-flash-short",
            "video-ad-sample-2"]


def load(name):
    with open(os.path.join(ROOT, "testdata", "examples", name, "report.json")) as f:
        r = json.load(f)
    with open(os.path.join(ROOT, "testdata", "examples", name, "source.url")) as f:
        url = f.read().strip()
    return {
        "name": name, "url": url, "verdict": r["verdict"], "passed": r["passed"],
        "dur": r["durationSeconds"], "fps": r["frameRate"], "frames": r["frames"],
        "w": r["width"], "h": r["height"], "lumFail": r["failures"]["luminance"],
        "redFail": r["failures"]["red"], "defects": r["defects"], "version": r["tool"]["version"], "standard": r["standard"],
    }


def mmss(sec):
    s = int(round(sec))
    return f"{s // 60}:{s % 60:02d}"


def meta(e):
    port = " portrait" if e["h"] > e["w"] else ""
    return f"{mmss(e['dur'])}  ·  {e['w']}×{e['h']}{port}  ·  {e['fps']:g} fps"


def span_text(d):
    return f"{d['startSeconds']:.1f}–{d['endSeconds']:.1f} s"


def kind(d):
    return "Luminance + red" if d["red"] and d["luminance"] else ("Red" if d["red"] else "Luminance")


def kind_col(d):
    return RED if d["red"] else LUM


# ---------------------------------------------------------------------------
# Components

def soft_shadow():
    return [shadow("#0F0F120A", 2, 1), shadow("#0F0F120F", 24, 8)]


def panel(b, p, x, y, w, h, radius=18, fill=SURF):
    return b.frame(x, y, w, h, parent=p, fill=fill, radius=radius, stroke=LINE, effects=soft_shadow())


def eyebrow(b, p, x, y, s, color=MUTED, width=None, align=None):
    return tx(b, p, x, y, s, 11, color, "SemiBold", ls="8%", upper=True, width=width, align=align)


def pill(b, p, x, y, s, fg, bg, dot=None, h=24, size=12):
    w = tw(s, size, "SemiBold") + (30 if dot else 20)
    b.rect(p, x, y, w, h, fill=bg, radius=h / 2)
    tx0 = x + 10
    if dot:
        b.ellipse(p, x + 10, y + h / 2 - 3, 6, 6, fill=dot)
        tx0 = x + 20
    b.text(p, tx0, y + (h - size * LH) / 2, s, size, fg, "SemiBold")
    return w


def verdict_pill(b, p, x, y, e):
    if e["passed"]:
        return pill(b, p, x, y, "Passed", PASS_FG, PASS_BG, dot=PASS)
    return pill(b, p, x, y, "Failed", RED_FG, RED_BG, dot=RED)


def button(b, p, x, y, s, kind="ghost", h=44, size=15, w=None, icon=None):
    content = tw(s, size, "SemiBold") + (24 if icon else 0)
    w = w or content + 40
    if kind == "primary":
        b.rect(p, x, y, w, h, fill=INK, radius=h / 2, effects=[shadow("#0F0F1233", 12, 4)])
        fg = "#FFFFFF"
    else:
        b.rect(p, x, y, w, h, fill=SURF, radius=h / 2, stroke=LINE)
        fg = INK
    sx = x + (w - content) / 2
    tx0 = sx
    if icon == "download":
        cx, cy = sx, y + h / 2 - 8
        b.vector(p, "M8 1 L8 11 M3.5 6.5 L8 11 L12.5 6.5 M2 15 L14 15", cx, cy, 16, 16, stroke=fg, sw=1.8)
        tx0 = sx + 24
    elif icon == "github":
        cx, cy = sx, y + h / 2 - 8
        b.ellipse(p, cx, cy, 16, 16, stroke=fg, sw=1.6)
        b.vector(p, "M5 12 L5 9 Q5 7 8 7 Q11 7 11 9 L11 12", cx, cy + 1, 6, 5, stroke=fg, sw=1.6)
        tx0 = sx + 24
    b.text(p, tx0, y + (h - size * LH) / 2, s, size, fg, "SemiBold")
    return w


def link(b, p, x, y, s, size=14, color=ACCENT, width=None, align=None):
    return tx(b, p, x, y, s, size, color, "SemiBold", width=width, align=align)


def logo(b, p, x, y, s=28):
    b.rect(p, x, y, s, s, radius=s * 56 / 256, fills=[lin(["#2346FF", "#1B2FB3"])])
    r = s * 58 / 256
    b.ellipse(p, x + s / 2 - r, y + s / 2 - r, 2 * r, 2 * r, stroke="#FFFFFF", sw=s * 20 / 256)
    dr = s * 20 / 256
    b.ellipse(p, x + s * 192 / 256 - dr, y + s * 64 / 256 - dr, 2 * dr, 2 * dr, fill=RED)


def copy_button(b, p, x, y):
    """Copy affordance, right-aligned at x (its right edge)."""
    w = 70
    b.rect(p, x - w, y, w, 26, fill="#FFFFFF14", radius=13, stroke="#FFFFFF1F")
    b.rect(p, x - w + 12, y + 9, 8, 9, radius=2, stroke=CODE_OUT, sw=1.3)
    b.rect(p, x - w + 15, y + 6, 8, 9, radius=2, stroke=CODE_OUT, sw=1.3, fill=CODE_HEAD)
    b.text(p, x - w + 30, y + 6, "Copy", 12, CODE_OUT, "SemiBold")


# Syntax highlighting: each line is one text node; token colours are applied as
# ranged fills (selection.setPaint with a character range), so runs cannot drift.
# Contrast on CODE_BG #16161C (WCAG formula): default 15.4, output 10.8, keyword 9.1,
# function 8.7, type 10.0, string 10.8, number 9.1, comment 6.4, prompt 5.3.
TOK = {
    "kw": "#A9B4FF",    # Go keywords, shell flags, JSON keys: light cobalt
    "fn": "#7CB8FF",    # Go calls, shell command names: light blue
    "type": "#5FD4C4",  # psv package identifier and types, shell | > &&: teal
    "str": "#9AD88A",   # strings, URLs: calm green
    "num": "#CFA6FF",   # numbers: soft violet
    "cmt": CODE_CMT,    # comments: dim grey
}
GO_KW = {"package", "import", "func", "for", "if", "else", "return", "range", "const", "defer", "var",
         "break", "continue", "type", "struct"}
GO_TYPES = {"byte", "int", "bool", "string", "error", "float64"}
GO_RE = re.compile(r'(?P<cmt>//.*)|(?P<str>"(?:\\.|[^"\\])*"|`[^`]*`)|(?P<num>\b\d+(?:\.\d+)?\b)'
                   r'|(?P<id>[A-Za-z_]\w*)')
JSON_RE = re.compile(r'(?P<key>"(?:\\.|[^"\\])*"(?=\s*:))|(?P<str>"(?:\\.|[^"\\])*")'
                     r'|(?P<num>(?<![\w.])-?\d+(?:\.\d+)?(?![\w.]))')
SH_OPS = {"|", ">", ">>", "&&"}


def go_tokens(line):
    out = []
    for m in GO_RE.finditer(line):
        s, e = m.span()
        kind = m.lastgroup
        if kind != "id":
            out.append((s, e, kind))
            continue
        w = m.group()
        nxt = line[e:e + 1]
        if w in GO_KW:
            out.append((s, e, "kw"))
        elif w == "psv" and nxt == ".":
            out.append((s, e, "type"))
        elif nxt == "(":
            out.append((s, e, "fn"))
        elif (line[:s].endswith("psv.") and w[:1].isupper()) or w in GO_TYPES:
            out.append((s, e, "type"))
    return out


def sh_tokens(line):
    out = []
    end = len(line)
    if line.lstrip().startswith("#"):
        return [(0, end, "cmt")]
    if "  #" in line:
        end = line.index("  #") + 2
        out.append((end, len(line), "cmt"))
    cmd = True
    for m in re.finditer(r"'[^']*'|\"[^\"]*\"|\S+", line[:end]):
        s, e = m.span()
        w = m.group()
        if w in SH_OPS:
            out.append((s, e, "type"))
            cmd = w in ("|", "&&")
        elif cmd:
            out.append((s, e, "fn"))
            cmd = False
        elif w[:1] in "'\"" or "://" in w:
            out.append((s, e, "str"))
        elif w.startswith("-") and len(w) > 1:
            out.append((s, e, "kw"))
    return out


def json_tokens(line):
    role = {"key": "kw", "str": "str", "num": "num"}
    return [(m.start(), m.end(), role[m.lastgroup]) for m in JSON_RE.finditer(line)]


def code_line(b, p, x, y, line, size, color, lang):
    """One mono line as one text node, highlighted with ranged fills."""
    if not line.strip():
        return
    tid = b.text(p, x, y, line, size, color, font=MONO)
    if lang == "go":
        toks = go_tokens(line)
    elif lang == "sh":
        toks = sh_tokens(line)
    elif lang == "text" and line.lstrip().startswith("{"):
        toks = json_tokens(line)
    else:
        toks = []
    for s, e, role in toks:
        b.raw("selection.setPaint", {"targets": [{"id": tid, "fills": True, "index": 0, "range": [s, e]}],
                                     "paint": TOK[role]})


def code(b, p, x, y, w, lines, label, lang="sh", output=None, m=False, prompt=True, copy=True):
    """Dark code block with a header (label + Copy), commands, and optional real output."""
    size = 12 if m else 13
    lh = 19 if m else 21
    pad = 16 if m else 22
    head = 40
    lines = [l.replace("\t", "    ") for l in lines]
    out = [l.replace("\t", "    ") for l in (output or [])]
    body_h = pad * 2 - 4 + lh * len(lines)
    out_h = (34 + lh * len(out) + pad - 4) if out else 0
    longest = max(len(l) + (2 if prompt and lang == "sh" else 0) for l in lines + out)
    overflow = longest * size * MONO_ADV > w - pad * 2
    scroll_h = 14 if overflow else 0
    h = head + body_h + out_h + scroll_h
    f = b.frame(x, y, w, h, parent=p, fill=CODE_BG, radius=14, name=("Code · " + label + " · " + (lines[0] if lines[0].strip() else lines[1])[:48]) + (" · phone" if m else ""))
    b.rect(f, 0, 0, w, head, fill=CODE_HEAD)
    b.text(f, pad, 13, label, 11, CODE_DIM, "SemiBold", ls="8%", upper=True)
    if copy:
        copy_button(b, f, w - 8, 7)
    yy = head + pad - 2
    for l in lines:
        lx = pad
        if prompt and lang == "sh" and l.strip() and not l.lstrip().startswith("#"):
            b.text(f, pad, yy, "$", size, CODE_DIM, font=MONO)
            lx = pad + 2 * size * MONO_ADV
        code_line(b, f, lx, yy, l, size, CODE_FG, lang)
        yy += lh
    if out:
        yy = head + body_h
        b.rect(f, 0, yy, w, 1, fill="#FFFFFF14")
        b.text(f, pad, yy + 12, "Output", 11, CODE_DIM, "SemiBold", ls="8%", upper=True)
        yy += 34
        for l in out:
            code_line(b, f, pad, yy, l, size, CODE_OUT, "text")
            yy += lh
    if overflow:
        b.rect(f, w - 44, head, 44, h - head - scroll_h, fills=[lin(["#16161C00", CODE_BG])])
        tw_ = w - pad * 2
        thumb = max(40, tw_ * (w - pad * 2) / (longest * size * MONO_ADV))
        b.rect(f, pad, h - 10, tw_, 4, fill="#FFFFFF10", radius=2)
        b.rect(f, pad, h - 10, thumb, 4, fill="#FFFFFF40", radius=2)
    return h


def section_head(b, p, x, y, w, eyebrow_s, title, lede, m, lede_w=680):
    eyebrow(b, p, x, y, eyebrow_s, ACCENT)
    y += 24
    size = 28 if m else 38
    _, h = tx(b, p, x, y, title, size, INK, "Bold", width=w, ls="-2.5%", lh="115%")
    y += h + (10 if m else 14)
    if lede:
        _, h = tx(b, p, x, y, lede, 16 if m else 18, MUTED, width=min(w, lede_w), lh="155%")
        y += h
    return y


# ---------------------------------------------------------------------------
# Example cards. Each preview is a screenshot of the real report UI (psv render),
# cropped to its verdict, metrics and "When it flashes" overview, in a light
# browser frame. See img/ and README.md for how they were captured.

IMG = os.path.join(HERE, "img")
IMG_SIZE = {"featured": (2240, 1688), "row": (1920, 1400), "phone": (1088, 1340)}
REPORT_BG = "#F7F6F3"


def summary(e):
    ds = e["defects"]
    if not ds:
        return "No failing moments."
    a, z = ds[0]["startSeconds"], ds[-1]["endSeconds"]
    if len(ds) > 1:
        return f"{len(ds)} failing moments between {a:.1f} s and {z:.1f} s."
    d = ds[0]
    whole = d["firstFrame"] <= 2 and d["lastFrame"] >= e["frames"] - 1
    return f"1 failing moment, {a:.1f}–{z:.1f} s" + (": almost the whole video." if whole else ".")


def stat(b, p, x, y, label, value, sub, col=INK, size=26):
    eyebrow(b, p, x, y, label)
    b.text(p, x, y + 20, value, size, col, "SemiBold", ls="-2%", tnum=True)
    b.text(p, x, y + 20 + size * LH + 2, sub, 12, MUTED)


def counts(b, p, x, y, w, e, size=22, n=2):
    items = [("Luminance", e["lumFail"], LUM_FG), ("Red", e["redFail"], RED_FG)]
    sw = w / n
    for i, (k, v, col) in enumerate(items):
        stat(b, p, x + i * sw, y, k, str(v), "failing frames", col if v else INK, size=size)
    return 20 + size * LH + 2 + 15


def moments_grid(b, p, x, y, w, e, m):
    ds = e["defects"]
    if m:
        rh = 34
        for i, d in enumerate(ds):
            yy = y + i * rh
            if i:
                b.rect(p, x, yy, w, 1, fill=LINE)
            b.ellipse(p, x, yy + 8, 18, 18, fill=INK)
            b.text(p, x, yy + 11, str(d["index"]), 10, "#FFFFFF", "Bold", width=18, align="center")
            b.text(p, x + 28, yy + 9, span_text(d), 14, INK, "SemiBold", tnum=True)
            k = kind(d)
            kw = tw(k, 12, "Medium")
            b.rect(p, x + w - kw - 20, yy + 13, 8, 8, fill=kind_col(d), radius=2)
            b.text(p, x + w - kw, yy + 10, k, 12, MUTED, "Medium")
        return len(ds) * rh
    cols = 4
    cw = (w - (cols - 1) * 12) / cols
    for i, d in enumerate(ds):
        cx = x + (i % cols) * (cw + 12)
        cy = y + (i // cols) * 58
        b.rect(p, cx, cy, cw, 48, fill=SOFT, radius=10)
        b.ellipse(p, cx + 12, cy + 15, 18, 18, fill=INK)
        b.text(p, cx + 12, cy + 18, str(d["index"]), 10, "#FFFFFF", "Bold", width=18, align="center")
        b.text(p, cx + 40, cy + 8, span_text(d), 14, INK, "SemiBold", tnum=True)
        b.rect(p, cx + 40, cy + 30, 8, 8, fill=kind_col(d), radius=2)
        b.text(p, cx + 54, cy + 27, kind(d), 12, MUTED, "Medium")
    return math.ceil(len(ds) / cols) * 58 - 10


def browser(b, p, x, y, w, e, variant):
    """A light browser window holding the report screenshot; returns its height."""
    iw, ih = IMG_SIZE[variant]
    bar = 32
    h_img = w * ih / iw
    fr = b.frame(x, y, w, bar + h_img, parent=p, fill=REPORT_BG, radius=12, stroke=LINE)
    b.rect(fr, 0, 0, w, bar, fill=SOFT)
    for i in range(3):
        b.ellipse(fr, 14 + i * 14, bar / 2 - 4, 8, 8, fill="#D9D5CD")
    url = e["name"] + ".html"
    if tw(url, 11, "Medium") + 24 > w - 120:
        url = "report.html"
    uw = tw(url, 11, "Medium") + 28
    b.rect(fr, (w - uw) / 2, 7, uw, bar - 14, fill=SURF, radius=(bar - 14) / 2)
    b.text(fr, (w - uw) / 2, 7 + (bar - 14 - 11 * LH) / 2, url, 11, MUTED, "Medium", width=uw, align="center")
    b._add("image.place", {"path": os.path.join(IMG, f"{e['name']}-{variant}.png"), "x": 0, "y": bar,
                           "width": w, "height": h_img, "parent": fr, "mode": "fill"})
    b.rect(fr, 0, bar - 1, w, 1, fill=LINE)
    fade = min(72, h_img * 0.16)
    b.rect(fr, 0, bar + h_img - fade, w, fade, fills=[lin([REPORT_BG + "00", REPORT_BG], horizontal=False)])
    return bar + h_img


def open_button(b, p, x, y, w=None):
    return button(b, p, x, y, "Open the full report →", "primary", w=w)


def featured_card(b, f, x, y, w, e):
    """Desktop: the hard case, full width."""
    pad = 32
    c = panel(b, f, x, y, w, 100)
    verdict_pill(b, c, pad, pad, e)
    tx(b, c, pad, pad + 36, e["name"], 24, INK, "SemiBold", ls="-1%")
    b.text(c, pad, pad + 72, meta(e) + f"  ·  {e['frames']} frames", 14, MUTED, tnum=True)
    b.text(c, pad, pad + 100, summary(e), 15, INK, "Medium")
    sx = w - pad - 3 * 150
    stat(b, c, sx, pad + 4, "Moments", str(len(e["defects"])), "that fail")
    stat(b, c, sx + 150, pad + 4, "Luminance", str(e["lumFail"]), "failing frames", LUM_FG)
    stat(b, c, sx + 300, pad + 4, "Red", str(e["redFail"]), "failing frames", RED_FG)
    yy = pad + 148
    yy += browser(b, c, pad, yy, w - pad * 2, e, "featured") + 28
    eyebrow(b, c, pad, yy, f"The {len(e['defects'])} moments")
    yy += 26
    yy += moments_grid(b, c, pad, yy, w - pad * 2, e, False) + 28
    b.rect(c, pad, yy, w - pad * 2, 1, fill=LINE)
    yy += 24
    bw = open_button(b, c, pad, yy)
    b.text(c, pad + bw + 20, yy + 13, "The full report adds every frame, each moment in detail and where on "
           "screen it flashes.", 14, MUTED)
    yy += 44 + pad
    b.raw("node.set", {"ids": [c], "props": {"height": yy}})
    return yy


def row_card(b, f, x, y, w, e):
    """Desktop: details on the left, the report preview on the right."""
    pad = 28
    lw = 300
    c = panel(b, f, x, y, w, 100)
    bx = pad * 2 + lw
    bh = browser(b, c, bx, pad, w - bx - pad, e, "row")
    verdict_pill(b, c, pad, pad, e)
    yy = pad + 38
    _, h = tx(b, c, pad, yy, e["name"], 22, INK, "SemiBold", width=lw, ls="-1%", lh="125%")
    yy += h + 6
    _, h = tx(b, c, pad, yy, meta(e), 14, MUTED, tnum=True)
    yy += h + 18
    _, h = tx(b, c, pad, yy, summary(e), 15, INK, "Medium", width=lw, lh="150%")
    yy += h + 22
    b.rect(c, pad, yy, lw, 1, fill=LINE)
    yy += 20
    yy += counts(b, c, pad, yy, lw, e) + 28
    H = max(pad * 2 + bh, yy + 44 + pad)
    open_button(b, c, pad, yy, w=lw)
    b.raw("node.set", {"ids": [c], "props": {"height": H}})
    return H


def phone_card(b, f, x, y, w, e):
    pad = 16
    iw = w - pad * 2
    c = panel(b, f, x, y, w, 100)
    verdict_pill(b, c, pad, pad + 4, e)
    yy = pad + 40
    _, h = tx(b, c, pad, yy, e["name"], 18, INK, "SemiBold", width=iw, ls="-1%")
    yy += h + 4
    _, h = tx(b, c, pad, yy, meta(e), 13, MUTED, tnum=True)
    yy += h + 10
    _, h = tx(b, c, pad, yy, summary(e), 14, INK, "Medium", width=iw, lh="150%")
    yy += h + 16
    yy += browser(b, c, pad, yy, iw, e, "phone") + 20
    if len(e["defects"]) > 1:
        eyebrow(b, c, pad, yy, f"The {len(e['defects'])} moments")
        yy += 20
        yy += moments_grid(b, c, pad, yy, iw, e, True) + 12
        b.rect(c, pad, yy, iw, 1, fill=LINE)
        yy += 16
    yy += counts(b, c, pad, yy, iw, e) + 18
    open_button(b, c, pad, yy, w=iw)
    yy += 44 + pad + 4
    b.raw("node.set", {"ids": [c], "props": {"height": yy}})
    return yy


# ---------------------------------------------------------------------------
# Page sections

def nav(b, f, W, x0, cw, m):
    h = 60 if m else 72
    b.rect(f, 0, 0, W, h, fill=BG)
    b.rect(f, 0, h - 1, W, 1, fill=LINE)
    logo(b, f, x0, (h - 28) / 2)
    b.text(f, x0 + 38, (h - 19 * LH) / 2, "psv", 19, INK, "Bold", ls="-2%")
    if m:
        button(b, f, x0 + cw - 102, (h - 36) / 2, "GitHub", h=36, size=14, w=102, icon="github")
        return h
    xx = x0 + 200
    for s in ["Examples", "What it checks", "Use it", "Docs"]:
        b.text(f, xx, 27, s, 14, MUTED, "Medium")
        xx += tw(s, 14, "Medium") + 32
    pill(b, f, x0 + cw - 220, 24, "v0.1.0", MUTED, SOFT)
    button(b, f, x0 + cw - 128, 16, "GitHub", h=40, size=14, w=128, icon="github")
    return h


INSTALL = [
    "curl -fLO https://github.com/brian-nunez/psv/releases/latest/download/psv-darwin-arm64",
    "curl -fLO https://github.com/brian-nunez/psv/releases/latest/download/SHA256SUMS",
    "shasum -a 256 -c --ignore-missing SHA256SUMS   # psv-darwin-arm64: OK",
    "chmod +x psv-darwin-arm64 && mv psv-darwin-arm64 /usr/local/bin/psv",
    "psv -version                                   # psv v0.1.0",
]

WCAG_QUOTE = ("“Web pages do not contain anything that flashes more than three times in any one "
              "second period, or the flash is below the general flash and red flash thresholds.”")


def hero(b, f, x0, cw, y, m):
    if m:
        y += 32
        xx = x0
        xx += pill(b, f, xx, y, "v0.1.0", MUTED, SOFT) + 8
        pill(b, f, xx, y, "Proof of concept", MUTED, SOFT)
        y += 40
        _, h = tx(b, f, x0, y, "Photosensitive epilepsy checker", 38, INK, "Bold", width=cw, ls="-3%",
                  lh="108%")
        y += h + 16
        _, h = tx(b, f, x0, y, "A Go library, CLI and API server that finds hazardous flashing in video "
                  "against WCAG 2.2 and broadcast guidelines.", 17, MUTED, width=cw, lh="150%")
        y += h + 24
        button(b, f, x0, y, "Download v0.1.0", "primary", w=cw, icon="download")
        y += 56
        button(b, f, x0, y, "See the examples", w=cw)
        y += 44 + 32
        y += wcag_box(b, f, x0, y, cw, m) + 32
        eyebrow(b, f, x0, y, "Quick start  ·  macOS arm64")
        y += 22
        y += code(b, f, x0, y, cw, INSTALL, "Terminal", m=True) + 12
        _, h = tx(b, f, x0, y, "Linux, macOS and Windows on amd64 and arm64. Needs ffmpeg and ffprobe "
                  "on PATH.", 13, MUTED, width=cw, lh="150%")
        return y + h
    y += 72
    xx = x0
    xx += pill(b, f, xx, y, "v0.1.0", MUTED, SOFT) + 8
    xx += pill(b, f, xx, y, "Proof of concept", MUTED, SOFT) + 8
    pill(b, f, xx, y, "MIT", MUTED, SOFT)
    top = y
    y += 44
    _, h = tx(b, f, x0, y, "Photosensitive epilepsy checker", 56, INK, "Bold", width=660, ls="-3%",
              lh="104%")
    y += h + 20
    _, h = tx(b, f, x0, y, "A Go library, CLI and API server that finds hazardous flashing in video "
              "against WCAG 2.2 and broadcast guidelines.", 20, MUTED, width=620, lh="150%")
    y += h + 32
    w1 = button(b, f, x0, y, "Download v0.1.0", "primary", icon="download")
    w2 = button(b, f, x0 + w1 + 12, y, "View on GitHub", icon="github")
    link(b, f, x0 + w1 + w2 + 36, y + 12, "See the examples ↓", 15)
    y += 44
    wcag_box(b, f, x0 + cw - 380, top + 44, 380, m)
    y += 64
    eyebrow(b, f, x0, y, "Quick start  ·  macOS arm64")
    b.text(f, x0, y, "Linux, macOS and Windows on amd64 and arm64  ·  needs ffmpeg and ffprobe on PATH",
           13, MUTED, width=cw, align="right")
    y += 24
    y += code(b, f, x0, y, cw, INSTALL, "Terminal")
    return y


def wcag_box(b, f, x, y, w, m):
    pad = 20 if m else 24
    c = panel(b, f, x, y, w, 100)
    eyebrow(b, c, pad, pad, "WCAG 2.2  ·  Success Criterion 2.3.1", ACCENT)
    yy = pad + 24
    _, h = tx(b, c, pad, yy, WCAG_QUOTE, 15, INK, "Medium", width=w - pad * 2, lh="150%")
    yy += h + 14
    b.rect(c, pad, yy, w - pad * 2, 1, fill=LINE)
    yy += 14
    _, h = tx(b, c, pad, yy, "Three flashes are six alternating transitions. By default PSV fails a "
              "video at more than six in any one second, over at least a quarter of any 341×256 px "
              "region.", 13, MUTED, width=w - pad * 2, lh="150%")
    yy += h + pad
    b.raw("node.set", {"ids": [c], "props": {"height": yy}})
    return yy


def showcase(b, f, x0, cw, y, m, ex):
    y = section_head(b, f, x0, y, cw, "Examples", "Four real videos, as PSV reported them",
                     "Each preview is the real psv report UI, made by psv render from the example's "
                     "report.json. No video is attached, so no frame of any video appears on this page.", m)
    y += 32 if m else 40
    if m:
        for e in ex:
            y += phone_card(b, f, x0, y, cw, e) + 16
    else:
        y += featured_card(b, f, x0, y, cw, ex[0]) + 24
        for e in ex[1:]:
            y += row_card(b, f, x0, y, cw, e) + 24
    y += 24
    y += warning(b, f, x0, y, cw, m, ex, expanded=False)
    return y


# Photosensitive seizure warning around the source links. Colours on WARN_BG:
# WARN_INK 7.6:1, WARN_TEXT 15.3:1, white on WARN_TAG 5.9:1 (measured, WCAG formula).
WARN_BG, WARN_INK, WARN_TEXT, WARN_TAG, WARN_SOFT = "#FFF1F2", "#9B1428", "#3D0A14", "#C21F33", "#F7C6CC"
WARN_P1 = ("Three of the four example videos contain intense flashing, including red flashing, that can "
           "trigger seizures in people with photosensitive epilepsy.")
WARN_P2 = ("Do not open the sources if you, or anyone who can see your screen, may be at risk. "
           "The full reports above contain no video.")


def warn_icon(b, p, x, y, s):
    b.vector(p, "M31 0 L62 54 L0 54 Z", x, y, s, s * 54 / 62, fill=RED)
    bw = s * 0.09
    b.rect(p, x + s / 2 - bw / 2, y + s * 0.26, bw, s * 0.27, fill="#FFFFFF", radius=bw / 2)
    b.ellipse(p, x + s / 2 - bw * 0.6, y + s * 0.6, bw * 1.2, bw * 1.2, fill="#FFFFFF")


def chevron(b, p, x, y, up, color):
    d = "M0 6 L5 1 L10 6" if up else "M0 1 L5 6 L10 1"
    b.vector(p, d, x, y, 10, 7, stroke=color, sw=1.8)


def warning(b, f, x, y, w, m, ex, expanded):
    pad = 20 if m else 40
    c = b.frame(x, y, w, 100, parent=f, fill=WARN_BG, radius=20, stroke=RED, sw=2)
    b.rect(c, 0, 0, w, 8, fill=RED)
    isz = 44 if m else 68
    if m:
        warn_icon(b, c, pad, pad + 10, isz)
        tx0, yy = pad, pad + 10 + isz + 14
        tw_ = w - pad * 2
        _, h = tx(b, c, tx0, yy, "Photosensitive seizure warning", 30, WARN_INK, "Bold", width=tw_, ls="-2%",
                  lh="112%")
    else:
        warn_icon(b, c, pad, pad + 14, isz)
        tx0, yy = pad + isz + 28, pad + 10
        tw_ = min(w - tx0 - pad, 820)
        _, h = tx(b, c, tx0, yy, "Photosensitive seizure warning", 40, WARN_INK, "Bold", width=tw_, ls="-2.5%",
                  lh="112%")
    yy += h + 14
    for s in (WARN_P1, WARN_P2):
        _, h = tx(b, c, tx0, yy, s, 16 if m else 18, WARN_TEXT, "Medium" if s is WARN_P2 else "Regular",
                  width=tw_, lh="155%")
        yy += h + 10
    yy += 14
    # disclosure control
    label = "Hide source links" if expanded else "I understand the risk — show source links"
    size = 15
    content = tw(label, size, "SemiBold") + 22
    bw = w - pad * 2 if m else content + 48
    lines = 1
    if content > bw - 32:
        lines = 2
    bh = 48 if lines == 1 else 64
    b.rect(c, tx0, yy, bw, bh, fill=SURF, radius=bh / 2 if lines == 1 else 18, stroke=WARN_TAG, sw=2)
    if lines == 1:
        lx = tx0 + (bw - content) / 2
        b.text(c, lx, yy + (bh - size * LH) / 2, label, size, WARN_INK, "SemiBold")
        chevron(b, c, lx + content - 10, yy + bh / 2 - 3, expanded, WARN_INK)
    else:
        b.text(c, tx0 + 24, yy + 12, label, size, WARN_INK, "SemiBold", width=bw - 64, align="center", lh="135%")
        chevron(b, c, tx0 + bw - 32, yy + bh / 2 - 3, expanded, WARN_INK)
    yy += bh + 12
    if not expanded:
        _, h = tx(b, c, tx0, yy, "The source links stay hidden until you confirm.", 14, WARN_TEXT, width=tw_)
        yy += h + pad
    else:
        yy += 8
        lw_ = w - tx0 - pad
        lst = b.frame(tx0, yy, lw_, 100, parent=c, fill=SURF, radius=14, stroke=WARN_SOFT)
        ly = 0
        for i, e in enumerate(ex):
            if i:
                b.rect(lst, 0, ly, lw_, 1, fill=WARN_SOFT)
            flashing = not e["passed"]
            ip = 16 if m else 20
            if m:
                b.text(lst, ip, ly + 14, e["name"], 15, INK, "SemiBold")
                b.text(lst, ip, ly + 40, "YouTube ↗", 14, WARN_INK, "SemiBold")
                if flashing:
                    tag(b, lst, lw_ - ip, ly + 38)
                ly += 72
            else:
                b.text(lst, ip, ly + 18, e["name"], 16, INK, "SemiBold")
                b.text(lst, ip + 330, ly + 19, e["url"].replace("https://www.", "") + "  ↗", 14, WARN_INK,
                       "SemiBold")
                if flashing:
                    tag(b, lst, lw_ - ip, ly + 16)
                ly += 58
        b.raw("node.set", {"ids": [lst], "props": {"height": ly}})
        yy += ly + pad
    b.raw("node.set", {"ids": [c], "props": {"height": yy}})
    return yy


def tag(b, p, right, y):
    s = "Contains flashing"
    w = tw(s, 12, "Bold") + 36
    b.rect(p, right - w, y, w, 26, fill=WARN_TAG, radius=13)
    b.vector(p, "M6 0 L12 10.5 L0 10.5 Z", right - w + 10, y + 7, 12, 10.5, fill="#FFFFFF")
    b.rect(p, right - w + 15.3, y + 10, 1.4, 4, fill=WARN_TAG)
    b.text(p, right - w + 28, y + 6, s, 12, "#FFFFFF", "Bold")


PRESETS = [
    ("wcag22", "ΔY\u00a0≥\u00a00.1, darker\u00a0<\u00a00.8", "Δu'v'\u00a0≥\u00a00.2, one side\u00a0R/(R+G+B)\u00a0≥\u00a00.8",
     "≥ 25% of any 341×256 px region", "> 6 alternating transitions in 1 s"),
    ("trace24", "as WCAG", "as WCAG", "≥ 25% of any 416×416 px region", "as WCAG"),
    ("broadcast", "as WCAG", "as WCAG", "≥ 25% of the screen", "as WCAG"),
    ("scene", "as WCAG", "as WCAG", "≥ 80% of the screen", "> 3 alternating transitions in 1 s"),
]
PRESET_NAMES = {"wcag22": "WCAG 2.2 SC 2.3.1", "trace24": "Trace R&D Center 2024",
                "broadcast": "ITU-R BT.1702, Ofcom, ISO 9241-391", "scene": "Experimental scene change"}
LIMITS = ("PSV checks flashing, not every visual hazard. Regular spatial patterns such as stripes and "
          "checkerboards are a separate hazard and are not checked, and display brightness, HDR and "
          "viewing distance are not modelled.")
BENCH = ("Tested against the 346 videos of the Trace RERC PSE test media, built to sit just above or "
         "just below each threshold of each standard: 378 checks, 369 pass, 0 fail, and 9 are skipped "
         "because their listing entries contradict the thresholds they test.")


def standards(b, f, x0, cw, y, m):
    y = section_head(b, f, x0, y, cw, "What it checks", "Four standards, one detector",
                     "PSV finds general (luminance) flashes and red flashes, measures how much of the screen "
                     "flashes together and how often, and reports a pass or fail verdict with the frames "
                     "and screen regions involved.", m)
    y += 32
    if m:
        for name, lum, red, area, count in PRESETS:
            pad = 18
            c = panel(b, f, x0, y, cw, 100, radius=14)
            b.text(c, pad, pad, name, 15, INK, "Bold", font=MONO)
            if name == "wcag22":
                pill(b, c, pad + tw(name, 15, "Bold", MONO) + 10, pad - 3, "default", ACCENT, "#E8ECFF", h=22,
                     size=11)
            yy = pad + 22
            _, h = tx(b, c, pad, yy, PRESET_NAMES[name], 13, MUTED, width=cw - pad * 2)
            yy += h + 12
            for k, v in [("Luminance", lum), ("Red", red), ("Area", area), ("Count", count)]:
                b.text(c, pad, yy + 1, k, 12, MUTED, "Medium")
                v = v.replace(", one side\u00a0", ",\none side\u00a0")
                _, h = tx(b, c, pad + 84, yy, v, 14, INK, width=cw - pad * 2 - 84, lh="140%")
                yy += max(h, 18) + 8
            yy += pad - 8
            b.raw("node.set", {"ids": [c], "props": {"height": yy}})
            y += yy + 12
        y += 4
    else:
        cols = [("Preset", 240), ("Luminance", 180), ("Red", 300), ("Area", 200), ("Fails when", 200)]
        c = panel(b, f, x0, y, cw, 100)
        b.rect(c, 0, 0, cw, 44, fill=SOFT)
        xx = 24
        for s, w in cols:
            eyebrow(b, c, xx, 16, s)
            xx += w
        yy = 44
        for i, row in enumerate(PRESETS):
            name = row[0]
            hs = [th(v, 14, w - 24, lh="140%") for v, (_, w) in zip(row[1:], cols[1:])]
            rh = max(max(hs), 40) + 32
            if i:
                b.rect(c, 0, yy, cw, 1, fill=LINE)
            b.text(c, 24, yy + 16, name, 15, INK, "Bold", font=MONO)
            if name == "wcag22":
                pill(b, c, 24 + tw(name, 15, "Bold", MONO) + 10, yy + 13, "default", ACCENT, "#E8ECFF", h=22,
                     size=11)
            tx(b, c, 24, yy + 38, PRESET_NAMES[name], 12, MUTED, width=cols[0][1] - 24)
            xx = 24 + cols[0][1]
            for v, (_, w) in zip(row[1:], cols[1:]):
                tx(b, c, xx, yy + 16, v, 14, INK if v != "as WCAG" else MUTED, width=w - 24, lh="140%")
                xx += w
            yy += rh
        b.raw("node.set", {"ids": [c], "props": {"height": yy}})
        y += yy + 14
        _, h = tx(b, f, x0, y, "ΔY is the change in relative luminance (0 to 1). On the 200 cd/m² display "
                  "the broadcast guidelines assume, 0.1 is their 20 cd/m² threshold.", 13, MUTED, width=cw)
        y += h + 24
    # benchmark + limits
    if m:
        y += info_card(b, f, x0, y, cw, "Benchmarked", BENCH, m, ACCENT) + 12
        y += info_card(b, f, x0, y, cw, "Limits", LIMITS, m, LUM_FG)
    else:
        w = (cw - 24) / 2
        h1 = info_card(b, f, x0, y, w, "Benchmarked", BENCH, m, ACCENT)
        h2 = info_card(b, f, x0 + w + 24, y, w, "Limits", LIMITS, m, LUM_FG)
        y += max(h1, h2)
    return y


def info_card(b, f, x, y, w, title, body, m, col):
    pad = 20 if m else 28
    c = panel(b, f, x, y, w, 100)
    eyebrow(b, c, pad, pad, title, col)
    _, h = tx(b, c, pad, pad + 24, body, 15, INK, width=w - pad * 2, lh="155%")
    hh = pad + 24 + h + pad
    b.raw("node.set", {"ids": [c], "props": {"height": hh}})
    return hh


CLI_CMD = ["./bin/psv testdata/examples/colors-epilepsy-warning/video.mp4"]
CLI_OUT = [
    "Result:                    Failed",
    "Frame rate:                30 fps",
    "Length:                    19.933s (598 frames)",
    "Video size:                1920x1080",
    "Luminance flash failures:  392 frames",
    "Red flash failures:        392 frames",
    "",
    "Defects: 1",
    "  #  Frames  Time          Luminance  Red",
    "  1  2-598   0.03s-19.93s  X          X",
]
RENDER = [
    "psv -format json -frames video.mp4 > report.json",
    "psv render report.json > report.html      # a self-contained page of the report UI",
    "psv render -open report.json              # write it to a temporary file and open it",
    "psv -format json -frames video.mp4 | psv render -o report.html",
]
API_CMD = ["curl -sN --data-binary @testdata/examples/colors-epilepsy-warning/video.mp4 "
           "'http://127.0.0.1:8080/v1/analyses?name=video.mp4'"]
API_OUT = [
    '{"type":"received","bytes":45713}',
    '{"type":"progress","frame":1,"total":598}',
    '{"type":"progress","frame":7,"total":598}',
    "…",
    '{"type":"progress","frame":598,"total":598}',
    '{"type":"report","report":{"schema":"psv.report/v1","tool":{"name":"psv","version":"v0.1.0"},…',
]
GO_GET = ["go get github.com/brian-nunez/psv"]
# A complete program around the README's AnalyzeFile and Report lines. The same
# text is checked with go vet in .tmp/check/main.go (see README.md).
GO1 = [
    "package main",
    "",
    "import (",
    '\t"context"',
    '\t"fmt"',
    '\t"log"',
    "",
    '\t"github.com/brian-nunez/psv"',
    ")",
    "",
    "func main() {",
    "\tctx := context.Background()",
    "",
    '\tanalysis, err := psv.AnalyzeFile(ctx, "video.mp4", psv.Options{})',
    "\tif err != nil {",
    "\t\tlog.Fatal(err)",
    "\t}",
    "",
    "\treport := analysis.Report(false)",
    "\tfmt.Println(report.Verdict, report.Failures.Luminance, len(report.Defects))",
    "}",
]
# Complete programs, byte-identical to .tmp/check/stream/main.go and
# .tmp/check/live/main.go, which are vetted and run (see README.md).
GO_STREAM_SRC = r"""package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/brian-nunez/psv"
)

func main() {
	ctx := context.Background()
	path := os.Args[1]

	info, err := psv.Probe(ctx, path, psv.Tools{})
	if err != nil {
		log.Fatal(err)
	}

	dec, err := psv.Decode(ctx, path, info, psv.Tools{}, psv.DecodeOptions{})
	if err != nil {
		log.Fatal(err)
	}
	defer dec.Close()

	a, err := psv.NewAnalyzer(info.FrameRate.Float64(), info.Width, info.Height, psv.Config{Thresholds: psv.WCAG22()})
	if err != nil {
		log.Fatal(err)
	}

	read, failing := 0, false
	for {
		f, err := dec.Next() // valid until the next call
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		read++

		n, err := a.Analyze(f) // n frames decided, about one second behind
		if err != nil {
			log.Fatal(err)
		}
		failing = warn(a.Analysis(), len(a.Analysis().Frames)-n, read, failing)
	}

	from := len(a.Analysis().Frames)
	analysis := a.Finish() // decides the last second
	warn(analysis, from, read, failing)

	verdict := analysis.Verdict()
	fmt.Println(verdict)
	if !verdict.Passed() {
		os.Exit(1)
	}
}

// warn prints a line when a run of failing frames starts in Frames[from:].
func warn(a *psv.Analysis, from, read int, failing bool) bool {
	for i := from; i < len(a.Frames); i++ {
		failed := a.Frames[i].Failed()
		if failed && !failing {
			fmt.Printf("warning: frame %d (%.2fs) fails WCAG 2.2, known after reading frame %d\n",
				i+1, a.FrameTime(i+1).Seconds(), read)
		}
		failing = failed
	}
	return failing
}
"""
GO_LIVE_SRC = r"""package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/brian-nunez/psv"
)

// The size and rate FFmpeg is told to produce: -vf scale=1280:720,fps=30.
const (
	width  = 1280
	height = 720
	fps    = 30
)

func main() {
	a, err := psv.NewAnalyzer(fps, width, height, psv.Config{Thresholds: psv.WCAG22()})
	if err != nil {
		log.Fatal(err)
	}

	// Analyze samples the pixels before it returns, so one buffer is reused.
	buf := make([]byte, width*height*3)
	read, failing := 0, false
	for {
		if _, err := io.ReadFull(os.Stdin, buf); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			log.Fatal(err)
		}
		read++

		n, err := a.Analyze(&psv.Frame{Width: width, Height: height, Pix: buf})
		if err != nil {
			log.Fatal(err)
		}
		failing = warn(a.Analysis(), len(a.Analysis().Frames)-n, read, failing)
	}

	from := len(a.Analysis().Frames)
	analysis := a.Finish()
	warn(analysis, from, read, failing)

	verdict := analysis.Verdict()
	fmt.Println(verdict)
	if !verdict.Passed() {
		os.Exit(1)
	}
}

// warn prints a line when a run of failing frames starts in Frames[from:].
func warn(a *psv.Analysis, from, read int, failing bool) bool {
	for i := from; i < len(a.Frames); i++ {
		failed := a.Frames[i].Failed()
		if failed && !failing {
			fmt.Printf("warning: frame %d (%.2fs) fails WCAG 2.2, known after reading frame %d\n",
				i+1, a.FrameTime(i+1).Seconds(), read)
		}
		failing = failed
	}
	return failing
}
"""
GO_STREAM = GO_STREAM_SRC.splitlines()
GO_LIVE = GO_LIVE_SRC.splitlines()
GO_STREAM_RUN = ["go run . testdata/clips/fail.mp4"]
# Output of the real runs on testdata/clips/fail.mp4, trimmed with "…".
GO_STREAM_OUT = ['warning: frame 22 (0.70s) fails WCAG 2.2, known after reading frame 51', 'warning: frame 25 (0.80s) fails WCAG 2.2, known after reading frame 54', 'warning: frame 28 (0.90s) fails WCAG 2.2, known after reading frame 57', '…', 'warning: frame 88 (2.90s) fails WCAG 2.2, known after reading frame 90', 'Failed']
GO_LIVE_RUN = ["ffmpeg -i testdata/clips/fail.mp4 -vf scale=1280:720,fps=30 -f rawvideo -pix_fmt rgb24 - | go run ."]
GO_LIVE_OUT = ['warning: frame 22 (0.70s) fails WCAG 2.2, known after reading frame 51', 'warning: frame 25 (0.80s) fails WCAG 2.2, known after reading frame 54', 'warning: frame 28 (0.90s) fails WCAG 2.2, known after reading frame 57', '…', 'warning: frame 88 (2.90s) fails WCAG 2.2, known after reading frame 90', 'Failed']
BUILD = [
    ("Your own thresholds", "Start from WCAG22(), Trace24(), Broadcast() or SceneChange() and make them stricter."),
    ("Your own frames", "Stream decoded frames from a screen recorder, game engine or camera, without FFmpeg."),
    ("Live warnings", "Each frame is decided about one second after it arrives."),
    ("Per-frame detail", "Transitions, area, the flashing region and a 64×36 mask for every frame."),
    ("A pipeline gate", "Branch on Verdict(), or use the CLI's exit status: 0 pass, 1 fail, 2 error."),
    ("The shared report", "Report() writes psv.report/v1, the JSON the CLI, API and report UI all use."),
]


def sub_head(b, f, x0, cw, y, n, title, body, m):
    b.ellipse(f, x0, y + 2, 28, 28, fill=INK)
    b.text(f, x0, y + 8, str(n), 13, "#FFFFFF", "Bold", width=28, align="center")
    size = 22 if m else 26
    tx(b, f, x0 + 42, y, title, size, INK, "Bold", ls="-2%")
    y += 44 if m else 46
    _, h = tx(b, f, x0, y, body, 15 if m else 16, MUTED, width=min(cw, 720), lh="155%")
    return y + h + 20


def go_step(b, f, x0, cw, y, title, body, m):
    tx(b, f, x0, y, title, 18 if m else 20, INK, "SemiBold", ls="-1%")
    y += 32
    _, h = tx(b, f, x0, y, body, 14 if m else 15, MUTED, width=min(cw, 720), lh="155%")
    return y + h + 16


def para(b, f, x0, cw, y, s, m):
    _, h = tx(b, f, x0, y, s, 14, MUTED, width=min(cw, 720), lh="155%")
    return h


def use_it(b, f, x0, cw, y, m):
    y = section_head(b, f, x0, y, cw, "Use it", "Command line, API server or Go package",
                     "All three run the same core and produce the same psv.report/v1 report. The commands "
                     "and Go code come from the README, with output from the v0.1.0 release binary.", m)
    y += 48 if not m else 36
    # 1 CLI
    y = sub_head(b, f, x0, cw, y, 1, "Command line",
                 "Check a video against WCAG 2.2. The exit status is 0 when it passes, 1 when it fails and 2 "
                 "on an error, so psv can gate a build or upload pipeline.", m)
    y += code(b, f, x0, y, cw, CLI_CMD, "Terminal", output=CLI_OUT, m=m) + 14
    y += exit_codes(b, f, x0, y, cw, m) + 28
    eyebrow(b, f, x0, y, "Open the report UI")
    y += 22
    y += code(b, f, x0, y, cw, RENDER, "Terminal", m=m) + 12
    y += para(b, f, x0, cw, y, "psv render needs the per-frame results that \u2011frames adds. Choose a standard with "
              "\u2011standard: wcag22 (the default), trace24, broadcast or scene.", m)
    y += 72 if not m else 56
    # 2 API
    y = sub_head(b, f, x0, cw, y, 2, "API server",
                 "psv serve runs the API and the report UI at http://127.0.0.1:8080. It stores nothing: the "
                 "video is the request body, the analysis streams back as newline-delimited JSON, and the "
                 "upload is deleted when the request ends.", m)
    y += code(b, f, x0, y, cw, ["psv serve"], "Terminal", m=m) + 12
    y += code(b, f, x0, y, cw, API_CMD, "Terminal", output=API_OUT, m=m) + 16
    y += endpoints(b, f, x0, y, cw, m)
    y += 72 if not m else 56
    # 3 Go
    y = sub_head(b, f, x0, cw, y, 3, "Go package",
                 "github.com/brian-nunez/psv is one core package with only standard-library imports. "
                 "AnalyzeFile decodes with FFmpeg; an Analyzer takes frames you already have.", m)
    y = go_step(b, f, x0, cw, y, "Get the package", "The module path is github.com/brian-nunez/psv.", m)
    y += code(b, f, x0, y, cw, GO_GET, "Terminal", m=m) + 48
    y = go_step(b, f, x0, cw, y, "Analyse a file",
                "AnalyzeFile decodes the whole video and returns the complete analysis.", m)
    y += code(b, f, x0, y, cw, GO1, "Go  ·  main.go", lang="go", m=m, prompt=False) + 24
    y += 24
    y = go_step(b, f, x0, cw, y, "Stream frames from a video",
                "Probe and Decode read the video with FFmpeg, frame by frame, into an Analyzer. Unlike "
                "AnalyzeFile, you see each decision as it happens, about one second behind the frame "
                "being read, because whether a frame flashes depends on what follows.", m)
    y += code(b, f, x0, y, cw, GO_STREAM, "Go  ·  main.go", lang="go", m=m, prompt=False) + 12
    y += code(b, f, x0, y, cw, GO_STREAM_RUN, "Terminal", output=GO_STREAM_OUT, m=m) + 48
    y = go_step(b, f, x0, cw, y, "Analyse a live stream",
                "Pipe raw frames from FFmpeg into the program, so any input FFmpeg reads works: a camera, an "
                "RTMP or RTSP stream, or a screen capture. The size and rate in the program match the "
                "scale and fps in the pipe.", m)
    y += code(b, f, x0, y, cw, GO_LIVE, "Go  ·  main.go", lang="go", m=m, prompt=False) + 12
    y += code(b, f, x0, y, cw, GO_LIVE_RUN, "Terminal", output=GO_LIVE_OUT, m=m) + 32
    y += 24
    eyebrow(b, f, x0, y, "What you can build")
    y += 26
    if m:
        for t, s in BUILD:
            b.rect(f, x0, y + 6, 6, 6, fill=ACCENT, radius=3)
            tx(b, f, x0 + 18, y, t, 15, INK, "SemiBold")
            _, h = tx(b, f, x0 + 18, y + 22, s, 14, MUTED, width=cw - 18, lh="150%")
            y += 22 + h + 14
    else:
        w = (cw - 48) / 3
        rows = []
        for i, (t, s) in enumerate(BUILD):
            cx = x0 + (i % 3) * (w + 24)
            ry = y + sum(rows[: i // 3])
            b.rect(f, cx, ry, w, 1, fill=LINE)
            tx(b, f, cx, ry + 16, t, 16, INK, "SemiBold")
            _, h = tx(b, f, cx, ry + 42, s, 14, MUTED, width=w - 8, lh="155%")
            if i % 3 == 2:
                rows.append(max(th(BUILD[j][1], 14, w - 8, lh="155%") for j in range(i - 2, i + 1)) + 42 + 36)
        y += sum(rows) - 36
    return y


def exit_codes(b, f, x0, y, cw, m):
    items = [("0", "passed", PASS_FG, PASS_BG), ("1", "failed", RED_FG, RED_BG), ("2", "error", MUTED, SOFT)]
    xx, yy = x0, y
    b.text(f, xx, yy + 4, "Exit status", 13, MUTED, "Medium")
    xx += 86
    for n, s, fg, bg in items:
        w = pill(b, f, xx, yy, f"{n}  {s}", fg, bg, h=26, size=13)
        xx += w + 8
    return 26


def endpoints(b, f, x0, y, cw, m):
    rows = [("POST", "/v1/analyses?standard=wcag22&name=video.mp4", "Video in the body; NDJSON events back"),
            ("GET", "/healthz", '{"status":"ok"}'), ("GET", "/", "The report UI")]
    c = panel(b, f, x0, y, cw, 100, radius=14)
    pad = 16 if m else 20
    yy = 0
    for i, (verb, path, desc) in enumerate(rows):
        if i:
            b.rect(c, 0, yy, cw, 1, fill=LINE)
        vw = tw(verb, 11, "Bold", MONO) + 14
        b.rect(c, pad, yy + 15, vw, 22, fill="#E8ECFF" if verb == "POST" else SOFT, radius=6)
        b.text(c, pad + 7, yy + 19, verb, 11, ACCENT if verb == "POST" else INK, "Bold", font=MONO)
        if m:
            b.text(c, pad + vw + 10, yy + 18, desc, 13, MUTED)
            b.text(c, pad, yy + 47, path, 11, INK, font=MONO)
            yy += 47 + 14 + 14
        else:
            b.text(c, pad + 64, yy + 18, path, 13, INK, font=MONO)
            b.text(c, cw - pad - 400, yy + 18, desc, 14, MUTED, width=400, align="right")
            yy += 52
    errs = "Errors before the stream starts: 400, 413, 422, 429 with {\"error\": …}. Closing the connection cancels."
    b.rect(c, 0, yy, cw, 1, fill=LINE)
    _, h = tx(b, c, pad, yy + 14, errs, 13, MUTED, width=cw - pad * 2, lh="150%")
    yy += 14 + h + 14
    b.raw("node.set", {"ids": [c], "props": {"height": yy}})
    return yy


def footer(b, f, W, x0, cw, y, m):
    b.rect(f, 0, y, W, 1, fill=LINE)
    y += 40
    logo(b, f, x0, y)
    b.text(f, x0 + 38, y + 4, "Photosafety Validator", 16, INK, "Bold", ls="-1%")
    links = ["GitHub", "Releases", "How it works", "Testing", "Examples", "Architecture decisions"]
    if m:
        y += 48
        for i, s in enumerate(links):
            b.text(f, x0 + (i % 2) * 170, y + (i // 2) * 28, s, 14, INK, "Medium")
        y += 3 * 28 + 16
    else:
        xx = x0 + cw
        for s in reversed(links):
            xx -= tw(s, 14, "Medium")
            b.text(f, xx, y + 5, s, 14, INK, "Medium")
            xx -= 28
        y += 52
    _, h = tx(b, f, x0, y, "MIT License  ·  Copyright © 2026 Brian Nunez  ·  PSV v0.1.0 is a public proof "
              "of concept.", 13, MUTED, width=cw, lh="150%")
    return y + h + 40


def page(b, X, m, ex):
    W = 375 if m else 1440
    cw = W - 32 if m else 1120
    x0 = (W - cw) / 2
    f = b.frame(X, 0, W, 1000, name="Site · Phone" if m else "Site · Desktop", fill=BG)
    y = nav(b, f, W, x0, cw, m)
    y = hero(b, f, x0, cw, y, m)
    gap = 96 if m else 140
    y = showcase(b, f, x0, cw, y + gap, m, ex)
    y = standards(b, f, x0, cw, y + gap, m)
    y = use_it(b, f, x0, cw, y + gap, m)
    y = footer(b, f, W, x0, cw, y + gap, m)
    b.raw("node.set", {"ids": [f], "props": {"height": y}})
    return f


def build(ex):
    b = B()
    page(b, 0, False, ex)
    page(b, 1540, True, ex)
    # the source-link disclosure, expanded
    for X, m in ((2015, False), (3555, True)):
        W = 375 if m else 1440
        cw = W - 32 if m else 1120
        f = b.frame(X, 0, W, 100, name="Warning expanded · " + ("Phone" if m else "Desktop"), fill=BG)
        h = warning(b, f, (W - cw) / 2, 40, cw, m, ex, expanded=True)
        b.raw("node.set", {"ids": [f], "props": {"height": h + 80}})
    return b


if __name__ == "__main__":
    ex = [load(n) for n in EXAMPLES]
    for _ in range(4):
        build(ex)
        if not M.resolve():
            break
    b = build(ex)
    if M.miss:
        sys.exit("unmeasured text remains")
    out = os.path.join(HERE, "site.figcraft")
    b.run(out)
    os.remove(out + ".script.json")
    png = os.path.join(HERE, "png")
    os.makedirs(png, exist_ok=True)
    subprocess.run([CLI, "export", out, "-o", png, "--format", "png"], check=True)
