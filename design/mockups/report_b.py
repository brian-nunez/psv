"""Version B report, v2: where on screen, where in time and per-frame levels."""
import math

from fcgen import (shadow, lin, LH, FPS, FRAMES, WIDTH, HEIGHT, FILE, CELL, LOC, t, frames_data,
                   failures, defects_from, heat)
from version_b import (BG, SURF, LINE, SOFT, INK, MUTED, FAINT, ACCENT, LUM, RED, MONO, X0, CW,
                       panel, eyebrow, chip, button, nav)

SEL = 291
WIN = (240, 372)


def admin_toggle(b, p, x, y, on):
    b.rect(p, x, y, 214, 32, fill=SURF, radius=16, stroke=ACCENT if on else LINE)
    b.rect(p, x + 6, y + 6, 46, 20, fill="#E8ECFF", radius=10)
    b.text(p, x + 6, y + 10, "ADMIN", 9, ACCENT, "Bold", width=46, align="center", ls="6%")
    b.text(p, x + 60, y + 8, "Technical details", 12, INK, "SemiBold")
    b.rect(p, x + 172, y + 7, 34, 18, fill=ACCENT if on else "#D9D5CD", radius=9)
    b.ellipse(p, x + (190 if on else 174), y + 9, 14, 14, fill=SURF)


def mix(c1, c2, p):
    a = [int(c1[i:i + 2], 16) for i in (1, 3, 5)]
    b = [int(c2[i:i + 2], 16) for i in (1, 3, 5)]
    return "#" + "".join(f"{round(x + (y - x) * p):02X}" for x, y in zip(a, b))


def polyline(b, p, pts, stroke=None, sw=2, fill=None):
    xs = [x for x, _ in pts]
    ys = [y for _, y in pts]
    mx, my = min(xs), min(ys)
    w, h = max(xs) - mx, max(ys) - my
    d = "M" + " L".join(f"{x - mx:.2f} {y - my:.2f}" for x, y in pts)
    if fill:
        d += " Z"
    return b.vector(p, d, mx, my, max(w, 0.5), max(h, 0.5), stroke=stroke, sw=sw, fill=fill)


def where_on_screen(b, f, y, data):
    g, cols, rows = heat(data)
    p = panel(b, f, X0, y, CW, 648)
    b.text(p, 32, 28, "Where on screen", 22, INK, "SemiBold", ls="-1%")
    b.text(p, 32, 60, "Screen cells covered by a failing region, across all 1843 frames. "
           "Each frame reports the region that flashed most.", 14, MUTED)
    # tabs
    tx = CW - 32 - 282
    b.rect(p, tx, 26, 282, 36, fill=SOFT, radius=18)
    b.rect(p, tx + 3, 29, 98, 30, fill=SURF, radius=15, effects=[shadow("#0F0F121A", 4, 1)])
    for i, (s, w) in enumerate([("Combined", 98), ("Luminance", 98), ("Red", 80)]):
        b.text(p, tx + 3 + i * 98, 36, s, 13, INK if i == 0 else MUTED, "SemiBold" if i == 0 else "Medium",
               width=w, align="center")

    # big combined map
    mw, mh = 672, 378
    mp = b.frame(32, 104, mw, mh, parent=p, radius=14, fills=[lin(["#1A1F2E", "#2A1A2A", "#14121C"])])
    cs = mw / cols
    peak = max(g["l"][r][c] + g["r"][r][c] for r in range(rows) for c in range(cols))
    for r in range(rows):
        for c in range(cols):
            lv, rv = g["l"][r][c], g["r"][r][c]
            tot = lv + rv
            if not tot:
                continue
            col = mix("#FFC542", "#FF3B4E", rv / tot)
            b.rect(mp, c * cs + 0.5, r * cs + 0.5, cs - 1, cs - 1, fill=col, radius=3,
                   opacity=0.18 + 0.82 * (tot / peak) ** 0.8)
    for i in (1, 2):
        b.rect(mp, mw * i / 3, 0, 1, mh, fill="#FFFFFF14")
        b.rect(mp, 0, mh * i / 3, mw, 1, fill="#FFFFFF14")
    sx = mw / WIDTH
    for idx, kinds in LOC.items():
        xs = [v[0] for v in kinds.values()]
        ys = [v[1] for v in kinds.values()]
        cx, cy = sum(xs) / len(xs) * sx, sum(ys) / len(ys) * sx
        b.ellipse(mp, cx - 11, cy - 11, 22, 22, fill="#FFFFFF", effects=[shadow("#00000066", 8, 2)])
        b.text(mp, cx - 11, cy - 7, str(idx), 11, INK, "Bold", width=22, align="center")
    b.text(mp, 10, 8, "0, 0", 10, "#FFFFFF99", font=MONO)
    b.text(mp, mw - 110, mh - 20, "1920, 1080", 10, "#FFFFFF99", font=MONO, width=100, align="right")

    # legend under map
    ly = 500
    b.text(p, 32, ly, "Fewer frames", 12, MUTED)
    for i in range(10):
        b.rect(p, 118 + i * 22, ly + 2, 20, 12, fill="#E8743F", opacity=0.18 + i * 0.09, radius=3)
    b.text(p, 346, ly, "More frames", 12, MUTED)
    b.rect(p, 456, ly + 2, 40, 12, radius=3, fills=[lin(["#FFC542", "#FF3B4E"])])
    b.text(p, 504, ly, "Luminance → red", 12, MUTED)

    # summary stats
    hot = max(((g["l"][r][c] + g["r"][r][c], r, c) for r in range(rows) for c in range(cols)))
    covered = sum(1 for r in range(rows) for c in range(cols) if g["l"][r][c] + g["r"][r][c])
    peak_area = max(max(data[i]["l"]["area"], data[i]["r"]["area"])
                    for i in range(1, FRAMES + 1) if data[i]["l"]["failed"] or data[i]["r"]["failed"])
    stats = [("Hottest cell", f"{hot[2] * CELL}–{(hot[2] + 1) * CELL} × {hot[1] * CELL}–{(hot[1] + 1) * CELL} px",
              f"in {hot[0]} failing frames"),
             ("Screen affected", f"{covered / (rows * cols) * 100:.0f}%", f"{covered} of {rows * cols} cells"),
             ("Peak area", f"{peak_area:.1f}%", "of a 341×256 region")]
    sw_ = (mw - 24) / 3
    for i, (k, v, s) in enumerate(stats):
        x = 32 + i * (sw_ + 12)
        b.rect(p, x, 540, sw_, 76, fill=SOFT, radius=12)
        eyebrow(b, p, x + 14, 552, k)
        b.text(p, x + 14, 570, v, 14 if i == 0 else 20, INK, "SemiBold", tnum=True)
        b.text(p, x + 14, 596, s, 11, MUTED)

    # small multiples
    rx, rw = 728, CW - 728 - 32
    rh = rw * 9 / 16
    nl, nr = failures(data)
    for j, (k, nm, col, n) in enumerate([("l", "Luminance", LUM, nl), ("r", "Red", RED, nr)]):
        yy = 104 + j * (rh + 52)
        b.ellipse(p, rx, yy + 5, 8, 8, fill=col)
        b.text(p, rx + 16, yy, nm, 14, INK, "SemiBold")
        b.text(p, rx, yy, f"{n} failing frames", 12, MUTED, width=rw, align="right", tnum=True)
        sm = b.frame(rx, yy + 26, rw, rh, parent=p, fill=SOFT, radius=12, stroke=LINE)
        kp = max(v for row in g[k] for v in row)
        scs = rw / cols
        for r in range(rows):
            for c in range(cols):
                v = g[k][r][c]
                if v:
                    b.rect(sm, c * scs + 0.4, r * scs + 0.4, scs - 0.8, scs - 0.8, fill=col, radius=1.5,
                           opacity=0.15 + 0.85 * (v / kp) ** 0.8)
        ssx = rw / WIDTH
        for idx, kinds in LOC.items():
            if k in kinds:
                cx, cy = kinds[k][0] * ssx, kinds[k][1] * ssx
                b.ellipse(sm, cx - 8, cy - 8, 16, 16, fill=INK)
                b.text(sm, cx - 8, cy - 5, str(idx), 9, "#FFFFFF", "Bold", width=16, align="center")
    return y + 648


def where_in_time(b, f, y, data):
    H = 800
    p = panel(b, f, X0, y, CW, H)
    b.text(p, 32, 28, "Where in time", 22, INK, "SemiBold", ls="-1%")
    b.text(p, 32, 60, "The overview shows the whole video. The window below it is shown frame by frame.",
           14, MUTED)

    gx0, gx1 = 132, CW - 32
    span = gx1 - gx0

    def ox(fr):
        return gx0 + (fr - 1) / (FRAMES - 1) * span

    # overview
    eyebrow(b, p, 32, 124, "Overview")
    buckets = 184
    per = FRAMES / buckets
    cw_ = span / buckets
    for li, (k, col) in enumerate([("l", LUM), ("r", RED)]):
        yy = 118 + li * 22
        for i in range(buckets):
            a = int(i * per) + 1
            z = int((i + 1) * per)
            m = max(data[q][k]["tr"] for q in range(a, z + 1))
            failed = any(data[q][k]["failed"] for q in range(a, z + 1))
            b.rect(p, gx0 + i * cw_, yy, cw_ - 1, 18, fill=col if m else "#D9D5CD",
                   opacity=(0.35 + min(m, 12) / 12 * 0.65) if failed else (0.15 + m * 0.04), radius=1)
    for i, (a, bb, _, _) in enumerate(defects_from(data)):
        cx = (ox(a) + ox(bb)) / 2
        b.ellipse(p, cx - 8, 94, 16, 16, fill=INK)
        b.text(p, cx - 8, 97, str(i + 1), 9, "#FFFFFF", "Bold", width=16, align="center")
    wx1, wx2 = ox(WIN[0]), ox(WIN[1])
    b.rect(p, wx1, 112, wx2 - wx1, 50, fill="#2346FF14", stroke=ACCENT, sw=1.5, radius=6)
    for hx in (wx1 - 3, wx2 - 3):
        b.rect(p, hx, 127, 6, 20, fill=ACCENT, radius=3)
    for s in range(0, 61, 10):
        xx = gx0 + s * FPS / (FRAMES - 1) * span
        b.text(p, xx - 20, 168, f"{s}s", 10, FAINT, font=MONO, width=40, align="center")

    b.rect(p, 32, 200, CW - 64, 1, fill=LINE)
    b.text(p, 32, 218, f"Frames {WIN[0]}–{WIN[1]}", 17, INK, "SemiBold", tnum=True)
    b.text(p, 206, 221, f"{t(WIN[0]):.2f}s – {t(WIN[1] + 1):.2f}s  ·  defect 02", 14, MUTED, tnum=True)
    for i, s in enumerate(["−", "+", "Fit"]):
        w = 34 if i < 2 else 52
        x = CW - 32 - 132 + (0 if i == 0 else 40 if i == 1 else 80)
        b.rect(p, x, 214, w, 30, fill=SURF, radius=15, stroke=LINE)
        b.text(p, x, 220, s, 13, INK, "SemiBold", width=w, align="center")

    n = WIN[1] - WIN[0] + 1
    slot = span / n

    def zx(fr):
        return gx0 + (fr - WIN[0]) * slot

    # Track 1: transitions
    ty, th = 270, 150
    b.text(p, 32, ty + 52, "Transitions", 13, INK, "SemiBold")
    b.text(p, 32, ty + 72, "per frame", 12, MUTED)
    for v in (0, 3, 6, 9, 12):
        yy = ty + th - v / 12 * th
        if v != 6:
            b.rect(p, gx0, yy, span, 1, fill="#EFECE6")
        b.text(p, gx0 - 30, yy - 7, str(v), 10, FAINT, font=MONO, width=20, align="right")
    ly6 = ty + th - 6.5 / 12 * th
    b.rect(p, gx0, ly6, span, 1.5, stroke=INK, sw=1.5, dashes=[5, 4])
    chip(b, p, gx1 - 80, ly6 - 24, "limit  6", "#FFFFFF", INK, w=72, h=20, size=10)
    for i in range(n):
        fr = WIN[0] + i
        for j, (k, col) in enumerate([("l", LUM), ("r", RED)]):
            v = data[fr][k]["tr"]
            if not v:
                continue
            hgt = min(v, 12) / 12 * th
            b.rect(p, zx(fr) + 0.6 + j * (slot / 2 - 0.3), ty + th - hgt, slot / 2 - 1.2, hgt, fill=col,
                   opacity=1 if data[fr][k]["failed"] else 0.35, radius=1)

    # Track 2: area
    ay, ah = 448, 120
    b.text(p, 32, ay + 38, "Area", 13, INK, "SemiBold")
    b.text(p, 32, ay + 58, "% of region", 12, MUTED)
    for v in (0, 25, 50):
        yy = ay + ah - v / 50 * ah
        if v != 25:
            b.rect(p, gx0, yy, span, 1, fill="#EFECE6")
        b.text(p, gx0 - 34, yy - 7, f"{v}%", 10, FAINT, font=MONO, width=26, align="right")
    l25 = ay + ah - 25 / 50 * ah
    for k, col in (("l", LUM), ("r", RED)):
        pts = [(zx(WIN[0] + i) + slot / 2, ay + ah - min(data[WIN[0] + i][k]["area"], 50) / 50 * ah)
               for i in range(n)]
        polyline(b, p, [(pts[0][0], ay + ah)] + pts + [(pts[-1][0], ay + ah)], fill=col + "22")
        polyline(b, p, pts, stroke=col, sw=2)
    b.rect(p, gx0, l25, span, 1.5, stroke=INK, sw=1.5, dashes=[5, 4])
    chip(b, p, gx1 - 80, l25 - 24, "limit 25%", "#FFFFFF", INK, w=72, h=20, size=10)

    # Track 3: sequence spans
    sy = 596
    b.text(p, 32, sy + 8, "Sequences", 13, INK, "SemiBold")
    b.text(p, 32, sy + 28, "first transition", 12, MUTED)
    b.text(p, 32, sy + 44, "to completing", 12, MUTED)
    b.text(p, 32, sy + 60, "frame", 12, MUTED)
    b.rect(p, gx0, sy - 4, span, 156, fill="#FAF9F7", radius=8)
    for j, (k, col, nm) in enumerate([("l", LUM, "Luminance"), ("r", RED, "Red")]):
        base = sy + 4 + j * 76
        eyebrow(b, p, gx1 - 120, base, nm, col, width=112, align="right")
        rows_ = [fr for fr in range(WIN[0], WIN[1] + 1) if data[fr][k]["failed"]][::2]
        for ri, fr in enumerate(rows_):
            s = data[fr][k]["since"]
            yy = base + ri * 3
            b.rect(p, zx(s), yy, zx(fr) - zx(s) + slot / 2, 2, fill=col, opacity=0.55)
            b.rect(p, zx(fr) + slot / 2 - 1.5, yy - 0.5, 3, 3, fill=col)

    # axis
    for fr in range(WIN[0], WIN[1] + 1, 20):
        xx = zx(fr) + slot / 2
        b.text(p, xx - 30, 760, str(fr), 10, MUTED, font=MONO, width=60, align="center")
        b.text(p, xx - 30, 774, f"{t(fr):.2f}s", 9, FAINT, font=MONO, width=60, align="center")

    # scrubber
    mx = zx(SEL) + slot / 2
    b.rect(p, mx - 1, 262, 2, 494, fill=ACCENT)
    b.ellipse(p, mx - 6, 256, 12, 12, fill=ACCENT, stroke="#FFFFFF", sw=2)
    tip = b.frame(mx - 246, 268, 232, 150, parent=p, fill=SURF, radius=12, stroke=LINE,
                  effects=[shadow("#0F0F1222", 24, 8)])
    L, R = data[SEL]["l"], data[SEL]["r"]
    b.text(tip, 14, 12, f"Frame {SEL}", 14, INK, "Bold")
    b.text(tip, 14, 12, f"{t(SEL):.3f}s", 12, MUTED, width=204, align="right", font=MONO)
    b.ellipse(tip, 112, 42, 7, 7, fill=LUM)
    b.text(tip, 122, 38, "Lum", 11, MUTED, "SemiBold")
    b.ellipse(tip, 172, 42, 7, 7, fill=RED)
    b.text(tip, 182, 38, "Red", 11, MUTED, "SemiBold")
    for i, (k, a, r) in enumerate([("Transitions", str(L["tr"]), str(R["tr"])),
                                   ("Area", f"{L['area']:.1f}%", f"{R['area']:.1f}%"),
                                   ("Since", str(L["since"]), str(R["since"])),
                                   ("Failed", "Yes", "Yes")]):
        yy = 60 + i * 21
        b.text(tip, 14, yy, k, 12, MUTED)
        b.text(tip, 112, yy, a, 12, RED if k == "Failed" else INK, "SemiBold", tnum=True)
        b.text(tip, 172, yy, r, 12, RED if k == "Failed" else INK, "SemiBold", tnum=True)
    return y + H


def defects_and_detail(b, f, y0, data):
    defects = defects_from(data)
    b.text(f, X0, y0, "Defects", 28, INK, "Bold", ls="-2%")
    b.text(f, X0 + 116, y0 + 10, str(len(defects)), 15, MUTED, "SemiBold")
    b.text(f, X0, y0 + 42, "Runs of frames covered by failing sequences.", 14, MUTED)
    ly = y0 + 84
    for i, (a, bb, l, r) in enumerate(defects):
        idx = i + 1
        on = a <= SEL <= bb
        row = b.frame(X0, ly, 520, 76, parent=f, fill=SURF, radius=14,
                      stroke=ACCENT if on else LINE, sw=1.5 if on else 1,
                      effects=[shadow("#2346FF26", 18, 6)] if on else None)
        b.rect(row, 16, 18, 40, 40, fill="#EEF1FF" if on else SOFT, radius=10)
        b.text(row, 16, 29, f"{idx:02d}", 14, ACCENT if on else INK, "Bold", width=40, align="center", tnum=True)
        b.text(row, 72, 16, f"Frames {a}–{bb}", 15, INK, "SemiBold", tnum=True)
        b.text(row, 72, 40, f"{t(a):.2f}s → {t(bb + 1):.2f}s  ·  {(bb - a + 1) / FPS:.2f}s", 13, MUTED, tnum=True)
        cx = 520 - 16
        if r:
            cx -= 58
            chip(b, row, cx, 25, "Red", RED, "#FDECEE", w=58, dot=RED)
        if l:
            cx -= 108
            chip(b, row, cx, 25, "Luminance", "#9A6400", "#FDF3DC", w=100, dot=LUM)
        b.rect(row, 72, 64, 260, 3, fill=SOFT, radius=1.5)
        b.rect(row, 72 + (a / FRAMES) * 260, 64, max(4, (bb - a) / FRAMES * 260), 3,
               fill=ACCENT if on else INK, radius=1.5)
        ly += 88

    fd = panel(b, f, X0 + 552, y0 + 84, CW - 552, 692)
    eyebrow(b, fd, 28, 26, "Frame detail")
    b.text(fd, 28, 46, f"Frame {SEL}", 30, INK, "Bold", ls="-2%")
    b.text(fd, 196, 58, f"{t(SEL):.3f}s  ·  in defect 02", 14, MUTED)
    for i, s in enumerate(["←", "→"]):
        b.ellipse(fd, CW - 552 - 28 - 84 + i * 46, 34, 38, 38, fill=SURF, stroke=LINE)
        b.text(fd, CW - 552 - 28 - 84 + i * 46, 43, s, 15, INK, "Medium", width=38, align="center")
    pw = CW - 552 - 56
    ph = pw * 9 / 16
    pv = b.frame(28, 100, pw, ph, parent=fd, radius=14, fills=[lin(["#20263A", "#4A1C33", "#191424"])])
    b.ellipse(pv, pw * 0.45, ph * 0.2, 260, 260, fills=[{"type": "radialGradient", "stops": ["#E5384A99", "#E5384A00"]}])
    b.ellipse(pv, 20, -60, 280, 280, fills=[{"type": "radialGradient", "stops": ["#E39B0B77", "#E39B0B00"]}])
    sx = pw / WIDTH
    L, R = data[SEL]["l"], data[SEL]["r"]
    for (x1, y1, x2, y2), col, nm in [(L["region"], "#FFC542", "Luminance region"),
                                      (R["region"], "#FF6B7A", "Red region")]:
        b.rect(pv, x1 * sx, y1 * sx, (x2 - x1) * sx, (y2 - y1) * sx, stroke=col, sw=2, radius=4, fill="#FFFFFF14")
        b.rect(pv, x1 * sx, y1 * sx - 24, len(nm) * 6.6 + 16, 20, fill=col, radius=6)
        b.text(pv, x1 * sx, y1 * sx - 21, nm, 11, INK, "SemiBold", width=len(nm) * 6.6 + 16, align="center")
    b.rect(pv, 12, ph - 34, 196, 22, fill="#00000066", radius=11)
    b.text(pv, 12, ph - 30, "1920×1080  ·  source pixels", 11, "#FFFFFFCC", width=196, align="center")

    my = 100 + ph + 20
    mw2 = (pw - 16) / 2
    for i, (nm, col, tint, m) in enumerate([("Luminance", LUM, "#FDF6E6", L), ("Red", RED, "#FDEEF0", R)]):
        cx = 28 + i * (mw2 + 16)
        c = b.frame(cx, my, mw2, 692 - my - 28, parent=fd, fill=tint, radius=14)
        b.ellipse(c, 18, 21, 8, 8, fill=col)
        b.text(c, 34, 16, nm, 14, INK, "SemiBold")
        chip(b, c, mw2 - 18 - 64, 14, "Failed", "#FFFFFF", RED, w=64, h=22, size=11)
        b.text(c, 18, 48, str(m["tr"]), 40, INK, "Bold", ls="-2%", tnum=True)
        b.text(c, 18 + len(str(m["tr"])) * 24 + 6, 70, "transitions  /  6 allowed", 12, MUTED)
        x1, y1, x2, y2 = m["region"]
        ry = 112
        for k, v in [("Since", f"frame {m['since']}"), ("Region", f"{x1},{y1} → {x2},{y2}")]:
            b.rect(c, 18, ry - 8, mw2 - 36, 1, fill="#0F0F1214")
            b.text(c, 18, ry, k, 12, MUTED)
            b.text(c, 18, ry, v, 12, INK, "Medium", font=MONO if k == "Region" else "Inter",
                   width=mw2 - 36, align="right", tnum=True)
            ry += 30
        b.rect(c, 18, ry - 8, mw2 - 36, 1, fill="#0F0F1214")
        b.text(c, 18, ry, "Area", 12, MUTED)
        b.text(c, 18, ry, f"{m['area']:.1f}%", 12, INK, "SemiBold", width=mw2 - 36, align="right", tnum=True)
        bw = mw2 - 36
        b.rect(c, 18, ry + 26, bw, 6, fill="#0F0F1214", radius=3)
        b.rect(c, 18, ry + 26, bw * m["area"] / 100, 6, fill=col, radius=3)
        b.rect(c, 18 + bw * 0.25 - 1, ry + 20, 2, 18, fill=INK)
        b.text(c, 18 + bw * 0.25 + 6, ry + 40, "25% limit", 10, MUTED)
    return y0 + 84 + 704


def thresholds(b, f, sy):
    b.text(f, X0, sy, "Thresholds", 28, INK, "Bold", ls="-2%")
    chip(b, f, X0 + 162, sy + 6, "WCAG 2.2 preset", ACCENT, "#E8ECFF", w=128)
    sp = panel(b, f, X0, sy + 56, CW, 248)
    rows = [("Luminance change", "0.1", "Relative luminance change of a transition"),
            ("Dark limit", "0.8", "The darker side must be below this"),
            ("Red change", "0.2", "u′v′ chromaticity change of a red transition"),
            ("Red saturation", "0.8", "R / (R+G+B) in linear light"),
            ("Region", "341 × 256 px", "Area measured at every position"),
            ("Area", "25%", "Share of a region flashing together"),
            ("Max transitions", "6", "Alternating transitions allowed per second"),
            ("Count window", "30 frames", "One second at 30 fps")]
    colw = CW / 2
    for i, (k, v, d) in enumerate(rows):
        cx = (i % 2) * colw
        cy = 8 + (i // 2) * 58
        if i // 2:
            b.rect(sp, cx + 28, cy, colw - 56, 1, fill=LINE)
        b.text(sp, cx + 28, cy + 12, k, 14, INK, "SemiBold")
        b.text(sp, cx + 28, cy + 32, d, 12, MUTED)
        b.text(sp, cx + colw - 28 - 160, cy + 18, v, 18, INK, "SemiBold", width=160, align="right", tnum=True)
    b.rect(sp, colw, 24, 1, 200, fill=LINE)
    return sy + 56 + 248


def report(b, X, Y):
    data = frames_data()
    nl, nr = failures(data)
    defects = defects_from(data)
    W, H = 1440, 3300
    f = b.frame(X, Y, W, H, name="B1 · Report · Admin", fill=BG)
    nav(b, f, "Reports")
    admin_toggle(b, f, X0 + CW - 430, 20, on=True)

    eyebrow(b, f, X0, 116, "Report  ·  WCAG 2.2  ·  22 Oct 2026")
    b.ellipse(f, X0, 172, 18, 18, fill=RED, effects=[shadow("#E5384A66", 14, 0)])
    b.text(f, X0 + 32, 140, "Fails WCAG 2.2", 64, INK, "Bold", ls="-3%")
    b.text(f, X0, 228, f"{len(defects)} flash sequences exceed six alternating transitions in one second. "
           f"{nl} frames fail on luminance and {nr} on red.", 19, MUTED, lh="150%", width=640)

    fc = panel(b, f, X0 + CW - 400, 112, 400, 196)
    b.rect(fc, 20, 20, 88, 50, radius=10, fills=[lin(["#1B2440", "#3B1630"])])
    b.vector(fc, "M0 0 L10 5 L0 10 Z", 59, 40, 9, 10, fill="#FFFFFF")
    b.text(fc, 124, 24, FILE, 15, INK, "SemiBold")
    b.text(fc, 124, 46, "h264  ·  30/1 fps  ·  61.433s", 12, MUTED, tnum=True)
    b.rect(fc, 20, 88, 360, 1, fill=LINE)
    button(b, fc, 20, 104, 176, "Export JSON", "primary", h=40)
    button(b, fc, 204, 104, 176, "Re-run", "ghost", h=40)
    b.rect(fc, 20, 158, 16, 16, radius=4, stroke=FAINT, sw=1.5)
    b.text(fc, 44, 158, "Include every frame's result (-frames)", 12, MUTED)

    ms = panel(b, f, X0, 340, CW, 120)
    metrics = [("Verdict", "Failed", "passed: false", RED), ("Frame rate", "30", "fps", None),
               ("Length", "1:01.43", "1843 frames", None), ("Video size", "1920×1080", "pixels", None),
               ("Luminance", str(nl), "failing frames", LUM), ("Red", str(nr), "failing frames", RED)]
    mw = CW / 6
    for i, (k, v, sub, col) in enumerate(metrics):
        cx = i * mw
        if i:
            b.rect(ms, cx, 24, 1, 72, fill=LINE)
        eyebrow(b, ms, cx + 24, 24, k)
        b.text(ms, cx + 24, 46, v, 26, col or INK, "SemiBold", ls="-2%", tnum=True)
        b.text(ms, cx + 24, 84, sub, 12, FAINT, font=MONO if sub.startswith("passed") else "Inter")

    y = where_in_time(b, f, 492, data)
    y = defects_and_detail(b, f, y + 56, data)
    y = where_on_screen(b, f, y + 32, data)
    y = thresholds(b, f, y + 48)
    b.raw("node.set", {"ids": [f], "props": {"height": y + 120}})
    b.text(f, X0, y + 56, f"Photosafety Validator  ·  WCAG 2.2 SC 2.3.1  ·  sample data", 12, FAINT)
    return f
