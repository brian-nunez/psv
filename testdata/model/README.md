# Model vectors

`vectors.json` holds the answers of the Lean model in [`proofs/`](../../proofs)
on fixed cases: 48 pixels and 48 region positions at frame rates from 1 to
100. `TestModelVectors` replays them against the detector's `channel`.

Each pixel case gives a step direction for every frame from 1, and the
model's answers: whether each step counts as a transition, `channel.recent`
after each counted one, and the flashing decision for each frame, both live
and once decided. Each region case gives the frames at which a region
position transitions, and the count and first frame `channel.record` must
return for each.

The file is generated: change it with `make model-vectors`, never by hand. CI
fails when it differs from the model's output.
