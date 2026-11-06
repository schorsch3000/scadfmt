package comments

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/schorsch3000/scadfmt/internal/lexer"
)

func attach(t *testing.T, src string) ([]lexer.Token, Map) {
	t.Helper()
	toks, err := lexer.Lex([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return toks, Attach(toks)
}

func texts(cs []Comment) []string {
	var s []string
	for _, c := range cs {
		s = append(s, c.Text)
	}
	return s
}

func eq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestClassification(t *testing.T) {
	src := "// head\n" + // 0: leading of `x`
		"x = 1; // trail x\n" + // trailing of `;`
		"\n" +
		"/* own */\n" + // leading of `y`, preceded by a blank line
		"y = /* inline */ 2; /* a */ // b\n" + // inline -> leading of 2; a, b trailing of second `;`
		"// before brace\n" +
		"{ } // end"
	toks, m := attach(t, src)
	find := func(text string, nth int) *lexer.Token {
		for i := range toks {
			if toks[i].Text == text {
				if nth == 0 {
					return &toks[i]
				}
				nth--
			}
		}
		t.Fatalf("token %q not found", text)
		return nil
	}

	if got := texts(m[find("x", 0)].Leading); !eq(got, []string{"// head"}) {
		t.Errorf("x leading = %v", got)
	}
	semi1 := find(";", 0)
	if got := texts(m[semi1].Trailing); !eq(got, []string{"// trail x"}) {
		t.Errorf("first ; trailing = %v", got)
	}
	yInfo := m[find("y", 0)]
	if got := texts(yInfo.Leading); !eq(got, []string{"/* own */"}) {
		t.Errorf("y leading = %v", got)
	}
	if yInfo.Leading[0].NewlinesBefore != 2 {
		t.Errorf("blank line before /* own */ lost: %d", yInfo.Leading[0].NewlinesBefore)
	}
	if yInfo.NewlinesBefore != 1 {
		t.Errorf("y NewlinesBefore = %d, want 1", yInfo.NewlinesBefore)
	}
	two := m[find("2", 0)]
	if got := texts(two.Leading); !eq(got, []string{"/* inline */"}) || two.Leading[0].NewlinesBefore != 0 || two.NewlinesBefore != 0 {
		t.Errorf("inline comment wrong: %+v", two)
	}
	if got := texts(m[find(";", 1)].Trailing); !eq(got, []string{"/* a */", "// b"}) {
		t.Errorf("second ; trailing = %v", got)
	}
	if got := texts(m[find("{", 0)].Leading); !eq(got, []string{"// before brace"}) {
		t.Errorf("{ leading = %v", got)
	}
	if got := texts(m[find("}", 0)].Trailing); !eq(got, []string{"// end"}) {
		t.Errorf("} trailing = %v", got)
	}
}

func TestEOFAndFirstToken(t *testing.T) {
	// A comment after the last token without a trailing newline is trailing.
	toks, m := attach(t, "a; // end")
	if got := texts(m[&toks[1]].Trailing); !eq(got, []string{"// end"}) {
		t.Errorf("got %v", got)
	}
	// File with only comments: they lead the EOF token.
	toks, m = attach(t, "// a\n/* b */\n")
	if got := texts(m[&toks[0]].Leading); !eq(got, []string{"// a", "/* b */"}) {
		t.Errorf("got %v", got)
	}
	// A comment on the first line, before any token, is leading.
	toks, m = attach(t, "/* h */ a;")
	if got := texts(m[&toks[0]].Leading); !eq(got, []string{"/* h */"}) {
		t.Errorf("got %v", got)
	}
	// Block comment followed by code on the same line is not trailing.
	toks, m = attach(t, "a; /* x */ b;")
	if len(m[&toks[1]].Trailing) != 0 || len(m[&toks[2]].Leading) != 1 {
		t.Errorf("inline block misclassified: %+v %+v", m[&toks[1]], m[&toks[2]])
	}
}

// TestNoCommentLost checks over the whole corpus that every comment ends up in
// exactly one place and in source order.
func TestNoCommentLost(t *testing.T) {
	root := "../../openscad"
	if _, err := os.Stat(root); err != nil {
		t.Skip("no openscad checkout")
	}
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".scad") {
			return nil
		}
		src, _ := os.ReadFile(p)
		toks, lerr := lexer.Lex(src)
		if lerr != nil {
			return nil
		}
		var want []string
		for _, tk := range toks {
			for _, tr := range tk.Leading {
				if tr.Kind == lexer.LineComment || tr.Kind == lexer.BlockComment {
					want = append(want, tr.Text)
				}
			}
		}
		m := Attach(toks)
		var all []Comment
		for i := range toks {
			info := m[&toks[i]]
			all = append(all, info.Leading...)
			all = append(all, info.Trailing...)
		}
		sort.Slice(all, func(a, b int) bool { return all[a].Pos.Offset < all[b].Pos.Offset })
		if !eq(texts(all), want) {
			t.Errorf("%s: comments lost, duplicated or reordered (%d in, %d out)", p, len(want), len(all))
		}
		return nil
	})
}
