import PsvModel

/-!
# Test vectors

`lake exe vectors` prints `testdata/model/vectors.json`: inputs for the
detector's per-pixel and per-region counting, with the answers of the proved
model. `TestModelVectors` in `model_internal_test.go` replays them against
the real `channel` in `detector.go`. The cases come from a fixed seed, so the
output only changes when the model or the generator changes.

The answers use the model's specification, `flashing` and `window`, over the
set of counted frames, not the mask-based `flashingBits`. The proofs in
`PsvModel.Window` show the two agree whenever the detector asks.
-/

open PsvModel

/-- SplitMix64, a small deterministic generator. -/
structure Rng where
  state : UInt64

def Rng.next (g : Rng) : UInt64 × Rng :=
  let s := g.state + 0x9E3779B97F4A7C15
  let z := (s ^^^ (s >>> 30)) * 0xBF58476D1CE4E5B9
  let z := (z ^^^ (z >>> 27)) * 0x94D049BB133111EB
  (z ^^^ (z >>> 31), ⟨s⟩)

abbrev Gen := StateM Rng

/-- A number below `n`, or 0 when `n` is 0. -/
def below (n : Nat) : Gen Nat := do
  let (v, g) := (← get).next
  set g
  return if n = 0 then 0 else v.toNat % n

def pick (xs : List Nat) : Gen Nat := do
  return xs.getD (← below xs.length) 0

/-- Common frame rates and every rate on each side of a change in the
number of history words, `(2*rate+63)/64`, up to `MaxFrameRate`. -/
def rates : List Nat :=
  [1, 2, 3, 5, 8, 12, 24, 25, 30, 31, 32, 33, 48, 50, 60, 63, 64, 65, 96, 97, 100, 120,
    128, 129, 144, 160, 161, 192, 193, 224, 225, 240]

def maxes : List Nat := [0, 1, 2, 3, 3, 6, 6, 6, 7, 20]

/-- One step direction for each frame from 1 to `n`: flashes, ramps, noise
and stills in random segments. -/
partial def directions (rate n : Nat) : Gen (Array Int) := do
  let mut out : Array Int := #[]
  while out.size < n do
    let len ← (· + 1) <$> below (2 * rate + 2)
    match ← below 4 with
    | 0 =>
      -- A flash: alternating steps every `q` frames.
      let q ← (· + 1) <$> below (max 1 (rate / 3))
      let mut dir : Int := if (← below 2) = 0 then 1 else -1
      for i in [0:len] do
        if i % q = 0 then
          out := out.push dir
          dir := -dir
        else
          out := out.push 0
    | 1 =>
      -- A ramp: one direction for several frames.
      let dir : Int := if (← below 2) = 0 then 1 else -1
      for _ in [0:len] do
        out := out.push dir
    | 2 =>
      for _ in [0:len] do
        let r ← below 3
        out := out.push ((r : Int) - 1)
    | _ =>
      for _ in [0:len] do
        out := out.push 0
  return out.extract 0 n

structure PixelCase where
  rate : Nat
  maxT : Nat
  dirs : Array Int
  counted : Array Bool
  recent : Array Int
  live : Array Bool
  decided : Array Bool

/-- Runs the model over one pixel's steps. Frame 0 has no step, frame `t`
steps by `dirs[t-1]`. `live[f]` is the decision for frame `f` knowing frames
up to `f`, `decided[f]` knowing frames up to `f+rate-1`, or to the last
frame for the final second, as `detector.decide` and `detector.flush`
ask. -/
def runPixel (rate maxT : Nat) (dirs : Array Int) : PixelCase := Id.run do
  let n := dirs.size
  let mut px : Pixel := {}
  let mut mem : Array Bool := Array.replicate (n + 1) false
  let mut counted : Array Bool := #[]
  let mut recent : Array Int := #[]
  let mut live : Array Bool := #[]
  let mut decided : Array Bool := Array.replicate (n + 1) false
  for t in [0:n + 1] do
    if t > 0 then
      let d := dirs[t - 1]!
      let c := px.counts d
      px := px.step d
      counted := counted.push c
      if c then
        mem := mem.set! t true
        let m := mem
        recent := recent.push (window (fun x => m.getD x false) (t + 1 - rate) t)
      else
        recent := recent.push (-1)
    let m := mem
    let memf := fun x => m.getD x false
    live := live.push (flashing rate maxT memf t t)
    if t + 1 ≥ rate then
      let f := t + 1 - rate
      decided := decided.set! f (flashing rate maxT memf f t)
    if t = n then
      for f in [n + 2 - min rate (n + 2):n + 1] do
        decided := decided.set! f (flashing rate maxT memf f n)
  return { rate, maxT, dirs, counted, recent, live, decided }

/-- A pixel at `rate`, over a stream of a few seconds, or, when `short`,
over less than a second, where only `detector.flush` decides frames. -/
def pixelCase (rate : Nat) (short : Bool) : Gen PixelCase := do
  let maxT ← pick maxes
  let n ← if short then below (rate - 1) else (2 * rate + 10 + ·) <$> below (rate + 1)
  return runPixel rate maxT (← directions rate n)

structure RegionCase where
  rate : Nat
  maxT : Nat
  frames : Array Nat
  counts : Array Nat
  since : Array Nat

/-- Records region transitions at increasing frames, with the count and
first frame `channel.record` must return for each. -/
def regionCase (rate maxT : Nat) : Gen RegionCase := do
  let keep := maxT + 1
  let records := 3 * keep + 5 + (← below (2 * keep))
  let mut frame ← below rate
  let mut frames : Array Nat := #[]
  let mut counts : Array Nat := #[]
  let mut since : Array Nat := #[]
  let mut newest : List Nat := []
  for _ in [0:records] do
    let gap ← if (← below 8) = 0 then (· + rate) <$> below rate
      else (· + 1) <$> below (max 1 (2 * rate / keep))
    frame := frame + gap
    newest := frame :: newest
    frames := frames.push frame
    counts := counts.push (ringCount rate keep newest frame)
    since := since.push (ringSince rate keep newest frame)
  return { rate, maxT, frames, counts, since }

def jsonArray {α : Type} (xs : Array α) (f : α → String) : String :=
  "[" ++ ",".intercalate (xs.toList.map f) ++ "]"

def jsonBool (b : Bool) : String := if b then "true" else "false"

def PixelCase.json (c : PixelCase) : String :=
  "{\"rate\":" ++ toString c.rate ++ ",\"maxTransitions\":" ++ toString c.maxT ++
  ",\"dirs\":" ++ jsonArray c.dirs toString ++
  ",\"counted\":" ++ jsonArray c.counted jsonBool ++
  ",\"recent\":" ++ jsonArray c.recent toString ++
  ",\"live\":" ++ jsonArray c.live jsonBool ++
  ",\"decided\":" ++ jsonArray c.decided jsonBool ++ "}"

def RegionCase.json (c : RegionCase) : String :=
  "{\"rate\":" ++ toString c.rate ++ ",\"maxTransitions\":" ++ toString c.maxT ++
  ",\"frames\":" ++ jsonArray c.frames toString ++
  ",\"counts\":" ++ jsonArray c.counts toString ++
  ",\"since\":" ++ jsonArray c.since toString ++ "}"

/-- One pixel and one region at every rate, short streams, and regions with
the largest ring `NewAnalyzer` allows, 255 entries. -/
def cases : Gen (Array PixelCase × Array RegionCase) := do
  let mut pixels := #[]
  for rate in rates do
    pixels := pixels.push (← pixelCase rate false)
  for _ in [0:12] do
    pixels := pixels.push (← pixelCase (max 3 (← pick rates)) true)
  let mut regions := #[]
  for rate in rates do
    regions := regions.push (← regionCase rate (← pick maxes))
  for _ in [0:4] do
    regions := regions.push (← regionCase (← pick rates) 254)
  return (pixels, regions)

def main : IO Unit := do
  let ((pixels, regions), _) := cases.run ⟨0x5053562D4D4F444C⟩
  IO.println "{"
  IO.println "\"generator\":\"proofs/Vectors.lean\","
  IO.println "\"pixel\":["
  IO.println (",\n".intercalate (pixels.toList.map PixelCase.json))
  IO.println "],"
  IO.println "\"region\":["
  IO.println (",\n".intercalate (regions.toList.map RegionCase.json))
  IO.println "]"
  IO.println "}"
