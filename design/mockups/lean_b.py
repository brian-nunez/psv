"""Version B lean report: the default view for end users."""
from fcgen import (shadow, lin, FPS, FRAMES, WIDTH, HEIGHT, FILE, CELL, LOC, frames_data, defects_from, heat)
from version_b import (BG, SURF, LINE, SOFT, INK, MUTED, FAINT, ACCENT, RED, MONO, X0, CW,
                       panel, eyebrow, chip, button, nav)
from report_b import SEL, WIN, polyline, admin_toggle

FLASH = "#E5484D"


def clock(sec):
    return f"{int(sec // 60)}:{sec % 60:04.1f}"


def where_words(cx, cy):
    col = ["left", "centre", "right"][min(2, int(cx / (WIDTH / 3)))]
    row = ["Top", "Middle", "Bottom"][min(2, int(cy / (HEIGHT / 3)))]
    if row == "Middle" and col == "centre":
        return "Centre"
    return f"{row} {col}"


def merged(data, fr):
    l, r = data[fr]["l"], data[fr]["r"]
    return max(l["tr"], r["tr"]), l["failed"] or r["failed"]


def when(b, f, y, data, defects):
    H = 520
    p = panel(b, f, X0, y, CW, H)
    b.text(p, 32, 28, "When it flashes", 22, INK, "SemiBold", ls="-1%")
    b.text(p, 32, 60, "Red marks the moments that could trigger a seizure. Select one to look closer.", 14, MUTED)

    gx0, gx1 = 32, CW - 32
    span = gx1 - gx0

    def ox(fr):
        return gx0 + (fr - 1) / (FRAMES - 1) * span

    buckets = 200
    per = FRAMES / buckets
    cw_ = span / buckets
    for i in range(buckets):
        a, z = int(i * per) + 1, int((i + 1) * per)
        hz = any(merged(data, q)[1] for q in range(a, z + 1))
        m = max(merged(data, q)[0] for q in range(a, z + 1))
        b.rect(p, gx0 + i * cw_, 130, cw_ - 1, 36, fill=FLASH if hz else "#D9D5CD",
               opacity=1 if hz else 0.25 + m * 0.06, radius=1.5)
    for i, (a, bb, _, _) in enumerate(defects):
        cx = (ox(a) + ox(bb)) / 2
        b.ellipse(p, cx - 10, 102, 20, 20, fill=INK)
        b.text(p, cx - 10, 105, str(i + 1), 10, "#FFFFFF", "Bold", width=20, align="center")
    wx1, wx2 = ox(WIN[0]), ox(WIN[1])
    b.rect(p, wx1, 124, wx2 - wx1, 48, fill="#2346FF14", stroke=ACCENT, sw=1.5, radius=6)
    for hx in (wx1 - 3, wx2 - 3):
        b.rect(p, hx, 138, 6, 20, fill=ACCENT, radius=3)
    for s in range(0, 61, 10):
        xx = gx0 + s * FPS / (FRAMES - 1) * span
        b.text(p, xx - 20, 176, f"0:{s:02d}" if s < 60 else "1:00", 11, FAINT, width=40, align="center")

    b.rect(p, 32, 212, CW - 64, 1, fill=LINE)
    b.text(p, 32, 232, "Moment 2", 17, INK, "SemiBold")
    b.text(p, 124, 235, f"{clock((WIN[0] - 1) / FPS)} – {clock(WIN[1] / FPS)}", 14, MUTED)
    for i, s in enumerate(["←  Previous", "Next  →"]):
        w = 112 if i == 0 else 92
        x = CW - 32 - 212 + (0 if i == 0 else 120)
        b.rect(p, x, 226, w, 32, fill=SURF, radius=16, stroke=LINE)
        b.text(p, x, 233, s, 13, INK, "SemiBold", width=w, align="center")

    ty, th = 290, 150
    n = WIN[1] - WIN[0] + 1
    slot = span / n

    def zx(fr):
        return gx0 + (fr - WIN[0]) * slot

    # hazardous span shading
    hz = [fr for fr in range(WIN[0], WIN[1] + 1) if merged(data, fr)[1]]
    first = min(data[fr][k]["since"] for fr in hz for k in ("l", "r") if data[fr][k]["failed"])
    b.rect(p, zx(first), ty - 6, zx(hz[-1]) + slot - zx(first), th + 12, fill="#E5484D12", radius=8)
    b.text(p, zx(first) + 10, ty + 2, "Hazardous flashing", 11, FLASH, "SemiBold")
    for i in range(n):
        fr = WIN[0] + i
        v, bad = merged(data, fr)
        if not v:
            continue
        hgt = min(v, 12) / 12 * (th - 20)
        b.rect(p, zx(fr) + 0.8, ty + th - hgt, slot - 1.6, hgt, fill=FLASH if bad else "#C9C4BA",
               opacity=1 if bad else 0.6, radius=1.5)
    ly = ty + th - 6.5 / 12 * (th - 20)
    b.rect(p, gx0, ly, span, 1.5, stroke=INK, sw=1.5, dashes=[5, 4])
    chip(b, p, gx1 - 90, ly - 24, "Safe limit", "#FFFFFF", INK, w=86, h=20, size=10)
    b.rect(p, gx0, ty + th, span, 1, fill=LINE)
    b.text(p, gx0, ty - 18, "Flash strength", 11, MUTED, "Medium")
    for fr in range(WIN[0], WIN[1] + 1, 30):
        b.text(p, zx(fr) - 30, ty + th + 10, clock((fr - 1) / FPS), 11, FAINT, width=60, align="center")

    mx = zx(SEL) + slot / 2
    b.rect(p, mx - 1, ty - 14, 2, th + 18, fill=ACCENT)
    b.ellipse(p, mx - 6, ty - 20, 12, 12, fill=ACCENT, stroke="#FFFFFF", sw=2)
    tip = b.frame(mx - 200, ty + 22, 186, 62, parent=p, fill=SURF, radius=12, stroke=LINE,
                  effects=[shadow("#0F0F1222", 24, 8)])
    b.text(tip, 14, 12, clock((SEL - 1) / FPS), 14, INK, "Bold")
    b.ellipse(tip, 14, 41, 8, 8, fill=FLASH)
    b.text(tip, 28, 36, "Flashing above the safe limit", 11, MUTED)
    return y + H


def moments(b, f, y0, data, defects):
    b.text(f, X0, y0, "Flashing moments", 28, INK, "Bold", ls="-2%")
    b.text(f, X0 + 252, y0 + 10, str(len(defects)), 15, MUTED, "SemiBold")
    total = sum((bb - a + 1) for a, bb, _, _ in defects) / FPS
    b.text(f, X0, y0 + 42, f"{len(defects)} moments, {total:.1f} seconds in total. Each needs editing before release.",
           14, MUTED)
    ly = y0 + 84
    for i, (a, bb, _, _) in enumerate(defects):
        idx = i + 1
        on = a <= SEL <= bb
        regs = [data[fr][k]["region"] for fr in range(a, bb + 1) for k in ("l", "r") if data[fr][k]["failed"]]
        cx = sum((r[0] + r[2]) / 2 for r in regs) / len(regs)
        cy = sum((r[1] + r[3]) / 2 for r in regs) / len(regs)
        row = b.frame(X0, ly, 520, 72, parent=f, fill=SURF, radius=14,
                      stroke=ACCENT if on else LINE, sw=1.5 if on else 1,
                      effects=[shadow("#2346FF26", 18, 6)] if on else None)
        b.rect(row, 16, 16, 40, 40, fill="#EEF1FF" if on else SOFT, radius=10)
        b.text(row, 16, 27, str(idx), 14, ACCENT if on else INK, "Bold", width=40, align="center")
        b.text(row, 72, 15, f"{clock((a - 1) / FPS)} – {clock(bb / FPS)}", 15, INK, "SemiBold", tnum=True)
        b.text(row, 72, 39, f"{(bb - a + 1) / FPS:.1f} seconds of flashing", 13, MUTED)
        # mini screen with location
        ms = b.frame(520 - 16 - 72, 14, 72, 44, parent=row, fill=SOFT, radius=6, stroke=LINE)
        b.ellipse(ms, cx / WIDTH * 72 - 6, cy / HEIGHT * 44 - 6, 12, 12, fill=FLASH,
                  effects=[shadow("#E5484D88", 6, 0)])
        b.text(row, 520 - 16 - 72 - 140, 27, where_words(cx, cy), 13, INK, "Medium", width=128, align="right")
        ly += 84

    fd = panel(b, f, X0 + 552, y0 + 84, CW - 552, 662)
    eyebrow(b, fd, 28, 26, "Moment 2")
    b.text(fd, 28, 46, "Here's what flashes", 26, INK, "Bold", ls="-2%")
    pw = CW - 552 - 56
    ph = pw * 9 / 16
    pv = b.frame(28, 100, pw, ph, parent=fd, radius=14, fills=[lin(["#20263A", "#4A1C33", "#191424"])])
    b.ellipse(pv, pw * 0.45, ph * 0.2, 260, 260, fills=[{"type": "radialGradient", "stops": ["#E5384A99", "#E5384A00"]}])
    b.ellipse(pv, 20, -60, 280, 280, fills=[{"type": "radialGradient", "stops": ["#E39B0B77", "#E39B0B00"]}])
    sx = pw / WIDTH
    regs = [data[SEL]["l"]["region"], data[SEL]["r"]["region"]]
    for x1, y1, x2, y2 in regs:
        b.rect(pv, x1 * sx, y1 * sx, (x2 - x1) * sx, (y2 - y1) * sx, stroke="#FFFFFF", sw=2, radius=6,
               fill="#E5484D40", dashes=[6, 4])
    ux, uy = min(r[0] for r in regs) * sx, min(r[1] for r in regs) * sx
    b.rect(pv, ux, uy - 26, 108, 22, fill="#FFFFFF", radius=11)
    b.ellipse(pv, ux + 10, uy - 18, 7, 7, fill=FLASH)
    b.text(pv, ux + 22, uy - 22, "Flashing area", 11, INK, "SemiBold")
    b.rect(pv, 12, ph - 34, 84, 22, fill="#00000066", radius=11)
    b.text(pv, 12, ph - 30, clock((SEL - 1) / FPS), 11, "#FFFFFFDD", width=84, align="center", font=MONO)

    ty = 100 + ph + 24
    a, bb = 276, 329
    rows = [("When", f"{clock((a - 1) / FPS)} – {clock(bb / FPS)}"),
            ("Where", "Centre, spreading to the right"),
            ("How long", f"{(bb - a + 1) / FPS:.1f} seconds")]
    for i, (k, v) in enumerate(rows):
        yy = ty + i * 40
        if i:
            b.rect(fd, 28, yy - 10, pw, 1, fill=LINE)
        b.text(fd, 28, yy, k, 14, MUTED)
        b.text(fd, 28, yy, v, 14, INK, "SemiBold", width=pw, align="right")
    b.rect(fd, 28, ty + 124, pw, 64, fill="#FDEEF0", radius=12)
    b.text(fd, 44, ty + 136, "Fix: slow the flashing to 3 or fewer per second,", 13, INK, "Medium")
    b.text(fd, 44, ty + 156, "or make the flashing area smaller.", 13, INK, "Medium")
    return y0 + 84 + 672


def where(b, f, y, data):
    g, cols, rows = heat(data)
    H = 760
    p = panel(b, f, X0, y, CW, H)
    b.text(p, 32, 28, "Where on screen", 22, INK, "SemiBold", ls="-1%")
    b.text(p, 32, 60, "Brighter areas flash more often across the whole video. Numbers match the moments above.",
           14, MUTED)
    mw = CW - 64
    mh = mw * 9 / 16
    mh = min(mh, 560)
    mw = mh * 16 / 9
    mx0 = (CW - mw) / 2
    mp = b.frame(mx0, 100, mw, mh, parent=p, radius=16, fills=[lin(["#1A1F2E", "#2A1A2A", "#14121C"])])
    cs = mw / cols
    tot = [[g["l"][r][c] + g["r"][r][c] for c in range(cols)] for r in range(rows)]
    peak = max(max(row) for row in tot)
    for r in range(rows):
        for c in range(cols):
            v = tot[r][c]
            if v:
                b.rect(mp, c * cs + 0.75, r * cs + 0.75, cs - 1.5, cs - 1.5, fill=FLASH, radius=4,
                       opacity=0.15 + 0.85 * (v / peak) ** 0.8)
    for i in (1, 2):
        b.rect(mp, mw * i / 3, 0, 1, mh, fill="#FFFFFF14")
        b.rect(mp, 0, mh * i / 3, mw, 1, fill="#FFFFFF14")
    sx = mw / WIDTH
    for idx, kinds in LOC.items():
        xs = [v[0] for v in kinds.values()]
        ys = [v[1] for v in kinds.values()]
        cx, cy = sum(xs) / len(xs) * sx, sum(ys) / len(ys) * sx
        b.ellipse(mp, cx - 13, cy - 13, 26, 26, fill="#FFFFFF", effects=[shadow("#00000066", 8, 2)])
        b.text(mp, cx - 13, cy - 8, str(idx), 12, INK, "Bold", width=26, align="center")

    ly = 100 + mh + 22
    b.text(p, mx0, ly, "Less often", 12, MUTED)
    for i in range(10):
        b.rect(p, mx0 + 74 + i * 22, ly + 2, 20, 12, fill=FLASH, opacity=0.15 + i * 0.09, radius=3)
    b.text(p, mx0 + 302, ly, "More often", 12, MUTED)
    hot = max((tot[r][c], r, c) for r in range(rows) for c in range(cols))
    covered = sum(1 for r in range(rows) for c in range(cols) if tot[r][c]) / (rows * cols)
    chips = [f"Most flashing: {where_words((hot[2] + 0.5) * CELL, (hot[1] + 0.5) * CELL).lower()}",
             f"Covers about {round(covered * 10) * 10}% of the screen"]
    x = mx0 + mw
    for s in reversed(chips):
        w = len(s) * 7 + 28
        x -= w
        chip(b, p, x, ly - 3, s, INK, SOFT, w=w, h=26, size=12)
        x -= 10
    return y + H


def report_lean(b, X, Y):
    data = frames_data()
    defects = defects_from(data)
    W = 1440
    f = b.frame(X, Y, W, 2400, name="B4 · Report · Users", fill=BG)
    nav(b, f, "Reports")

    eyebrow(b, f, X0, 116, FILE)
    b.ellipse(f, X0, 168, 16, 16, fill=FLASH, effects=[shadow("#E5484D66", 14, 0)])
    b.text(f, X0 + 30, 140, "Not safe to publish", 56, INK, "Bold", ls="-3%")
    total = sum((bb - a + 1) for a, bb, _, _ in defects) / FPS
    b.text(f, X0, 216, f"This video has {len(defects)} moments of flashing that could trigger seizures in people "
           f"with photosensitive epilepsy. Together they last {total:.1f} seconds.", 18, MUTED, lh="150%", width=660)

    admin_toggle(b, f, X0 + CW - 430, 20, on=False)
    fc = panel(b, f, X0 + CW - 360, 128, 360, 124)
    button(b, fc, 20, 20, 156, "Download report", "primary", h=40)
    button(b, fc, 184, 20, 156, "Check another", "ghost", h=40)
    b.text(fc, 20, 84, "Checked against WCAG 2.2 on 22 Oct 2026", 12, FAINT)

    y = when(b, f, 332, data, defects)
    y = moments(b, f, y + 56, data, defects)
    y = where(b, f, y + 32, data)
    b.raw("node.set", {"ids": [f], "props": {"height": y + 100}})
    b.text(f, X0, y + 44, "Photosafety Validator  ·  sample data", 12, FAINT)
    return f
