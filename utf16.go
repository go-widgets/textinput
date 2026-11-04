package textinput

import (
	"strings"
	"unicode/utf16"
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
	u := 0
	for i := 0; i < len(s); {
		if u >= off {
			return i
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		w := width(r)
		if u+w > off {
			return i
		}
		u += w
		i += size
	}
	return len(s)
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

// invalidBase is added to an invalid byte to make its symbol in [units].
const invalidBase = 0x10000

// units returns one symbol per UTF-16 unit of s, counting like [Len]. A BMP
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
