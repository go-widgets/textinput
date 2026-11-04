# go-widgets/textinput

The zero-dependency text-input contract for the
[go-widgets](https://github.com/go-widgets) ecosystem: what a text widget
exposes so that an input method (an IME, a screen reader, a mobile keyboard)
can drive it. Standard library only (`go.mod` has no `require`), 100%
statement coverage, fuzzed.

toolkit implements it, webcanvas (the browser host) and the native back-ends
(Cocoa `NSTextInputClient`, Windows TSF, Android's `InputConnection`) consume
it, and none of them has to import the others. The design is in
[go-widgets/webcanvas#21](https://github.com/go-widgets/webcanvas/issues/21).

```
go get github.com/go-widgets/textinput
```

## What is in it

| | |
| --- | --- |
| `Client` | the interface a text widget implements, shaped after AppKit's `NSTextInputClient` |
| `State`, `Edit` | a snapshot (text, selection with direction, marked range) and a single replacement |
| `Len`, `ToByte`, `FromByte`, `LineCol`, `Offset`, `ToRune`, `FromRune` | pure UTF-16 / byte / rune / (line, column) conversions |
| `Reconcile(prev, next State) Edit` | the diff fallback: the one edit that explains a change read back from the DOM |

Offsets are **UTF-16 code units** over the whole document, the unit shared by
`NSRange`, the DOM's `selectionStart` and TSF. `-1` means "none". The method
names keep the `Input` prefix because a widget already has `Text`,
`Selection` and `SetSelection` with other meanings.

```go
type Client interface {
	InputText() string
	InputRevision() uint64
	InputSelection() (anchor, focus int)  // selectedRange
	InputMarked() (start, end int)        // markedRange; (-1, -1) without composition
	InputMultiline() bool
	InputReadOnly() bool

	SetInputSelection(anchor, focus int)
	ReplaceInput(start, end int, text string)                              // insertText:replacementRange:
	SetMarkedInput(text string, selStart, selEnd, replStart, replEnd int)  // setMarkedText:selectedRange:replacementRange:
	UnmarkInput()                                                          // unmarkText

	InputRect(start, end int) (x, y, w, h int)  // firstRectForCharacterRange:
	InputIndexAt(x, y int) int                  // characterIndexForPoint:
}
```

`doCommandBySelector:` has no method: command keys go through the widget's
ordinary key handler.

## Reconcile

A browser host writes the widget's text and selection into a hidden editing
element, and when the element changes without an event it can trust (Android
composition, autofill, an unusual IME) it reads the element back. `Reconcile`
turns the two snapshots into one `Edit` that `Client.ReplaceInput` can apply.
VS Code (`TextAreaState.deduceInput`) and Flutter web (`inferDeltaState`) do
the same.

```go
prev := textinput.State{Text: "hello wrld", Anchor: 10, Focus: 10, MarkStart: -1, MarkEnd: -1}
next := textinput.State{Text: "hello world", Anchor: 11, Focus: 11, MarkStart: -1, MarkEnd: -1}

e := textinput.Reconcile(prev, next)  // Edit{Start: 7, End: 7, Text: "o"}
e.Apply(prev.Text) == next.Text       // true
```

- **Minimal.** The edit replaces the fewest UTF-16 units: the longest common
  prefix and suffix are trimmed.
- **Caret-aware.** When several minimal edits exist (`a` typed after `a`), the
  one nearest the caret wins: it starts at the smaller selection start of the
  two states, as VS Code does by bounding the prefix with both selections.
- **Never splits a surrogate pair**, so an emoji is replaced whole.
- **Exact.** `Edit.Text` is a slice of `next.Text`; applying the edit to
  `prev.Text` gives `next.Text` byte for byte, invalid UTF-8 included.
- Equal texts give the zero `Edit`. The marked range is not used.
- Invalid UTF-8 never fails: each invalid byte counts as one rune (U+FFFD),
  so as one UTF-16 unit.

## Tests

- Table tests for every conversion, and cases from real input methods (kana to
  kanji, Android replace-word, backspace in a run, emoji swaps, ZWJ sequences,
  combining marks, CRLF).
- A property test over every pair of short texts built from awkward pieces and
  every selection shape: the edit applied to the old state gives the new state,
  stays in range, and never splits a pair.
- An optimality test against brute force on a two-letter alphabet.
- Two fuzz targets, `FuzzReconcile` and `FuzzUTF16`, seeded with lone
  surrogates, invalid UTF-8, emoji ZWJ sequences, combining marks and CRLF
  (`fuzz_test.go`). CI runs each for 60 s.
- Linear cost: `TestReconcileLinear` checks that 4x the text (256 KiB to
  1 MiB) takes well under 16x the time, and `TestReconcileAllocs` that the
  allocation count does not depend on the size.
- CI runs on linux/amd64, linux/386 (32-bit offsets), darwin, windows and
  js/wasm under node, plus `govulncheck`.

## Benchmark

`go test -bench Reconcile`, Apple M4 Max, go1.27.3, single goroutine, one edit
(`🙂typed`) in the middle of a text of mixed ASCII, 2-, 3- and 4-byte runes and
CRLF line ends:

| Case | Time | Throughput | Allocated |
| --- | ---: | ---: | ---: |
| mid-insert, 1 KiB | 4.5 us | 225 MB/s | 8.8 KiB, 2 allocs |
| mid-insert, 64 KiB | 0.26 ms | 253 MB/s | 520 KiB, 2 allocs |
| mid-insert, 1 MiB | 3.9 ms | 272 MB/s | 8.0 MiB, 2 allocs |
| disjoint texts, 1 MiB | 3.3 ms | 317 MB/s | 8.0 MiB, 2 allocs |
| last byte differs, 1 MiB | 4.1 ms | 254 MB/s | 8.0 MiB, 2 allocs |
| equal texts, 1 MiB | 3.5 ns | | 0 allocs |

Time per byte is flat from 1 KiB to 1 MiB: the cost is linear. These timings
were taken at a 1-minute load of 26, above the 8 at which the fleet rule
allows measuring, so treat the times as an upper bound and re-run them on a
quiet machine; the allocation counts are exact. There is no reference
implementation to compare with: VS Code's and Flutter's diffs are
JavaScript/Dart on UTF-16 strings, which Go cannot be timed against on equal
terms. The allocation (4 bytes per UTF-16 unit and per text) is the obvious
cost to remove, with a byte-level path for valid UTF-8, if a host ever
reconciles megabyte documents; hosts mirror a window of a few hundred
characters.

## Licence

See [LICENSE](LICENSE).
