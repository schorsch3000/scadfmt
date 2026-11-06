package lexer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func roundTrip(toks []Token) string {
	var b strings.Builder
	for _, t := range toks {
		for _, tr := range t.Leading {
			b.WriteString(tr.Text)
		}
		b.WriteString(t.Text)
	}
	return b.String()
}

func kinds(t *testing.T, src string) []Kind {
	t.Helper()
	toks, err := Lex([]byte(src))
	if err != nil {
		t.Fatalf("%q: %v", src, err)
	}
	var ks []Kind
	for _, tk := range toks {
		ks = append(ks, tk.Kind)
	}
	return ks
}

func TestKinds(t *testing.T) {
	cases := []struct {
		src  string
		want []Kind
	}{
		{"cube(1);", []Kind{Ident, LParen, Number, RParen, Semicolon, EOF}},
		{"a<=b>>c", []Kind{Ident, LE, Ident, Shr, Ident, EOF}},
		{"#cube();", []Kind{Hash, Ident, LParen, RParen, Semicolon, EOF}},
		{"$fn=8;", []Kind{Ident, Assign, Number, Semicolon, EOF}},
		{"x=.5e-3+1.+0x1F;", []Kind{Ident, Assign, Number, Plus, Number, Plus, Number, Semicolon, EOF}},
		{"2d", []Kind{Ident, EOF}},
		{"1e3", []Kind{Number, EOF}},
		{"include <a/b.scad>\nuse<c.scad>", []Kind{Include, Use, EOF}},
		{"include = 1;", []Kind{Ident, Assign, Number, Semicolon, EOF}},
		{"module f() {}", []Kind{Module, Ident, LParen, RParen, LBrace, RBrace, EOF}},
		{`"a\"b"`, []Kind{String, EOF}},
		{"a.b[0]", []Kind{Ident, Dot, Ident, LBracket, Number, RBracket, EOF}},
		{"[0:2:10]", []Kind{LBracket, Number, Colon, Number, Colon, Number, RBracket, EOF}},
		{"größe=1;", []Kind{Ident, Assign, Number, Semicolon, EOF}},
	}
	for _, c := range cases {
		got := kinds(t, c.src)
		if len(got) != len(c.want) {
			t.Errorf("%q: got %v want %v", c.src, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%q: got %v want %v", c.src, got, c.want)
				break
			}
		}
	}
}

func TestTrivia(t *testing.T) {
	toks, err := Lex([]byte("// a\r\n/* b\nc */ x /*d*/;\n"))
	if err != nil {
		t.Fatal(err)
	}
	x := toks[0]
	var kinds []TriviaKind
	for _, tr := range x.Leading {
		kinds = append(kinds, tr.Kind)
	}
	want := []TriviaKind{LineComment, Newline, BlockComment, Whitespace}
	if len(kinds) != len(want) {
		t.Fatalf("got %v want %v", kinds, want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("got %v want %v", kinds, want)
		}
	}
	if x.Text != "x" || x.Pos.Line != 3 || x.Pos.Col != 6 {
		t.Errorf("x = %+v", x)
	}
	if last := toks[len(toks)-1]; last.Kind != EOF || len(last.Leading) != 1 || last.Leading[0].Kind != Newline {
		t.Errorf("eof = %+v", last)
	}
}

func TestErrors(t *testing.T) {
	for _, src := range []string{`"abc`, "/* x", "include <a", "@"} {
		toks, err := Lex([]byte(src))
		if err == nil {
			t.Errorf("%q: want error", src)
		}
		if roundTrip(toks) != src {
			t.Errorf("%q: round trip broken: %q", src, roundTrip(toks))
		}
	}
}

// TestCorpusRoundTrip lexes every .scad file in the vendored OpenSCAD tree and
// checks that concatenating tokens and trivia reproduces the input.
func TestCorpusRoundTrip(t *testing.T) {
	root := "../../openscad"
	if _, err := os.Stat(root); err != nil {
		t.Skip("no openscad checkout")
	}
	n := 0
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".scad") {
			return nil
		}
		src, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		n++
		toks, _ := Lex(src) // some corpus files are deliberately broken
		if got := roundTrip(toks); got != string(src) {
			t.Errorf("%s: round trip mismatch", p)
		}
		return nil
	})
	t.Logf("%d files", n)
}
