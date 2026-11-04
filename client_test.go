package textinput

import "testing"

// buffer is a minimal Client: it proves the interface can be implemented
// with builtin types alone, and that StateOf reads every accessor.
type buffer struct {
	text          string
	anchor, focus int
	ms, me        int
	rev           uint64
}

func (b *buffer) InputText() string                   { return b.text }
func (b *buffer) InputRevision() uint64               { return b.rev }
func (b *buffer) InputSelection() (int, int)          { return b.anchor, b.focus }
func (b *buffer) InputMarked() (int, int)             { return b.ms, b.me }
func (b *buffer) InputMultiline() bool                { return true }
func (b *buffer) InputReadOnly() bool                 { return false }
func (b *buffer) SetInputSelection(a, f int)          { b.anchor, b.focus = a, f }
func (b *buffer) UnmarkInput()                        { b.ms, b.me = -1, -1 }
func (b *buffer) InputRect(s, e int) (x, y, w, h int) { return s, 0, e - s, 1 }
func (b *buffer) InputIndexAt(x, y int) int           { return x }
func (b *buffer) ReplaceInput(s, e int, t string) {
	if s < 0 {
		s, e = b.anchor, b.focus
	}
	b.text = Edit{Start: s, End: e, Text: t}.Apply(b.text)
	b.rev++
}
func (b *buffer) SetMarkedInput(t string, ss, se, rs, re int) {
	b.ReplaceInput(rs, re, t)
}

var _ Client = (*buffer)(nil)

func TestStateOf(t *testing.T) {
	b := &buffer{text: "héllo", anchor: 3, focus: 1, ms: -1, me: -1}
	got := StateOf(b)
	want := State{Text: "héllo", Anchor: 3, Focus: 1, MarkStart: -1, MarkEnd: -1}
	if got != want {
		t.Errorf("StateOf = %+v, want %+v", got, want)
	}
	b.SetInputSelection(2, 2)
	b.ReplaceInput(-1, -1, "XY")
	if b.text != "héXYllo" {
		t.Errorf("text = %q", b.text)
	}
	// The interface is exercised end to end: a host reads the state back and
	// reconciles it with an earlier one.
	e := Reconcile(got, StateOf(b))
	if e.Apply(got.Text) != b.text {
		t.Errorf("Reconcile/Apply disagree with the client: %+v", e)
	}
	b.UnmarkInput()
	b.SetMarkedInput("z", 0, 1, 0, 0)
	if !b.InputMultiline() || b.InputReadOnly() || b.InputRevision() == 0 {
		t.Error("accessors")
	}
	if x, _, w, _ := b.InputRect(2, 5); x != 2 || w != 3 || b.InputIndexAt(4, 0) != 4 {
		t.Error("geometry")
	}
}

func TestEdit(t *testing.T) {
	if !(Edit{}).IsZero() || (Edit{Text: "x"}).IsZero() || (Edit{Start: 1, End: 1}).IsZero() {
		t.Error("IsZero")
	}
	cases := []struct {
		text string
		e    Edit
		want string
	}{
		{"hello", Edit{}, "hello"},
		{"hello", Edit{1, 3, "ipp"}, "hipplo"},
		{"hello", Edit{3, 1, "-"}, "h-lo"}, // reversed range
		{"hello", Edit{5, 99, "!"}, "hello!"},
		{"hello", Edit{-4, 2, ">"}, ">llo"},
		{"a🙂b", Edit{1, 3, ""}, "ab"},
		{"a🙂b", Edit{1, 1, "é"}, "aé🙂b"},
		{"", Edit{0, 0, "x"}, "x"},
	}
	for _, c := range cases {
		if got := c.e.Apply(c.text); got != c.want {
			t.Errorf("%+v.Apply(%q) = %q, want %q", c.e, c.text, got, c.want)
		}
	}
}
