/-!
# Pixel transitions

Models `channel.step` in `detector.go` without its bitmask history, which
`PsvModel.History` covers.

A pixel steps in direction `+1`, `-1` or `0` between two frames. A step
starts a transition when it is nonzero and differs from the previous frame's
step, so a run of steps in one direction is one transition. A transition is
counted only when it is opposite to the pixel's last counted one.
-/

namespace PsvModel

/-- The transition state of one pixel: `channel.stepDir[i]` and
`channel.lastDir[i]`. -/
structure Pixel where
  stepDir : Int := 0
  lastDir : Int := 0
  deriving Repr, DecidableEq

/-- Whether a step in direction `dir` starts a counted transition, as
`channel.step` returns. -/
def Pixel.counts (p : Pixel) (dir : Int) : Bool :=
  dir != 0 && dir != p.stepDir && dir != p.lastDir

/-- The state after a step in direction `dir`. -/
def Pixel.step (p : Pixel) (dir : Int) : Pixel :=
  if p.counts dir then { stepDir := dir, lastDir := dir }
  else { p with stepDir := dir }

/-- The directions of the counted transitions of a sequence of steps, in
order. -/
def Pixel.run : Pixel → List Int → List Int
  | _, [] => []
  | p, d :: ds => if p.counts d then d :: (p.step d).run ds else (p.step d).run ds

/-- Directions that start from `last` and then alternate, none of them zero. -/
def Alternates : Int → List Int → Prop
  | _, [] => True
  | last, d :: ds => d ≠ 0 ∧ d ≠ last ∧ Alternates d ds

/-- A counted transition is nonzero, starts a transition and reverses the
last counted one. -/
theorem Pixel.counts_spec (p : Pixel) (dir : Int) :
    p.counts dir = true ↔ dir ≠ 0 ∧ dir ≠ p.stepDir ∧ dir ≠ p.lastDir := by
  simp [Pixel.counts, Bool.and_eq_true, bne_iff_ne, and_assoc]

theorem Pixel.step_lastDir (p : Pixel) (dir : Int) :
    (p.step dir).lastDir = if p.counts dir then dir else p.lastDir := by
  unfold Pixel.step
  split <;> rfl

theorem Pixel.step_stepDir (p : Pixel) (dir : Int) : (p.step dir).stepDir = dir := by
  unfold Pixel.step
  split <;> rfl

/-- Counted transitions always alternate in direction, starting opposite to
the pixel's last counted transition, and none is zero. -/
theorem Pixel.run_alternates (p : Pixel) (ds : List Int) :
    Alternates p.lastDir (p.run ds) := by
  induction ds generalizing p with
  | nil => trivial
  | cons d ds ih =>
    unfold Pixel.run
    have ihd := ih (p.step d)
    rw [Pixel.step_lastDir] at ihd
    by_cases h : p.counts d = true
    · simp only [h, ite_true] at ihd ⊢
      have := (Pixel.counts_spec p d).mp h
      exact ⟨this.1, this.2.2, ihd⟩
    · simp only [h, Bool.false_eq_true, ite_false] at ihd ⊢
      exact ihd

/-- Once a pixel has stepped in direction `d`, more steps in direction `d`
count nothing. -/
theorem Pixel.run_repeat (p : Pixel) (d : Int) (n : Nat) (h : p.stepDir = d) :
    p.run (List.replicate n d) = [] := by
  induction n generalizing p with
  | zero => rfl
  | succ n ih =>
    simp only [List.replicate_succ, Pixel.run]
    have hc : p.counts d = false := by
      cases hc : p.counts d with
      | false => rfl
      | true => exact absurd h.symm ((Pixel.counts_spec p d).mp hc).2.1
    simp only [hc, Bool.false_eq_true, ite_false]
    exact ih (p.step d) (Pixel.step_stepDir p d)

/-- A run of steps in one direction counts at most one transition. -/
theorem Pixel.run_replicate_length (p : Pixel) (d : Int) (n : Nat) :
    (p.run (List.replicate n d)).length ≤ 1 := by
  cases n with
  | zero => simp [Pixel.run]
  | succ n =>
    simp only [List.replicate_succ, Pixel.run]
    rw [Pixel.run_repeat (p.step d) d n (Pixel.step_stepDir p d)]
    split <;> simp

end PsvModel
