"""Version B — "Daylight": light, editorial, single-column flow."""
from fcgen import (B, shadow, lin, LH, FPS, FRAMES, DURATION, WIDTH, HEIGHT, FILE, DEFECTS,
                   FAIL_LUM, FAIL_RED, t, series)

BG = "#F7F6F3"
SURF = "#FFFFFF"
LINE = "#E8E5DF"
SOFT = "#F1EFEA"
INK = "#0F0F12"
MUTED = "#6B6A73"
FAINT = "#A09FA8"
ACCENT = "#2346FF"
LUM = "#E39B0B"
RED = "#E5384A"
PASS = "#12A150"
MONO = "Menlo"
X0, CW = 160, 1120


def soft_shadow():
    return [shadow("#0F0F120A", 2, 1), shadow("#0F0F120F", 24, 8)]


def panel(b, p, x, y, w, h, radius=18):
    return b.frame(x, y, w, h, parent=p, fill=SURF, radius=radius, stroke=LINE, effects=soft_shadow())


def eyebrow(b, p, x, y, s, color=MUTED, width=None, align=None):
    return b.text(p, x, y, s, 11, color, "SemiBold", ls="8%", upper=True, width=width, align=align)


def chip(b, p, x, y, s, fg, bg, w=None, h=26, size=12, dot=None):
    w = w or int(len(s) * size * 0.58 + (34 if dot else 22))
    b.rect(p, x, y, w, h, fill=bg, radius=h / 2)
    tx = x
    if dot:
        b.ellipse(p, x + 11, y + h / 2 - 3, 6, 6, fill=dot)
        tx = x + 8
    b.text(p, tx, y + (h - size * LH) / 2, s, size, fg, "SemiBold", width=w - (tx - x), align="center")
    return w


def button(b, p, x, y, w, s, kind="ghost", h=42):
    if kind == "primary":
        b.rect(p, x, y, w, h, fill=INK, radius=h / 2, effects=[shadow("#0F0F1233", 12, 4)])
        b.text(p, x, y + (h - 14 * LH) / 2, s, 14, "#FFFFFF", "SemiBold", width=w, align="center")
    elif kind == "accent":
        b.rect(p, x, y, w, h, fill=ACCENT, radius=h / 2, effects=[shadow("#2346FF55", 16, 6)])
        b.text(p, x, y + (h - 14 * LH) / 2, s, 14, "#FFFFFF", "SemiBold", width=w, align="center")
    else:
        b.rect(p, x, y, w, h, fill=SURF, radius=h / 2, stroke=LINE)
        b.text(p, x, y + (h - 14 * LH) / 2, s, 14, INK, "Medium", width=w, align="center")


def nav(b, f, active):
    b.rect(f, 0, 0, 1440, 72, fill="#F7F6F3")
    b.rect(f, 0, 71, 1440, 1, fill=LINE)
    b.rect(f, X0, 22, 28, 28, fill=INK, radius=8)
    b.ellipse(f, X0 + 7, 29, 14, 14, stroke="#FFFFFF", sw=2.2)
    b.ellipse(f, X0 + 18, 26, 6, 6, fill=RED)
    b.text(f, X0 + 40, 24, "psv", 19, INK, "Bold", ls="-2%")
    x = X0 + 250
    for it in ["Analyze", "Reports", "Standards", "Docs"]:
        on = it == active
        w = len(it) * 8 + 28
        if on:
            b.rect(f, x - 14, 20, w, 32, fill=SURF, radius=16, stroke=LINE)
        b.text(f, x, 27, it, 14, INK if on else MUTED, "Medium")
        x += w + 16
    chip(b, f, X0 + CW - 196, 23, "v1.0.0", MUTED, SOFT, w=72)
    b.ellipse(f, X0 + CW - 110, 22, 28, 28, fill="#DCD8FF")
    b.text(f, X0 + CW - 110, 29, "BN", 11, ACCENT, "Bold", width=28, align="center")
    b.text(f, X0 + CW - 74, 27, "Local", 13, MUTED, "Medium")


def report(b, X, Y):
    W, H = 1440, 2060
    f = b.frame(X, Y, W, H, name="B1 · Report", fill=BG)
    nav(b, f, "Reports")

    # Hero
    eyebrow(b, f, X0, 116, "Report  ·  WCAG 2.2  ·  22 Oct 2026")
    b.ellipse(f, X0, 172, 18, 18, fill=RED, effects=[shadow("#E5384A66", 14, 0)])
    b.text(f, X0 + 32, 140, "Fails WCAG 2.2", 64, INK, "Bold", ls="-3%")
    b.text(f, X0, 228,
           "Eight flash sequences exceed six alternating transitions in one second. "
           "338 frames fail on luminance and 385 on red.", 19, MUTED, lh="150%", width=640)

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

    # Metric strip
    ms = panel(b, f, X0, 340, CW, 120)
    metrics = [("Verdict", "Failed", "passed: false", RED), ("Frame rate", "30", "fps", None),
               ("Length", "1:01.43", "1843 frames", None), ("Video size", "1920×1080", "pixels", None),
               ("Luminance", str(FAIL_LUM), "failing frames", LUM), ("Red", str(FAIL_RED), "failing frames", RED)]
    mw = CW / 6
    for i, (k, v, sub, col) in enumerate(metrics):
        cx = i * mw
        if i:
            b.rect(ms, cx, 24, 1, 72, fill=LINE)
        eyebrow(b, ms, cx + 24, 24, k)
        b.text(ms, cx + 24, 46, v, 26, col or INK, "SemiBold", ls="-2%", tnum=True)
        b.text(ms, cx + 24, 84, sub, 12, FAINT, font=MONO if sub.startswith("passed") else "Inter")

    # Timeline heat strip
    tp = panel(b, f, X0, 492, CW, 300)
    b.text(tp, 32, 28, "Every frame, at a glance", 22, INK, "SemiBold", ls="-1%")
    b.text(tp, 32, 60, "Colour shows the most alternating transitions in any region. Brackets mark defects.",
           14, MUTED)
    # legend scale
    lx = CW - 32 - 260
    eyebrow(b, tp, lx, 34, "Transitions")
    for i in range(7):
        a = int(18 + i * 37)
        b.rect(tp, lx + 92 + i * 22, 32, 20, 14, fill=f"{LUM}{a:02X}" if False else RED, radius=3,
               opacity=0.1 + i * 0.15)
    b.text(tp, lx + 92, 50, "0", 10, FAINT, font=MONO)
    b.text(tp, lx + 92 + 6 * 22, 50, "12+", 10, FAINT, font=MONO)

    lum, red = series()
    gx0, gx1 = 132, CW - 32
    span = gx1 - gx0
    buckets = 184
    per = FRAMES / buckets
    cwid = span / buckets

    def fx(frame):
        return gx0 + (frame - 1) / (FRAMES - 1) * span

    # defect brackets
    for idx, a, bb, l, r in DEFECTS:
        x1, x2 = fx(a), fx(bb)
        b.rect(tp, x1, 112, x2 - x1, 2, fill=INK)
        b.rect(tp, x1, 112, 2, 8, fill=INK)
        b.rect(tp, x2 - 2, 112, 2, 8, fill=INK)
        b.ellipse(tp, (x1 + x2) / 2 - 10, 92, 20, 20, fill=INK)
        b.text(tp, (x1 + x2) / 2 - 10, 95, str(idx), 11, "#FFFFFF", "Bold", width=20, align="center")
    for li, (nm, data, col) in enumerate([("Luminance", lum, LUM), ("Red", red, RED)]):
        ly = 132 + li * 58
        b.text(tp, 32, ly + 15, nm, 13, INK, "Medium")
        for k in range(buckets):
            a = int(k * per) + 1
            z = int((k + 1) * per)
            m = max(lum_ for lum_ in data[a:z + 1])
            op = 0.06 + min(m, 12) / 12 * 0.94
            b.rect(tp, gx0 + k * cwid, ly, cwid - 1, 48, fill=col if m > 6 else (col if m else "#D9D5CD"),
                   opacity=op if m > 6 else (0.18 + m * 0.05), radius=1.5)
    # axis + scrubber
    for s in range(0, 61, 10):
        xx = gx0 + s * FPS / (FRAMES - 1) * span
        b.text(tp, xx - 20, 266, f"{s}s", 11, FAINT, font=MONO, width=40, align="center")
    sel = 291
    mx = fx(sel)
    b.rect(tp, mx - 1, 126, 2, 120, fill=ACCENT)
    b.ellipse(tp, mx - 7, 240, 14, 14, fill=SURF, stroke=ACCENT, sw=3)
    b.rect(tp, mx + 12, 238, 124, 22, fill=ACCENT, radius=11)
    b.text(tp, mx + 12, 242, "Frame 291 · 9.667s", 11, "#FFFFFF", "SemiBold", width=124, align="center")

    # Defects list
    y0 = 824
    b.text(f, X0, y0, "Defects", 28, INK, "Bold", ls="-2%")
    b.text(f, X0 + 116, y0 + 10, "8", 15, MUTED, "SemiBold")
    b.text(f, X0, y0 + 42, "Runs of frames covered by failing sequences.", 14, MUTED)
    ly = y0 + 84
    for idx, a, bb, l, r in DEFECTS:
        on = idx == 2
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
        # position bar
        b.rect(row, 72, 64, 260, 3, fill=SOFT, radius=1.5)
        b.rect(row, 72 + (a / FRAMES) * 260, 64, max(4, (bb - a) / FRAMES * 260), 3,
               fill=ACCENT if on else INK, radius=1.5)
        ly += 88

    # Frame detail
    fd = panel(b, f, X0 + 552, y0 + 84, CW - 552, 692)
    eyebrow(b, fd, 28, 26, "Frame detail")
    b.text(fd, 28, 46, "Frame 291", 30, INK, "Bold", ls="-2%")
    b.text(fd, 196, 58, "9.667s  ·  in defect 02", 14, MUTED)
    for i, s in enumerate(["←", "→"]):
        b.ellipse(fd, CW - 552 - 28 - 84 + i * 46, 34, 38, 38, fill=SURF, stroke=LINE)
        b.text(fd, CW - 552 - 28 - 84 + i * 46, 43, s, 15, INK, "Medium", width=38, align="center")
    pw = CW - 552 - 56
    ph = pw * 9 / 16
    pv = b.frame(28, 100, pw, ph, parent=fd, radius=14, fills=[lin(["#20263A", "#4A1C33", "#191424"])])
    b.ellipse(pv, pw * 0.45, ph * 0.2, 260, 260, fills=[{"type": "radialGradient", "stops": ["#E5384A99", "#E5384A00"]}])
    b.ellipse(pv, 20, -60, 280, 280, fills=[{"type": "radialGradient", "stops": ["#E39B0B77", "#E39B0B00"]}])
    sx = pw / WIDTH
    for (x1, y1, x2, y2), col, nm in [((512, 320, 853, 576), "#FFC542", "Luminance region"),
                                      ((960, 480, 1301, 736), "#FF6B7A", "Red region")]:
        b.rect(pv, x1 * sx, y1 * sx, (x2 - x1) * sx, (y2 - y1) * sx, stroke=col, sw=2, radius=4,
               fill="#FFFFFF14")
        b.rect(pv, x1 * sx, y1 * sx - 24, len(nm) * 6.6 + 16, 20, fill=col, radius=6)
        b.text(pv, x1 * sx, y1 * sx - 21, nm, 11, INK, "SemiBold", width=len(nm) * 6.6 + 16, align="center")
    b.rect(pv, 12, ph - 34, 196, 22, fill="#00000066", radius=11)
    b.text(pv, 12, ph - 30, "1920×1080  ·  source pixels", 11, "#FFFFFFCC", width=196, align="center")

    # two measurement panels
    my = 100 + ph + 20
    mw2 = (pw - 16) / 2
    for i, (nm, col, tint, tr, since, reg, area) in enumerate([
            ("Luminance", LUM, "#FDF6E6", 9, 276, "512,320 → 853,576", 41.8),
            ("Red", RED, "#FDEEF0", 7, 280, "960,480 → 1301,736", 27.3)]):
        cx = 28 + i * (mw2 + 16)
        c = b.frame(cx, my, mw2, 692 - my - 28, parent=fd, fill=tint, radius=14)
        b.ellipse(c, 18, 21, 8, 8, fill=col)
        b.text(c, 34, 16, nm, 14, INK, "SemiBold")
        chip(b, c, mw2 - 18 - 64, 14, "Failed", "#FFFFFF", RED, w=64, h=22, size=11)
        b.text(c, 18, 48, str(tr), 40, INK, "Bold", ls="-2%", tnum=True)
        b.text(c, 18 + len(str(tr)) * 24 + 6, 70, "transitions  /  6 allowed", 12, MUTED)
        rows = [("Since", f"frame {since}"), ("Region", reg)]
        ry = 112
        for k, v in rows:
            b.rect(c, 18, ry - 8, mw2 - 36, 1, fill="#0F0F1214")
            b.text(c, 18, ry, k, 12, MUTED)
            b.text(c, 18, ry, v, 12, INK, "Medium", font=MONO if k == "Region" else "Inter",
                   width=mw2 - 36, align="right", tnum=True)
            ry += 30
        b.rect(c, 18, ry - 8, mw2 - 36, 1, fill="#0F0F1214")
        b.text(c, 18, ry, "Area", 12, MUTED)
        b.text(c, 18, ry, f"{area}%", 12, INK, "SemiBold", width=mw2 - 36, align="right", tnum=True)
        bw = mw2 - 36
        b.rect(c, 18, ry + 26, bw, 6, fill="#0F0F1214", radius=3)
        b.rect(c, 18, ry + 26, bw * area / 100, 6, fill=col, radius=3)
        b.rect(c, 18 + bw * 0.25 - 1, ry + 20, 2, 18, fill=INK)
        b.text(c, 18 + bw * 0.25 + 6, ry + 40, "25% limit", 10, MUTED)

    # Spec sheet
    sy = y0 + 84 + 692 + 48
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
    if True:
        b.rect(sp, colw, 24, 1, 200, fill=LINE)

    b.text(f, X0, H - 60, "Photosafety Validator  ·  WCAG 2.2 SC 2.3.1  ·  Trace RERC conformance: 369 / 369",
           12, FAINT)
    return f


def analyze(b, X, Y):
    W, H = 1440, 1240
    f = b.frame(X, Y, W, H, name="B2 · Analyze", fill=BG)
    nav(b, f, "Analyze")
    cx0, cw = 260, 920

    # stepper
    steps = [("1", "Source", "done"), ("2", "Standard", "active"), ("3", "Analyse", "todo")]
    sx = cx0 + 160
    for i, (n, nm, st) in enumerate(steps):
        x = sx + i * 220
        if st == "done":
            b.ellipse(f, x, 112, 28, 28, fill=INK)
            b.vector(f, "M0 5 L4 9 L11 1", x + 8, 121, 12, 9, stroke="#FFFFFF", sw=2)
        elif st == "active":
            b.ellipse(f, x, 112, 28, 28, fill=ACCENT, effects=[shadow("#2346FF55", 10, 2)])
            b.text(f, x, 117, n, 13, "#FFFFFF", "Bold", width=28, align="center")
        else:
            b.ellipse(f, x, 112, 28, 28, fill=SURF, stroke=LINE)
            b.text(f, x, 117, n, 13, FAINT, "Bold", width=28, align="center")
        b.text(f, x + 38, 117, nm, 14, INK if st != "todo" else FAINT, "SemiBold")
        if i < 2:
            b.rect(f, x + 120, 125, 84, 2, fill=INK if st == "done" else LINE, radius=1)

    b.text(f, cx0, 172, "Choose a standard", 48, INK, "Bold", width=cw, align="center", ls="-3%")
    b.text(f, cx0, 236, "Each preset sets intensity, area and count thresholds. You can fine-tune any of them.",
           17, MUTED, width=cw, align="center")

    # source summary
    sp = panel(b, f, cx0, 296, cw, 96)
    b.rect(sp, 20, 20, 96, 56, radius=10, fills=[lin(["#1B2440", "#3B1630"])])
    b.vector(sp, "M0 0 L10 5 L0 10 Z", 64, 43, 9, 10, fill="#FFFFFF")
    b.text(sp, 136, 24, FILE, 16, INK, "SemiBold")
    b.text(sp, 136, 50, "248.6 MB  ·  Change file", 13, MUTED)
    probe = [("Codec", "h264"), ("Size", "1920×1080"), ("Rate", "30/1"), ("Frames", "1843"), ("Duration", "61.433s")]
    px = 470
    for k, v in probe:
        eyebrow(b, sp, px, 26, k)
        b.text(sp, px, 46, v, 15, INK, "SemiBold", tnum=True)
        px += 92 if k != "Size" else 112

    # presets
    presets = [("WCAG 2.2", "wcag22", "Web content", "341×256", "25%", "6", True),
               ("Trace 2024", "trace24", "Proposed guideline", "416×416", "25%", "6", False),
               ("Broadcast", "broadcast", "ITU-R · Ofcom · ISO", "Full screen", "25%", "6", False),
               ("Scene change", "scene", "Experimental", "Full screen", "80%", "3", False)]
    pw = (cw - 3 * 16) / 4
    for i, (nm, flag, who, reg, area, mx, on) in enumerate(presets):
        x = cx0 + i * (pw + 16)
        c = b.frame(x, 424, pw, 236, parent=f, fill=SURF, radius=18, stroke=ACCENT if on else LINE,
                    sw=2 if on else 1, effects=[shadow("#2346FF2E", 24, 10)] if on else soft_shadow())
        b.ellipse(c, pw - 40, 20, 20, 20, fill=ACCENT if on else SURF, stroke=None if on else LINE, sw=1.5)
        if on:
            b.vector(c, "M0 4 L3.5 7.5 L9 1", pw - 35, 27, 10, 7, stroke="#FFFFFF", sw=2)
        eyebrow(b, c, 20, 22, who, ACCENT if on else FAINT)
        b.text(c, 20, 44, nm, 22, INK, "Bold", ls="-2%")
        b.text(c, 20, 74, f"-standard {flag}", 11, FAINT, font=MONO)
        for j, (k, v) in enumerate([("Region", reg), ("Area", area), ("Max / s", mx)]):
            ry = 110 + j * 38
            b.rect(c, 20, ry, pw - 40, 1, fill=LINE)
            b.text(c, 20, ry + 12, k, 13, MUTED)
            b.text(c, 20, ry + 12, v, 13, INK, "SemiBold", width=pw - 40, align="right", tnum=True)

    # advanced
    ad = panel(b, f, cx0, 692, cw, 400)
    b.text(ad, 28, 24, "Advanced", 18, INK, "SemiBold")
    b.text(ad, 28, 50, "Blank fields keep the preset. 1 override set.", 13, MUTED)
    b.text(ad, cw - 28 - 160, 30, "Reset to preset", 13, ACCENT, "SemiBold", width=160, align="right")
    fields = [("Luminance change", "0.1", None, ""), ("Dark limit", "0.8", None, ""),
              ("Red change", "0.2", None, "Δu′v′"), ("Red saturation", "0.8", None, "R/ΣRGB"),
              ("Region width", "341", None, "px"), ("Region height", "256", None, "px"),
              ("Area", "25", "30", "%"), ("Max transitions", "6", None, "/ s")]
    fw = (cw - 56 - 3 * 16) / 4
    for i, (k, ph, val, unit) in enumerate(fields):
        x = 28 + (i % 4) * (fw + 16)
        y = 92 + (i // 4) * 88
        on = val is not None
        b.text(ad, x, y, k, 12, INK if on else MUTED, "SemiBold" if on else "Medium")
        b.rect(ad, x, y + 22, fw, 44, fill="#F2F4FF" if on else SOFT, radius=12,
               stroke=ACCENT if on else None, sw=1.5)
        b.text(ad, x + 14, y + 35, val or ph, 15, INK if on else FAINT, "SemiBold" if on else "Regular", tnum=True)
        if unit:
            b.text(ad, x + fw - 74, y + 37, unit, 12, FAINT, width=60, align="right")
    b.rect(ad, 28, 278, cw - 56, 1, fill=LINE)
    # decoding toggles
    b.text(ad, 28, 300, "Convert frame rate", 14, INK, "SemiBold")
    b.text(ad, 28, 322, "Resample variable-rate video before analysis", 12, MUTED)
    b.rect(ad, 28, 346, 150, 36, fill=SOFT, radius=10)
    b.text(ad, 42, 356, "Off", 14, FAINT)
    b.text(ad, 120, 357, "fps", 12, FAINT, width=44, align="right")
    b.text(ad, 480, 300, "Deinterlace", 14, INK, "SemiBold")
    b.text(ad, 480, 322, "Only frames flagged as interlaced", 12, MUTED)
    b.rect(ad, 480, 350, 48, 28, fill="#D9D5CD", radius=14)
    b.ellipse(ad, 483, 353, 22, 22, fill=SURF, effects=[shadow("#0000002A", 3, 1)])

    # action bar
    b.rect(f, 0, H - 112, W, 112, fill="#FFFFFFE6")
    b.rect(f, 0, H - 112, W, 1, fill=LINE)
    b.text(f, cx0, H - 72, "WCAG 2.2  ·  area 30% (override)  ·  1843 frames at 30 fps", 14, MUTED)
    button(b, f, cx0 + cw - 340, H - 80, 140, "Back", "ghost", h=48)
    button(b, f, cx0 + cw - 188, H - 80, 188, "Start analysis  →", "accent", h=48)
    return f


def analysing(b, X, Y):
    W, H = 1440, 900
    f = b.frame(X, Y, W, H, name="B3 · Analysing", fill=BG)
    nav(b, f, "Analyze")
    p = panel(b, f, 360, 140, 720, 640, radius=24)
    cxr = 360
    # ring
    tid = b.ellipse(p, cxr - 110, 52, 220, 220, fill=SOFT)
    rid = b.ellipse(p, cxr - 110, 52, 220, 220, fill=ACCENT)
    b.raw("node.set", {"ids": [tid], "props": {"arcInner": 0.86}})
    b.raw("node.set", {"ids": [rid], "props": {"arcStart": -90, "arcSweep": 234, "arcInner": 0.86}})
    b.text(p, cxr - 110, 118, "65%", 52, INK, "Bold", width=220, align="center", ls="-3%", tnum=True)
    b.text(p, cxr - 110, 186, "1204 of 1843 frames", 13, MUTED, width=220, align="center", tnum=True)
    b.text(p, 0, 300, "Analysing " + FILE, 22, INK, "SemiBold", width=720, align="center")
    b.text(p, 0, 334, "0:40 of 1:01 read  ·  about 6 seconds left  ·  frames are decided one second behind",
           14, MUTED, width=720, align="center")

    # live strip
    lum, red = series()
    gx, gw = 48, 624
    buckets = 120
    per = FRAMES / buckets
    done = int(buckets * 1175 / FRAMES)
    eyebrow(b, p, gx, 384, "Live results")
    b.text(p, gx, 384, "1175 frames decided", 12, MUTED, width=gw, align="right", tnum=True)
    for li, (data, col) in enumerate([(lum, LUM), (red, RED)]):
        y = 410 + li * 34
        for k in range(buckets):
            a = int(k * per) + 1
            z = int((k + 1) * per)
            m = max(data[a:z + 1])
            cw_ = gw / buckets
            if k >= done:
                b.rect(p, gx + k * cw_, y, cw_ - 1, 28, fill=SOFT, radius=1.5)
            else:
                b.rect(p, gx + k * cw_, y, cw_ - 1, 28, fill=col if m else "#D9D5CD",
                       opacity=(0.06 + min(m, 12) / 12 * 0.94) if m > 6 else (0.18 + m * 0.05), radius=1.5)
    from fcgen import frames_data, failures, defects_from
    live = frames_data()
    ln, rn = failures(live, 1175)
    stats = [("Luminance failures", str(ln), LUM), ("Red failures", str(rn), RED),
             ("Defects so far", str(len(defects_from(live, 1175))), INK)]
    sw_ = (gw - 32) / 3
    for i, (k, v, col) in enumerate(stats):
        x = gx + i * (sw_ + 16)
        b.rect(p, x, 500, sw_, 84, fill=SOFT, radius=14)
        eyebrow(b, p, x + 16, 516, k)
        b.text(p, x + 16, 536, v, 28, col, "Bold", tnum=True, ls="-2%")
    button(b, f, 660, 812, 120, "Cancel", "ghost", h=44)
    return f


if __name__ == "__main__":
    import report_b
    b = B()
    report_b.report(b, 0, 0)
    import lean_b
    lean_b.report_lean(b, -1540, 0)
    analyze(b, 1540, 0)
    analysing(b, 1540, 1340)
    b.run("version_b.figcraft")
