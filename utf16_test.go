package textinput

import (
	"testing"
	"unicode/utf16"
)

// Texts the helpers must survive. Each is checked against the standard
// library's UTF-16 encoder where it is valid UTF-8.
var texts = []string{
	"",
	"a",
	"hello\nworld",
	"a\r\nb\r\n",
	"\n\n",
	"é",              // 2 bytes, 1 unit
	"世界",             // 3 bytes, 1 unit
	"🙂",              // 4 bytes, 2 units
	"é",             // combining mark: 2 runes
	"👨‍👩‍👧",          // ZWJ sequence: 5 runes, 8 units
	"a🙂b\n🙂\nc",      // astral runes across lines
	"\xff",           // invalid byte
	"a\xe2\x82",      // truncated sequence
	"\xed\xa0\x80",   // encoded lone surrogate: 3 invalid bytes
	"x\xf0\x9f\x99y", // truncated astral
	"�",              // a real U+FFFD
}

func TestLen(t *testing.T) {
	for _, s := range texts {
		want := len(utf16.Encode([]rune(s)))
		if got := Len(s); got != want {
			t.Errorf("Len(%q) = %d, want %d", s, got, want)
		}
		if got := len(units(s)); got != Len(s) {
			t.Errorf("len(units(%q)) = %d, Len = %d", s, got, Len(s))
		}
	}
	if got := Len("🙂"); got != 2 {
		t.Errorf("Len(emoji) = %d, want 2", got)
	}
}

func TestToByteFromByte(t *testing.T) {
	cases := []struct {
		s      string
		off, b int
	}{
		{"", 0, 0}, {"", 5, 0}, {"", -3, 0},
		{"abc", 0, 0}, {"abc", 2, 2}, {"abc", 3, 3}, {"abc", 9, 3}, {"abc", -1, 0},
		{"é!", 1, 2}, {"世界", 1, 3},
		{"a🙂b", 1, 1}, {"a🙂b", 2, 1}, // inside the pair: rounds down
		{"a🙂b", 3, 5}, {"a🙂b", 4, 6},
		{"\xffa", 1, 1}, {"\xed\xa0\x80z", 3, 3},
	}
	for _, c := range cases {
		if got := ToByte(c.s, c.off); got != c.b {
			t.Errorf("ToByte(%q, %d) = %d, want %d", c.s, c.off, got, c.b)
		}
	}
	back := []struct {
		s    string
		b    int
		want int
	}{
		{"", 0, 0}, {"abc", 2, 2}, {"abc", 99, 3},
		{"é", 1, 0}, // inside the rune: it does not count yet
		{"é", 2, 1},
		{"a🙂b", 5, 3}, {"a🙂b", 3, 1},
		{"\xff", 1, 1},
	}
	for _, c := range back {
		if got := FromByte(c.s, c.b); got != c.want {
			t.Errorf("FromByte(%q, %d) = %d, want %d", c.s, c.b, got, c.want)
		}
	}
}

func TestByteRoundTrip(t *testing.T) {
	for _, s := range texts {
		for off := 0; off <= Len(s); off++ {
			b := ToByte(s, off)
			u := FromByte(s, b)
			if u > off || off-u > 1 {
				t.Errorf("%q: offset %d -> byte %d -> %d", s, off, b, u)
			}
			if ToByte(s, u) != b {
				t.Errorf("%q: offset %d is not stable", s, off)
			}
		}
	}
}

func TestLineColOffset(t *testing.T) {
	cases := []struct {
		s         string
		off       int
		line, col int
	}{
		{"", 0, 0, 0},
		{"ab\ncd", 0, 0, 0}, {"ab\ncd", 2, 0, 2}, {"ab\ncd", 3, 1, 0}, {"ab\ncd", 5, 1, 2},
		{"ab\ncd", 99, 1, 2},
		{"a🙂b", 3, 0, 2}, {"a🙂b", 2, 0, 1}, // inside the pair
		{"🙂\n🙂x", 5, 1, 1},
		{"é\n", 2, 0, 2}, {"é\n", 3, 1, 0},
		{"a\r\nb", 3, 1, 0}, {"a\r\nb", 1, 0, 1},
		{"\n\n", 1, 1, 0}, {"\n\n", 2, 2, 0},
	}
	for _, c := range cases {
		l, col := LineCol(c.s, c.off)
		if l != c.line || col != c.col {
			t.Errorf("LineCol(%q, %d) = (%d, %d), want (%d, %d)", c.s, c.off, l, col, c.line, c.col)
		}
	}
	for _, s := range texts {
		for off := 0; off <= Len(s); off++ {
			l, col := LineCol(s, off)
			got := Offset(s, l, col)
			if got > off || off-got > 1 {
				t.Errorf("%q: %d -> (%d,%d) -> %d", s, off, l, col, got)
			}
			if l2, c2 := LineCol(s, got); l2 != l || c2 != col {
				t.Errorf("%q: (%d,%d) not stable: (%d,%d)", s, l, col, l2, c2)
			}
		}
	}
}

func TestOffsetClamps(t *testing.T) {
	s := "ab\ncd🙂\nef"
	cases := []struct {
		line, col, want int
	}{
		{0, 0, 0}, {0, 1, 1}, {0, 2, 2},
		{0, 99, 2}, // past the end of the line: before its "\n"
		{1, 3, 7},  // after the emoji
		{1, 99, 7},
		{2, 1, 9}, {2, 99, 10},
		{3, 0, 10}, {99, 99, 10}, // past the last line
		{-1, 0, 0}, {0, -4, 0}, {-1, -1, 0},
	}
	for _, c := range cases {
		if got := Offset(s, c.line, c.col); got != c.want {
			t.Errorf("Offset(%d, %d) = %d, want %d", c.line, c.col, got, c.want)
		}
	}
	if got := Offset("", 0, 0); got != 0 {
		t.Errorf("Offset on empty text = %d", got)
	}
}

func TestRuneOffsets(t *testing.T) {
	cases := []struct {
		s         string
		off, rune int
	}{
		{"", 0, 0}, {"abc", 2, 2}, {"abc", 9, 3},
		{"a🙂b", 1, 1}, {"a🙂b", 2, 1}, {"a🙂b", 3, 2}, {"a🙂b", 4, 3},
		{"é", 2, 2},
		{"\xff\xfe", 2, 2},
	}
	for _, c := range cases {
		if got := ToRune(c.s, c.off); got != c.rune {
			t.Errorf("ToRune(%q, %d) = %d, want %d", c.s, c.off, got, c.rune)
		}
	}
	back := []struct {
		s         string
		rune, off int
	}{
		{"", 0, 0}, {"", 3, 0}, {"abc", 2, 2}, {"abc", -1, 0}, {"abc", 99, 3},
		{"a🙂b", 2, 3}, {"a🙂b", 3, 4}, {"\xff\xfe", 2, 2},
	}
	for _, c := range back {
		if got := FromRune(c.s, c.rune); got != c.off {
			t.Errorf("FromRune(%q, %d) = %d, want %d", c.s, c.rune, got, c.off)
		}
	}
}

func TestUnitsMatchEncoder(t *testing.T) {
	for _, s := range []string{"", "a🙂é世", "👨‍👩‍👧"} {
		var want []uint32
		for _, u := range utf16.Encode([]rune(s)) {
			want = append(want, uint32(u))
		}
		got := units(s)
		if len(got) != len(want) {
			t.Fatalf("units(%q) = %v, want %v", s, got, want)
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("units(%q)[%d] = %x, want %x", s, i, got[i], want[i])
			}
		}
	}
	// An invalid byte never equals a real U+FFFD.
	if a, b := units("\xff"), units("�"); a[0] == b[0] {
		t.Errorf("invalid byte and U+FFFD share a symbol: %x", a[0])
	}
}
