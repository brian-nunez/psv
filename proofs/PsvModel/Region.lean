/-!
# Region transitions

Models `channel.record` in `detector.go`. Each region position keeps the
frames of its most recent alternating transitions in a ring of
`keep = maxT + 1` entries, and counts those within the second ending at the
frame being decided.

The ring forgets older transitions, so its count is capped at `keep`. The
theorems show that this cap never changes a verdict: the ring's count is the
true count capped at `keep`, so it exceeds `maxT` exactly when the true count
does. Frames are decided in order, so the recorded frames, newest first, are
sorted in decreasing order.
-/

namespace PsvModel

/-- The predicate for a transition in frame `a` lying within the second
ending at frame `f`: `at > f-rate`. -/
def inSecond (rate f a : Nat) : Bool := decide (f < a + rate)

/-- The count `channel.record` returns: the transitions among the last `keep`
recorded, newest first in `rs`, within the second ending at frame `f`. -/
def ringCount (rate keep : Nat) (rs : List Nat) (f : Nat) : Nat :=
  (rs.take keep).countP (inSecond rate f)

/-- The frame of the first of those transitions, as `channel.record`
returns it: `f` when there are none. -/
def ringSince (rate keep : Nat) (rs : List Nat) (f : Nat) : Nat :=
  ((rs.take keep).filter (inSecond rate f)).foldl min f

/-- Every transition the region made within the second ending at `f`. -/
def trueCount (rate : Nat) (rs : List Nat) (f : Nat) : Nat :=
  rs.countP (inSecond rate f)

/-- In a decreasing list, an upward-closed predicate holds on a prefix, so
taking `k` elements counts `min k` of the matches. -/
theorem countP_take_sorted {p : Nat → Bool} {rs : List Nat} (k : Nat)
    (hs : rs.Pairwise (· ≥ ·)) (hp : ∀ a b, b ≤ a → p b = true → p a = true) :
    (rs.take k).countP p = min k (rs.countP p) := by
  induction rs generalizing k with
  | nil => simp
  | cons a rest ih =>
    cases k with
    | zero => simp
    | succ k =>
      have hcons := List.pairwise_cons.mp hs
      simp only [List.take_succ_cons, List.countP_cons]
      by_cases ha : p a = true
      · simp only [ha, ite_true, ih k hcons.2]
        omega
      · -- Nothing after `a` matches, since every later frame is older.
        have hnone : ∀ b ∈ rest, p b = false := by
          intro b hb
          cases hpb : p b with
          | false => rfl
          | true => exact absurd (hp a b (hcons.1 b hb) hpb) ha
        have hz : rest.countP p = 0 := by
          rw [List.countP_eq_zero]
          intro b hb
          simp [hnone b hb]
        have hz' : (rest.take k).countP p = 0 := by
          rw [List.countP_eq_zero]
          intro b hb
          simp [hnone b (List.mem_of_mem_take hb)]
        simp [ha, hz, hz']

theorem inSecond_up (rate f : Nat) :
    ∀ a b, b ≤ a → inSecond rate f b = true → inSecond rate f a = true := by
  intro a b hab hb
  simp only [inSecond, decide_eq_true_eq] at hb ⊢
  omega

/-- The ring's count is the true count, capped at `keep`. -/
theorem ringCount_eq_min (rate keep f : Nat) {rs : List Nat} (hs : rs.Pairwise (· ≥ ·)) :
    ringCount rate keep rs f = min keep (trueCount rate rs f) :=
  countP_take_sorted keep hs (inSecond_up rate f)

/-- A ring of `maxT + 1` entries decides failure exactly: it counts more than
`maxT` transitions exactly when the region made more than `maxT`. -/
theorem ring_failed_exact (rate maxT f : Nat) {rs : List Nat} (hs : rs.Pairwise (· ≥ ·)) :
    maxT < ringCount rate (maxT + 1) rs f ↔ maxT < trueCount rate rs f := by
  rw [ringCount_eq_min rate (maxT + 1) f hs]
  omega

/-- Below the cap, the ring's count is the true count. -/
theorem ringCount_exact (rate keep f : Nat) {rs : List Nat} (hs : rs.Pairwise (· ≥ ·))
    (hc : trueCount rate rs f ≤ keep) :
    ringCount rate keep rs f = trueCount rate rs f := by
  rw [ringCount_eq_min rate keep f hs]
  omega

end PsvModel
