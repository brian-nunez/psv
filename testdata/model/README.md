# Model vectors

`vectors.json` holds the answers of the Lean model in [`proofs/`](../../proofs)
on fixed cases: a pixel and a region position at each of 32 frame rates from 1
to 240, including every rate on each side of a change in the number of history
words, 12 pixels over streams shorter than a second, and 4 regions with the
largest ring `NewAnalyzer` allows. `TestModelVectors` replays them against the
detector's `channel`.

Each pixel case gives a step direction for every frame from 1, and the
model's answers: whether each step counts as a transition, `channel.recent`
after each counted one, and the flashing decision for each frame, both live
and once decided. Each region case gives the frames at which a region
position transitions, and the count and first frame `channel.record` must
return for each.

The file is generated: change it with `make model-vectors`, never by hand. CI
fails when it differs from the model's output.
