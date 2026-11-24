import Lean
import PsvModel

/-!
# Proof gate

Built with the model by `make proofs`. It fails the build when

* any theorem in `PsvModel` depends on an axiom beyond Lean's standard
  `propext`, `Classical.choice` and `Quot.sound`, which catches `sorry`
  (`sorryAx`) and any added `axiom`, and
* a key theorem is deleted or its statement changes: each is restated here,
  so weakening one means editing this file, which a reviewer sees.
-/

open Lean Elab Command

/-- Fails unless every theorem in `PsvModel` depends only on the standard
axioms, and there is at least one. -/
elab "#check_model_axioms" : command => do
  let env ← getEnv
  let allowed := [``propext, ``Classical.choice, ``Quot.sound]
  let thms := env.constants.fold (init := #[]) fun acc name info =>
    if (`PsvModel).isPrefixOf name && info matches .thmInfo _ then acc.push name else acc
  if thms.isEmpty then
    throwError "no theorems found in PsvModel"
  for name in thms do
    for ax in ← liftCoreM (collectAxioms name) do
      unless allowed.contains ax do
        throwError "{name} depends on the axiom {ax}"
  logInfo m!"{thms.size} theorems in PsvModel use only the standard axioms"

#check_model_axioms

namespace PsvModel

example : ∀ (p : Pixel) (ds : List Int), Alternates p.lastDir (p.run ds) :=
  Pixel.run_alternates

example : ∀ (p : Pixel) (ds : List Int), (∀ d ∈ ds, IsDir d) → IsDir p.lastDir →
    SignAlternates p.lastDir (p.run ds) :=
  Pixel.run_sign_alternates

example : ∀ (p : Pixel) (d : Int) (n : Nat), (p.run (List.replicate n d)).length ≤ 1 :=
  Pixel.run_replicate_length

example : ∀ {bits t : Nat} {h : History} {mem : Nat → Bool}, 0 < bits →
    Represents bits h mem → h.updated < t →
    Represents bits (h.record bits t) (fun x => x == t || mem x) :=
  represents_record

example : ∀ {bits lo hi : Nat} {h : History} {mem : Nat → Bool},
    Represents bits h mem → h.updated < lo + bits →
    h.count bits lo hi = window mem lo hi :=
  count_eq_window

example : ∀ {bits rate : Nat} {h : History} {mem : Nat → Bool},
    Represents bits h mem → rate ≤ bits →
    h.recent rate = window mem (h.updated + 1 - rate) h.updated :=
  recent_eq_window

example : ∀ {rate maxT f now now' : Nat} {mem mem' : Nat → Bool},
    (∀ x, x ≤ now → mem x = true → mem' x = true) → now ≤ now' →
    flashing rate maxT mem f now = true → flashing rate maxT mem' f now' = true :=
  flashing_mono

example : ∀ {rate maxT t now : Nat} {mem : Nat → Bool},
    0 < rate → maxT < window mem (t + 1 - rate) t → t ≤ now →
    flashing rate maxT mem t now = true :=
  flashing_of_recent

example : ∀ {rate maxT f now now' : Nat} {mem mem' : Nat → Bool},
    f + (rate - 1) ≤ now → (∀ x, x ≤ now → mem x = mem' x) → now ≤ now' →
    flashing rate maxT mem' f now' = flashing rate maxT mem f now :=
  flashing_final

example : ∀ {rate maxT bits f now : Nat} {h : History} {mem : Nat → Bool},
    0 < rate → Represents bits h mem → h.updated ≤ now → now ≤ f + (rate - 1) →
    2 * rate ≤ bits →
    flashingBits rate maxT bits h f now = flashing rate maxT mem f now :=
  flashingBits_eq

example : ∀ {rate maxT bits t now : Nat} {h h' : History} {mem mem' : Nat → Bool},
    0 < rate → 2 * rate ≤ bits →
    Represents bits h mem → h.updated = t → maxT < h.recent rate →
    Represents bits h' mem' → (∀ x, x ≤ t → mem x = true → mem' x = true) →
    h'.updated ≤ now → t ≤ now → now ≤ t + (rate - 1) →
    flashingBits rate maxT bits h' t now = true :=
  live_flashing_is_final

example : ∀ (rate keep f : Nat) {rs : List Nat}, rs.Pairwise (· ≥ ·) →
    ringCount rate keep rs f = min keep (trueCount rate rs f) :=
  ringCount_eq_min

example : ∀ (rate maxT f : Nat) {rs : List Nat}, rs.Pairwise (· ≥ ·) →
    (maxT < ringCount rate (maxT + 1) rs f ↔ maxT < trueCount rate rs f) :=
  ring_failed_exact

example : ∀ (rate keep f : Nat) {rs : List Nat}, rs.Pairwise (· ≥ ·) →
    trueCount rate rs f ≤ keep →
    ringSince rate keep rs f = (rs.filter (inSecond rate f)).foldl min f :=
  ringSince_exact

end PsvModel
