package format

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/schorsch3000/scadfmt/internal/lexer"
	"github.com/schorsch3000/scadfmt/internal/syntax"
)

// fuzzOptions returns one of several option sets, chosen by n.
func fuzzOptions(n uint8) Options {
	o := DefaultOptions()
	switch n % 5 {
	case 1:
		o.Chain, o.ChainIndent = ChainBreak, ChainIndentSame
	case 2:
		o.Chain, o.Matrix = ChainFlat, MatrixReflow
	case 3:
		o.MaxWidth, o.Indent = 24, 3
		o.Matrix = MatrixAlign
		o.AlignAssignments, o.AlignDecimal, o.AlignParams, o.AlignTrailingComments = true, true, true, true
	case 4:
		o.MaxWidth, o.UseTabs = 40, true
		o.NormalizeNumbers = true
		o.AlignAssignments, o.AlignDecimal = true, true
	}
	return o
}

// FuzzFormat checks, for any input that parses, that formatting succeeds (which
// includes the built-in equivalence check) and is idempotent.
//
//	go test ./internal/format -fuzz=FuzzFormat -fuzztime=60s
func FuzzFormat(f *testing.F) {
	for _, s := range []string{
		"cube([1,2,3]);",
		"x=1; // c\ny = [[1,0],\n[0,1]];\n",
		"module m(a=1,b=[1,2]) { translate([a,0,0]) rotate(90) cube(b); }",
		"function f(x) = let(a = x * 2) a > 1 ? [for (i = [0:a]) i] : [];",
		"if (a) b(); else if (c) d(); else { e(); }",
		"/* c */ a = /* d */ 1 /* e */; // f\n/** doc\n * more */\nb = 2;",
		"for (i = [0:2], j = [1:2]) translate([i, j]) { cube(1); sphere(2); }",
		"x = [each a, if (b) c else d, for (i = 0; i < 3; i = i + 1) i];",
		"include <a.scad>\nuse <b.scad>\n$fn=8;",
	} {
		f.Add([]byte(s), uint8(0))
		f.Add([]byte(s), uint8(3))
	}
	if root := "../../openscad/tests/data/scad/misc"; true {
		files, _ := filepath.Glob(filepath.Join(root, "*.scad"))
		for i, p := range files {
			if src, err := os.ReadFile(p); err == nil && len(src) < 4000 && !strings.Contains(p, "include") {
				f.Add(src, uint8(i))
			}
		}
	}

	f.Fuzz(func(t *testing.T, src []byte, which uint8) {
		toks, err := lexer.Lex(src)
		if err != nil {
			return
		}
		if _, err := syntax.Parse(toks); err != nil {
			return
		}
		opts := fuzzOptions(which)
		out, err := Format(src, opts)
		if err != nil {
			t.Fatalf("Format failed on valid input: %v\ninput: %q", err, src)
		}
		again, err := Format(out, opts)
		if err != nil {
			t.Fatalf("formatted output fails to format: %v\noutput: %q", err, out)
		}
		if string(again) != string(out) {
			t.Fatalf("not idempotent\ninput:  %q\nfirst:  %q\nsecond: %q", src, out, again)
		}
	})
}
