package textinput

import (
	"testing"
	"unicode/utf16"
	"unicode/utf8"
)

// Seeds shared by every fuzz target. They are the inputs that broke, or
// nearly broke, comparable code elsewhere: lone surrogates (encoded as
// UTF-8, which Go reads as three invalid bytes), invalid UTF-8, emoji ZWJ
// sequences, combining marks, CRLF and bare CR.
var seeds = []string{
	"",
	"a",
	"hello\nworld",
	"a\r\nb\r\nc",
	"a\rb",
	"\n\n\n",
	"é世界",
	"éé",
	"🙂",
	"a🙂b🙂c",
	"👨‍👩‍👧‍👦 family",
	"🏳️‍🌈",
	"\xed\xa0\x80",             // U+D800 as UTF-8: a lone high surrogate
	"\xed\xb0\x80",             // U+DC00: a lone low surrogate
	"\xed\xa0\xbd\xed\xb8\x82", // a surrogate pair encoded as two 3-byte forms (CESU-8)
	"\xff\xfe\xfd",
	"\xc0\xaf",         // overlong
	"x\xf0\x9f\x99",    // truncated 4-byte rune
	"\xf4\x90\x80\x80", // beyond U+10FFFF
	"a\xe2\x82",        // truncated 3-byte rune
	"�",
	"\x00\x01",
}

func FuzzReconcile(f *testing.F) {
	for _, p := range seeds {
		for _, n := range seeds {
			f.Add(p, n, 0, 0, 0, 0)
		}
		f.Add(p, p+"🙂", len(p), len(p), len(p)+2, len(p)+2)
		f.Add(p, "", 0, len(p), 0, 0)
		f.Add("", p, -1, -1, -1, -1)
	}
	f.Add("aa", "aaa", 1, 1, 2, 2)
	f.Add("🙂", "🙃", 2, 2, 2, 2)
	f.Add("abc", "abd", 3, 0, 1, 99)
	f.Fuzz(func(t *testing.T, prev, next string, pa, pf, na, nf int) {
		check(t,
			State{Text: prev, Anchor: pa, Focus: pf, MarkStart: -1, MarkEnd: -1},
			State{Text: next, Anchor: na, Focus: nf, MarkStart: -1, MarkEnd: -1})
	})
}

func FuzzUTF16(f *testing.F) {
	for _, s := range seeds {
		f.Add(s, 0, 0)
		f.Add(s, 1, 1)
		f.Add(s, 3, 2)
		f.Add(s, 99, 99)
		f.Add(s, -1, -1)
	}
	f.Fuzz(func(t *testing.T, s string, off, n int) {
		l := Len(s)
		if utf8.ValidString(s) {
			if want := len(utf16.Encode([]rune(s))); l != want {
				t.Fatalf("Len(%q) = %d, encoder says %d", s, l, want)
			}
		}
		if len(units(s)) != l {
			t.Fatalf("units(%q) has %d symbols, Len says %d", s, len(units(s)), l)
		}
		// Offsets map to a byte offset at a rune boundary, within one unit
		// of where they started, and the mapping is stable.
		b := ToByte(s, off)
		if b < 0 || b > len(s) {
			t.Fatalf("ToByte(%q, %d) = %d", s, off, b)
		}
		u := FromByte(s, b)
		clamped := min(max(off, 0), l)
		if u > clamped || clamped-u > 1 || ToByte(s, u) != b {
			t.Fatalf("%q: offset %d -> byte %d -> %d", s, off, b, u)
		}
		// Line and column.
		line, col := LineCol(s, off)
		if line < 0 || col < 0 {
			t.Fatalf("LineCol(%q, %d) = %d, %d", s, off, line, col)
		}
		if got := Offset(s, line, col); got != u {
			t.Fatalf("%q: LineCol(%d) = (%d, %d), Offset gives %d, want %d", s, off, line, col, got, u)
		}
		// Offset never panics, whatever the line and column.
		if got := Offset(s, off, n); got < 0 || got > l {
			t.Fatalf("Offset(%q, %d, %d) = %d outside [0, %d]", s, off, n, got, l)
		}
		// Rune offsets.
		r := ToRune(s, off)
		if r < 0 || r > utf8.RuneCountInString(s) || FromRune(s, r) != u {
			t.Fatalf("%q: offset %d -> rune %d -> %d, want %d", s, off, r, FromRune(s, r), u)
		}
		if got := FromRune(s, n); got < 0 || got > l {
			t.Fatalf("FromRune(%q, %d) = %d outside [0, %d]", s, n, got, l)
		}
		// Apply never panics and replacing a range by itself is the identity.
		e := Edit{Start: off, End: n, Text: ""}
		_ = e.Apply(s)
		lo, hi := min(off, n), max(off, n)
		same := Edit{Start: lo, End: hi, Text: s[ToByte(s, lo):ToByte(s, hi)]}
		if got := same.Apply(s); got != s {
			t.Fatalf("%q: replacing [%d,%d) by itself gave %q", s, lo, hi, got)
		}
	})
}
