package format

import (
	"strings"

	"scadfmt/internal/comments"
	"scadfmt/internal/doc"
	"scadfmt/internal/lexer"
	"scadfmt/internal/syntax"
)

type printer struct {
	toks  []lexer.Token
	index map[tok]int
	info  comments.Map
	opts  Options
}

func newPrinter(toks []lexer.Token, opts Options) *printer {
	p := &printer{toks: toks, index: make(map[tok]int, len(toks)), info: comments.Attach(toks), opts: opts}
	for i := range toks {
		p.index[&toks[i]] = i
	}
	return p
}

// hasComments reports whether any comment sits between from and to inclusive,
// counting comments before from's successors and after to.
func (p *printer) hasComments(from, to tok) bool {
	for i := p.index[from]; i <= p.index[to]; i++ {
		info := p.info[&p.toks[i]]
		if len(info.Leading) > 0 || len(info.Trailing) > 0 {
			return true
		}
	}
	return false
}

// tokBreak prints t and, if a comment trails it, forces the enclosing group to
// break so the comment cannot merge with later text.
func (p *printer) tokBreak(t tok) doc.Doc {
	if len(p.info[t].Trailing) > 0 {
		return doc.Concat(p.tok(t), doc.BreakParent{})
	}
	return p.tok(t)
}

type tok = syntax.Tok

// raw prints text that may contain line breaks and must keep its layout.
func raw(s string) doc.Doc {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	if !strings.Contains(s, "\n") {
		return doc.Text(s)
	}
	var parts []doc.Doc
	for i, line := range strings.Split(s, "\n") {
		if i > 0 {
			parts = append(parts, doc.LiteralLine{})
		}
		parts = append(parts, doc.Text(line))
	}
	return doc.Concat(parts...)
}

func commentDoc(c comments.Comment) doc.Doc {
	if !c.Block {
		return doc.Text(strings.TrimRight(c.Text, " \t"))
	}
	text := strings.ReplaceAll(c.Text, "\r\n", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) == 1 {
		return doc.Text(text)
	}
	// Doc-comment style (every continuation line starts with `*`): re-indent.
	star := true
	for _, l := range lines[1:] {
		if !strings.HasPrefix(strings.TrimLeft(l, " \t"), "*") {
			star = false
		}
	}
	if !star {
		return raw(text)
	}
	parts := []doc.Doc{doc.Text(strings.TrimRight(lines[0], " \t"))}
	for _, l := range lines[1:] {
		parts = append(parts, doc.Hardline, doc.Text(" "+strings.TrimSpace(l)))
	}
	return doc.Concat(parts...)
}

// breaks emits n line breaks, capped at one blank line.
func breaks(n int) doc.Doc {
	switch {
	case n <= 0:
		return doc.Nil
	case n == 1:
		return doc.Hardline
	}
	return doc.Concat(doc.Hardline, doc.Hardline)
}

// inlineLeading reports whether t's leading comments are all block comments on
// the same line as both the previous token and t.
func (p *printer) inlineLeading(t tok) bool {
	info := p.info[t]
	if len(info.Leading) == 0 || info.NewlinesBefore != 0 {
		return false
	}
	for _, c := range info.Leading {
		if !c.Block || c.NewlinesBefore != 0 {
			return false
		}
	}
	return true
}

func (p *printer) hasLeading(t tok) bool { return len(p.info[t].Leading) > 0 }

// leading prints the comments before t.
func (p *printer) leading(t tok) doc.Doc {
	info := p.info[t]
	if len(info.Leading) == 0 {
		return doc.Nil
	}
	if (t.Kind == lexer.Comma || t.Kind == lexer.Semicolon) && p.inlineLeading(t) {
		var parts []doc.Doc
		for _, c := range info.Leading {
			parts = append(parts, doc.Space, commentDoc(c))
		}
		return doc.Concat(parts...)
	}
	var parts []doc.Doc
	for i, c := range info.Leading {
		if i > 0 {
			if c.NewlinesBefore == 0 {
				parts = append(parts, doc.Space)
			} else {
				parts = append(parts, breaks(c.NewlinesBefore))
			}
		}
		parts = append(parts, commentDoc(c))
	}
	last := info.Leading[len(info.Leading)-1]
	switch {
	case info.NewlinesBefore > 0:
		parts = append(parts, breaks(info.NewlinesBefore))
	case !last.Block:
		parts = append(parts, doc.Hardline)
	default:
		parts = append(parts, doc.Space)
	}
	return doc.Concat(parts...)
}

// trailing prints the comments that follow t on its line. They are deferred to
// the end of the line; for tokens inside brackets they also force the
// enclosing group to break, so the comment stays next to what it annotates.
func (p *printer) trailing(t tok) doc.Doc {
	cs := p.info[t].Trailing
	if len(cs) == 0 {
		return doc.Nil
	}
	var parts []doc.Doc
	for _, c := range cs {
		parts = append(parts, doc.LineSuffix{D: doc.Concat(doc.Space, commentDoc(c))})
	}
	switch t.Kind {
	case lexer.Semicolon, lexer.LBrace, lexer.RBrace:
	default:
		parts = append(parts, doc.BreakParent{})
	}
	return doc.Concat(parts...)
}

func (p *printer) text(t tok) doc.Doc {
	switch t.Kind {
	case lexer.String, lexer.Include, lexer.Use:
		return raw(t.Text)
	}
	return doc.Text(t.Text)
}

// tok prints a token with its comments.
func (p *printer) tok(t tok) doc.Doc {
	return doc.Concat(p.leading(t), p.text(t), p.trailing(t))
}

// bare prints a token with trailing but without leading comments.
func (p *printer) bare(t tok) doc.Doc {
	return doc.Concat(p.text(t), p.trailing(t))
}

// closer splits a closing token into the comments that precede it, printed
// inside the indented body, and the token itself.
func (p *printer) closer(t tok) (inside, close doc.Doc) {
	info := p.info[t]
	if len(info.Leading) == 0 {
		return doc.Nil, p.bare(t)
	}
	if p.inlineLeading(t) {
		var parts []doc.Doc
		for _, c := range info.Leading {
			parts = append(parts, doc.Space, commentDoc(c))
		}
		return doc.Nil, doc.Concat(doc.Concat(parts...), p.bare(t))
	}
	return p.danglingComments(info), p.bare(t)
}

// danglingComments prints comments that sit alone before a closing token, each
// on its own line, preceded by a line break.
func (p *printer) danglingComments(info comments.Info) doc.Doc {
	var parts []doc.Doc
	for _, c := range info.Leading {
		parts = append(parts, breaks(max(c.NewlinesBefore, 1)), commentDoc(c))
	}
	return doc.Concat(parts...)
}

// blankBefore reports whether the source had a blank line before t.
func (p *printer) blankBefore(t tok) bool {
	info := p.info[t]
	if len(info.Leading) > 0 {
		return info.Leading[0].NewlinesBefore >= 2
	}
	return info.NewlinesBefore >= 2
}
