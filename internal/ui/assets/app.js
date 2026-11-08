// PSV report UI. One script serves both frontends: `psv serve`, where a video
// is streamed to the API and the report it returns is shown, and
// `psv render`, which inlines a report in a #psv-report element.
"use strict";

const SCHEMA = "psv.report/v1";
const PRESETS = {
  wcag22: { name: "WCAG 2.2", who: "Web content", region: "341×256", area: "25%", max: 6 },
  trace24: { name: "Trace 2024", who: "Proposed guideline", region: "416×416", area: "25%", max: 6 },
  broadcast: { name: "Broadcast", who: "ITU-R · Ofcom · ISO", region: "Full screen", area: "25%", max: 6 },
  scene: { name: "Scene change", who: "Experimental", region: "Full screen", area: "80%", max: 3 },
};
const S = { screen: "upload", file: null, standard: "wcag22", prog: null, report: null, d: null,
  embedded: false, moment: 0, frame: 1, win: [1, 1], error: "" };
const $ = (s) => document.querySelector(s);
const esc = (s) => String(s).replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c]));
const plural = (n, one, many) => `${n} ${n === 1 ? one : many}`;
const clock = (sec) => `${Math.floor(sec / 60)}:${(sec % 60).toFixed(1).padStart(4, "0")}`;

// ---------- screens ----------
function render() {
  $("#app").innerHTML = S.screen === "upload" ? upload() : S.screen === "progress" ? progressView() : report();
  if (S.screen === "report") loadPreview();
}

// show validates a psv.report/v1 report and displays it.
function show(rep) {
  if (!rep || rep.schema !== SCHEMA) throw new Error(`Not a ${SCHEMA} report.`);
  if (!Array.isArray(rep.results) || rep.results.length !== rep.frames) {
    throw new Error("This report has no per-frame results. Create it with psv -format json -frames.");
  }
  S.report = rep;
  S.d = derive(rep);
  S.moment = 0;
  if (S.d.moments.length) selectMoment(0);
  else { S.frame = 1; S.win = [1, Math.min(rep.frames, 4 * rep.frameRate)]; }
  S.screen = "report";
  S.error = "";
  render();
}

function upload() {
  const presets = Object.entries(PRESETS).map(([k, p]) => `
    <button class="preset ${k === S.standard ? "on" : ""}" data-preset="${k}">
      <div class="eyebrow" style="color:${k === S.standard ? "var(--accent)" : "var(--faint)"}">${p.who}</div>
      <h4>${p.name}</h4><code>-standard ${k}</code>
      <dl><dt>Region</dt><dd>${p.region}</dd><dt>Area</dt><dd>${p.area}</dd><dt>Max / s</dt><dd>${p.max}</dd></dl>
    </button>`).join("");
  return `<div class="center">
    <h1>Check a video for flashing</h1>
    <p class="sub" style="font-size:17px">The video is streamed to this PSV server, measured frame by frame with FFmpeg and deleted when the analysis ends.</p>
    <label class="drop ${S.file ? "has" : ""}">
      <input type="file" id="file" accept="video/*" hidden>
      <strong>${S.file ? esc(S.file.name) : "Drop a video here or click to choose"}</strong>
      <div class="sub">${S.file ? (S.file.size / 1e6).toFixed(1) + " MB · click to change" : "MP4, MOV, MKV, WebM…"}</div>
    </label>
    <div class="presets">${presets}</div>
    <div style="margin-top:32px"><button class="btn accent" id="start" ${S.file ? "" : "disabled"}>Start analysis →</button></div>
    <p class="alt">Have a report already? <button class="link" id="openreport">Open a psv.report/v1 JSON file</button></p>
    ${S.error ? `<div class="error">${esc(S.error)}</div>` : ""}
  </div>`;
}

function progressView() {
  const p = S.prog || { phase: "upload", frame: 0, total: 0 };
  const pct = p.total ? Math.min(100, Math.round(p.frame * 100 / p.total)) : 0;
  const c = 2 * Math.PI * 100;
  const uploading = p.phase === "upload";
  return `<div class="center panel" style="max-width:720px;padding:52px 48px">
    <div class="ring"><svg viewBox="0 0 220 220" width="220" height="220">
      <circle cx="110" cy="110" r="100" fill="none" stroke="var(--soft)" stroke-width="14"/>
      <circle cx="110" cy="110" r="100" fill="none" stroke="var(--accent)" stroke-width="14" stroke-linecap="round"
        stroke-dasharray="${c * pct / 100} ${c}" transform="rotate(-90 110 110)"/></svg>
      <div><strong>${uploading ? "…" : `${pct}%`}</strong><span class="sub">${uploading ? "uploading" : `${p.frame} of ${p.total || "?"} frames`}</span></div></div>
    <h2 style="margin-top:28px">${uploading ? "Uploading" : "Analysing"} ${esc(S.file ? S.file.name : "")}</h2>
    <p class="sub">${uploading ? "The analysis starts once the whole video has arrived." : "Frames are decided one second behind the frame being read."}</p>
    <p class="alt"><button class="link" id="cancel">Cancel</button></p></div>`;
}

// ---------- derived data ----------
const MC = 64, MR = 36;

function derive(r) {
  const R = r.results;
  const rect = (b) => ({ x1: b.Min.X, y1: b.Min.Y, x2: b.Max.X, y2: b.Max.Y });
  const decode = (b64) => Uint8Array.from(atob(b64), (c) => c.charCodeAt(0));
  const grid = { luminance: new Float64Array(MC * MR), red: new Float64Array(MC * MR) };
  for (const fr of R) {
    for (const k of ["luminance", "red"]) {
      const m = fr[k];
      m.cells = m.mask ? decode(m.mask) : null;
      if (m.cells) for (let i = 0; i < m.cells.length; i++) grid[k][i] += m.cells[i];
    }
  }
  const cellX = (i) => ((i % MC) + .5) / MC * r.width;
  const cellY = (i) => (Math.floor(i / MC) + .5) / MR * r.height;
  // footprint keeps the cells where a moment flashes in at least a tenth as
  // many frames as its busiest cell, so stray pixels don't spread it.
  const footprint = (g) => {
    const peak = Math.max(0, ...g);
    const fp = new Uint8Array(g.length);
    let n = 0, w = 0, sx = 0, sy = 0;
    for (let i = 0; i < g.length; i++) {
      if (!peak || g[i] < peak * .1) continue;
      fp[i] = 1; n++; w += g[i]; sx += g[i] * cellX(i); sy += g[i] * cellY(i);
    }
    const cover = n / g.length;
    return { g, peak, fp, cover, whole: cover >= .85, cx: w ? sx / w : r.width / 2, cy: w ? sy / w : r.height / 2 };
  };
  const moments = r.defects.map((df) => {
    const gk = { luminance: new Float64Array(MC * MR), red: new Float64Array(MC * MR) };
    let first = null;
    for (let f = df.firstFrame; f <= df.lastFrame; f++) {
      for (const k of ["luminance", "red"]) {
        const m = R[f - 1][k];
        if (!m.failed) continue;
        if (first === null) first = f;
        if (m.cells) for (let i = 0; i < m.cells.length; i++) gk[k][i] += m.cells[i];
      }
    }
    const all = footprint(gk.luminance.map((v, i) => v + gk.red[i]));
    // A flash is a pair of opposing transitions. A frame reports a measurement
    // only when some region makes an alternating transition in it, so those
    // frames are the transitions. Their reported count stops just past the
    // limit, so the rate comes from the busiest one-second window instead.
    const times = [];
    for (let f = df.firstFrame; f <= df.lastFrame; f++) {
      const fr = R[f - 1];
      if (fr.luminance.transitions || fr.red.transitions) times.push(f);
    }
    const transitions = times.length;
    let peak = 0;
    for (let i = 0, j = 0; j < times.length; j++) {
      while (times[j] - times[i] >= r.frameRate) i++;
      peak = Math.max(peak, j - i + 1);
    }
    const stats = {};
    for (const k of ["luminance", "red"]) {
      const ts = [];
      let maxArea = 0, failed = false;
      for (let f = df.firstFrame; f <= df.lastFrame; f++) {
        const m = R[f - 1][k];
        if (m.transitions) ts.push(f);
        if (m.failed) { failed = true; maxArea = Math.max(maxArea, m.area); }
      }
      let pk = 0;
      for (let i = 0, j = 0; j < ts.length; j++) {
        while (ts[j] - ts[i] >= r.frameRate) i++;
        pk = Math.max(pk, j - i + 1);
      }
      stats[k] = { failed, flashes: Math.round(ts.length / 2), perSecond: Math.floor(pk / 2), maxArea };
    }
    return { ...df, ...all, kinds: { luminance: footprint(gk.luminance), red: footprint(gk.red) }, first: first || df.firstFrame, stats,
      flashes: Math.max(1, Math.round(transitions / 2)), perSecond: Math.floor(peak / 2), peak };
  });
  return { r, R, rect, moments, grid, flashes: moments.reduce((n, m) => n + m.flashes, 0) };
}

function whereWords(cx, cy, w, h) {
  const col = ["left", "centre", "right"][Math.min(2, Math.floor(cx / (w / 3)))];
  const row = ["Top", "Middle", "Bottom"][Math.min(2, Math.floor(cy / (h / 3)))];
  return row === "Middle" && col === "centre" ? "Centre" : `${row} ${col}`;
}

function selectMoment(i) {
  const D = S.d;
  const m = D.moments[i];
  if (!m) return;
  const pad = D.r.frameRate;
  S.moment = i;
  S.frame = m.first;
  S.win = [Math.max(1, m.firstFrame - pad), Math.min(D.r.frames, m.lastFrame + pad)];
}

function centreWindow(f) {
  const D = S.d;
  const len = S.win[1] - S.win[0];
  let a = Math.max(1, Math.round(f - len / 2));
  const b = Math.min(D.r.frames, a + len);
  a = Math.max(1, b - len);
  S.win = [a, b];
  S.frame = f;
  const mi = D.moments.findIndex((m) => f >= m.firstFrame && f <= m.lastFrame);
  if (mi >= 0) S.moment = mi;
}

// ---------- report ----------
function report() {
  const D = S.d, r = D.r;
  const failed = !r.passed;
  const std = PRESETS[r.standard] ? PRESETS[r.standard].name : r.standard || "Custom";
  const source = r.source || "Video";
  const total = D.moments.reduce((s, m) => s + (m.lastFrame - m.firstFrame + 1), 0) / r.frameRate;
  const facts = [["Length", `${clock(r.durationSeconds)}`], ["Frames", r.frames], ["Frame rate", `${r.frameRate} fps`],
    ["Size", `${r.width}×${r.height}`], ["Standard", std]];
  return `<div class="wrap">
    <div class="hero">
      <div><div class="eyebrow">${esc(source)}</div>
        <h1><span class="dot" style="background:${failed ? "var(--flash)" : "var(--pass)"}"></span>${failed ? "Not safe to publish" : "Safe to publish"}</h1>
        <p>${failed ? `This video has ${D.moments.length} moment${D.moments.length === 1 ? "" : "s"} of flashing that could trigger seizures in people with photosensitive epilepsy. Together they last ${total.toFixed(1)} seconds and contain ${plural(D.flashes, "flash", "flashes")}.`
          : "We found no flashing that could trigger seizures in people with photosensitive epilepsy."}</p></div>
      <div class="card" style="width:auto;padding:0"><div class="row">
        ${S.embedded ? "" : `<button class="btn" id="again">New analysis</button>`}
        <button class="btn primary" id="export">Download report</button></div></div>
    </div>
    <div class="facts-row panel">${facts.map(([k, v]) => `<div><span class="eyebrow">${k}</span><strong>${v}</strong></div>`).join("")}</div>
    ${timePanel(true)}
    ${momentsSection()}
    ${mapPanel(true)}
    <footer>Photosafety Validator · ${esc(r.tool.name)} ${esc(r.tool.version || "")} · ${esc(r.schema)} · ${esc(source)}</footer></div>`;
}

const W = 1056;

function timePanel(admin) {
  const D = S.d, r = D.r, R = D.R;
  const n = r.frames;
  const x0 = admin ? 100 : 0, span = W - x0;
  const ox = (f) => x0 + (f - 1) / Math.max(1, n - 1) * span;
  const buckets = Math.min(n, 220);
  const per = n / buckets, cw = span / buckets;
  const strip = [];
  const kinds = admin ? [["luminance", "var(--lum)"], ["red", "var(--red)"]] : [["both", "var(--flash)"]];
  kinds.forEach(([k, col], li) => {
    const y = 34 + li * (admin ? 22 : 0), h = admin ? 18 : 36;
    for (let i = 0; i < buckets; i++) {
      let mx = 0, bad = false;
      for (let f = Math.floor(i * per); f < Math.floor((i + 1) * per); f++) {
        const fr = R[f];
        const ms = k === "both" ? [fr.luminance, fr.red] : [fr[k]];
        for (const m of ms) { mx = Math.max(mx, m.transitions); bad = bad || m.failed; }
      }
      strip.push(`<rect x="${x0 + i * cw}" y="${y}" width="${Math.max(cw - 1, .5)}" height="${h}" rx="1.5"
        fill="${bad ? col : mx ? col : "#D9D5CD"}" opacity="${bad ? 1 : 0.15 + Math.min(mx, 12) * 0.04}"/>`);
    }
  });
  if (admin) strip.push(`<text x="0" y="52" font-size="11" font-weight="600" fill="var(--muted)" letter-spacing=".08em">OVERVIEW</text>`);
  D.moments.forEach((m, i) => {
    const cx = (ox(m.firstFrame) + ox(m.lastFrame)) / 2;
    strip.push(`<g class="hit" data-moment="${i}" style="cursor:pointer"><circle cx="${cx}" cy="14" r="10" fill="var(--ink)"/>
      <text x="${cx}" y="18" font-size="10" font-weight="700" fill="#fff" text-anchor="middle">${m.index}</text></g>`);
  });
  const [a, b] = S.win;
  strip.push(`<rect x="${ox(a)}" y="28" width="${Math.max(ox(b) - ox(a), 4)}" height="48" rx="6" fill="#2346FF14" stroke="var(--accent)" stroke-width="1.5"/>`);
  const secs = Math.ceil(r.durationSeconds);
  const step = secs > 120 ? 30 : 10;
  for (let s = 0; s <= secs; s += step) {
    const x = x0 + s * r.frameRate / Math.max(1, n - 1) * span;
    if (x <= W) strip.push(`<text x="${x}" y="92" font-size="11" fill="var(--faint)" text-anchor="${s === 0 ? "start" : x > W - 20 ? "end" : "middle"}">${clock(s).replace(".0", "")}</text>`);
  }
  const overview = `<svg id="overview" viewBox="0 0 ${W} 100" width="100%" style="cursor:crosshair" data-x0="${x0}" data-span="${span}">${strip.join("")}</svg>`;

  const m = D.moments[S.moment];
  const inMoment = m && S.frame >= m.firstFrame && S.frame <= m.lastFrame;
  const zoom = `<div class="zoomhead"><strong>${inMoment ? `Moment ${m.index}` : "Selected window"}</strong>
    <span class="sub" style="margin:0">${inMoment ? `${clock(m.startSeconds)} – ${clock(m.endSeconds)}` : `${clock((a - 1) / r.frameRate)} – ${clock(b / r.frameRate)}`}</span>
    ${inMoment ? `<span class="pill red">${plural(m.flashes, "flash", "flashes")} · about ${m.perSecond} per second</span>` : ""}
    <span class="sub" style="margin:0;font-size:12px">frames ${a}–${b}</span>
    <span class="spacer"></span>
    ${D.moments.length ? `<button class="btn" id="mprev" style="padding:6px 14px">← Previous</button><button class="btn" id="mnext" style="padding:6px 14px">Next →</button>` : ""}</div>
    <div class="chart">${zoomAdmin()}</div>`;

  return `<section class="panel"><div class="head"><div><h2>When it flashes</h2>
    <p class="sub">Strong colour marks the moments that could trigger a seizure. Click the overview or a number to look closer; the window below shows every frame.</p></div></div>
    <div class="chart">${overview}</div>${zoom}</section>`;
}

function zoomGeom(x0) {
  const [a, b] = S.win;
  const n = b - a + 1, span = W - x0, slot = span / n;
  return { a, b, n, slot, zx: (f) => x0 + (f - a) * slot };
}

function marker(g, top, bottom) {
  if (S.frame < g.a || S.frame > g.b) return "";
  const x = g.zx(S.frame) + g.slot / 2;
  return `<line x1="${x}" x2="${x}" y1="${top}" y2="${bottom}" stroke="var(--accent)" stroke-width="1.5" opacity=".7" pointer-events="none"/>`;
}

function tipSVG(f) {
  const admin = true;
  const D = S.d, r = D.r, fr = D.R[f - 1];
  const g = zoomGeom(admin ? 100 : 0);
  const top = admin ? 12 : 16, bottom = admin ? 512 : 180;
  const bad = fr.luminance.failed || fr.red.failed;
  const lines = admin ? [`Frame ${f} · ${((f - 1) / r.frameRate).toFixed(3)}s`,
    `Transitions  L ${fr.luminance.transitions}   R ${fr.red.transitions}`,
    `Area  L ${fr.luminance.area.toFixed(1)}%   R ${fr.red.area.toFixed(1)}%`,
    `Since  L ${fr.luminance.since || "—"}   R ${fr.red.since || "—"}`]
    : [clock((f - 1) / r.frameRate), bad ? "Flashing above the safe limit"
      : Math.max(fr.luminance.transitions, fr.red.transitions) ? "Flashing below the safe limit" : "No flashing"];
  const x = g.zx(f) + g.slot / 2;
  const tx = x > W - 240 ? x - 226 : x + 12;
  const h = 26 + lines.length * 19;
  return `<line x1="${x}" x2="${x}" y1="${top}" y2="${bottom}" stroke="var(--ink)" stroke-width="1" stroke-dasharray="3 3"/>
    <rect x="${tx}" y="${top + 8}" width="214" height="${h}" rx="12" fill="#fff" stroke="var(--line)" style="filter:drop-shadow(0 8px 12px #0F0F121A)"/>
    ${lines.map((l, i) => `<text x="${tx + 14}" y="${top + 30 + i * 19}" font-size="${i ? 12 : 14}" font-weight="${i ? 500 : 700}" fill="${i ? "var(--muted)" : "var(--ink)"}">${l}</text>`).join("")}`;
}

function zoomAdmin() {
  const D = S.d, R = D.R, r = D.r;
  const x0 = 100, g = zoomGeom(x0), span = W - x0;
  const out = [];
  const maxT = r.thresholds.maxTransitions, ymax = Math.max(12, maxT * 2);
  const label = (y, a, b) => `<text x="0" y="${y}" font-size="13" font-weight="600" fill="var(--ink)">${a}</text><text x="0" y="${y + 18}" font-size="12" fill="var(--muted)">${b}</text>`;
  // transitions
  let ty = 20, th = 150;
  out.push(label(ty + 60, "Transitions", "per frame"));
  for (let v = 0; v <= ymax; v += ymax / 4) {
    const y = ty + th - v / ymax * th;
    out.push(`<line x1="${x0}" x2="${W}" y1="${y}" y2="${y}" stroke="#EFECE6"/><text x="${x0 - 8}" y="${y + 4}" font-size="10" fill="var(--faint)" text-anchor="end" font-family="var(--mono)">${v}</text>`);
  }
  for (let f = g.a; f <= g.b; f++) {
    out.push(`<rect class="fbar" data-frame="${f}" x="${g.zx(f)}" y="${ty}" width="${g.slot}" height="${th}" fill="transparent"/>`);
    [["luminance", "var(--lum)"], ["red", "var(--red)"]].forEach(([k, col], j) => {
      const mm = R[f - 1][k];
      if (!mm.transitions) return;
      const h = Math.min(mm.transitions, ymax) / ymax * th;
      out.push(`<rect x="${g.zx(f) + j * g.slot / 2 + .3}" y="${ty + th - h}" width="${Math.max(g.slot / 2 - .6, .5)}" height="${h}" fill="${col}" opacity="${mm.failed ? 1 : .35}" pointer-events="none"/>`);
    });
  }
  const l6 = ty + th - (maxT + .5) / ymax * th;
  out.push(`<line x1="${x0}" x2="${W}" y1="${l6}" y2="${l6}" stroke="var(--ink)" stroke-width="1.5" stroke-dasharray="5 4"/>
    <rect x="${W - 76}" y="${l6 - 24}" width="76" height="20" rx="10" fill="var(--ink)"/><text x="${W - 38}" y="${l6 - 10}" font-size="10" font-weight="600" fill="#fff" text-anchor="middle">limit ${maxT}</text>`);
  // area
  const ay = 200, ah = 120, amax = Math.max(50, r.thresholds.areaPercent * 2);
  out.push(label(ay + 40, "Area", "% of region"));
  for (const v of [0, amax / 2, amax]) {
    const y = ay + ah - v / amax * ah;
    out.push(`<line x1="${x0}" x2="${W}" y1="${y}" y2="${y}" stroke="#EFECE6"/><text x="${x0 - 8}" y="${y + 4}" font-size="10" fill="var(--faint)" text-anchor="end" font-family="var(--mono)">${v}%</text>`);
  }
  [["luminance", "var(--lum)", "#E39B0B"], ["red", "var(--red)", "#E5384A"]].forEach(([k, col, hex]) => {
    const pts = [];
    for (let f = g.a; f <= g.b; f++) pts.push(`${g.zx(f) + g.slot / 2},${ay + ah - Math.min(R[f - 1][k].area, amax) / amax * ah}`);
    out.push(`<polygon points="${g.zx(g.a) + g.slot / 2},${ay + ah} ${pts.join(" ")} ${g.zx(g.b) + g.slot / 2},${ay + ah}" fill="${hex}22"/>
      <polyline points="${pts.join(" ")}" fill="none" stroke="${col}" stroke-width="2" stroke-linejoin="round"/>`);
  });
  const la = ay + ah - r.thresholds.areaPercent / amax * ah;
  out.push(`<line x1="${x0}" x2="${W}" y1="${la}" y2="${la}" stroke="var(--ink)" stroke-width="1.5" stroke-dasharray="5 4"/>
    <rect x="${W - 76}" y="${la - 24}" width="76" height="20" rx="10" fill="var(--ink)"/><text x="${W - 38}" y="${la - 10}" font-size="10" font-weight="600" fill="#fff" text-anchor="middle">limit ${r.thresholds.areaPercent}%</text>`);
  // sequences
  const sy = 350;
  out.push(label(sy + 20, "Sequences", "first transition"), `<text x="0" y="${sy + 54}" font-size="12" fill="var(--muted)">to completing frame</text>`);
  out.push(`<rect x="${x0}" y="${sy - 4}" width="${span}" height="160" rx="8" fill="#FAF9F7"/>`);
  [["luminance", "var(--lum)", "LUMINANCE"], ["red", "var(--red)", "RED"]].forEach(([k, col, nm], j) => {
    const base = sy + 6 + j * 78;
    out.push(`<text x="${W - 8}" y="${base + 6}" font-size="11" font-weight="600" fill="${col}" text-anchor="end" letter-spacing=".08em">${nm}</text>`);
    const fails = [];
    for (let f = g.a; f <= g.b; f++) if (R[f - 1][k].failed) fails.push(f);
    const every = Math.max(1, Math.ceil(fails.length / 22));
    fails.filter((_, i) => i % every === 0).forEach((f, i) => {
      const s = Math.max(g.a, R[f - 1][k].since);
      const y = base + i * 3;
      out.push(`<rect x="${g.zx(s)}" y="${y}" width="${g.zx(f) - g.zx(s) + g.slot / 2}" height="2" fill="${col}" opacity=".55"/>
        <rect x="${g.zx(f) + g.slot / 2 - 1.5}" y="${y - .5}" width="3" height="3" fill="${col}"/>`);
    });
  });
  const step = Math.max(1, Math.round((g.b - g.a) / 7 / 10) * 10 || 10);
  for (let f = Math.ceil(g.a / step) * step; f <= g.b; f += step) {
    out.push(`<text x="${g.zx(f)}" y="532" font-size="10" fill="var(--muted)" text-anchor="middle" font-family="var(--mono)">${f}</text>
      <text x="${g.zx(f)}" y="546" font-size="9" fill="var(--faint)" text-anchor="middle" font-family="var(--mono)">${((f - 1) / r.frameRate).toFixed(2)}s</text>`);
  }
  out.push(marker(g, 12, 512));
  return `<svg id="zoom" viewBox="0 0 ${W} 556" width="100%">${out.join("")}<g id="tipLayer" pointer-events="none"></g></svg>`;
}

function momentsSection() {
  const D = S.d, r = D.r;
  if (!D.moments.length) return `<section><h3>Flashing moments</h3><div class="panel empty" style="margin-top:20px">No hazardous flashing found.</div></section>`;
  const total = D.moments.reduce((s, m) => s + (m.lastFrame - m.firstFrame + 1), 0) / r.frameRate;
  const list = D.moments.map((m, i) => {
    const on = i === S.moment;
    const dur = (m.lastFrame - m.firstFrame + 1) / r.frameRate;
    const kinds = `${m.luminance ? `<span class="pill lum sm">Luminance</span>` : ""}${m.red ? `<span class="pill red sm">Red</span>` : ""}`;
    return `<button class="moment ${on ? "on" : ""}" data-moment="${i}"><span class="n">${m.index}</span>
      <span class="t"><strong>${clock(m.startSeconds)} – ${clock(m.endSeconds)} ${kinds}</strong>
      <span>${plural(m.flashes, "flash", "flashes")} in ${dur.toFixed(1)} s</span>
      <small>frames ${m.firstFrame}–${m.lastFrame} · peak ${m.peak}/s</small></span>
      <span class="where">${m.whole ? "Whole screen" : whereWords(m.cx, m.cy, r.width, r.height)}</span>
      <span class="mini">${m.whole ? `<b></b>` : `<i style="left:${m.cx / r.width * 100}%;top:${m.cy / r.height * 100}%"></i>`}</span></button>`;
  }).join("");
  return `<section><h3>Flashing moments <small class="sub" style="font-size:15px">${D.moments.length}</small></h3>
    <p class="sub">${D.moments.length} moment${D.moments.length === 1 ? "" : "s"}, ${plural(D.flashes, "flash", "flashes")} over ${total.toFixed(1)} seconds. Each needs editing before release.</p>
    <div class="split"><div class="list">${list}</div>${detail()}</div></section>`;
}

function detail() {
  const D = S.d, r = D.r, fr = D.R[S.frame - 1], t = r.thresholds;
  const m = D.moments[S.moment];
  const cw = r.width / MC, ch = r.height / MR, u = r.width / 100;
  const label = (x, y, text, bg) => `<rect x="${x}" y="${Math.max(0, y - u * 4)}" width="${text.length * u * 1.05 + u * 2.4}" height="${u * 3.2}" rx="${u * 1.6}" fill="${bg}"/>
    <text x="${x + u * 1.2}" y="${Math.max(0, y - u * 4) + u * 2.25}" font-size="${u * 1.9}" font-weight="600" fill="#0F0F12">${text}</text>`;
  const failing = [["luminance", "#FFC542", "Luminance"], ["red", "#FF6B7A", "Red"]].filter(([k]) => fr[k].failed);
  const boxes = failing.map(([k, col, nm], i) => {
    const mm = fr[k], b = D.rect(mm.flashing);
    const cells = [];
    if (mm.cells) mm.cells.forEach((v, c) => { if (v) cells.push(`<rect x="${(c % MC) * cw}" y="${Math.floor(c / MC) * ch}" width="${cw}" height="${ch}" fill="${col}" opacity="${.2 + .55 * v / 100}"/>`); });
    return cells.join("") + `<rect x="${b.x1}" y="${b.y1}" width="${b.x2 - b.x1}" height="${b.y2 - b.y1}" rx="${u}" fill="none" stroke="${col}" stroke-width="${u * .35}"/>`
      + label(b.x1, i ? b.y2 + u * 4.4 : b.y1, `${nm} flashing`, col);
  }).join("");
  const maxFlashes = Math.floor(t.maxTransitions / 2);
  const inMoment = m && S.frame >= m.firstFrame && S.frame <= m.lastFrame;
  const row = (k, nm, col) => {
    const st = m.stats[k];
    const result = st.failed ? `<span class="tag">Too fast</span>` : st.flashes ? `<span class="tag ok">Below limit</span>` : `<span class="sub">None</span>`;
    return `<tr><td><span style="color:${col}">●</span> ${nm}</td>
      <td>${st.flashes}</td>
      <td>${st.flashes ? `${st.perSecond}<span class="sub"> / ${maxFlashes} max</span>` : "—"}</td>
      <td>${st.failed ? `${st.maxArea.toFixed(0)}%<span class="sub"> / ${t.areaPercent}%</span>` : "—"}</td>
      <td>${result}</td></tr>`;
  };
  return `<div class="panel detail"><div class="eyebrow">${inMoment ? `Moment ${m.index} · ` : ""}frame ${S.frame} · ${clock((S.frame - 1) / r.frameRate)}</div>
    <h3 style="margin-top:4px">Here's what flashes</h3>
    <div class="preview">${Frames.video ? `<img id="frameimg" alt="Frame ${S.frame}">`
      : `<div class="none"><span>Attach the video to see this frame.</span><button class="btn" id="attach">Attach video</button></div>`}
      <svg viewBox="0 0 ${r.width} ${r.height}" preserveAspectRatio="xMidYMid meet">${boxes}</svg>
      <span class="stamp">${clock((S.frame - 1) / r.frameRate)}</span></div>
    ${inMoment ? `<div class="facts"><div><span>When</span><strong>${clock(m.startSeconds)} – ${clock(m.endSeconds)}</strong></div>
      <div><span>Where</span><strong>${m.whole ? "Whole screen" : whereWords(m.cx, m.cy, r.width, r.height)}</strong></div>
      <div><span>Flashes</span><strong>${plural(m.flashes, "flash", "flashes")} in ${((m.lastFrame - m.firstFrame + 1) / r.frameRate).toFixed(1)} seconds</strong></div>
      <div><span>How fast</span><strong>About ${m.perSecond} per second <span class="sub" style="font-weight:500">(safe: ${maxFlashes} or fewer)</span></strong></div></div>` : ""}
    ${inMoment ? `<table class="kindtable"><thead><tr><th>Moment ${m.index}</th><th>Flashes</th><th>Per second</th><th>Peak area</th><th>Result</th></tr></thead>
      <tbody>${row("luminance", "Luminance", "var(--lum)")}${row("red", "Red", "var(--red)")}</tbody></table>` : ""}
    ${inMoment ? `<div class="fix">Fix: slow the flashing to ${maxFlashes} or fewer per second, or make the flashing area smaller.</div>` : ""}</div>`;
}

function footprintSVG(w, h, kind, labels, scale = 1) {
  const D = S.d, r = D.r, cw = w / MC, ch = h / MR;
  const items = D.moments.map((m, i) => ({ m, i, f: kind ? m.kinds[kind] : m }))
    .filter(({ m, f }) => f.peak && (!kind || m[kind]));
  // whole-screen moments first, then larger footprints under smaller ones,
  // and the selected moment on top
  items.sort((a, b) => (a.i === S.moment) - (b.i === S.moment) || b.f.whole - a.f.whole || b.f.cover - a.f.cover);
  const colour = (m) => {
    if (kind) return kind === "red" ? "#FF4D5E" : "#FFC542";
    return m.luminance && m.red ? "#FF7A45" : m.red ? "#FF4D5E" : "#FFC542";
  };
  const out = [], tags = [];
  for (const { m, i, f } of items) {
    const sel = i === S.moment, col = colour(m);
    const g = [];
    if (f.whole) {
      g.push(`<rect x="3" y="3" width="${w - 6}" height="${h - 6}" rx="${w / 70}" fill="${col}" fill-opacity="${sel ? .14 : .05}"
        stroke="${sel ? "#fff" : col}" stroke-width="${sel ? 4 : 3}" stroke-dasharray="${w / 60} ${w / 100}" pointer-events="stroke"/>`);
    } else {
      for (let c = 0; c < f.fp.length; c++) {
        if (!f.fp[c]) continue;
        const v = f.g[c] / f.peak;
        g.push(`<rect x="${(c % MC) * cw}" y="${Math.floor(c / MC) * ch}" width="${cw + .3}" height="${ch + .3}" fill="${col}" opacity="${sel ? .3 + .6 * v : .14 + .3 * v}"/>`);
      }
      let d = "";
      for (let c = 0; c < f.fp.length; c++) {
        if (!f.fp[c]) continue;
        const x = c % MC, y = Math.floor(c / MC), X = x * cw, Y = y * ch;
        if (y === 0 || !f.fp[c - MC]) d += `M${X} ${Y}h${cw}`;
        if (y === MR - 1 || !f.fp[c + MC]) d += `M${X} ${Y + ch}h${cw}`;
        if (x === 0 || !f.fp[c - 1]) d += `M${X} ${Y}v${ch}`;
        if (x === MC - 1 || !f.fp[c + 1]) d += `M${X + cw} ${Y}v${ch}`;
      }
      g.push(`<path d="${d}" fill="none" stroke="${sel ? "#fff" : col}" stroke-width="${sel ? 2 : 1.2}" stroke-opacity="${sel ? 1 : .8}" stroke-linecap="square"/>`);
    }
    out.push(`<g class="fp ${sel ? "sel" : ""}" data-moment="${i}" style="cursor:pointer">${g.join("")}</g>`);
    if (labels) {
      const rad = (sel ? 15 : 11) * w / 960 * (w < 500 ? 2 : 1) * scale;
      const x = f.whole ? w / 2 : f.cx / r.width * w, y = f.whole ? rad + 10 : f.cy / r.height * h;
      const text = f.whole ? `${m.index} · whole screen` : `${m.index}`;
      const pw = f.whole ? rad * 7.5 : rad * 2;
      tags.push(`<g data-moment="${i}" style="cursor:pointer"><rect x="${x - pw / 2}" y="${y - rad}" width="${pw}" height="${rad * 2}" rx="${rad}"
        fill="${sel ? "#fff" : "#0F0F12"}" stroke="${sel ? "var(--accent)" : "#ffffff55"}" stroke-width="${sel ? 3 : 1}"/>
        <text x="${x}" y="${y + rad * .38}" font-size="${rad * (f.whole ? .8 : 1)}" font-weight="700" fill="${sel ? "#0F0F12" : "#fff"}" text-anchor="middle">${text}</text></g>`);
    }
  }
  return out.join("") + tags.join("");
}

function mapPanel(admin) {
  const D = S.d, r = D.r;
  const mw = 960, mh = mw * r.height / r.width;
  const thirds = [1, 2].map((i) => `<line x1="${mw * i / 3}" x2="${mw * i / 3}" y1="0" y2="${mh}" stroke="#ffffff14"/><line y1="${mh * i / 3}" y2="${mh * i / 3}" x1="0" x2="${mw}" stroke="#ffffff14"/>`).join("");
  const bigSVG = `<svg class="map" viewBox="0 0 ${mw} ${mh}" width="100%" style="border-radius:16px;background:linear-gradient(90deg,#1A1F2E,#2A1A2A,#14121C);display:block">${thirds}${footprintSVG(mw, mh, null, true, admin ? 1.6 : 1)}</svg>`;
  const whole = D.moments.filter((m) => m.whole).map((m) => m.index);
  const sel = D.moments[S.moment];
  const legend = `<div class="legend"><span class="ramp">${Array.from({ length: 6 }, (_, i) => `<i style="opacity:${.2 + i * .15};background:${admin ? "#FF7A45" : "var(--flash)"}"></i>`).join("")}</span>
    Brighter: flashes there more often within that moment
    <span class="chips">${sel ? `<span>Moment ${sel.index}: ${sel.whole ? "whole screen" : whereWords(sel.cx, sel.cy, r.width, r.height).toLowerCase()} · ${plural(sel.flashes, "flash", "flashes")}</span>` : ""}
    ${whole.length ? `<span>Whole screen: moment${whole.length > 1 ? "s" : ""} ${whole.join(", ")}</span>` : ""}</span></div>`;
  let side = "";
  if (admin) {
    const sw = 360, sh = sw * r.height / r.width;
    side = `<div>${[["luminance", "Luminance", "var(--lum)", r.failures.luminance], ["red", "Red", "var(--red)", r.failures.red]].map(([k, nm, col, n]) =>
      `<div style="display:flex;justify-content:space-between;margin-bottom:8px"><strong><span style="color:${col}">●</span> ${nm}</strong><span class="sub" style="margin:0">${n} failing frames</span></div>
        <svg class="map" viewBox="0 0 ${sw} ${sh}" width="100%" style="background:#1A1F2E;border-radius:12px;display:block;margin-bottom:20px">${footprintSVG(sw, sh, k, true)}</svg>`).join("")}</div>`;
  }
  const blurb = admin ? "Where each defect's failing frames flash. Shading is scaled within each defect, so small and large defects both show; a dashed border marks a defect that covers the whole screen."
    : "Each numbered area shows where that moment flashes. Hover to highlight one; click to look closer.";
  return `<section class="panel"><div class="head"><div><h2>Where on screen</h2><p class="sub">${blurb}</p></div></div>
    <div class="mapwrap"><div class="maps"><div>${bigSVG}${legend}</div>${side}</div></div></section>`;
}

// ---------- events ----------
document.addEventListener("click", (e) => {
  const t = e.target;
  const preset = t.closest("[data-preset]");
  if (preset) { S.standard = preset.dataset.preset; return render(); }
  if (t.closest("#start")) return start();
  if (t.closest("#cancel")) return cancel();
  if (t.closest("#openreport")) return $("#pickreport").click();
  if (t.closest("#attach")) return $("#pickvideo").click();
  if (t.closest("#again")) { S.screen = "upload"; S.file = null; S.report = null; Frames.set(null); return render(); }
  if (t.closest("#export")) return exportJSON();
  if (t.closest("#mprev")) { selectMoment((S.moment - 1 + S.d.moments.length) % S.d.moments.length); return render(); }
  if (t.closest("#mnext")) { selectMoment((S.moment + 1) % S.d.moments.length); return render(); }
  const mo = t.closest("[data-moment]");
  if (mo) { selectMoment(+mo.dataset.moment); return render(); }
  const bar = t.closest("[data-frame]");
  if (bar) { S.frame = +bar.dataset.frame; return render(); }
  const ov = t.closest("#overview");
  if (ov) {
    const box = ov.getBoundingClientRect();
    const x = (e.clientX - box.left) / box.width * W;
    const x0 = +ov.dataset.x0, span = +ov.dataset.span;
    const f = Math.round((x - x0) / span * (S.d.r.frames - 1)) + 1;
    if (f >= 1 && f <= S.d.r.frames) { centreWindow(f); render(); }
  }
});
document.addEventListener("keydown", (e) => {
  if (S.screen !== "report" || e.target.closest("input")) return;
  if (e.key === "ArrowLeft" || e.key === "ArrowRight") {
    const f = Math.min(S.d.r.frames, Math.max(1, S.frame + (e.key === "ArrowLeft" ? -1 : 1)));
    if (f < S.win[0] || f > S.win[1]) centreWindow(f); else S.frame = f;
    render();
  }
});
document.addEventListener("mousemove", (e) => {
  const layer = document.getElementById("tipLayer");
  if (!layer || S.screen !== "report") return;
  const bar = e.target.closest && e.target.closest("#zoom [data-frame]");
  layer.innerHTML = bar ? tipSVG(+bar.dataset.frame) : "";
});
document.addEventListener("change", async (e) => {
  const t = e.target, f = t.files && t.files[0];
  t.value = "";
  if (t.id === "file") { S.file = f || null; S.error = ""; render(); }
  if (t.id === "pickvideo" && f) { Frames.set(f); render(); }
  if (t.id === "pickreport" && f) {
    try { Frames.set(null); show(JSON.parse(await f.text())); }
    catch (err) { S.error = `${f.name}: ${err.message}`; render(); }
  }
});
document.addEventListener("dragover", (e) => {
  if (S.screen !== "upload") return;
  e.preventDefault();
  const drop = $(".drop");
  if (drop) drop.classList.toggle("over", !!e.target.closest(".drop"));
});
document.addEventListener("dragleave", (e) => {
  if (!e.relatedTarget && $(".drop")) $(".drop").classList.remove("over");
});
document.addEventListener("drop", (e) => {
  if (S.screen !== "upload") return;
  e.preventDefault();
  const f = [...e.dataTransfer.files].find((x) => x.type.startsWith("video/") || /\.(mp4|mov|mkv|webm|avi|m4v)$/i.test(x.name));
  if (f) { S.file = f; S.error = ""; }
  else if (e.dataTransfer.files.length) S.error = "That file doesn't look like a video.";
  render();
});
// ---------- API ----------
// start streams the video to POST v1/analyses and reads the newline-delimited
// JSON events it returns: received, progress, then report or error.
let abort = null;

async function start() {
  const file = S.file;
  abort = new AbortController();
  S.screen = "progress"; S.prog = { phase: "upload", frame: 0, total: 0 }; S.error = ""; render();
  try {
    const q = new URLSearchParams({ standard: S.standard, name: file.name });
    const res = await fetch(`v1/analyses?${q}`, {
      method: "POST", body: file, signal: abort.signal,
      headers: { "Content-Type": file.type || "application/octet-stream" },
    });
    if (!res.ok) {
      const body = await res.json().catch(() => ({}));
      throw new Error(body.error || `${res.status} ${res.statusText}`);
    }
    const rep = await readEvents(res.body);
    Frames.set(file);
    show(rep);
  } catch (err) {
    S.screen = "upload";
    S.error = err.name === "AbortError" ? "" : err.message;
    render();
  } finally {
    abort = null;
  }
}

async function readEvents(body) {
  const reader = body.getReader(), dec = new TextDecoder();
  let buf = "";
  for (;;) {
    const { value, done } = await reader.read();
    buf += dec.decode(value || new Uint8Array(), { stream: !done });
    let nl;
    while ((nl = buf.indexOf("\n")) >= 0) {
      const line = buf.slice(0, nl).trim();
      buf = buf.slice(nl + 1);
      if (!line) continue;
      const ev = JSON.parse(line);
      if (ev.type === "report") return ev.report;
      if (ev.type === "error") throw new Error(ev.error);
      if (ev.type === "received") S.prog = { phase: "analyse", frame: 0, total: 0 };
      if (ev.type === "progress") S.prog = { phase: "analyse", frame: ev.frame, total: ev.total };
      if (S.screen === "progress") render();
    }
    if (done) throw new Error("The server closed the connection before sending a report.");
  }
}

function cancel() {
  if (abort) abort.abort();
}

function exportJSON() {
  const r = S.report;
  const blob = new Blob([JSON.stringify(r, null, 2)], { type: "application/json" });
  const a = document.createElement("a");
  a.href = URL.createObjectURL(blob);
  a.download = (r.source || "video").replace(/\.[^.]+$/, "") + ".psv.json";
  a.click();
  setTimeout(() => URL.revokeObjectURL(a.href), 0);
}

// ---------- frame preview ----------
// Frames draws report frames from the viewer's own copy of the video; the
// server keeps nothing once the analysis ends.
const Frames = {
  video: null, url: null, cache: new Map(), busy: Promise.resolve(),
  set(file) {
    if (this.url) URL.revokeObjectURL(this.url);
    this.cache.clear();
    this.video = this.url = null;
    if (!file) return;
    this.url = URL.createObjectURL(file);
    const v = document.createElement("video");
    v.muted = true; v.playsInline = true; v.preload = "auto"; v.src = this.url;
    this.video = v;
  },
  // get resolves to a JPEG data URL of frame f (from 1), or null.
  get(f, rate) {
    if (this.cache.has(f)) return Promise.resolve(this.cache.get(f));
    const v = this.video;
    if (!v) return Promise.resolve(null);
    this.busy = this.busy.then(() => new Promise((resolve) => {
      if (v !== this.video) return resolve(null);
      const done = () => {
        v.removeEventListener("seeked", done); v.removeEventListener("error", done);
        let url = null;
        if (v.videoWidth) {
          const c = document.createElement("canvas");
          c.width = Math.min(960, v.videoWidth);
          c.height = Math.round(c.width * v.videoHeight / v.videoWidth);
          c.getContext("2d").drawImage(v, 0, 0, c.width, c.height);
          url = c.toDataURL("image/jpeg", 0.85);
          this.cache.set(f, url);
        }
        resolve(url);
      };
      v.addEventListener("seeked", done); v.addEventListener("error", done);
      v.currentTime = (f - 0.5) / rate;
    }));
    return this.busy;
  },
};

function loadPreview() {
  const img = $("#frameimg");
  if (!img) return;
  const f = S.frame;
  Frames.get(f, S.report.frameRate).then((url) => {
    if (url && f === S.frame && img.isConnected) img.src = url;
  });
}

// ---------- start ----------
const embedded = document.getElementById("psv-report");
if (embedded) {
  S.embedded = true;
  try { show(JSON.parse(embedded.textContent)); }
  catch (err) { $("#app").innerHTML = `<div class="center"><h1>Can't show this report</h1><p class="error">${esc(err.message)}</p></div>`; }
} else {
  render();
}
