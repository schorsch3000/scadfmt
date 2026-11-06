package format

import (
	"fmt"
	"sort"
	"strings"

	"github.com/schorsch3000/scadfmt/internal/lexer"
	"github.com/schorsch3000/scadfmt/internal/syntax"
)

// verify checks that out is the same program as the input and that no comment
// was lost. Format runs it on every result, so a printer bug can never silently
// change a file: it fails instead.
func verify(srcToks []lexer.Token, srcTree *syntax.File, out []byte, opts Options) error {
	outToks, err := lexer.Lex(out)
	if err != nil {
		return fmt.Errorf("output does not lex: %w", err)
	}
	outTree, err := syntax.Parse(outToks)
	if err != nil {
		return fmt.Errorf("output does not parse: %w", err)
	}
	eq := syntax.EqualOptions{}
	if opts.NormalizeNumbers {
		eq.Number = normalizeNumber
	}
	if err := syntax.Equal(srcTree, outTree, eq); err != nil {
		return fmt.Errorf("output differs from input: %w", err)
	}
	if a, b := commentSet(srcToks), commentSet(outToks); !equalStrings(a, b) {
		return fmt.Errorf("comments changed (%d before, %d after)", len(a), len(b))
	}
	return nil
}

// commentSet returns the comments of a token stream, sorted, with the
// whitespace changes the formatter may make (trailing spaces, indentation of
// doc-comment continuation lines) removed.
func commentSet(toks []lexer.Token) []string {
	var cs []string
	for _, t := range toks {
		for _, tr := range t.Leading {
			switch tr.Kind {
			case lexer.LineComment:
				cs = append(cs, strings.TrimRight(tr.Text, " \t\r"))
			case lexer.BlockComment:
				lines := strings.Split(strings.ReplaceAll(tr.Text, "\r\n", "\n"), "\n")
				for i := range lines {
					lines[i] = strings.TrimSpace(lines[i])
				}
				cs = append(cs, strings.Join(lines, "\n"))
			}
		}
	}
	sort.Strings(cs)
	return cs
}

func equalStrings(a, b []string) bool {
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
