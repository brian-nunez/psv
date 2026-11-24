---
status: proposed
date: 2026-11-23
decision-makers: Brian Nunez
---

# Prove the detector's counting with a Lean model kept in step by tests

## Context and Problem Statement

The detector's verdicts rest on a few pieces of counting logic in `detector.go`: which pixel steps count as transitions, a multi-word bitmask history of them, the one-second windows a decision looks at, and a ring that keeps a region's recent transitions. Two promises depend on that logic being exactly right. `Analyzer.Watch` documents `Activity.Flashing` as final, and PSV Guard covers the screen on the strength of it. The detector decides each frame `rate-1` frames late on the claim that no later frame can change the decision.

The conformance suite and the example videos test these on real footage, but they cannot show that the promises hold for every input, such as frame rates above 32 where the history spans several words, or a region whose ring has forgotten older transitions. How do we get that assurance without changing the detector?

## Decision Drivers

* Guarantees for every input, not just the inputs a test happens to try.
* No change to the detector, its results, the core's dependencies or the release.
* The guarantees must break loudly when the Go code drifts from them.
* Cheap to run in CI.

## Considered Options

* A Lean 4 model with proofs, tied to the Go code by generated vectors, a property test and fuzzing
* More example-based and property tests in Go only
* Verifying the Go code directly with a Go verification tool

## Decision Outcome

Chosen option: "A Lean 4 model with proofs, tied to the Go code by generated vectors, a property test and fuzzing", because it is the only option that proves the promises for every input while leaving the detector untouched.

The model lives in `proofs/` and uses only Lean's core library, with the Lean version pinned in `proofs/lean-toolchain`. `lake exe vectors` writes the model's answers to `testdata/model/vectors.json`, which `TestModelVectors` replays against the real `channel`. A Go translation of the model in `model_internal_test.go` is checked against the same vectors, then compared with the real code on random input by `TestChannelMatchesModel` and three fuzz targets.

### Consequences

* Good, because the alternation of transitions, the exactness of the bitmask history, the finality of live flashing, the finality of a decision after one second and the exactness of the region ring are proved for every frame rate, limit and sequence of steps.
* Good, because a change to the counting that changes any answer fails `go test`, and CI fails when the committed vectors differ from the model.
* Good, because the Go module gains no dependency: Lean runs only in the `proofs` CI job and on machines that run `make proofs`.
* Bad, because the proofs are about a model. The tests tie it to the Go code on many inputs, which is evidence, not proof, that the two agree.
* Bad, because changing the counting now also means changing the model and its proofs, which needs some Lean.
* Neutral, because floating-point colour science, the area test and the thresholds themselves stay outside the model.

### Confirmation

The `proofs` CI job runs `make proofs`, which fails on any warning, on any file in `proofs/PsvModel` that is not imported, and, through `proofs/Check.lean`, on any theorem that uses `sorry` or an added axiom or whose statement changed. It then runs `make model-vectors` and fails when the vectors change. The `check` job runs `TestModelVectors`, `TestChannelMatchesModel`, `TestDetectorScheduleMatchesModel`, `TestRegionsMatchReference` and the fuzz seeds in `make test`, and fuzzes each target for 15 seconds. `proofs/README.md` lists the parts of the Go code these tests check without a proof.

## Pros and Cons of the Options

### A Lean 4 model with proofs, tied to the Go code by generated vectors, a property test and fuzzing

* Good, because Lean checks every proof mechanically, for all inputs.
* Good, because Lean compiles the model to a program, so the same definitions the proofs are about produce the vectors.
* Good, because core Lean is enough, so the project builds in seconds without Mathlib.
* Bad, because it adds a second language and toolchain to the repository.

### More example-based and property tests in Go only

* Good, because it needs nothing new.
* Bad, because tests sample inputs. They cannot show a property holds for every frame rate and sequence of steps.

### Verifying the Go code directly with a Go verification tool

* Good, because there would be no model to keep in step.
* Bad, because Go verification tools are research prototypes that do not cover the concurrency, slices and bit operations `detector.go` uses.

## More Information

`proofs/README.md` lists every theorem and the Go function it covers.
