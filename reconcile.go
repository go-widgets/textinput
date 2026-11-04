package textinput

// Reconcile deduces the one [Edit] that turns prev.Text into next.Text. It
// is the fallback of a host whose editing element changed without a
// trustworthy event (Android composition, autofill, an odd IME): prev is
// what the host last wrote to the element, next is what it reads back. The
// same diff is VS Code's TextAreaState.deduceInput and Flutter web's
// inferDeltaState.
//
// The result is minimal: it replaces the fewest UTF-16 units, found by
// trimming the longest common prefix and suffix. When several minimal edits
// exist (typing "a" after "a" gives "aa" whichever "a" is new), the one that
// starts nearest to the caret is chosen: the smaller of the two selection
// starts, which is the rule VS Code applies by bounding the prefix with
// both selections. A selection with a negative anchor or focus is ignored,
// and without any selection the edit is placed as far right as possible.
//
// Start and End are offsets in prev.Text and never split a surrogate pair.
// Edit.Text is a slice of next.Text, so invalid UTF-8 is kept byte for byte,
// and prev.Text with the Edit applied is always next.Text. Equal texts give
// the zero Edit, whatever the selections. The marked range of both states is
// not used: it is the caller's business to re-mark after applying the Edit.
//
// The cost is linear in the length of both texts.
func Reconcile(prev, next State) Edit {
	if prev.Text == next.Text {
		return Edit{}
	}
	a, b := units(prev.Text), units(next.Text)
	la, lb := len(a), len(b)
	m := min(la, lb)

	pmax := 0
	for pmax < m && a[pmax] == b[pmax] {
		pmax++
	}
	smax := 0
	for smax < m && a[la-1-smax] == b[lb-1-smax] {
		smax++
	}

	// The edit covers la-p-s units of prev. It is smallest when p+s is as
	// large as the two trims allow without overlapping, and then p can slide
	// anywhere in [lo, hi].
	total := min(pmax+smax, m)
	lo := max(0, total-smax)
	hi := min(pmax, total)
	p := hi
	if t, ok := caretTarget(prev, next); ok {
		p = min(max(t, lo), hi)
	}
	s := total - p

	// A boundary between the two halves of a surrogate pair is not an edit
	// boundary: widen the edit to take the whole pair.
	if p > 0 && isHigh(a[p-1]) {
		p--
	}
	if s > 0 && isLow(a[la-s]) {
		s--
	}
	return Edit{
		Start: p,
		End:   la - s,
		Text:  next.Text[ToByte(next.Text, p):ToByte(next.Text, lb-s)],
	}
}

// caretTarget returns the smaller selection start of the two states, and
// false when neither has a selection.
func caretTarget(prev, next State) (int, bool) {
	t, ok := 0, false
	for _, st := range [2]State{prev, next} {
		if st.Anchor < 0 || st.Focus < 0 {
			continue
		}
		v := min(st.Anchor, st.Focus)
		if !ok || v < t {
			t, ok = v, true
		}
	}
	return t, ok
}

func isHigh(u uint32) bool { return u >= 0xD800 && u < 0xDC00 }
func isLow(u uint32) bool  { return u >= 0xDC00 && u < 0xE000 }
