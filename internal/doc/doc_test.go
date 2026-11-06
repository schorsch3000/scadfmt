package doc

import "testing"

func p(d Doc, width int) string {
	return Print(d, Options{MaxWidth: width, Indent: 2})
}

func call(args ...string) Doc {
	var ds []Doc
	for _, a := range args {
		ds = append(ds, Text(a))
	}
	return NewGroup(
		Text("f("),
		Nest(Concat(Softline, Join(Concat(Text(","), LineBr), ds))),
		Softline,
		Text(")"),
	)
}

func TestGroupFits(t *testing.T) {
	if got := p(call("a", "b"), 80); got != "f(a, b)" {
		t.Errorf("got %q", got)
	}
}

func TestGroupBreaks(t *testing.T) {
	want := "f(\n  aaaa,\n  bbbb\n)"
	if got := p(call("aaaa", "bbbb"), 8); got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestExactWidth(t *testing.T) {
	if got := p(call("a", "b"), 7); got != "f(a, b)" {
		t.Errorf("width == length must fit, got %q", got)
	}
	if got := p(call("a", "b"), 6); got == "f(a, b)" {
		t.Errorf("must break at width 6")
	}
}

func TestHardlineBreaksParents(t *testing.T) {
	d := NewGroup(Text("{"), Nest(Concat(LineBr, Text("a;"), Hardline, Text("b;"))), LineBr, Text("}"))
	want := "{\n  a;\n  b;\n}"
	if got := p(d, 80); got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestInnerGroupStaysFlat(t *testing.T) {
	inner := call("x", "y")
	d := NewGroup(Text("{"), Nest(Concat(LineBr, inner, Hardline, Text("z;"))), LineBr, Text("}"))
	want := "{\n  f(x, y)\n  z;\n}"
	if got := p(d, 80); got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestIfBreakTrailingComma(t *testing.T) {
	mk := func() Doc {
		return NewGroup(Text("["), Nest(Concat(Softline, Text("a,"), LineBr, Text("b"), IfBreak{Broken: Text(",")})), Softline, Text("]"))
	}
	if got := p(mk(), 80); got != "[a, b]" {
		t.Errorf("flat: %q", got)
	}
	if got := p(mk(), 4); got != "[\n  a,\n  b,\n]" {
		t.Errorf("broken: %q", got)
	}
}

func TestLineSuffix(t *testing.T) {
	d := Concat(Text("a;"), LineSuffix{Text(" // c")}, Hardline, Text("b;"))
	if got := p(d, 80); got != "a; // c\nb;" {
		t.Errorf("got %q", got)
	}
	d = Concat(Text("a;"), LineSuffix{Text(" // end")})
	if got := p(d, 80); got != "a; // end" {
		t.Errorf("at end: %q", got)
	}
}

func TestLineSuffixDoesNotCountForFit(t *testing.T) {
	d := Concat(NewGroup(Text("f("), Softline, Text("a"), Softline, Text(")")), LineSuffix{Text(" // long comment here")})
	if got := p(d, 8); got != "f(a) // long comment here" {
		t.Errorf("got %q", got)
	}
}

func TestBreakParent(t *testing.T) {
	d := NewGroup(Text("("), Nest(Concat(Softline, Text("a"), LineSuffix{Text(" // c")}, BreakParent{})), Softline, Text(")"))
	want := "(\n  a // c\n)"
	if got := p(d, 80); got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestTrailingWhitespaceTrimmed(t *testing.T) {
	d := Concat(Text("a "), Hardline, Nest(Concat(Hardline, Text("b"))))
	if got := p(d, 80); got != "a\n\n  b" {
		t.Errorf("got %q", got)
	}
}

func TestTabs(t *testing.T) {
	d := Concat(Text("{"), Nest(Concat(Hardline, Text("a"), Nest(Concat(Hardline, Text("b"))))), Hardline, Text("}"))
	got := Print(d, Options{MaxWidth: 80, Indent: 4, UseTabs: true})
	if got != "{\n\ta\n\t\tb\n}" {
		t.Errorf("got %q", got)
	}
}

func TestUnicodeWidth(t *testing.T) {
	// "größe" is 5 columns but 7 bytes.
	d := NewGroup(Text("f("), Softline, Text("größe"), Softline, Text(")"))
	if got := p(d, 9); got != "f(größe)" {
		t.Errorf("got %q", got)
	}
}

func TestDeterministic(t *testing.T) {
	a := p(call("aaaa", "bbbb"), 8)
	for i := 0; i < 3; i++ {
		if b := p(call("aaaa", "bbbb"), 8); a != b {
			t.Fatal("nondeterministic")
		}
	}
}

func TestRestOfLineCountsForFit(t *testing.T) {
	// The ";" after the group must be accounted for when deciding to fit.
	d := Concat(call("a", "b"), Text(";"))
	if got := p(d, 7); got == "f(a, b);" {
		t.Errorf("trailing text ignored in fits: %q", got)
	}
	if got := p(d, 8); got != "f(a, b);" {
		t.Errorf("got %q", got)
	}
}
