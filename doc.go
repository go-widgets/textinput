// Package textinput is the contract between a text widget and an input
// method (an IME, a screen reader, a mobile keyboard), with no dependency
// beyond the standard library.
//
// It holds three things:
//
//   - [Client], the interface a text widget implements. It has the shape of
//     AppKit's NSTextInputClient (insertText:replacementRange:,
//     setMarkedText:selectedRange:replacementRange:, unmarkText,
//     selectedRange, markedRange, firstRectForCharacterRange:actualRange:,
//     characterIndexForPoint:), uses builtin types only, and counts offsets
//     in UTF-16 code units over the whole document. NSRange, the DOM's
//     selectionStart and Windows TSF all count that way.
//   - The pure UTF-16 helpers [Len], [ToByte], [FromByte], [LineCol],
//     [Offset], [ToRune] and [FromRune], which convert between that unit,
//     Go's UTF-8 byte offsets, rune offsets (AT-SPI counts characters) and
//     (line, rune column) pairs.
//   - [Reconcile], the diff fallback: given the text and selection last
//     written to a host's editing element and the ones read back from it,
//     it deduces the single [Edit] that explains the change. VS Code
//     (TextAreaState.deduceInput) and Flutter web (inferDeltaState) do the
//     same when an event's data cannot be trusted, as on Android.
//
// # Text model
//
// A document is a Go string, with "\n" as its only line end ("\r" is an
// ordinary character, so CRLF text keeps its "\r" at the end of each line).
// Invalid UTF-8 never makes a function fail: every invalid byte counts as
// one rune, U+FFFD, and so as one UTF-16 unit, and slicing keeps the
// original bytes. Offsets are clamped to the document; an offset that falls
// inside a surrogate pair (a rune outside the BMP counts for two units)
// rounds down to the start of that rune.
//
// # Cost
//
// Every function is linear in the length of its text arguments and never
// allocates more than a few bytes per input byte.
package textinput
