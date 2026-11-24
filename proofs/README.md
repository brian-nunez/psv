# Proofs

A [Lean 4](https://lean-lang.org) model of the counting at the heart of the
detector in [`detector.go`](../detector.go), with machine-checked proofs of
the guarantees the detector and `Analyzer.Watch` rely on. The model uses only
Lean's core library.

The proofs cover the logic that turns per-pixel step directions into
verdicts: which steps count as transitions, how the bitmask history stores
them, which one-second windows a decision looks at, and how a region's ring
of recent transitions decides failure. They do not cover colour science (the
luminance and red thresholds, which run on floating point), the area test,
or whether the WCAG thresholds themselves prevent seizures.

## What is proved

| File | Models | Theorems |
| --- | --- | --- |
| [`Step.lean`](PsvModel/Step.lean) | `channel.step` | Counted transitions always alternate in direction and are never zero (`Pixel.run_alternates`), and with directions of `-1`, `0` and `+1` they strictly alternate in sign (`Pixel.run_sign_alternates`). A run of steps in one direction counts at most once (`Pixel.run_replicate_length`). |
| [`History.lean`](PsvModel/History.lean) | `hist`, `age`, `channel.recent`, the counting loop of `channel.flashing` | Recording a transition keeps the mask exact: bit `k` is set exactly when a transition happened `k` frames before the last one, within the mask's width (`represents_record`). Counting bits equals counting frames in any window that starts within the mask (`count_eq_window`, `recent_eq_window`). |
| [`Window.lean`](PsvModel/Window.lean) | `channel.flashing` | Later frames can only add flashing pixels (`flashing_mono`). A pixel whose live count passes the limit flashes in the decided result for that frame, whatever follows (`live_flashing_is_final`), which is why `Activity.Flashing` is final. The decision for a frame cannot change one second after it (`flashing_final`), which justifies deciding `rate-1` frames late. The mask-based count gives the model's decision whenever the detector asks, because the mask spans two seconds (`flashingBits_eq`). |
| [`Region.lean`](PsvModel/Region.lean) | `channel.record` | A ring of `MaxTransitions+1` entries counts the true number of transitions capped at its size (`ringCount_eq_min`), so it decides failure exactly (`ring_failed_exact`). The first frame it returns is exact below the cap (`ringSince_exact`) and never after the frame decided (`ringSince_le`). |

[`Count.lean`](PsvModel/Count.lean) holds the counting lemmas the others
share. [`Check.lean`](Check.lean) gates the build: it fails when any theorem
depends on an axiom beyond Lean's standard three, which catches `sorry`, and
it restates every theorem above, so changing one's statement means changing
that file too.

## What is tested, not proved

The model states some parts of the Go code more simply than the code runs
them. The tests below check these on every input they generate, which is
evidence, not proof:

* The `words == 1` fast path of `channel.flashing`, which counts with one
  shift and mask instead of the loop the model describes.
* The multi-word carry loop in `age`, which the model writes as one shift of
  one number.
* Negative window starts at the beginning of a stream, which the model
  starts at frame 0.
* The ring buffer in `channel.record`, with its `head` index and
  `math.MinInt32` empty slots, which the model writes as a list.
* `rebase`, which moves frame numbers back every 2^30 frames. The model counts
  from frame 0 forever.
* The detector's scheduling, `delay`, the queue of transitions, `measure`
  and `flush`, and the region search in `channel.regions`.

## How the model stays in step with the Go code

A proof is about the model, so these checks tie the model to `detector.go`:

1. `lake exe vectors` prints the model's answers on fixed cases to
   [`testdata/model/vectors.json`](../testdata/model/vectors.json).
   `TestModelVectors` replays them against the real `channel`, at one pixel
   or region position of four, and fails on any difference. CI regenerates the
   file and fails when it differs from the committed copy.
2. `model_internal_test.go` translates the model to Go. `TestModelVectors`
   checks the translation against the vectors too, and
   `TestChannelMatchesModel`, a property test, compares the real `channel`
   with it on random channels of several pixels and region positions, at any
   frame rate and limit `NewAnalyzer` allows, over short and long streams,
   with and without `rebase`.
3. `FuzzPixelMatchesModel`, `FuzzRegionMatchesModel` and
   `FuzzAgeMatchesShift` fuzz the same comparisons, and the multi-word shift
   in `age` against the mask's definition in the model.
4. `TestDetectorScheduleMatchesModel` runs `Analyzer.Watch` and checks that
   each frame is decided once, a second late, from the transitions queued
   when it was seen, that each decision is the model's final one, and that
   the live measurement counts what the model counts.
5. `TestRegionsMatchReference` checks `channel.regions` against a
   brute-force search over every region position.

A change to the counting in `detector.go` that changes an answer fails these
tests. Change the model to match, fix its proofs and `Check.lean`, and run
`make model-vectors`.

## Running

Install [elan](https://github.com/leanprover/elan), which reads the Lean
version from `lean-toolchain`, then:

```bash
make proofs          # check every proof
make model-vectors   # regenerate testdata/model/vectors.json
make fuzz            # fuzz the detector against the model, FUZZTIME=30s per target
```

`make proofs` also fails when a file in `PsvModel/` is not imported by
`PsvModel.lean`, since Lean would never build it.
