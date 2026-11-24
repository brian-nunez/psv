import PsvModel.History

/-!
# Flashing pixels

Models `channel.flashing` in `detector.go`: a pixel flashes in frame `f`,
knowing the frames up to `now`, when it makes more than `maxT` transitions
within some one-second window of `rate` frames that contains `f`. Windows
start at frames `f-(rate-1) .. f` and are cut off at `now`.

Go starts windows at negative frames near the start of a stream. Frame
numbers here are natural numbers, so those windows start at 0 instead. Each
of them counts no more than the window starting at frame 0, which is always
among the starts, so the decision is the same. The differential tests in
`model_internal_test.go` check that equivalence on the real code.

The theorems are the guarantees the detector and `Analyzer.Watch` rely on:

* `flashing_mono`: later frames can only add flashing pixels.
* `flashing_of_recent`: a pixel whose live count passes `maxT` in frame `t`
  is flashing in the decided result for frame `t`, whatever follows. This is
  why `Activity.Flashing` is final.
* `flashing_final`: one second after frame `f` the decision for `f` cannot
  change, which justifies the detector's `delay` of `rate-1` frames.
* `flashingBits_eq`: the mask-based count Go runs gives the same decision
  whenever the detector asks, since its history spans `2*rate` frames.
-/

namespace PsvModel

/-- Whether a pixel with transitions `mem` flashes in frame `f`, knowing the
frames up to `now`. -/
def flashing (rate maxT : Nat) (mem : Nat → Bool) (f now : Nat) : Bool :=
  (List.range (f + 1)).any fun s =>
    decide (f - (rate - 1) ≤ s) && decide (maxT < window mem s (min (s + (rate - 1)) now))

/-- `channel.flashing`, counting from the mask as Go does. -/
def flashingBits (rate maxT bits : Nat) (h : History) (f now : Nat) : Bool :=
  (List.range (f + 1)).any fun s =>
    decide (f - (rate - 1) ≤ s) && decide (maxT < h.count bits s (min (s + (rate - 1)) now))

theorem any_congr_mem {l : List Nat} {p q : Nat → Bool} (h : ∀ x ∈ l, p x = q x) :
    l.any p = l.any q := by
  induction l with
  | nil => rfl
  | cons a l ih =>
    simp only [List.any_cons]
    rw [h a (by simp), ih (fun x hx => h x (by simp [hx]))]

/-- More transitions and a later end never count fewer. -/
theorem window_mono {mem mem' : Nat → Bool} {lo hi hi' : Nat}
    (hm : ∀ x, lo ≤ x → x ≤ hi → mem x = true → mem' x = true) (hh : hi ≤ hi') :
    window mem lo hi ≤ window mem' lo hi' := by
  have h1 : window mem lo hi ≤ window mem' lo hi :=
    cnt_mono fun j hj => hm (lo + j) (by omega) (by omega)
  have h2 : window mem' lo hi ≤ window mem' lo hi' := by
    rw [window_eq_cnt mem' lo hi (hi' + 1) (by omega),
      window_eq_cnt mem' lo hi' (hi' + 1) (by omega)]
    exact cnt_mono fun j _ hj => by
      simp only [Bool.and_eq_true, decide_eq_true_eq] at hj ⊢
      exact ⟨⟨hj.1.1, by omega⟩, hj.2⟩
  omega

/-- Later frames can only add flashing pixels: a pixel flashing in frame `f`,
knowing frames up to `now`, still flashes once later frames add
transitions. -/
theorem flashing_mono {rate maxT f now now' : Nat} {mem mem' : Nat → Bool}
    (hm : ∀ x, x ≤ now → mem x = true → mem' x = true) (hn : now ≤ now')
    (h : flashing rate maxT mem f now = true) :
    flashing rate maxT mem' f now' = true := by
  simp only [flashing, List.any_eq_true, List.mem_range, Bool.and_eq_true,
    decide_eq_true_eq] at h ⊢
  obtain ⟨s, hs, hlo, hc⟩ := h
  refine ⟨s, hs, hlo, Nat.lt_of_lt_of_le hc ?_⟩
  exact window_mono (fun x _ hx => hm x (by omega)) (by omega)

/-- A pixel with more than `maxT` transitions in the second ending at frame
`t` flashes in frame `t` once `t` is known. -/
theorem flashing_of_recent {rate maxT t now : Nat} {mem : Nat → Bool}
    (hr : 0 < rate) (hc : maxT < window mem (t + 1 - rate) t) (ht : t ≤ now) :
    flashing rate maxT mem t now = true := by
  simp only [flashing, List.any_eq_true, List.mem_range, Bool.and_eq_true,
    decide_eq_true_eq]
  refine ⟨t + 1 - rate, by omega, by omega, Nat.lt_of_lt_of_le hc ?_⟩
  exact window_mono (fun _ _ _ hx => hx) (by omega)

/-- The decision for frame `f` is final one second after it: frames after
`now ≥ f+rate-1` change nothing. -/
theorem flashing_final {rate maxT f now now' : Nat} {mem mem' : Nat → Bool}
    (hf : f + (rate - 1) ≤ now) (hm : ∀ x, x ≤ now → mem x = mem' x) (hn : now ≤ now') :
    flashing rate maxT mem' f now' = flashing rate maxT mem f now := by
  unfold flashing
  apply any_congr_mem
  intro s hs
  simp only [List.mem_range] at hs
  rw [show min (s + (rate - 1)) now' = s + (rate - 1) by omega,
    show min (s + (rate - 1)) now = s + (rate - 1) by omega,
    window_congr (fun x _ hx => (hm x (by omega)).symm)]

/-- Go's mask-based decision equals the model whenever the detector asks:
the pixel's history is up to date (`updated ≤ now`), the frame is at most a
second old (`now ≤ f+rate-1`), and the mask spans two seconds. -/
theorem flashingBits_eq {rate maxT bits f now : Nat} {h : History} {mem : Nat → Bool}
    (hr0 : 0 < rate) (hr : Represents bits h mem) (hu : h.updated ≤ now)
    (hf : now ≤ f + (rate - 1)) (hb : 2 * rate ≤ bits) :
    flashingBits rate maxT bits h f now = flashing rate maxT mem f now := by
  unfold flashingBits flashing
  apply any_congr_mem
  intro s _
  by_cases hs : f - (rate - 1) ≤ s
  · have hd : h.updated < s + bits := by omega
    rw [count_eq_window hr hd]
  · simp [hs]

/-- The guarantee behind `Activity.Flashing`, in Go's terms. If, right after
recording the transition of frame `t`, `channel.recent` exceeds `maxT`, then
the decision for frame `t`, made later from any history that extends this
one, marks the pixel as flashing. -/
theorem live_flashing_is_final {rate maxT bits t now : Nat}
    {h h' : History} {mem mem' : Nat → Bool}
    (hr0 : 0 < rate) (hb : 2 * rate ≤ bits)
    (hr : Represents bits h mem) (ht : h.updated = t) (hc : maxT < h.recent rate)
    (hr' : Represents bits h' mem') (hext : ∀ x, x ≤ t → mem x = true → mem' x = true)
    (hu : h'.updated ≤ now) (htn : t ≤ now) (hn : now ≤ t + (rate - 1)) :
    flashingBits rate maxT bits h' t now = true := by
  rw [flashingBits_eq hr0 hr' hu hn hb]
  rw [recent_eq_window hr (by omega), ht] at hc
  exact flashing_mono hext htn (flashing_of_recent hr0 hc (Nat.le_refl t))

end PsvModel
