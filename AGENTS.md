# Photosafety Validator

PSV finds hazardous flashing in video against WCAG 2.2 Success Criterion 2.3.1 and the Trace 2024, broadcast and scene-change rules. It is a Go library with a CLI (`psv`), an API server (`psv serve`) and a report UI, for people checking videos before publishing and for pipelines that gate on the result. It is a public proof of concept; releases start at v0.1.0.

## Layout

- Repository root (package `psv`): the core. Detection, thresholds, results and the `psv.report/v1` report. It imports only the standard library; FFmpeg is used only by `Probe`, `Decode` and `AnalyzeFile`. All detection and report logic goes here, never in a frontend ([ADR-0002](docs/adr/0002-keep-detection-in-one-core-go-package.md)).
- `cmd/psv`: the CLI and its subcommands (`analyze`, `render`, `serve`). Wiring only.
- `internal/standards`: the preset names shared by the CLI and the API.
- `internal/server`: the API. It stores nothing beyond the life of a request.
- `internal/ui`: the report UI. `assets/` holds the only copy of the UI's HTML, CSS and JavaScript, which `psv serve` serves and `psv render` inlines ([ADR-0003](docs/adr/0003-build-the-cli-api-and-ui-as-thin-frontends-over-the-core.md)).
- `testdata/pse-test-media`: the vendored conformance benchmark. `testdata/clips`: small generated clips for the CLI and API tests. `testdata/examples`: full-resolution real-world videos with their reports.
- `scripts/`: release scripts, tested by `make test-scripts`.
- `design/mockups`: report design mockups and their generator scripts; not product code.
- `docs/`: `how-it-works.md`, `testing.md` and `adr/`.

## Commands

Use the `Makefile` targets:

- `make build`: `bin/psv`.
- `make test`: race detector on a subset, then every test; what CI runs. Needs `ffmpeg` on `PATH` for the CLI and API tests.
- `make test-examples [EXAMPLE=<folder>]`: the full-resolution example videos.
- `make lint`, `make fmt`, `make fmt-check`, `make vet`.
- `make examples`: regenerate `testdata/examples/*/report.json` with the current build.
- `make clips`: regenerate `testdata/clips` with FFmpeg.
- `make serve`: build and run the API server and report UI.
- `make release VERSION=vX.Y.Z`: the release binaries and `SHA256SUMS` in `dist/`. Releases are made by GitHub Actions, never locally.

## Conventions

- `psv.report/v1` is the only report format. Every frontend writes and reads it; an incompatible change to a field is a new schema name with its own ADR ([ADR-0001](docs/adr/0001-use-psv-report-v1-as-the-shared-report-format.md)).
- The report UI renders reports and never analyses video. Figures it derives from per-frame results stay in `app.js` until another frontend needs them, at which point they move into the core.
- CLI and API tests decode `testdata/clips`, never the example videos, so `make test` stays fast. Real footage is checked only by `TestExamples`.
- `testdata/examples/*/report.json` and `testdata/clips/*.mp4` are generated: change them with `make examples` and `make clips`, review the diff, and commit the result. A change to example reports means detection results changed and needs a reason in the commit body.
- README and documentation examples are real output of a released binary, never hand-typed values.
- `psv -version` and `tool.version` come from the build: the release tag in release binaries, `git describe` for `make build`. Never set a version by hand.

## Commits

Every commit follows [Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/). The release workflow derives the next version from them (`scripts/next-version.sh`): `feat` is a minor release, `fix` and other types a patch, a breaking change a major, and `docs`, `ci`, `chore`, `test` or `style` alone no release.

```text
<type>(<scope>): <summary>

<body>

<footers>
```

- **Type**: `feat` (new behavior), `fix` (corrected behavior), `docs`, `refactor` (no behavior change), `perf`, `test`, `build`, `ci`, `style` (formatting only), `chore`, `revert`.
- **Scope**: the area changed, from this project's areas: `core`, `report`, `cli`, `server`, `ui`, `testdata`, `scripts`. Omit it only when the change spans the whole repository.
- **Summary**: imperative mood, lowercase, no trailing period, 72 characters or fewer. It names the change in behavior: `fix(server): delete the upload when the client disconnects`, not `fix(server): update server.go`.
- **Body**: required unless the summary alone tells a reviewer everything. Wrap at 72 columns. State:
    - **Why**: the problem, bug or need that prompted the change.
    - **What**: the approach, and what now behaves differently.
    - **Impact**: anything a reader must know: changed detection results, config changes, follow-ups, trade-offs taken.

    Every sentence carries information a reviewer cannot get from the diff. A body with nothing to add is left out.
- **Footers**: `BREAKING CHANGE: <what breaks and how to migrate>` for incompatible changes (and `!` after the type or scope); `Refs: #<issue>` or `Closes: #<issue>` for tracked work.
- One logical change per commit.

## Decisions

Decisions are recorded in [`docs/adr/`](docs/adr/README.md). A decision that is costly to reverse gets a MADR record before implementation, per `docs/adr/README.md`.

## Boundaries

- `testdata/pse-test-media` is vendored third-party data under its own BSD 3-Clause licence: never edit it. Changing the pinned commit needs Brian's approval.
- Do not add `testdata/examples` videos or regenerate their reports without Brian's approval; they are the real-footage baseline.
- Do not add third-party imports to the core package.
- Do not create tags or releases by hand; merging to `main` releases through the Release workflow.
