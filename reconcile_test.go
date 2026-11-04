package textinput

import (
	"strings"
	"testing"
	"time"
)

// check asserts the contract of Reconcile for one pair: the Edit applied to
// the old text gives the new text, its range lies in the old text without
// splitting a surrogate pair, and no smaller common-affix edit exists.
func check(t testing.TB, prev, next State) Edit {
	t.Helper()
	e := Reconcile(prev, next)
	if got := e.Apply(prev.Text); got != next.Text {
		t.Fatalf("Reconcile(%q, %q) = %+v; Apply gives %q", prev.Text, next.Text, e, got)
	}
	n := Len(prev.Text)
	if e.Start < 0 || e.Start > e.End || e.End > n {
		t.Fatalf("Reconcile(%q, %q) = %+v: range outside [0, %d]", prev.Text, next.Text, e, n)
	}
	for _, off := range []int{e.Start, e.End} {
		if FromByte(prev.Text, ToByte(prev.Text, off)) != off {
			t.Fatalf("Reconcile(%q, %q) = %+v: offset %d splits a pair", prev.Text, next.Text, e, off)
		}
	}
	if (prev.Text == next.Text) != e.IsZero() {
		t.Fatalf("Reconcile(%q, %q) = %+v: zero only for equal texts", prev.Text, next.Text, e)
	}
	return e
}

func caret(text string, at int) State {
	return State{Text: text, Anchor: at, Focus: at, MarkStart: -1, MarkEnd: -1}
}

func TestReconcileCases(t *testing.T) {
	sel := func(text string, a, f int) State {
		return State{Text: text, Anchor: a, Focus: f, MarkStart: -1, MarkEnd: -1}
	}
	none := func(text string) State { return sel(text, -1, -1) }
	cases := []struct {
		name       string
		prev, next State
		want       Edit
	}{
		{"equal", caret("abc", 1), caret("abc", 3), Edit{}},
		{"equal empty", none(""), none(""), Edit{}},
		{"type at end", caret("ab", 2), caret("abc", 3), Edit{2, 2, "c"}},
		{"type in the middle", caret("ac", 1), caret("abc", 2), Edit{1, 1, "b"}},
		{"type at start", caret("bc", 0), caret("abc", 1), Edit{0, 0, "a"}},
		{"type into empty", caret("", 0), caret("x", 1), Edit{0, 0, "x"}},
		{"backspace", caret("abc", 3), caret("ab", 2), Edit{2, 3, ""}},
		{"delete forward", caret("abc", 1), caret("ac", 1), Edit{1, 2, ""}},
		{"clear", caret("abc", 3), caret("", 0), Edit{0, 3, ""}},
		{"replace selection", sel("hello", 0, 5), caret("x", 1), Edit{0, 5, "x"}},
		{"replace inverted selection", sel("hello", 5, 0), caret("x", 1), Edit{0, 5, "x"}},
		{"replace middle", sel("a123b", 1, 4), caret("aXb", 2), Edit{1, 4, "X"}},
		{"smaller than the selection", sel("abcd", 0, 4), caret("abXd", 3), Edit{2, 3, "X"}},
		// "a" typed after "a": the new one is the one at the caret.
		{"double letter after", caret("a", 1), caret("aa", 2), Edit{1, 1, "a"}},
		{"double letter before", caret("a", 0), caret("aa", 1), Edit{0, 0, "a"}},
		{"double letter no selection", none("a"), none("aa"), Edit{1, 1, "a"}},
		{"double letter next selection only", none("a"), caret("aa", 1), Edit{1, 1, "a"}},
		{"double letter next selection at 0", none("a"), caret("aa", 0), Edit{0, 0, "a"}},
		{"prev selection only", caret("a", 0), none("aa"), Edit{0, 0, "a"}},
		{"backspace in a run", caret("aaa", 2), caret("aa", 1), Edit{1, 2, ""}},
		{"delete in a run", caret("aaa", 0), caret("aa", 0), Edit{0, 1, ""}},
		{"android replace word", caret("hello wrld", 10), caret("hello world", 11), Edit{7, 7, "o"}},
		{"ime kana to kanji", caret("かんじ", 3), caret("漢字", 2), Edit{0, 3, "漢字"}},
		{"selection moved too", caret("abcd", 4), caret("abd", 0), Edit{2, 3, ""}},
		// Surrogate pairs are never cut in two.
		{"type emoji", caret("ab", 1), caret("a🙂b", 3), Edit{1, 1, "🙂"}},
		{"delete emoji", caret("a🙂b", 3), caret("ab", 1), Edit{1, 3, ""}},
		{"swap emoji, shared high surrogate", caret("🙂", 2), caret("🙃", 2), Edit{0, 2, "🙃"}},
		{"swap emoji, shared low surrogate", caret("\U0001F642", 2), caret("\U0001F742", 2), Edit{0, 2, "\U0001F742"}},
		{"emoji in a run", caret("🙂🙂", 4), caret("🙂", 2), Edit{2, 4, ""}},
		{"zwj sequence loses a member", caret("👨‍👩‍👧", 8), caret("👨‍👩", 5), Edit{5, 8, ""}},
		// Combining marks are separate runes, so they diff on their own.
		{"combining mark added", caret("e", 1), caret("é", 2), Edit{1, 1, "́"}},
		{"combining mark removed", caret("é", 2), caret("e", 1), Edit{1, 2, ""}},
		// Line ends.
		{"crlf break typed", caret("a", 1), caret("a\r\n", 3), Edit{1, 1, "\r\n"}},
		{"crlf to lf", caret("a\r\nb", 4), caret("a\nb", 3), Edit{1, 2, ""}},
		{"line break typed", caret("ab", 1), caret("a\nb", 2), Edit{1, 1, "\n"}},
		// Invalid UTF-8 is kept byte for byte.
		{"invalid byte typed", caret("ab", 1), caret("a\xffb", 2), Edit{1, 1, "\xff"}},
		{"invalid vs replacement char", caret("a\xff", 2), caret("a�", 2), Edit{1, 2, "�"}},
		{"selection beyond the text", sel("ab", 9, 9), caret("abc", 3), Edit{2, 2, "c"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := check(t, c.prev, c.next)
			if got != c.want {
				t.Errorf("Reconcile = %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestReconcileIgnoresMarks(t *testing.T) {
	a := State{Text: "ab", Anchor: 2, Focus: 2, MarkStart: 0, MarkEnd: 2}
	b := State{Text: "abc", Anchor: 3, Focus: 3, MarkStart: 1, MarkEnd: 3}
	c := State{Text: "abc", Anchor: 3, Focus: 3, MarkStart: -1, MarkEnd: -1}
	if Reconcile(a, b) != Reconcile(a, c) {
		t.Error("marks changed the edit")
	}
}

// Every pair of short texts built from awkward pieces, with every selection
// shape: the property holds for all of them.
func TestReconcileExhaustive(t *testing.T) {
	pieces := []string{"a", "b", "🙂", "\xff", "é", "\n"}
	all := []string{""}
	level := []string{""}
	for d := 0; d < 3; d++ {
		var nxt []string
		for _, s := range level {
			for _, p := range pieces {
				nxt = append(nxt, s+p)
			}
		}
		all = append(all, nxt...)
		level = nxt
	}
	sels := func(text string) []State {
		n := Len(text)
		return []State{
			{text, -1, -1, -1, -1},
			{text, 0, 0, -1, -1},
			{text, n, n, -1, -1},
			{text, n / 2, n, -1, -1},
			{text, n, n / 2, -1, -1},
		}
	}
	for _, p := range all {
		ps := sels(p)
		for _, n := range all {
			for _, a := range ps {
				for _, b := range sels(n) {
					check(t, a, b)
				}
			}
		}
	}
}

// Of all edits that turn prev into next on a small alphabet, Reconcile's is
// as small as any (size = replaced units + inserted units).
func TestReconcileMinimal(t *testing.T) {
	alpha := []string{"a", "b"}
	var all []string
	level := []string{""}
	all = append(all, level...)
	for d := 0; d < 5; d++ {
		var nxt []string
		for _, s := range level {
			for _, p := range alpha {
				nxt = append(nxt, s+p)
			}
		}
		all = append(all, nxt...)
		level = nxt
	}
	for _, p := range all {
		for _, n := range all {
			e := Reconcile(caret(p, 0), caret(n, 0))
			got := (e.End - e.Start) + Len(e.Text)
			best := len(p) + len(n)
			for i := 0; i <= len(p); i++ {
				for j := i; j <= len(p); j++ {
					for k := 0; k <= len(n); k++ {
						for l := k; l <= len(n); l++ {
							if p[:i]+n[k:l]+p[j:] == n {
								best = min(best, (j-i)+(l-k))
							}
						}
					}
				}
			}
			if got != best {
				t.Fatalf("Reconcile(%q, %q) = %+v: size %d, best %d", p, n, e, got, best)
			}
		}
	}
}

func TestCaretTarget(t *testing.T) {
	if _, ok := caretTarget(State{Anchor: -1, Focus: 0}, State{Anchor: 0, Focus: -1}); ok {
		t.Error("a half-negative selection counts as none")
	}
	if v, ok := caretTarget(State{Anchor: 7, Focus: 3}, State{Anchor: 5, Focus: 9}); !ok || v != 3 {
		t.Errorf("caretTarget = %d, %v", v, ok)
	}
	if v, ok := caretTarget(State{Anchor: 2, Focus: 2}, State{Anchor: 5, Focus: 9}); !ok || v != 2 {
		t.Errorf("caretTarget = %d, %v", v, ok)
	}
}

// big returns a text of n bytes made of lines of mixed content, so every
// decoding path (ASCII, 2-, 3- and 4-byte runes, invalid bytes) is on the
// hot path.
func big(n int) string {
	const unit = "func main() { fmt.Println(\"héllo 世界 🙂\\xff\") }\r\n"
	return strings.Repeat(unit, n/len(unit)+1)[:n]
}

// bigPair edits the middle of a text: the common prefix and suffix are each
// about half of it, the worst case for the trimming loops.
func bigPair(n int) (State, State) {
	s := big(n)
	mid := ToByte(s, Len(s)/2)
	prev := caret(s, Len(s)/2)
	next := caret(s[:mid]+"🙂typed"+s[mid:], Len(s)/2+8)
	return prev, next
}

// A text four times longer must not take much more than four times as long:
// a quadratic Reconcile would take sixteen times. The best of several runs
// is compared, with a wide margin for noisy machines.
func TestReconcileLinear(t *testing.T) {
	if testing.Short() {
		t.Skip("timing test")
	}
	best := func(n int) time.Duration {
		prev, next := bigPair(n)
		d := time.Duration(1 << 62)
		for i := 0; i < 7; i++ {
			t0 := time.Now()
			Reconcile(prev, next)
			d = min(d, time.Since(t0))
		}
		return d
	}
	small, large := best(256<<10), best(1<<20)
	ratio := float64(large) / float64(small)
	t.Logf("256 KiB: %v, 1 MiB: %v, ratio %.1f (linear = 4, quadratic = 16)", small, large, ratio)
	if ratio > 10 {
		t.Errorf("4x the text took %.1fx the time: not linear", ratio)
	}
}

// The memory used is also linear: a fixed number of allocations whatever the
// size, each proportional to it.
func TestReconcileAllocs(t *testing.T) {
	for _, n := range []int{1 << 10, 1 << 20} {
		prev, next := bigPair(n)
		allocs := testing.AllocsPerRun(5, func() { Reconcile(prev, next) })
		if allocs > 2 {
			t.Errorf("%d bytes: %v allocations, want at most 2", n, allocs)
		}
	}
}

func TestReconcileBigIsCorrect(t *testing.T) {
	prev, next := bigPair(1 << 20)
	e := check(t, prev, next)
	if e.Start != e.End || e.Text != "🙂typed" {
		t.Errorf("edit = %+v", e)
	}
}

func BenchmarkReconcile(b *testing.B) {
	for _, c := range []struct {
		name string
		n    int
	}{{"1KiB", 1 << 10}, {"64KiB", 64 << 10}, {"1MiB", 1 << 20}} {
		prev, next := bigPair(c.n)
		b.Run("mid-insert/"+c.name, func(b *testing.B) {
			b.SetBytes(int64(len(prev.Text)))
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				Reconcile(prev, next)
			}
		})
	}
	// Equal texts: the string comparison alone.
	s := caret(big(1<<20), 0)
	b.Run("equal/1MiB", func(b *testing.B) {
		b.SetBytes(int64(len(s.Text)))
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			Reconcile(s, s)
		}
	})
	// Nothing in common: both trims stop at once, but a whole text is replaced.
	x, y := caret(strings.Repeat("a", 1<<20), 0), caret(strings.Repeat("b", 1<<20), 0)
	b.Run("disjoint/1MiB", func(b *testing.B) {
		b.SetBytes(int64(len(x.Text)))
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			Reconcile(x, y)
		}
	})
	// One byte differs at the very end: the prefix loop runs the whole text.
	z := caret(strings.Repeat("a", 1<<20-1)+"b", 0)
	b.Run("last-byte/1MiB", func(b *testing.B) {
		b.SetBytes(int64(len(x.Text)))
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			Reconcile(x, z)
		}
	})
}
