package textinput

import "unicode/utf8"

// Reconcile deduces the one [Edit] that turns prev.Text into next.Text. It
// is the fallback of a host whose editing element changed without a
// trustworthy event (Android composition, autofill, an unusual IME): prev is
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
// The cost is linear in the length of both texts, and Reconcile allocates
// nothing: the scans work on the UTF-8 bytes, and only the runes before the
// edit and inside the common suffix are counted, never copied.
func Reconcile(prev, next State) Edit {
	a, b := prev.Text, next.Text
	if a == b {
		return Edit{}
	}

	// Common prefix and suffix, in symbols: a rune, or one invalid byte. The
	// suffix is found on bytes, then moved to a place that is a symbol
	// boundary in both texts.
	bp, pmax := commonPrefix(a, b)
	ta, tb, smax := commonSuffix(a, b, bp)
	la := length(a, bp, pmax, ta, smax)
	lb := length(b, bp, pmax, tb, smax)
	m := min(la, lb)

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

	// Byte offsets in next of both ends of the edit. The ends of the common
	// trims are already known; any other place is found by counting.
	//
	// A boundary between the two halves of a surrogate pair is not an edit
	// boundary: widen the edit to take the whole pair. The common prefix and
	// suffix are the same symbols in both texts, so b answers for a.
	start, end := bp, tb
	if p != pmax {
		var mid bool
		if start, mid = locate(b, p); mid {
			p--
		}
	}
	if s != smax {
		var mid bool
		if end, mid = locate(b, lb-s); mid {
			s--
			end += chunk(b, end)
		}
	}
	return Edit{Start: p, End: la - s, Text: b[start:end]}
}

// commonPrefix returns the byte length of the longest common prefix of a and
// b made of whole, identical symbols, and its length in UTF-16 units. A
// symbol is a rune or one invalid byte; two runes that start at the same
// offset are the same symbol when their bytes are, and the invalid bytes
// 0xFF and 0xFE are different symbols although both count as U+FFFD.
func commonPrefix(a, b string) (bytes, units int) {
	i := 0
	for i < len(a) && i < len(b) {
		if a[i] < utf8.RuneSelf {
			if a[i] != b[i] {
				break
			}
			units++
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(a[i:])
		_, sizeB := utf8.DecodeRuneInString(b[i:])
		if size != sizeB || a[i:i+size] != b[i:i+size] {
			break
		}
		units += width(r)
		i += size
	}
	return i, units
}

// commonSuffix returns where the longest common suffix of whole, identical
// symbols starts in a and in b, and its length in UTF-16 units. bp is a byte
// offset that is a symbol boundary in both texts (the end of the common
// prefix).
//
// Bytes alone cannot say where a suffix starts, because whether a byte
// starts a symbol depends on what precedes it. So the common byte suffix is
// found first, then each text is walked forward, from a known boundary, to
// its first symbol boundary inside it; the two boundaries are then merged
// until they leave the same number of bytes after them. From there the two
// texts hold the same bytes and are cut into the same symbols.
func commonSuffix(a, b string, bp int) (ta, tb, units int) {
	e := 0
	for e < len(a) && e < len(b) && a[len(a)-1-e] == b[len(b)-1-e] {
		e++
	}
	ta, tb = boundaryFrom(a, len(a)-e, bp), boundaryFrom(b, len(b)-e, bp)
	for ra, rb := len(a)-ta, len(b)-tb; ra != rb; ra, rb = len(a)-ta, len(b)-tb {
		if ra > rb {
			ta += chunk(a, ta)
		} else {
			tb += chunk(b, tb)
		}
	}
	return ta, tb, Len(a[ta:])
}

// boundaryFrom returns the first symbol boundary of s at or after target,
// walking forward from the boundary bp when it is not past target, else from
// the start.
func boundaryFrom(s string, target, bp int) int {
	i := 0
	if bp <= target {
		i = bp
	}
	for i < target {
		i += chunk(s, i)
	}
	return i
}

// length returns the length of s in UTF-16 units, given that its first bp
// bytes (pmax units) and its bytes from t on (smax units) are already
// counted.
func length(s string, bp, pmax, t, smax int) int {
	if t < bp {
		// The two ends overlap: the whole text is the cheapest way to count.
		return Len(s)
	}
	return pmax + Len(s[bp:t]) + smax
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
