import PsvModel.Count

/-!
# Transition history

Models the per-pixel history of `channel` in `detector.go`: `hist`, a bitmask
of `bits = 64 * words` bits where bit `k` marks a counted transition `k`
frames before `updated`. Recording a transition at frame `t` ages the mask
by `t - updated` frames (`age`) and sets bit 0.

The abstract model is the set of frames of counted transitions, `mem`. The
proofs show that the mask holds exactly the transitions of the last `bits`
frames, and that counting its bits, as `channel.flashing` and
`channel.recent` do, equals counting frames of `mem` in a window, as long as
the window lies within the last `bits` frames.
-/

namespace PsvModel

/-- The history of one pixel: `channel.hist[i]` as one number, and
`channel.updated[i]`. -/
structure History where
  hist : Nat := 0
  updated : Nat := 0
  deriving Repr

/-- Records a counted transition in frame `t`: `age(h, t-updated)`, then
`h[0] |= 1`. -/
def History.record (bits : Nat) (h : History) (t : Nat) : History :=
  { hist := (h.hist <<< (t - h.updated)) % 2 ^ bits ||| 1, updated := t }

/-- The number of frames of `mem` in frames `lo..hi`. -/
def window (mem : Nat → Bool) (lo hi : Nat) : Nat :=
  cnt (hi + 1 - lo) fun j => mem (lo + j)

/-- The transitions in frames `lo..hi` counted from the mask, as the loop in
`channel.flashing` counts them: bits `k` with `upd-hi ≤ k ≤ upd-lo`. -/
def History.count (bits : Nat) (h : History) (lo hi : Nat) : Nat :=
  cnt bits fun k =>
    decide (k ≤ h.updated) && decide (lo ≤ h.updated - k) &&
      decide (h.updated - k ≤ hi) && h.hist.testBit k

/-- The transitions within the `rate` frames ending at `updated`, as
`channel.recent` counts them: the low `rate` bits. -/
def History.recent (rate : Nat) (h : History) : Nat :=
  cnt rate fun k => h.hist.testBit k

/-- `h` holds exactly the transitions of `mem` within its last `bits`
frames, and `mem` has none after `h.updated`. -/
structure Represents (bits : Nat) (h : History) (mem : Nat → Bool) : Prop where
  bit : ∀ k, h.hist.testBit k =
    (decide (k < bits) && decide (k ≤ h.updated) && mem (h.updated - k))
  past : ∀ x, mem x = true → x ≤ h.updated

theorem testBit_one (k : Nat) : Nat.testBit 1 k = decide (k = 0) := by
  cases k with
  | zero => simp
  | succ k => simp [Nat.testBit_add_one]

/-- A new channel's history represents no transitions. -/
theorem represents_empty (bits : Nat) : Represents bits {} (fun _ => false) where
  bit k := by simp [Nat.zero_testBit]
  past _ h := by simp at h

/-- Recording a transition at a later frame keeps the mask exact. -/
theorem represents_record {bits t : Nat} {h : History} {mem : Nat → Bool}
    (hb : 0 < bits) (hr : Represents bits h mem) (ht : h.updated < t) :
    Represents bits (h.record bits t) (fun x => x == t || mem x) where
  bit k := by
    simp only [History.record, Nat.testBit_or, Nat.testBit_mod_two_pow,
      Nat.testBit_shiftLeft, testBit_one, hr.bit]
    by_cases k0 : k = 0
    · subst k0
      simp [hb]
    · by_cases kn : k < t - h.updated
      · -- Frames after `updated` hold no older transition.
        have hm : mem (t - k) = false := by
          cases hmk : mem (t - k) with
          | false => rfl
          | true => have := hr.past _ hmk; omega
        by_cases kb : k < bits
        · by_cases kt : k ≤ t
          · have hne : (t - k == t) = false := by simp; omega
            simp [k0, kb, kt, hne, hm, show ¬ (k ≥ t - h.updated) by omega]
          · simp [k0, kt, show ¬ (k ≥ t - h.updated) by omega]
        · simp [k0, kb]
      · have hidx : h.updated - (k - (t - h.updated)) = t - k := by omega
        have hne : (t - k == t) = false := by simp; omega
        by_cases kb : k < bits
        · by_cases kt : k ≤ t
          · simp [k0, kb, kt, hidx, hne, show k ≥ t - h.updated by omega,
              show k - (t - h.updated) < bits by omega,
              show k - (t - h.updated) ≤ h.updated by omega]
          · simp [k0, kt, show ¬ (k - (t - h.updated) ≤ h.updated) by omega]
        · simp [k0, kb]
  past x hx := by
    simp only [Bool.or_eq_true, beq_iff_eq] at hx
    rcases hx with hx | hx
    · simp [History.record, hx]
    · have := hr.past x hx
      simp only [History.record]
      omega

/-- Counts agree when the predicates agree on the window. -/
theorem window_congr {mem mem' : Nat → Bool} {lo hi : Nat}
    (h : ∀ x, lo ≤ x → x ≤ hi → mem x = mem' x) :
    window mem lo hi = window mem' lo hi :=
  cnt_congr fun j hj => h (lo + j) (by omega) (by omega)

/-- A window counted from frame 0 with a filter on `lo..hi`. -/
theorem window_eq_cnt (mem : Nat → Bool) (lo hi n : Nat) (hn : hi + 1 ≤ n) :
    window mem lo hi =
      cnt n fun x => decide (lo ≤ x) && decide (x ≤ hi) && mem x := by
  let P : Nat → Bool := fun x => decide (lo ≤ x) && decide (x ≤ hi) && mem x
  have htrunc : cnt n P = cnt (hi + 1) P :=
    cnt_trunc hn fun j hj _ => by simp [P]; omega
  show window mem lo hi = cnt n P
  rw [htrunc]
  by_cases hl : lo ≤ hi + 1
  · have hsplit := cnt_add lo (hi + 1 - lo) P
    rw [Nat.add_sub_cancel' hl] at hsplit
    rw [hsplit, cnt_eq_zero (fun j hj => by simp [P]; omega), Nat.zero_add]
    apply cnt_congr
    intro j hj
    simp [P]
    omega
  · rw [cnt_eq_zero (fun j hj => by simp [P]; omega)]
    unfold window
    rw [show hi + 1 - lo = 0 by omega]
    rfl

/-- Counting a window from the mask counts the window of `mem`, as long as
the window starts within the last `bits` frames. -/
theorem count_eq_window {bits lo hi : Nat} {h : History} {mem : Nat → Bool}
    (hr : Represents bits h mem) (hd : h.updated < lo + bits) :
    h.count bits lo hi = window mem lo hi := by
  let P : Nat → Bool := fun x => decide (lo ≤ x) && decide (x ≤ hi) && mem x
  -- The window of `mem`, counted over frames `0..updated`.
  have hw : window mem lo hi = cnt (h.updated + 1) P := by
    rw [window_eq_cnt mem lo hi (max (hi + 1) (h.updated + 1)) (by omega)]
    by_cases hu : h.updated ≤ hi
    · exact cnt_trunc (by omega) fun j hj hj' => by
        have : mem j = false := by
          cases hm : mem j with
          | false => rfl
          | true => have := hr.past j hm; omega
        simp [this]
    · rw [show max (hi + 1) (h.updated + 1) = h.updated + 1 by omega]
  -- The mask's count, read back to front over frames `0..updated`.
  let G : Nat → Bool := fun k => decide (k ≤ h.updated) && P (h.updated - k)
  have hg : h.count bits lo hi = cnt bits G := by
    apply cnt_congr
    intro k hk
    simp only [hr.bit, G, P]
    by_cases a : k ≤ h.updated <;> by_cases b : lo ≤ h.updated - k <;>
      by_cases c : h.updated - k ≤ hi <;> simp [a, b, c, hk]
  have hupd : cnt bits G = cnt (h.updated + 1) G := by
    by_cases hbu : h.updated + 1 ≤ bits
    · exact cnt_trunc hbu fun j hj _ => by simp [G]; omega
    · exact (cnt_trunc (by omega) fun j hj hj' => by simp [G, P]; omega).symm
  have hrefl : cnt (h.updated + 1) G = cnt (h.updated + 1) P := by
    rw [← cnt_reflect (h.updated + 1) P]
    apply cnt_congr
    intro k hk
    simp [G, show k ≤ h.updated by omega]
  rw [hg, hupd, hrefl, hw]

/-- `channel.recent` counts the transitions within the second ending at
`updated`: frames `updated+1-rate .. updated`. -/
theorem recent_eq_window {bits rate : Nat} {h : History} {mem : Nat → Bool}
    (hr : Represents bits h mem) (hrb : rate ≤ bits) :
    h.recent rate = window mem (h.updated + 1 - rate) h.updated := by
  rw [← count_eq_window hr (by omega)]
  unfold History.recent History.count
  rw [cnt_trunc (g := fun k => decide (k ≤ h.updated) &&
      decide (h.updated + 1 - rate ≤ h.updated - k) &&
      decide (h.updated - k ≤ h.updated) && h.hist.testBit k) hrb
    (fun j hj _ => by simp; omega)]
  apply cnt_congr
  intro k hk
  rw [hr.bit k]
  by_cases hku : k ≤ h.updated
  · simp [hku]
    omega
  · simp [hku]

end PsvModel
