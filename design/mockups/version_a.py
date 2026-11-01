"""Version A — "Obsidian": dark, dense, console-style dashboard."""
from fcgen import (B, shadow, lin, LH, FPS, FRAMES, DURATION, WIDTH, HEIGHT, FILE, DEFECTS,
                   FAIL_LUM, FAIL_RED, t, series, fmt_dur)

BG = "#0A0B0F"
SIDE = "#0D0F14"
CARD = "#12141B"
CARD2 = "#171A23"
LINE = "#1E2230"
GRID = "#1A1D27"
INK = "#ECEEF3"
MUTED = "#8A91A3"
FAINT = "#5B6275"
ACCENT = "#8B7CFF"
CYAN = "#22D3EE"
LUM = "#F5C451"
RED = "#FF4D5E"
PASS = "#34D399"
MONO = "Menlo"


def card(b, p, x, y, w, h, title=None, sub=None):
    c = b.frame(x, y, w, h, parent=p, fill=CARD, radius=14, stroke=LINE)
    if title:
        b.text(c, 24, 20, title, 15, INK, "SemiBold")
    if sub:
        b.text(c, 24, 42, sub, 12, FAINT)
    return c


def label(b, p, x, y, s, color=FAINT, width=None, align=None):
    return b.text(p, x, y, s, 11, color, "Medium", ls="6%", upper=True, width=width, align=align)


def pill(b, p, x, y, s, fg, bg, h=24, size=12, w=None, stroke=None):
    w = w or int(len(s) * size * 0.6 + 22)
    b.rect(p, x, y, w, h, fill=bg, radius=h / 2, stroke=stroke)
    b.text(p, x, y + (h - size * LH) / 2, s, size, fg, "SemiBold", width=w, align="center")
    return w


def button(b, p, x, y, w, s, primary=False):
    if primary:
        b.rect(p, x, y, w, 38, fills=[lin([ACCENT, "#6D5BFF"])], radius=10,
               effects=[shadow("#8B7CFF55", 18, 6)])
        b.text(p, x, y + 11, s, 13, "#FFFFFF", "SemiBold", width=w, align="center")
    else:
        b.rect(p, x, y, w, 38, fill=CARD2, radius=10, stroke=LINE)
        b.text(p, x, y + 11, s, 13, INK, "Medium", width=w, align="center")


def sidebar(b, f, h, active):
    b.rect(f, 0, 0, 248, h, fill=SIDE)
    b.rect(f, 247, 0, 1, h, fill=LINE)
    b.rect(f, 24, 26, 34, 34, fills=[lin([ACCENT, CYAN])], radius=10,
           effects=[shadow("#8B7CFF66", 16, 4)])
    b.ellipse(f, 33, 35, 16, 16, stroke="#FFFFFF", sw=2.5)
    b.text(f, 70, 26, "PSV", 16, INK, "Bold", ls="4%")
    b.text(f, 70, 46, "Photosafety Validator", 11, FAINT)

    items = ["New analysis", "Reports", "Standards", "Settings"]
    y = 104
    for it in items:
        if it == active:
            b.rect(f, 16, y - 8, 216, 36, fill=CARD2, radius=9)
            b.rect(f, 16, y + 2, 3, 16, fill=ACCENT, radius=2)
        b.rect(f, 32, y + 3, 14, 14, radius=4, stroke=ACCENT if it == active else FAINT, sw=1.5)
        b.text(f, 58, y + 1, it, 13, INK if it == active else MUTED, "Medium")
        y += 40

    label(b, f, 32, 290, "Recent")
    recent = [("launch-trailer-final.mp4", RED, "Failed"), ("promo-30s-v4.mov", PASS, "Passed"),
              ("ident-sting.mp4", PASS, "Passed"), ("gameplay-capture.mkv", RED, "Failed")]
    y = 320
    for name, col, _ in recent:
        b.ellipse(f, 34, y + 5, 7, 7, fill=col)
        b.text(f, 52, y, name, 12, MUTED if name != FILE else INK, width=180)
        y += 30

    b.rect(f, 16, h - 84, 216, 60, fill=CARD, radius=10, stroke=LINE)
    b.text(f, 32, h - 72, "psv v1.0.0", 12, INK, "Medium")
    b.text(f, 32, h - 52, "ffmpeg 7.1 · ffprobe 7.1", 11, FAINT, font=MONO)


def report(b, X, Y):
    W, H = 1440, 1478
    f = b.frame(X, Y, W, H, name="A1 · Report", fill=BG)
    sidebar(b, f, H, "Reports")
    x0, cw = 288, 1112

    # Header
    b.text(f, x0, 30, "Reports  /  " + FILE, 12, FAINT)
    b.text(f, x0, 52, FILE, 28, INK, "Bold", ls="-1%")
    b.text(f, x0, 92, "h264  ·  1920×1080  ·  30/1 fps  ·  WCAG 2.2  ·  analysed 22 Oct 2026, 16:42",
           13, MUTED)
    button(b, f, x0 + cw - 300, 50, 140, "Download JSON")
    button(b, f, x0 + cw - 148, 50, 148, "Re-run analysis", primary=True)

    # Verdict card
    v = b.frame(x0, 140, 360, 196, parent=f, radius=14, stroke="#3A1F28",
                fills=[lin(["#24121A", CARD], horizontal=False)])
    b.ellipse(v, 220, -90, 240, 240, fills=[{"type": "radialGradient", "stops": ["#FF4D5E40", "#FF4D5E00"]}])
    label(b, v, 24, 22, "Verdict")
    b.rect(v, 24, 50, 132, 44, fill="#FF4D5E1F", radius=22, stroke="#FF4D5E66")
    b.ellipse(v, 42, 66, 12, 12, fill=RED, effects=[shadow("#FF4D5ECC", 10, 0)])
    b.text(v, 62, 59, "Failed", 22, "#FFD5DA", "Bold")
    b.text(v, 24, 112, "Some region flashes more often than", 13, MUTED)
    b.text(v, 24, 130, "WCAG 2.2 allows.", 13, MUTED)
    b.rect(v, 24, 158, 312, 1, fill=LINE)
    b.text(v, 24, 168, "passed: false", 12, FAINT, font=MONO)
    b.text(v, 196, 168, "exit status 1", 12, FAINT, font=MONO, width=140, align="right")

    # Stat grid
    stats = [
        ("Frame rate", "30", "fps", None),
        ("Length", "1m1.433s", "1843 frames", None),
        ("Video size", "1920×1080", "source pixels", None),
        ("Luminance failures", str(FAIL_LUM), "frames", LUM),
        ("Red failures", str(FAIL_RED), "frames", RED),
        ("Defects", str(len(DEFECTS)), "runs of failing frames", ACCENT),
    ]
    sw_, sh_ = (cw - 376 - 32) / 3, 90
    for i, (k, val, sub, col) in enumerate(stats):
        cx = x0 + 376 + (i % 3) * (sw_ + 16)
        cy = 140 + (i // 3) * (sh_ + 16)
        c = b.frame(cx, cy, sw_, sh_, parent=f, fill=CARD, radius=14, stroke=LINE)
        if col:
            b.rect(c, 0, 0, 3, sh_, fill=col)
        label(b, c, 20, 16, k)
        b.text(c, 20, 38, val, 26, col or INK, "SemiBold", tnum=True, ls="-1%")
        b.text(c, sw_ - 160, 16, sub, 11, FAINT, width=140, align="right")

    # Timeline
    tl = card(b, f, x0, 360, cw, 340, "Flash timeline",
              "Most alternating transitions in any region, per frame. Threshold: more than 6 within one second.")
    lx = cw - 470
    for name, col, kind in [("Luminance", LUM, "bar"), ("Red", RED, "bar"), ("Limit (6)", MUTED, "dash"),
                            ("Defect", ACCENT, "band")]:
        if kind == "bar":
            b.rect(tl, lx, 27, 10, 10, fill=col, radius=3)
        elif kind == "dash":
            b.rect(tl, lx, 31, 14, 2, fill=col)
        else:
            b.rect(tl, lx, 25, 12, 14, fill="#8B7CFF33", radius=3, stroke="#8B7CFF88")
        b.text(tl, lx + 18, 24, name, 12, MUTED)
        lx += 30 + len(name) * 7.2 + 18

    cx0, cx1 = 124, cw - 24
    span = cx1 - cx0
    lum, red = series()
    buckets = 184
    per = FRAMES / buckets

    def fx(frame):
        return cx0 + (frame - 1) / (FRAMES - 1) * span

    lanes = [("Luminance", lum, LUM, 84), ("Red", red, RED, 200)]
    lane_h = 96
    ymax = 12
    # defect bands
    for _, a, bb, l, r in DEFECTS:
        b.rect(tl, fx(a), 76, max(fx(bb) - fx(a), 2), 228, fill="#8B7CFF14", radius=3)
    for nm, data, col, ly in lanes:
        b.text(tl, 24, ly + lane_h / 2 - 16, nm, 12, INK, "Medium")
        b.text(tl, 24, ly + lane_h / 2 + 2, "transitions", 11, FAINT)
        for v_ in (0, 6, 12):
            yy = ly + lane_h - v_ / ymax * lane_h
            b.rect(tl, cx0, yy, span, 1, fill=GRID)
            b.text(tl, cx0 - 24, yy - 7, str(v_), 10, FAINT, font=MONO, width=18, align="right")
        # threshold dashed
        ty = ly + lane_h - 6.5 / ymax * lane_h
        b.rect(tl, cx0, ty, span, 1, stroke=MUTED, sw=1, dashes=[4, 4], fill=None)
        bw = span / buckets
        for k in range(buckets):
            a = int(k * per) + 1
            z = int((k + 1) * per)
            m = max(data[a:z + 1])
            if m == 0:
                continue
            hgt = min(m, ymax) / ymax * lane_h
            over = m > 6
            b.rect(tl, cx0 + k * bw + 0.6, ly + lane_h - hgt, bw - 1.2, hgt,
                   fill=col if over else (col + "55"), radius=1.2)
    # x axis
    for s in range(0, 61, 10):
        xx = cx0 + s * FPS / (FRAMES - 1) * span
        b.text(tl, xx - 20, 310, f"{s}s", 10, FAINT, font=MONO, width=40, align="center")
    # selected frame marker
    sel = 291
    mx = fx(sel)
    b.rect(tl, mx, 70, 1.5, 236, fill="#FFFFFF")
    b.rect(tl, mx - 58, 60, 116, 20, fill="#FFFFFF", radius=6)
    b.text(tl, mx - 58, 63, "Frame 291 · 9.667s", 11, "#0A0B0F", "SemiBold", width=116, align="center")

    # Defects table
    dt = card(b, f, x0, 724, 620, 500, "Defects",
              "Runs of frames covered by failing flash sequences, first transition to completing frame.")
    cols = [("#", 24), ("Frames", 64), ("Time", 192), ("Length", 340), ("Luminance", 430), ("Red", 530)]
    b.rect(dt, 0, 76, 620, 34, fill="#0F1117")
    for nm, cx in cols:
        label(b, dt, cx, 87, nm)
    y = 110
    for idx, a, bb, l, r in DEFECTS:
        selected = idx == 2
        if selected:
            b.rect(dt, 0, y, 620, 46, fill="#8B7CFF14")
            b.rect(dt, 0, y, 3, 46, fill=ACCENT)
        else:
            b.rect(dt, 0, y + 45, 620, 1, fill=LINE)
        ty = y + 15
        b.text(dt, 24, ty, str(idx), 13, MUTED, "Medium", font=MONO)
        b.text(dt, 64, ty, f"{a}–{bb}", 13, INK, "Medium", tnum=True)
        b.text(dt, 192, ty, f"{t(a):.2f}s – {t(bb + 1):.2f}s", 13, MUTED, tnum=True)
        b.text(dt, 340, ty, f"{(bb - a + 1) / FPS:.2f}s", 13, MUTED, tnum=True)
        if l:
            pill(b, dt, 430, y + 11, "Luminance", LUM, "#F5C4511F", w=84)
        else:
            b.text(dt, 430, ty, "—", 13, FAINT)
        if r:
            pill(b, dt, 530, y + 11, "Red", RED, "#FF4D5E1F", w=48)
        else:
            b.text(dt, 530, ty, "—", 13, FAINT)
        y += 46

    # Frame inspector
    fi = card(b, f, x0 + 636, 724, 476, 500)
    label(b, fi, 24, 22, "Frame inspector")
    b.text(fi, 24, 40, "Frame 291", 20, INK, "Bold")
    b.text(fi, 132, 46, "9.667s", 13, MUTED, font=MONO)
    for i, s in enumerate(["‹", "›"]):
        b.rect(fi, 476 - 92 + i * 36, 26, 30, 30, fill=CARD2, radius=8, stroke=LINE)
        b.text(fi, 476 - 92 + i * 36, 30, s, 16, INK, "Medium", width=30, align="center")
    pv = b.frame(24, 80, 428, 241, parent=fi, radius=10,
                 fills=[lin(["#1B2440", "#3B1630", "#120F1F"])])
    b.ellipse(pv, 220, 40, 200, 200, fills=[{"type": "radialGradient", "stops": ["#FF4D5E88", "#FF4D5E00"]}])
    b.ellipse(pv, 40, -30, 220, 220, fills=[{"type": "radialGradient", "stops": ["#F5C45166", "#F5C45100"]}])
    sx = 428 / WIDTH
    # luminance region (512,320)-(853,576); red region (960,480)-(1301,736)
    for (x1, y1, x2, y2), col, nm in [((512, 320, 853, 576), LUM, "L 41.8%"),
                                      ((960, 480, 1301, 736), RED, "R 27.3%")]:
        b.rect(pv, x1 * sx, y1 * sx, (x2 - x1) * sx, (y2 - y1) * sx, fill=col + "22", stroke=col, sw=1.5)
        b.rect(pv, x1 * sx, y1 * sx - 18, 58, 16, fill=col, radius=3)
        b.text(pv, x1 * sx, y1 * sx - 16, nm, 10, "#0A0B0F", "Bold", width=58, align="center")
    b.text(pv, 12, 218, "1920×1080 · regions in source pixels", 10, "#FFFFFFAA")

    rows = [("Transitions", "9", "7"), ("Since frame", "276", "280"),
            ("Region", "512,320 → 853,576", "960,480 → 1301,736"), ("Area", "41.8%", "27.3%"),
            ("Failed", "Yes", "Yes")]
    c1, c2 = 150, 312
    label(b, fi, 24, 340, "Measurement")
    b.rect(fi, c1, 338, 8, 8, fill=LUM, radius=2)
    label(b, fi, c1 + 14, 336, "Luminance", LUM)
    b.rect(fi, c2, 338, 8, 8, fill=RED, radius=2)
    label(b, fi, c2 + 14, 336, "Red", RED)
    y = 362
    for k, a, r in rows:
        b.rect(fi, 24, y - 6, 428, 1, fill=LINE)
        b.text(fi, 24, y + 2, k, 12, MUTED)
        if k == "Failed":
            pill(b, fi, c1, y - 1, "Failed", RED, "#FF4D5E1F", h=22, size=11, w=60)
            pill(b, fi, c2, y - 1, "Failed", RED, "#FF4D5E1F", h=22, size=11, w=60)
        else:
            fnt = MONO if k == "Region" else "Inter"
            sz = 11 if k == "Region" else 13
            b.text(fi, c1, y + 2, a, sz, INK, "Medium", font=fnt, tnum=True)
            b.text(fi, c2, y + 2, r, sz, INK, "Medium", font=fnt, tnum=True)
        y += 26

    # Thresholds
    th = card(b, f, x0, 1248, cw, 190, "Thresholds", None)
    pill(b, th, 128, 19, "WCAG 2.2 preset", ACCENT, "#8B7CFF1F", h=22, size=11, w=112)
    b.text(th, cw - 324, 22, "Override any value from New analysis", 12, FAINT, width=300, align="right")
    cells = [("Luminance change", "0.1", "ΔY relative luminance"), ("Dark limit", "0.8", "darker side below"),
             ("Red change", "0.2", "Δu′v′ chromaticity"), ("Red saturation", "0.8", "R / (R+G+B), linear"),
             ("Region", "341×256", "pixels, any position"), ("Area", "25%", "of a region, together"),
             ("Max transitions", "6", "alternating, per second"), ("Count window", "30", "frames at 30 fps")]
    cwid = (cw - 48 - 3 * 12) / 4
    for i, (k, val, sub) in enumerate(cells):
        cx = 24 + (i % 4) * (cwid + 12)
        cy = 58 + (i // 4) * 62
        b.rect(th, cx, cy, cwid, 54, fill="#0F1117", radius=10, stroke=LINE)
        b.text(th, cx + 14, cy + 9, k, 11, FAINT, "Medium")
        b.text(th, cx + 14, cy + 26, val, 16, INK, "SemiBold", font="Inter", tnum=True)
        b.text(th, cx + cwid - 194, cy + 30, sub, 11, FAINT, width=180, align="right")
    return f


def new_analysis(b, X, Y):
    W, H = 1440, 920
    f = b.frame(X, Y, W, H, name="A2 · New analysis", fill=BG)
    sidebar(b, f, H, "New analysis")
    x0 = 288
    b.text(f, x0, 30, "New analysis", 12, FAINT)
    b.text(f, x0, 52, "Check a video for hazardous flashing", 28, INK, "Bold", ls="-1%")
    b.text(f, x0, 92, "Decoded once with FFmpeg, every frame measured. Nothing leaves this machine.", 13, MUTED)

    # Source
    src = card(b, f, x0, 140, 680, 260, "Source")
    b.rect(src, 24, 56, 632, 76, fill="#0F1117", radius=12, stroke="#8B7CFF66", dashes=[6, 5])
    b.rect(src, 40, 72, 44, 44, fills=[lin(["#8B7CFF33", "#22D3EE22"])], radius=10)
    b.vector(src, "M0 0 L12 6 L0 12 Z", 58, 88, 11, 12, fill=INK)
    b.text(src, 100, 74, FILE, 14, INK, "SemiBold")
    b.text(src, 100, 96, "248.6 MB  ·  probed with ffprobe", 12, FAINT)
    button(b, src, 656 - 112, 75, 96, "Replace")
    probe = [("Codec", "h264"), ("Resolution", "1920×1080"), ("Frame rate", "30/1"),
             ("Analysed at", "30 fps"), ("Frames", "1843"), ("Duration", "61.433s")]
    pw = (632 - 2 * 10) / 3
    for i, (k, val) in enumerate(probe):
        cx = 24 + (i % 3) * (pw + 10)
        cy = 148 + (i // 3) * 48
        b.rect(src, cx, cy, pw, 40, fill="#0F1117", radius=10, stroke=LINE)
        label(b, src, cx + 14, cy + 13, k)
        b.text(src, cx + pw - 134, cy + 11, val, 14, INK, "SemiBold", tnum=True, width=120, align="right")

    # Standard
    st = card(b, f, x0, 424, 680, 296, "Standard", "Threshold preset. Individual values can be overridden.")
    opts = [("WCAG 2.2", "wcag22", "≥ 25% of any 341×256 px region\n> 6 alternating transitions in 1 s", True),
            ("Trace 2024", "trace24", "≥ 25% of any 416×416 px region\nas WCAG otherwise", False),
            ("Broadcast", "broadcast", "≥ 25% of the whole screen\nITU-R BT.1702 · Ofcom · ISO 9241-391", False),
            ("Scene change", "scene", "≥ 80% of the screen\n> 3 transitions in 1 s · experimental", False)]
    ow = (632 - 12) / 2
    for i, (nm, flag, desc, on) in enumerate(opts):
        cx = 24 + (i % 2) * (ow + 12)
        cy = 76 + (i // 2) * 104
        if on:
            b.rect(st, cx, cy, ow, 92, fill="#8B7CFF14", radius=12, stroke=ACCENT, sw=1.5)
        else:
            b.rect(st, cx, cy, ow, 92, fill="#0F1117", radius=12, stroke=LINE)
        b.ellipse(st, cx + ow - 34, cy + 16, 18, 18, stroke=ACCENT if on else FAINT, sw=1.5)
        if on:
            b.ellipse(st, cx + ow - 29, cy + 21, 8, 8, fill=ACCENT)
        b.text(st, cx + 16, cy + 14, nm, 14, INK, "SemiBold")
        b.text(st, cx + 16 + len(nm) * 8.4 + 8, cy + 16, flag, 11, FAINT, font=MONO)
        b.text(st, cx + 16, cy + 40, desc, 12, MUTED, lh="150%", width=ow - 32)

    # Decoding
    dc = card(b, f, x0, 744, 680, 136, "Decoding")
    b.text(dc, 24, 60, "Convert frame rate", 13, INK, "Medium")
    b.text(dc, 24, 80, "For variable frame rate video", 12, FAINT)
    b.rect(dc, 220, 62, 110, 38, fill="#0F1117", radius=9, stroke=LINE)
    b.text(dc, 234, 73, "Off", 13, FAINT)
    b.text(dc, 300, 73, "fps", 12, FAINT)
    b.rect(dc, 360, 56, 1, 50, fill=LINE)
    b.text(dc, 392, 60, "Deinterlace", 13, INK, "Medium")
    b.text(dc, 392, 80, "Frames flagged as interlaced", 12, FAINT)
    b.rect(dc, 610, 68, 46, 26, fill=ACCENT, radius=13)
    b.ellipse(dc, 632, 71, 20, 20, fill="#FFFFFF")

    # Overrides
    ov = card(b, f, x0 + 696, 140, 416, 452, "Threshold overrides", "Empty fields keep the preset value.")
    b.text(ov, 416 - 124, 22, "Reset all", 12, ACCENT, "Medium", width=100, align="right")
    fields = [("Luminance change", "0.1", None), ("Dark limit", "0.8", None), ("Red change", "0.2", None),
              ("Red saturation", "0.8", None), ("Region", "341×256", None), ("Area", "25", "30"),
              ("Max transitions", "6", None)]
    y = 80
    for k, ph, val in fields:
        b.text(ov, 24, y + 10, k, 13, MUTED)
        on = val is not None
        b.rect(ov, 236, y, 156, 36, fill="#0F1117", radius=9, stroke=ACCENT if on else LINE, sw=1.5 if on else 1)
        b.text(ov, 250, y + 10, val or ph, 13, INK if on else FAINT, "Medium" if on else "Regular", tnum=True)
        unit = {"Area": "%", "Region": "px", "Max transitions": "/ s"}.get(k)
        if unit:
            b.text(ov, 236 + 156 - 48, y + 11, unit, 12, FAINT, width=36, align="right")
        if on:
            b.ellipse(ov, 222, y + 15, 6, 6, fill=ACCENT)
        y += 50

    # Progress
    pr = b.frame(x0 + 696, 616, 416, 264, parent=f, radius=14, stroke="#2A2550",
                 fills=[lin(["#16132A", CARD], horizontal=False)])
    label(b, pr, 24, 22, "Analysing")
    b.text(pr, 416 - 124, 20, "Cancel", 12, MUTED, "Medium", width=100, align="right")
    b.text(pr, 24, 44, "Frame 1204 of 1843", 22, INK, "Bold", tnum=True)
    b.text(pr, 416 - 104, 48, "65%", 18, ACCENT, "SemiBold", width=80, align="right", tnum=True)
    b.rect(pr, 24, 88, 368, 8, fill="#232634", radius=4)
    b.rect(pr, 24, 88, 368 * 0.65, 8, fills=[lin([ACCENT, CYAN])], radius=4,
           effects=[shadow("#8B7CFF88", 10, 0)])
    b.text(pr, 24, 106, "0:40 of 1:01  ·  ~6s left", 12, FAINT, tnum=True)
    b.rect(pr, 24, 136, 368, 1, fill=LINE)
    live = [("Decided", "1175", "1 s behind", INK), ("Luminance", "212", "frames failed", LUM),
            ("Red", "240", "frames failed", RED)]
    for i, (k, val, sub, col) in enumerate(live):
        cx = 24 + i * 124
        label(b, pr, cx, 152, k)
        b.text(pr, cx, 172, val, 22, col, "SemiBold", tnum=True)
        b.text(pr, cx, 202, sub, 11, FAINT)
    b.text(pr, 24, 232, "Defects appear in the report as each frame is decided.", 11, FAINT)
    return f


if __name__ == "__main__":
    b = B()
    report(b, 0, 0)
    new_analysis(b, 1540, 0)
    b.run("version_a.figcraft")
