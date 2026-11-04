package textinput

import (
	"strings"
	"unicode/utf8"
)

// width returns the number of UTF-16 units of the rune r that
// utf8.DecodeRuneInString returned: 2 outside the BMP, else 1. An invalid
// byte decodes as U+FFFD with size 1 and so counts as one unit.
func width(r rune) int {
	if r >= 0x10000 {
		return 2
	}
	return 1
}

// Len returns the number of UTF-16 code units of s.
func Len(s string) int {
	n := 0
	for i := 0; i < len(s); {
		if s[i] < utf8.RuneSelf {
			n++
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		n += width(r)
		i += size
	}
	return n
}

// ToByte converts the UTF-16 offset off into a byte offset in s. off is
// clamped to [0, Len(s)], and an offset inside a surrogate pair rounds down
// to the start of the rune.
func ToByte(s string, off int) int {
	b, _ := locate(s, off)
	return b
}

// locate is ToByte that also reports whether off falls inside a surrogate
// pair, in which case the byte offset is that of the pair's start.
func locate(s string, off int) (b int, mid bool) {
	u := 0
	for i := 0; i < len(s); {
		if u >= off {
			return i, false
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		w := width(r)
		if u+w > off {
			return i, true
		}
		u += w
		i += size
	}
	return len(s), false
}

// chunk returns the byte length of the rune, or of the one invalid byte,
// that starts s[i:]. It is the step by which every function here walks.
func chunk(s string, i int) int {
	if s[i] < utf8.RuneSelf {
		return 1
	}
	_, size := utf8.DecodeRuneInString(s[i:])
	return size
}

// FromByte converts the byte offset b in s into a UTF-16 offset. b is
// clamped to [0, len(s)], and only the runes that end at or before b count.
func FromByte(s string, b int) int {
	if b > len(s) {
		b = len(s)
	}
	u := 0
	for i := 0; i < b; {
		r, size := utf8.DecodeRuneInString(s[i:])
		if i+size > b {
			break
		}
		u += width(r)
		i += size
	}
	return u
}

// LineCol converts the UTF-16 offset off into a line (counted from 0, "\n"
// ends a line) and a column counted in runes from the start of that line.
// An offset inside a surrogate pair rounds down to the start of the rune.
func LineCol(s string, off int) (line, col int) {
	b := ToByte(s, off)
	head := s[:b]
	line = strings.Count(head, "\n")
	col = utf8.RuneCountInString(head[strings.LastIndexByte(head, '\n')+1:])
	return line, col
}

// Offset is the inverse of [LineCol]: it returns the UTF-16 offset of the
// rune column col of line line. A line past the end gives Len(s), and a
// column past the end of its line gives the offset of that line's end, just
// before its "\n". Negative arguments count as 0.
func Offset(s string, line, col int) int {
	i := 0
	for n := 0; n < line; n++ {
		j := strings.IndexByte(s[i:], '\n')
		if j < 0 {
			return Len(s)
		}
		i += j + 1
	}
	for c := 0; c < col && i < len(s) && s[i] != '\n'; c++ {
		_, size := utf8.DecodeRuneInString(s[i:])
		i += size
	}
	return FromByte(s, i)
}

// ToRune converts the UTF-16 offset off into a rune offset, which is what
// AT-SPI counts as a character offset. An offset inside a surrogate pair
// rounds down to the rune.
func ToRune(s string, off int) int {
	return utf8.RuneCountInString(s[:ToByte(s, off)])
}

// FromRune is the inverse of [ToRune]: it converts the rune offset r into a
// UTF-16 offset. r is clamped to [0, runes in s].
func FromRune(s string, r int) int {
	u := 0
	for i := 0; i < len(s) && r > 0; r-- {
		c, size := utf8.DecodeRuneInString(s[i:])
		u += width(c)
		i += size
	}
	return u
}
