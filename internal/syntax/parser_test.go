package syntax

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/schorsch3000/scadfmt/internal/lexer"
)

func parse(t *testing.T, src string) (*File, []lexer.Token, error) {
	t.Helper()
	toks, err := lexer.Lex([]byte(src))
	if err != nil {
		return nil, toks, err
	}
	f, err := Parse(toks)
	return f, toks, err
}

// collect gathers every token pointer referenced from n, skipping Span fields
// (which only repeat tokens held elsewhere).
func collect(v reflect.Value, out map[Tok]int) {
	switch v.Kind() {
	case reflect.Interface, reflect.Ptr:
		if v.IsNil() {
			return
		}
		if t, ok := v.Interface().(Tok); ok {
			out[t]++
			return
		}
		collect(v.Elem(), out)
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).Type == reflect.TypeOf(Span{}) {
				continue
			}
			collect(v.Field(i), out)
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			collect(v.Index(i), out)
		}
	}
}

// checkCoverage asserts that each token appears exactly once in the tree.
func checkCoverage(t *testing.T, name string, f *File, toks []lexer.Token) {
	t.Helper()
	seen := map[Tok]int{}
	collect(reflect.ValueOf(f), seen)
	var bad []string
	for i := range toks {
		if n := seen[&toks[i]]; n != 1 {
			bad = append(bad, toks[i].Text)
		}
	}
	if len(seen) != len(toks) || len(bad) > 0 {
		sort.Strings(bad)
		t.Errorf("%s: token coverage wrong: %d in tree, %d lexed, bad=%v", name, len(seen), len(toks), bad)
	}
}

func TestParseOK(t *testing.T) {
	cases := []string{
		"",
		"cube(1);",
		"x = 1;",
		"use <a.scad>\ninclude <b.scad>\ncube();",
		"module m(a, b = 2, c = [1, 2],) { cube(a); }",
		"function f(x) = x * 2;",
		"function f(x = 1) = let(y = x) y + 1;",
		"translate([1, 0, 0]) rotate(90) cube(1);",
		"#cube(1); %sphere(2); !cylinder(); *cube();",
		"if (a) cube(); else if (b) sphere(1); else { cylinder(); }",
		"for (i = [0:10], j = [1:2:5]) { translate([i, j, 0]) cube(); }",
		"intersection_for(i = [1, 2]) cube();",
		"x = a ? b : c ? d : e;",
		"x = -a ^ 2 ^ 3 + !b * ~c % 4 / 5 - 6;",
		"x = a || b && c == d != e < f <= g > h >= i | j & k << l >> m;",
		"x = f(1, a = 2)[0].y.z;",
		"x = [for (i = [0:3]) i * 2];",
		"x = [for (i = [0:3]) if (i > 1) i else -i, 5,];",
		"x = [for (i = 0; i < 3; i = i + 1) i];",
		"x = [each a, each [1, 2]];",
		"x = [let (a = 1) for (i = [0:a]) i];",
		"x = [let (a = 1) a * 2];",
		"x = [(for (i = [0:2]) i)];",
		"x = [let (a = 1) (for (i = [0:a]) i)];",
		"x = [[1, 0], [0, 1]];",
		"x = [];",
		"x = [1:2];",
		"f = function(a, b = 1) a + b;",
		"x = assert(a > 0) a;",
		"x = echo(a) a;",
		"assert(true);",
		"echo(\"hi\");",
		"x = 1e-3 + .5 + 0x1f + \"s\\\"t\" + undef + true;",
		"{ a = 1; cube(a); }",
		";",
		"module m() ;",
		"let(a = 1) cube(a);",
		"$fn = 32;",
		"x = $preview ? 1 : 2;",
		"translate([0, 0, 0]) { }",
		"cube(1) // comment\n;",
	}
	for _, src := range cases {
		f, toks, err := parse(t, src)
		if err != nil {
			t.Errorf("%q: %v", src, err)
			continue
		}
		checkCoverage(t, src, f, toks)
	}
}

func TestParseTypes(t *testing.T) {
	f, _, err := parse(t, "x = [let (a = 1) a * 2, let (a = 1) for (i = [0:a]) i];")
	if err != nil {
		t.Fatal(err)
	}
	v := f.Stmts[0].(*Assignment).Value.(*Vector)
	if _, ok := v.Elems[0].(*Keyword); !ok {
		t.Errorf("elem 0 = %T, want *Keyword", v.Elems[0])
	}
	if _, ok := v.Elems[1].(*LcLet); !ok {
		t.Errorf("elem 1 = %T, want *LcLet", v.Elems[1])
	}

	f, _, _ = parse(t, "x = -2 ^ 2;")
	u := f.Stmts[0].(*Assignment).Value.(*Unary)
	if _, ok := u.X.(*Binary); !ok {
		t.Errorf("-2^2 should be Unary(Binary), got Unary(%T)", u.X)
	}
}

func TestParseErrors(t *testing.T) {
	for _, src := range []string{
		"cube(", "x = ;", "x = 1", "module () {}", "{", "x = [1, 2;", "x = 1 +;",
		"cube() x = 1;", "else cube();", "x = [1:];", "function f() 1;",
	} {
		if _, _, err := parse(t, src); err == nil {
			t.Errorf("%q: want error", src)
		}
	}
}

// TestCorpus parses the OpenSCAD examples, which must all be valid, and
// checks token coverage for every file in the vendored tree that parses.
func TestCorpus(t *testing.T) {
	root := "../../openscad"
	if _, err := os.Stat(root); err != nil {
		t.Skip("no openscad checkout")
	}
	ok, failed := 0, 0
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".scad") {
			return nil
		}
		src, _ := os.ReadFile(p)
		toks, lerr := lexer.Lex(src)
		var f *File
		var perr error
		if lerr == nil {
			f, perr = Parse(toks)
		}
		if lerr != nil || perr != nil {
			failed++
			if strings.Contains(p, "/examples/") {
				t.Errorf("%s: %v %v", p, lerr, perr)
			}
			return nil
		}
		ok++
		checkCoverage(t, p, f, toks)
		return nil
	})
	t.Logf("parsed %d, failed %d", ok, failed)
}
