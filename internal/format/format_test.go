package format

import "testing"

func fmtWith(t *testing.T, src string, mut func(*Options)) string {
	t.Helper()
	opts := DefaultOptions()
	if mut != nil {
		mut(&opts)
	}
	out, err := Format([]byte(src), opts)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestChainModes(t *testing.T) {
	src := "translate([1,0,0]) rotate(90) cube(1);\n"
	cases := []struct{ chain, narrow, want string }{
		{ChainAuto, "", "translate([1, 0, 0]) rotate(90) cube(1);\n"},
		{ChainFlat, "", "translate([1, 0, 0]) rotate(90) cube(1);\n"},
		{ChainBreak, "", "translate([1, 0, 0])\n  rotate(90)\n    cube(1);\n"},
	}
	for _, c := range cases {
		got := fmtWith(t, src, func(o *Options) { o.Chain = c.chain })
		if got != c.want {
			t.Errorf("%s: got %q want %q", c.chain, got, c.want)
		}
	}
	// Too long for the line: auto breaks, flat keeps one line.
	narrow := func(chain string) func(*Options) {
		return func(o *Options) { o.Chain = chain; o.MaxWidth = 30 }
	}
	if got, want := fmtWith(t, src, narrow(ChainAuto)), "translate([1, 0, 0])\n  rotate(90)\n    cube(1);\n"; got != want {
		t.Errorf("auto narrow: got %q want %q", got, want)
	}
	// Flat never breaks between calls; only their arguments wrap.
	if got, want := fmtWith(t, src, narrow(ChainFlat)), "translate([1, 0, 0]) rotate(\n  90\n) cube(1);\n"; got != want {
		t.Errorf("flat narrow: got %q want %q", got, want)
	}
}

func TestBlankLines(t *testing.T) {
	got := fmtWith(t, "a=1;\n\n\n\nb=2;\nc=3;\n{\n\n\nd=4;\n\n}\n", nil)
	want := "a = 1;\n\nb = 2;\nc = 3;\n{\n  d = 4;\n}\n"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestEmptyAndCommentOnly(t *testing.T) {
	for _, src := range []string{"", "  \n\n"} {
		if got := fmtWith(t, src, nil); got != "" {
			t.Errorf("%q: got %q", src, got)
		}
	}
	if got := fmtWith(t, "// a\n\n\n// b\n", nil); got != "// a\n\n// b\n" {
		t.Errorf("comment-only: got %q", got)
	}
}

func TestTrailingCommaKeptOnlyWhenBroken(t *testing.T) {
	if got := fmtWith(t, "x = [1, 2,];\n", nil); got != "x = [1, 2];\n" {
		t.Errorf("flat: got %q", got)
	}
	got := fmtWith(t, "x = [1, 2,];\n", func(o *Options) { o.MaxWidth = 8 })
	if got != "x = [\n  1,\n  2,\n];\n" {
		t.Errorf("broken: got %q", got)
	}
}

func TestSyntaxErrorReported(t *testing.T) {
	if _, err := Format([]byte("cube("), DefaultOptions()); err == nil {
		t.Error("want error")
	}
}

func TestMatrixModes(t *testing.T) {
	src := "m = [[1,0],\n     [0,1]];\nf(\n  a, b);\n"
	preserve := "m = [\n  [1, 0],\n  [0, 1]\n];\nf(\n  a,\n  b\n);\n"
	reflow := "m = [[1, 0], [0, 1]];\nf(a, b);\n"
	for mode, want := range map[string]string{
		MatrixPreserve: preserve,
		MatrixAlign:    preserve, // until column alignment exists
		MatrixReflow:   reflow,
	} {
		got := fmtWith(t, src, func(o *Options) { o.Matrix = mode })
		if got != want {
			t.Errorf("%s: got %q want %q", mode, got, want)
		}
	}
}

func TestPreservedRowsKeepGrouping(t *testing.T) {
	src := "x = [1, 2,\n     3, 4,\n     5];\n"
	want := "x = [\n  1, 2,\n  3, 4,\n  5\n];\n"
	if got := fmtWith(t, src, nil); got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestLongRowWrapsAsFill(t *testing.T) {
	src := "x = [\n  1111, 2222, 3333, 4444, 5555, 6666\n];\n"
	got := fmtWith(t, src, func(o *Options) { o.MaxWidth = 20 })
	want := "x = [\n  1111, 2222, 3333,\n  4444, 5555, 6666\n];\n"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
	// Formatting the wrapped result again must not change it.
	again := fmtWith(t, got, func(o *Options) { o.MaxWidth = 20 })
	if again != got {
		t.Errorf("not idempotent: %q", again)
	}
}

func TestChainIndentSame(t *testing.T) {
	src := "translate([1,0,0]) rotate(90) cube(1);\n"
	same := func(o *Options) { o.Chain = ChainBreak; o.ChainIndent = ChainIndentSame }
	if got, want := fmtWith(t, src, same), "translate([1, 0, 0])\n  rotate(90)\n  cube(1);\n"; got != want {
		t.Errorf("got %q want %q", got, want)
	}
	src = "translate(1) rotate(2) { cube(1); }\n"
	want := "translate(1)\n  rotate(2) {\n    cube(1);\n  }\n"
	if got := fmtWith(t, src, same); got != want {
		t.Errorf("block: got %q want %q", got, want)
	}
	// Default is unchanged.
	if got, want := fmtWith(t, "a() b() c();\n", func(o *Options) { o.Chain = ChainBreak }), "a()\n  b()\n    c();\n"; got != want {
		t.Errorf("step: got %q want %q", got, want)
	}
}
