package textinput

import (
	"unicode/utf16"
	"unicode/utf8"
)

// This file holds the first implementation of Reconcile, which converts both
// texts to one symbol per UTF-16 unit and diffs the slices. It allocates
// 4 bytes per unit, so the shipped Reconcile works on bytes instead, but it
// is simple enough to be obviously right, and every test that checks
// Reconcile also checks that it gives the same Edit.

// invalidBase is added to an invalid byte to make its symbol in units.
const invalidBase = 0x10000

// units returns one symbol per UTF-16 unit of s, counting like Len. A BMP
// rune is its own symbol and an astral rune gives its two surrogates, so a
// high surrogate is always followed by its low one. An invalid byte is
// invalidBase plus the byte, which no valid rune or surrogate can equal:
// two texts are equal exactly when their symbols are, even when invalid
// bytes and a real U+FFFD would both count as the unit 0xFFFD.
func units(s string) []uint32 {
	u := make([]uint32, 0, len(s))
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && size == 1:
			u = append(u, invalidBase+uint32(s[i]))
		case r >= 0x10000:
			hi, lo := utf16.EncodeRune(r)
			u = append(u, uint32(hi), uint32(lo))
		default:
			u = append(u, uint32(r))
		}
		i += size
	}
	return u
}

func reconcileRef(prev, next State) Edit {
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
	// Trims are of whole symbols: a trim that ends inside a pair does not
	// count the half pair (🙂 and 🙔 share their high surrogate).
	if pmax > 0 && a[pmax-1] >= 0xD800 && a[pmax-1] < 0xDC00 {
		pmax--
	}
	if smax > 0 && a[la-smax] >= 0xDC00 && a[la-smax] < 0xE000 {
		smax--
	}
	total := min(pmax+smax, m)
	lo := max(0, total-smax)
	hi := min(pmax, total)
	p := hi
	if t, ok := caretTarget(prev, next); ok {
		p = min(max(t, lo), hi)
	}
	s := total - p
	high := func(u uint32) bool { return u >= 0xD800 && u < 0xDC00 }
	low := func(u uint32) bool { return u >= 0xDC00 && u < 0xE000 }
	if p > 0 && high(a[p-1]) {
		p--
	}
	if s > 0 && low(a[la-s]) {
		s--
	}
	return Edit{
		Start: p,
		End:   la - s,
		Text:  next.Text[ToByte(next.Text, p):ToByte(next.Text, lb-s)],
	}
}
