package format

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"scadfmt/internal/lexer"
)

// signature returns the token texts of src without trailing commas (which the
// formatter may drop when a list is laid out flat), plus the comments sorted.
func signature(t *testing.T, name string, src []byte) (toks, cmts []string) {
	t.Helper()
	ts, err := lexer.Lex(src)
	if err != nil {
		t.Fatalf("%s: lex: %v", name, err)
	}
	for i, tk := range ts {
		for _, tr := range tk.Leading {
			if tr.Kind == lexer.LineComment || tr.Kind == lexer.BlockComment {
				c := tr.Text
				if tr.Kind == lexer.LineComment {
					c = strings.TrimRight(c, " \t")
				} else {
					// Re-indentation of doc comments may change leading blanks.
					lines := strings.Split(c, "\n")
					for j := range lines {
						lines[j] = strings.TrimSpace(lines[j])
					}
					c = strings.Join(lines, "\n")
				}
				cmts = append(cmts, c)
			}
		}
		if tk.Kind == lexer.Comma && i+1 < len(ts) && (ts[i+1].Kind == lexer.RParen || ts[i+1].Kind == lexer.RBracket) {
			continue
		}
		if tk.Kind == lexer.EOF {
			continue
		}
		text := tk.Text
		if tk.Kind == lexer.Include || tk.Kind == lexer.Use {
			// The formatter normalizes the space between keyword and path.
			i := strings.IndexByte(text, '<')
			text = strings.Join(strings.Fields(text[:i]), "") + text[i:]
		}
		toks = append(toks, text)
	}
	sort.Strings(cmts)
	return toks, cmts
}

func equal(a, b []string) bool {
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

// TestCorpus formats every parseable file of the vendored OpenSCAD tree with
// each chain mode and checks that (1) no token or comment is lost or changed,
// (2) the output parses, and (3) formatting is idempotent.
func TestCorpus(t *testing.T) {
	root := "../../openscad"
	if _, err := os.Stat(root); err != nil {
		t.Skip("no openscad checkout")
	}
	n := 0
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".scad") {
			return nil
		}
		src, _ := os.ReadFile(p)
		for _, chain := range []string{ChainAuto, ChainFlat, ChainBreak} {
			opts := DefaultOptions()
			opts.Chain = chain
			out, err := Format(src, opts)
			if err != nil {
				return nil // deliberately invalid file
			}
			if chain == ChainAuto {
				n++
			}
			name := p + " [" + chain + "]"
			wantT, wantC := signature(t, name, src)
			gotT, gotC := signature(t, name, out)
			if !equal(wantT, gotT) {
				t.Errorf("%s: tokens changed%s", name, firstDiff(wantT, gotT))
			}
			if !equal(wantC, gotC) {
				t.Errorf("%s: comments changed%s", name, firstDiff(wantC, gotC))
			}
			again, err := Format(out, opts)
			if err != nil {
				t.Errorf("%s: output does not parse: %v", name, err)
				continue
			}
			if string(again) != string(out) {
				t.Errorf("%s: not idempotent%s", name, firstLineDiff(string(out), string(again)))
			}
		}
		return nil
	})
	t.Logf("%d files", n)
}

func firstDiff(a, b []string) string {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return ": first difference at " + a[i] + " vs " + b[i]
		}
	}
	return ": length " + string(rune('0'+len(a)%10)) + " vs other"
}

func firstLineDiff(a, b string) string {
	la, lb := strings.Split(a, "\n"), strings.Split(b, "\n")
	for i := 0; i < len(la) && i < len(lb); i++ {
		if la[i] != lb[i] {
			return "\n  first:  " + la[i] + "\n  second: " + lb[i]
		}
	}
	return ""
}
