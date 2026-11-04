package textinput

// Client is a text widget as an input method sees it. Offsets count UTF-16
// code units over InputText(): NSTextInputClient's NSRange, the DOM's
// selectionStart and TSF's ACP all count this way. -1 means "none".
//
// The names keep the Input prefix because a widget already has methods such
// as Text, Selection and SetSelection with other meanings. The comment on
// each method gives the NSTextInputClient selector it corresponds to.
// doCommandBySelector: has no counterpart: command keys go through the
// widget's ordinary key handler, and attributedSubstringForProposedRange: is
// derived from InputText.
type Client interface {
	// InputText returns the whole document, with "\n" line ends.
	InputText() string
	// InputRevision changes on every edit, so a host can skip rewriting its
	// editing element when nothing changed.
	InputRevision() uint64
	// InputSelection returns the selection, direction kept: anchor == focus
	// is a caret. [selectedRange]
	InputSelection() (anchor, focus int)
	// InputMarked returns the marked (composition) range, or (-1, -1)
	// without composition. [markedRange, hasMarkedText]
	InputMarked() (start, end int)
	// InputMultiline reports whether Enter inserts a line break.
	InputMultiline() bool
	// InputReadOnly reports whether the widget refuses edits.
	InputReadOnly() bool

	// SetInputSelection moves the selection.
	SetInputSelection(anchor, focus int)
	// ReplaceInput replaces [start, end) with text, as a committed, undoable
	// edit, and ends any composition. (-1, -1) means the current marked
	// range, or the selection when there is none: committing a composition
	// replaces its preview. [insertText:replacementRange:]
	ReplaceInput(start, end int, text string)
	// SetMarkedInput replaces [replStart, replEnd) with text and marks it as
	// the composition preview; (-1, -1) means the current marked range, or
	// the selection when there is none. selStart and selEnd are the
	// selection inside text, relative to its start.
	// [setMarkedText:selectedRange:replacementRange:]
	SetMarkedInput(text string, selStart, selEnd, replStart, replEnd int)
	// UnmarkInput commits the marked text as typed; it does nothing without
	// marked text. [unmarkText]
	UnmarkInput()

	// InputRect returns the surface pixels of the first line of [start, end).
	// [firstRectForCharacterRange:actualRange:]
	InputRect(start, end int) (x, y, w, h int)
	// InputIndexAt returns the offset nearest to the surface point (x, y).
	// [characterIndexForPoint:]
	InputIndexAt(x, y int) int
}

// State is a snapshot of a text widget: what a host last wrote to its
// editing element, or what it read back. Offsets are UTF-16 code units.
//
// Anchor and Focus are the selection with its direction (Anchor == Focus is
// a caret); a negative value means "no selection". MarkStart and MarkEnd are
// the marked range, (-1, -1) without composition.
type State struct {
	Text               string
	Anchor, Focus      int
	MarkStart, MarkEnd int
}

// StateOf snapshots c.
func StateOf(c Client) State {
	a, f := c.InputSelection()
	ms, me := c.InputMarked()
	return State{Text: c.InputText(), Anchor: a, Focus: f, MarkStart: ms, MarkEnd: me}
}

// Edit replaces the UTF-16 range [Start, End) of a text with Text. It is
// what [Reconcile] deduces and what Client.ReplaceInput takes. The zero Edit
// changes nothing.
type Edit struct {
	Start, End int
	Text       string
}

// IsZero reports whether e is the zero Edit, which changes nothing.
func (e Edit) IsZero() bool { return e == Edit{} }

// Apply returns text with e applied. Offsets are clamped to the text, and a
// reversed range is put in order.
func (e Edit) Apply(text string) string {
	s, t := e.Start, e.End
	if t < s {
		s, t = t, s
	}
	bs := ToByte(text, s)
	bt := ToByte(text, t)
	return text[:bs] + e.Text + text[bt:]
}
