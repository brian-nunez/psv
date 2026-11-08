# GitHub Pages site mockup

A design mockup of the PSV landing page, not the site itself.

## Files

- `site.py`: the generator. It reuses `../fcgen.py`.
- `capture.sh`: renders each example report with `psv render` and takes the screenshots shown in the example cards.
- `img/`: the cropped report screenshots that `site.py` places in the cards.
- `site.figcraft`: the design file, with four frames:
  - `Site · Desktop` (1440 wide).
  - `Site · Phone` (375 wide).
  - `Warning expanded · Desktop`: the source-link disclosure after confirmation.
  - `Warning expanded · Phone`: the same on a phone.
- `png/`: one export per frame, at full height.
- `reports/` and `.tmp/`: the rendered reports, the raw screenshots and Edge's temporary profile. Both are gitignored.

## Regenerate

```sh
make build                       # bin/psv, from the repository root
design/mockups/site/capture.sh   # reports/, img/
cd design/mockups/site && python3 -I site.py   # site.figcraft, png/
```

- `capture.sh` needs Microsoft Edge and `ffmpeg`. It runs Edge headless with `--user-data-dir=.tmp/edge`.
- Screenshots are taken at 2× and cropped to the report's verdict, metrics and "When it flashes" section. No video is attached, so no video frame appears in any of them; every crop was checked by eye.
- `site.py` needs `figcraft-cli` at `~/.local/bin/figcraft-cli`. It reads `testdata/examples/*/report.json` and `source.url` and never writes to them. It measures every wrapped text with figcraft before building the page.
- Run it with `-I`, which keeps the script's folder off `sys.path`.
- The Go programs in `site.py` (`GO1`, `GO_STREAM_SRC`, `GO_LIVE_SRC`) must stay identical to the files in `.tmp/check/`. After a change, re-run `go vet ./...` and `compare.py` there.

## Brief

- **Audience:** developers first, then accessibility experts and product owners.
- **Goal:** understand what PSV checks in one screen. Then see real results on four real videos, and get to the CLI, the API or Go usage in one click.
- **Structure:** a single column with content at most 1120 px wide: hero, examples, the seizure warning, what it checks, use it, footer. A phone gets the same order with a 16 px gutter.
- **Hard rule:** no video playback, stills, frames, thumbnails or upload. The examples appear only through their reports.
- **Style:** the report UI's Daylight tokens. The logo is the brand icon, a cobalt gradient tile.

## Design decisions

- **The real report UI:** the cards show screenshots of the real report UI, made by `psv render` from each `report.json`, not a custom chart. Each screenshot sits in a light browser frame and fades out at the bottom to show the page continues.
- **Layout:**
  - Porygon, the hard case with 8 moments in 61 s, gets a full-width featured card. Its screenshot is the 1440 px report, cropped through the first moment's chart. Its 8 moments are also listed as text.
  - The three other examples are full-width rows, with details on the left and a screenshot of the 960 px report on the right. A row stays readable at about 0.75× scale; three cards side by side would have shrunk the screenshots to about 0.3×.
- **What each card shows:** verdict, name, length · resolution · fps, a plain summary of its moments, the luminance and red failing-frame counts, and a primary "Open the full report →" button.
- **Phone cards:** each card uses the 560 px report, cropped to the verdict, metrics and overview.
- **Seizure warning:**
  - A full-width panel: a red 2 px border, a red top band, a red-tinted surface, a large warning icon and a 40 px heading (30 px on a phone).
  - The source links sit behind an explicit disclosure. By default only the warning and the button "I understand the risk — show source links" appear. Once confirmed, the links show, and each one except `video-ad-sample-2` is labelled "Contains flashing".
  - Measured contrast on `#FFF1F2`: heading `#9B1428` 7.6:1, body `#3D0A14` 15.3:1. White on the label's `#C21F33` is 5.9:1.
- **Phone help line:** the flags in the help line under the render commands use a non-breaking hyphen, so `-frames` and `-standard` stay on one line.
- **Code blocks:** dark, with a Copy button and real v0.1.0 output. On a phone, long lines are clipped with a fade and a scroll bar.
- **Syntax highlighting:** every code block is highlighted, on desktop and phone.
  - Each line is one text node, and token colours are ranged fills on it (`selection.setPaint` with a character range). Runs cannot drift out of alignment, and the text of each line is unchanged.
  - Plain CLI output stays in the output colour. Output lines that start with `{` (NDJSON) are highlighted.
  - The tokens avoid amber and red, which mean luminance and red flashes on this site.
  - Contrast below is measured against the code background `#16161C` with the WCAG formula. Every token colour passes 4.5:1.

  | Role | Go | Shell | JSON | Colour | Contrast |
  | --- | --- | --- | --- | --- | --- |
  | Default | punctuation, identifiers | arguments | punctuation | `#EEEDEA` | 15.4:1 |
  | Output | | | | `#C8C7CF` | 10.8:1 |
  | Keyword | `package import func for if return range const defer var break` (also `else continue type struct`) | flags (`-i`, `--data-binary`) | keys | `#A9B4FF` light cobalt | 9.1:1 |
  | Function | calls (`Probe(`, `Fatal(`, `len(`) | command names (`curl`, `go`, `./bin/psv`) | | `#7CB8FF` light blue | 8.7:1 |
  | Type | `psv`, `psv.Frame`, `byte int bool float64` | `\|`, `>`, `&&` | | `#5FD4C4` teal | 10.0:1 |
  | String | `"…"` | quoted strings, URLs | string values | `#9AD88A` green | 10.8:1 |
  | Number | `0`, `1280` | | numbers | `#CFA6FF` violet | 9.1:1 |
  | Comment | `// …` | `# …` | | `#9A99A3` grey | 6.4:1 |
  | Prompt | | `$` | | `#8B8A94` dim grey | 5.3:1 |

- **Go package section:** four steps, each a complete program or command.
  - **Get the package:** `go get github.com/brian-nunez/psv`, the module path from `go.mod`.
  - **Analyse a file:** a complete `main.go` around the README's `AnalyzeFile` and `Report` lines, which are unchanged.
  - **Stream frames from a video:**
    - `psv.Probe`, `psv.Decode` and an `Analyzer`, looping `dec.Next()` until `io.EOF`.
    - A live warning prints when a run of failing frames starts, with the frame number, `Analysis.FrameTime` (frames count from 1) and the frame being read when PSV knew. That shows the decision coming about one second later.
    - It also warns about frames that `Finish` decides, and exits with status 1 on a fail.
  - **Analyse a live stream:**
    - Raw RGB frames are piped from FFmpeg on stdin: 1280×720 at 30 fps, matching the pipe's `scale` and `fps`.
    - The program reuses one buffer. `Analyze` samples `Pix` into its own picture before it returns (`detector.process`), so it keeps no reference to the buffer.
  - Under each streaming program, a terminal block shows the command and real output.
- **Go check:** done with Go 1.25 (gvm, `GOTOOLCHAIN=local`) in `.tmp/check/` (gitignored). Its `go.mod` replaces `github.com/brian-nunez/psv` with the repository root.
  - `main.go`, `stream/main.go` and `live/main.go` are the page's three programs. `compare.py` checks that the text in `site.py` matches them byte for byte (`python3 -I .tmp/check/compare.py`).
  - `gofmt -l .` reports nothing, `go vet ./...` passes, and all three build.
  - `go run ./stream` on `testdata/clips/fail.mp4` printed 23 warnings, then `Failed`, with exit status 1. On `testdata/clips/pass.mp4` it printed `Passed`, with exit status 0.
  - `ffmpeg -i testdata/clips/fail.mp4 -vf scale=1280:720,fps=30 -f rawvideo -pix_fmt rgb24 - | go run ./live` printed the same 23 warnings and `Failed`, with exit status 1.
  - The outputs on the page are those runs, trimmed with "…". The page shows `go run .` for the program's own folder; the check ran `go run ./stream` and `go run ./live` from the module root.
- **Type and pill colours:**
  - Section headings are 38 px, 28 px on a phone.
  - The Failed pill's text is `#C21F33`, 5.2:1 on `#FDECEE`. The token `#E5384A` is below 4.5:1 there.

## Simulated or not designed

- The buttons, nav links, Copy buttons and source links are placeholders. "Open the full report →" would open the `psv render` page, which the site would have to publish next to it. That hosting is not designed.
- The Copy button on a block with output would copy the command only.
- The disclosure's two states are separate frames. Its keyboard and focus behaviour is not designed: it should be a real `<button>` with `aria-expanded`, and focus should stay on it when it opens.
- Hover, focus and pressed states are not designed. The phone nav shows only the GitHub button and has no menu.
- On a phone, the text inside a screenshot is small, at about 0.57×. It is a preview; the button opens the report.
- Only two widths are designed, 1440 and 375. Widths in between are not.
