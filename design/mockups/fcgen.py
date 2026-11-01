"""Tiny builder that emits figcraft-cli run scripts with predicted node ids."""
import json
import math
import os
import random
import subprocess
import sys

LH = 1.21  # Inter auto line height


class B:
    def __init__(self, first_id=2):
        self.cmds = []
        self.next = first_id
        self.expected = []

    def _add(self, command, params):
        nid = self.next
        self.next += 1
        self.cmds.append({"command": command, "params": params})
        self.expected.append(nid)
        return nid

    def raw(self, command, params):
        self.cmds.append({"command": command, "params": params})
        self.expected.append(None)

    def frame(self, x, y, w, h, parent=None, name=None, fill=None, radius=0, stroke=None, sw=1,
              clip=True, effects=None, fills=None):
        props = {"clip": clip}
        if fills is not None:
            props["fills"] = fills
        elif fill:
            props["fill"] = fill
        else:
            props["fills"] = []
        if radius:
            props["cornerRadius"] = radius
        if stroke:
            props["strokes"] = [stroke]
            props["strokeWeight"] = sw
            props["strokeAlign"] = "inside"
        if effects:
            props["effects"] = effects
        p = {"x": x, "y": y, "width": w, "height": h, "props": props}
        if parent is not None:
            p["parent"] = parent
        if name:
            p["name"] = name
        return self._add("frame.create", p)

    def rect(self, parent, x, y, w, h, fill=None, radius=0, stroke=None, sw=1, name=None,
             fills=None, opacity=None, effects=None, dashes=None, sides=None):
        props = {}
        if fills is not None:
            props["fills"] = fills
        elif fill:
            props["fill"] = fill
        else:
            props["fills"] = []
        if radius:
            props["cornerRadius"] = radius
        if stroke:
            props["strokes"] = [stroke]
            props["strokeWeight"] = sw
            props["strokeAlign"] = "inside"
            if dashes:
                props["dashes"] = dashes
        if opacity is not None:
            props["opacity"] = opacity
        if effects:
            props["effects"] = effects
        p = {"type": "rectangle", "x": x, "y": y, "width": max(w, 0.5), "height": max(h, 0.5),
             "parent": parent, "props": props}
        if name:
            p["name"] = name
        return self._add("node.create", p)

    def ellipse(self, parent, x, y, w, h, fill=None, stroke=None, sw=1, fills=None, effects=None):
        props = {}
        if fills is not None:
            props["fills"] = fills
        elif fill:
            props["fill"] = fill
        else:
            props["fills"] = []
        if stroke:
            props["strokes"] = [stroke]
            props["strokeWeight"] = sw
        if effects:
            props["effects"] = effects
        return self._add("node.create", {"type": "ellipse", "x": x, "y": y, "width": w, "height": h,
                                         "parent": parent, "props": props})

    def vector(self, parent, path, x, y, w, h, stroke=None, sw=1.5, fill=None, cap="round"):
        props = {}
        props["fills"] = [fill] if fill else []
        if stroke:
            props["strokes"] = [stroke]
            props["strokeWeight"] = sw
            props["strokeCap"] = cap
            props["strokeJoin"] = "round"
        return self._add("vector.create", {"path": path, "x": x, "y": y, "width": w, "height": h,
                                           "parent": parent, "props": props})

    def text(self, parent, x, y, s, size=14, color="#000", weight="Regular", font="Inter", width=None,
             align=None, ls=None, lh=None, tnum=False, upper=False, name=None):
        props = {"fontSize": size, "fill": color, "fontFamily": font, "fontStyle": weight}
        if align:
            props["textAlign"] = align
        if ls is not None:
            props["letterSpacing"] = ls
        if lh is not None:
            props["lineHeight"] = lh
        if tnum:
            props["features"] = {"tnum": True}
        if upper:
            props["textCase"] = "upper"
        p = {"text": s, "x": x, "y": y, "parent": parent, "props": props}
        if width is not None:
            p["width"] = width
        if name:
            p["name"] = name
        return self._add("text.create", p)

    def run(self, out):
        script = out + ".script.json"
        with open(script, "w") as f:
            json.dump(self.cmds, f)
        res = subprocess.run([os.path.expanduser("~/.local/bin/figcraft-cli"), "run", script, "--out", out],
                             capture_output=True, text=True)
        lines = [l for l in res.stdout.splitlines() if l.startswith("{")]
        for i, l in enumerate(lines):
            r = json.loads(l)
            if not r.get("ok"):
                print("FAIL", r, file=sys.stderr)
                sys.exit(1)
            exp = self.expected[i]
            got = (r.get("result") or {}).get("id")
            if exp is not None and got != exp:
                print(f"id mismatch at step {i}: expected {exp}, got {got} ({r['command']})", file=sys.stderr)
                sys.exit(1)
        if res.returncode != 0:
            print(res.stderr, file=sys.stderr)
            sys.exit(1)
        print(f"ok: {len(self.cmds)} commands -> {out}")


def shadow(color="#00000040", radius=24, y=8, spread=0):
    return {"type": "dropShadow", "color": color, "radius": radius, "offset": [0, y], "spread": spread}


def lin(stops, horizontal=True):
    if horizontal:
        return {"type": "linearGradient", "stops": stops, "from": [0, 0.5], "to": [1, 0.5]}
    return {"type": "linearGradient", "stops": stops, "from": [0.5, 0], "to": [0.5, 1]}


# ---------------------------------------------------------------------------
# Sample analysis data, shaped exactly like psv.Report / FrameReport.

FPS = 30
FRAMES = 1843
DURATION = FRAMES / FPS
WIDTH, HEIGHT = 1920, 1080
FILE = "launch-trailer-final.mp4"

DEFECTS = [
    # index, first, last, luminance, red
    (1, 72, 115, True, False),
    (2, 276, 329, True, True),
    (3, 512, 561, False, True),
    (4, 690, 748, True, True),
    (5, 901, 944, True, False),
    (6, 1120, 1189, False, True),
    (7, 1402, 1450, True, True),
    (8, 1688, 1731, False, True),
]
FAIL_LUM, FAIL_RED = 338, 385


def t(frame):
    return (frame - 1) / FPS


def series():
    """Per-frame transitions for luminance and red."""
    rnd = random.Random(7)
    lum = [0] * (FRAMES + 1)
    red = [0] * (FRAMES + 1)
    for f in range(1, FRAMES + 1):
        lum[f] = max(0, int(rnd.gauss(1.2, 1.1)))
        red[f] = max(0, int(rnd.gauss(0.8, 0.9)))
    for _, a, b, l, r in DEFECTS:
        for f in range(a, b + 1):
            p = (f - a) / max(1, b - a)
            peak = 7 + int(4 * math.sin(math.pi * p))
            ramp = min(1.0, p * 3)
            if l:
                lum[f] = max(lum[f], int(round(peak * ramp + rnd.uniform(-1, 1))))
            if r:
                red[f] = max(red[f], int(round((peak - 1) * ramp + rnd.uniform(-1, 1))))
    return lum, red


def fmt_dur(sec):
    m = int(sec // 60)
    s = sec - m * 60
    return f"{m}m{s:.3f}s" if m else f"{s:.3f}s"


# ---------------------------------------------------------------------------
# Full per-frame results (psv.FrameResult), generated so that the failure
# counts and defects follow from them exactly as psv.Analysis computes them.

RW, RH = 341, 256
LOC = {
    1: {"l": (620, 420)},
    2: {"l": (680, 450), "r": (1130, 610)},
    3: {"r": (1450, 300)},
    4: {"l": (960, 540), "r": (1000, 560)},
    5: {"l": (420, 760)},
    6: {"r": (1500, 820)},
    7: {"l": (900, 260), "r": (1200, 300)},
    8: {"r": (300, 300)},
}
CELL = 60  # heatmap cell, source pixels


def _region(cx, cy):
    x1 = int(min(max(cx - RW / 2, 0), WIDTH - RW))
    y1 = int(min(max(cy - RH / 2, 0), HEIGHT - RH))
    return (x1, y1, x1 + RW, y1 + RH)


def frames_data():
    rnd = random.Random(11)
    out = [None]
    for f in range(1, FRAMES + 1):
        rec = {}
        for k in ("l", "r"):
            tr = max(0, int(rnd.gauss(1.0, 1.0)))
            if tr == 0:
                rec[k] = {"tr": 0, "since": 0, "region": None, "area": 0.0, "failed": False}
            else:
                rec[k] = {"tr": tr, "since": max(1, f - rnd.randint(0, 20)),
                          "region": _region(rnd.uniform(0, WIDTH), rnd.uniform(0, HEIGHT)),
                          "area": rnd.uniform(3, 18), "failed": False}
        out.append(rec)
    for idx, a, bb, l, r in DEFECTS:
        for k, on in (("l", l), ("r", r)):
            if not on:
                continue
            cx0, cy0 = LOC[idx][k]
            for f in range(a, bb + 1):
                p = (f - a) / (bb - a)
                cx = cx0 + 40 * math.sin(f / 9) + rnd.gauss(0, 8)
                cy = cy0 + 30 * math.cos(f / 11) + rnd.gauss(0, 8)
                if f >= a + 12:
                    tr = min(12, 7 + round(4 * math.sin(math.pi * p)) + rnd.randint(0, 1))
                    area = max(25.5, 26 + 18 * math.sin(math.pi * p) + rnd.gauss(0, 2))
                    since = a if f - a < 30 else f - 29
                    failed = True
                else:
                    tr = 1 + int((f - a) / 12 * 5)
                    area = 10 + (f - a) * 1.2
                    since, failed = a, False
                out[f][k] = {"tr": tr, "since": since, "region": _region(cx, cy), "area": area,
                             "failed": failed}
    out[291]["l"].update(tr=9, area=41.8)
    out[291]["r"].update(tr=7, area=27.3)
    return out


def failures(data, upto=FRAMES):
    return (sum(1 for f in range(1, upto + 1) if data[f]["l"]["failed"]),
            sum(1 for f in range(1, upto + 1) if data[f]["r"]["failed"]))


def defects_from(data, upto=FRAMES):
    out = []
    for f in range(1, upto + 1):
        for k in ("l", "r"):
            m = data[f][k]
            if not m["failed"]:
                continue
            d = [m["since"], f, k == "l", k == "r"]
            if out and d[0] <= out[-1][1] + 1:
                o = out[-1]
                o[0], o[1] = min(o[0], d[0]), max(o[1], d[1])
                o[2], o[3] = o[2] or d[2], o[3] or d[3]
            else:
                out.append(d)
    return out


def heat(data, kinds=("l", "r")):
    cols, rows = WIDTH // CELL, HEIGHT // CELL
    g = {k: [[0] * cols for _ in range(rows)] for k in kinds}
    for f in range(1, FRAMES + 1):
        for k in kinds:
            m = data[f][k]
            if not m["failed"]:
                continue
            x1, y1, x2, y2 = m["region"]
            for cy in range(y1 // CELL, min(rows, (y2 - 1) // CELL + 1)):
                for cx in range(x1 // CELL, min(cols, (x2 - 1) // CELL + 1)):
                    g[k][cy][cx] += 1
    return g, cols, rows
