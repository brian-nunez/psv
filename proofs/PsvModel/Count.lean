/-!
# Counting

`cnt n g` counts the indices below `n` that satisfy `g`. The detector's
window counts, bitmask counts and ring counts are all stated with it, so the
proofs about them share one small set of lemmas.
-/

namespace PsvModel

/-- The number of `j < n` with `g j`. -/
def cnt : Nat → (Nat → Bool) → Nat
  | 0, _ => 0
  | n + 1, g => cnt n g + if g n then 1 else 0

@[simp] theorem cnt_zero (g : Nat → Bool) : cnt 0 g = 0 := rfl

theorem cnt_succ (n : Nat) (g : Nat → Bool) :
    cnt (n + 1) g = cnt n g + if g n then 1 else 0 := rfl

/-- Counts agree when the predicates agree below `n`. -/
theorem cnt_congr {n : Nat} {g g' : Nat → Bool}
    (h : ∀ j, j < n → g j = g' j) : cnt n g = cnt n g' := by
  induction n with
  | zero => rfl
  | succ n ih =>
    rw [cnt_succ, cnt_succ, ih (fun j hj => h j (by omega)), h n (by omega)]

/-- A weaker predicate never counts more. -/
theorem cnt_mono {n : Nat} {g g' : Nat → Bool}
    (h : ∀ j, j < n → g j = true → g' j = true) : cnt n g ≤ cnt n g' := by
  induction n with
  | zero => exact Nat.le_refl _
  | succ n ih =>
    rw [cnt_succ, cnt_succ]
    have hlo := ih (fun j hj => h j (by omega))
    have hn := h n (by omega)
    by_cases a : g n = true
    · simp [a, hn a]
      omega
    · have a' : g n = false := by simpa using a
      simp only [a', Bool.false_eq_true, ite_false]
      split <;> omega

/-- A count never exceeds the number of indices. -/
theorem cnt_le (n : Nat) (g : Nat → Bool) : cnt n g ≤ n := by
  induction n with
  | zero => exact Nat.le_refl _
  | succ n ih =>
    rw [cnt_succ]
    split <;> omega

/-- A predicate false everywhere below `n` counts nothing. -/
theorem cnt_eq_zero {n : Nat} {g : Nat → Bool}
    (h : ∀ j, j < n → g j = false) : cnt n g = 0 := by
  induction n with
  | zero => rfl
  | succ n ih =>
    rw [cnt_succ, ih (fun j hj => h j (by omega)), h n (by omega)]
    rfl

/-- Splits a count at `a`. -/
theorem cnt_add (a n : Nat) (g : Nat → Bool) :
    cnt (a + n) g = cnt a g + cnt n (fun j => g (a + j)) := by
  induction n with
  | zero => rfl
  | succ n ih =>
    rw [← Nat.add_assoc, cnt_succ, ih, cnt_succ]
    omega

/-- Takes the first index off a count. -/
theorem cnt_succ_left (n : Nat) (g : Nat → Bool) :
    cnt (n + 1) g = (if g 0 then 1 else 0) + cnt n (fun j => g (j + 1)) := by
  have := cnt_add 1 n g
  rw [Nat.add_comm 1 n] at this
  rw [this]
  simp only [cnt, Nat.zero_add]
  congr 1
  apply cnt_congr
  intro j _
  rw [Nat.add_comm]

/-- Counting in reverse order counts the same. -/
theorem cnt_reflect (n : Nat) (g : Nat → Bool) :
    cnt n (fun k => g (n - 1 - k)) = cnt n g := by
  induction n generalizing g with
  | zero => rfl
  | succ n ih =>
    rw [cnt_succ, cnt_succ_left]
    have hn : (n + 1 - 1 - n) = 0 := by omega
    rw [hn]
    have hrest : cnt n (fun k => g (n + 1 - 1 - k)) =
        cnt n (fun k => (fun j => g (j + 1)) (n - 1 - k)) := by
      apply cnt_congr
      intro j hj
      congr 1
      omega
    rw [hrest, ih (fun j => g (j + 1))]
    omega

/-- Indices at or above `m` that never satisfy `g` add nothing. -/
theorem cnt_trunc {m n : Nat} {g : Nat → Bool} (hmn : m ≤ n)
    (h : ∀ j, m ≤ j → j < n → g j = false) : cnt n g = cnt m g := by
  have hsplit := cnt_add m (n - m) g
  rw [Nat.add_sub_cancel' hmn] at hsplit
  rw [hsplit, cnt_eq_zero (fun j hj => h (m + j) (by omega) (by omega))]
  rfl

end PsvModel
