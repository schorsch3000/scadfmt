package format

import "testing"

func aligned(mut func(*Options)) func(*Options) {
	return func(o *Options) {
		o.AlignAssignments = true
		if mut != nil {
			mut(o)
		}
	}
}

func TestAlignAssignments(t *testing.T) {
	src := "a=1;\nlonger=2;\nmid=3;\n"
	want := "a      = 1;\nlonger = 2;\nmid    = 3;\n"
	if got := fmtWith(t, src, aligned(nil)); got != want {
		t.Errorf("got %q want %q", got, want)
	}
	// Off by default.
	if got := fmtWith(t, src, nil); got != "a = 1;\nlonger = 2;\nmid = 3;\n" {
		t.Errorf("default changed: %q", got)
	}
}

func TestAlignGroupsEndAtBlankLineOrOtherStatement(t *testing.T) {
	src := "a=1;\nbbb=2;\n\nc=3;\ndddd=4;\ncube(1);\ne=5;\nff=6;\n"
	want := "a   = 1;\nbbb = 2;\n\nc    = 3;\ndddd = 4;\ncube(1);\ne  = 5;\nff = 6;\n"
	if got := fmtWith(t, src, aligned(nil)); got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestAlignMaxGap(t *testing.T) {
	src := "a=1;\nb=2;\nvery_long_name=3;\nc=4;\n"
	got := fmtWith(t, src, aligned(func(o *Options) { o.AlignMaxGap = 3 }))
	want := "a = 1;\nb = 2;\nvery_long_name = 3;\nc = 4;\n"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
	got = fmtWith(t, src, aligned(func(o *Options) { o.AlignMaxGap = 0 }))
	want = "a              = 1;\nb              = 2;\nvery_long_name = 3;\nc              = 4;\n"
	if got != want {
		t.Errorf("no limit: got %q want %q", got, want)
	}
}

func TestAlignDecimal(t *testing.T) {
	src := "w=2.5;\nhh=100;\nd=12.25;\nneg=-3;\n"
	want := "w   =   2.5;\nhh  = 100;\nd   =  12.25;\nneg =  -3;\n"
	got := fmtWith(t, src, aligned(func(o *Options) { o.AlignDecimal = true }))
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
	// Without AlignDecimal only the names line up.
	if got := fmtWith(t, src, aligned(nil)); got != "w   = 2.5;\nhh  = 100;\nd   = 12.25;\nneg = -3;\n" {
		t.Errorf("names only: %q", got)
	}
}

func TestAlignDecimalSkipsNonNumbers(t *testing.T) {
	src := "a=1.5;\nb=x+1;\nc=22.5;\n"
	got := fmtWith(t, src, aligned(func(o *Options) { o.AlignDecimal = true }))
	want := "a = 1.5;\nb = x + 1;\nc = 22.5;\n"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestAlignTrailingComments(t *testing.T) {
	src := "a = 1; // x\nlonger = 22; // y\ncube(1); // z\n"
	want := "a = 1;       // x\nlonger = 22; // y\ncube(1);     // z\n"
	got := fmtWith(t, src, func(o *Options) { o.AlignTrailingComments = true })
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestAlignCombined(t *testing.T) {
	src := "w=2.5; // [1:10]\nheight=100; // [1:200]\n"
	want := "w      =   2.5; // [1:10]\nheight = 100;   // [1:200]\n"
	got := fmtWith(t, src, aligned(func(o *Options) { o.AlignDecimal, o.AlignTrailingComments = true, true }))
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestAlignSkipsMultilineStatements(t *testing.T) {
	src := "a=1;\nb=[\n 1,\n 2];\nccc=3;\n"
	got := fmtWith(t, src, aligned(nil))
	want := "a = 1;\nb = [\n  1,\n  2\n];\nccc = 3;\n"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestAlignParamsOnlyWhenBroken(t *testing.T) {
	src := "module m(a = 1, longer = 2) {}\n"
	opts := func(width int) func(*Options) {
		return func(o *Options) { o.AlignParams = true; o.MaxWidth = width }
	}
	if got := fmtWith(t, src, opts(80)); got != "module m(a = 1, longer = 2) {}\n" {
		t.Errorf("flat: %q", got)
	}
	want := "module m(\n  a      = 1,\n  longer = 2\n) {}\n"
	if got := fmtWith(t, src, opts(20)); got != want {
		t.Errorf("broken: got %q want %q", got, want)
	}
}

func TestMatrixAlign(t *testing.T) {
	src := "m = [[1.5, 0, 100],\n [10, -2.25, 3]];\n"
	want := "m = [\n  [ 1.5,  0,    100],\n  [10,   -2.25,   3]\n];\n"
	got := fmtWith(t, src, func(o *Options) { o.Matrix = MatrixAlign })
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestMatrixAlignFallsBack(t *testing.T) {
	// A comment inside, or non-vector rows, keep the preserved layout.
	src := "m = [[1, 2], // c\n [30, 4]];\n"
	want := "m = [\n  [1, 2], // c\n  [30, 4]\n];\n"
	if got := fmtWith(t, src, func(o *Options) { o.Matrix = MatrixAlign }); got != want {
		t.Errorf("got %q want %q", got, want)
	}
	src = "m = [\n 1,\n 2];\n"
	if got := fmtWith(t, src, func(o *Options) { o.Matrix = MatrixAlign }); got != "m = [\n  1,\n  2\n];\n" {
		t.Errorf("non-table: %q", got)
	}
}

func TestNormalizeNumbers(t *testing.T) {
	for in, want := range map[string]string{
		".5": "0.5", "1.50": "1.5", "1.": "1.0", "1.0": "1.0", "10": "10",
		"1.5e3": "1.5e3", "2.50E-2": "2.5E-2", "0x1F": "0x1F", "100.00": "100.0",
	} {
		if got := normalizeNumber(in); got != want {
			t.Errorf("%s: got %s want %s", in, got, want)
		}
	}
	got := fmtWith(t, "x = [.5, 1.50, 3];\n", func(o *Options) { o.NormalizeNumbers = true })
	if got != "x = [0.5, 1.5, 3];\n" {
		t.Errorf("got %q", got)
	}
	if got := fmtWith(t, "x = .5;\n", nil); got != "x = .5;\n" {
		t.Errorf("must be opt-in: %q", got)
	}
}

// A table written on one line stays on one line if it fits, and becomes an
// aligned table if it does not, the same result as formatting it twice.
func TestMatrixAlignWideSingleLine(t *testing.T) {
	src := "m = [[1.5, 0, 100], [10, -2.25, 3]];\n"
	narrow := func(o *Options) { o.Matrix = MatrixAlign; o.MaxWidth = 20 }
	got := fmtWith(t, src, narrow)
	want := "m = [\n  [ 1.5,  0,    100],\n  [10,   -2.25,   3]\n];\n"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
	if again := fmtWith(t, got, narrow); again != got {
		t.Errorf("not idempotent: %q", again)
	}
	if got := fmtWith(t, src, func(o *Options) { o.Matrix = MatrixAlign }); got != "m = [[1.5, 0, 100], [10, -2.25, 3]];\n" {
		t.Errorf("fits, so stays flat: %q", got)
	}
}
