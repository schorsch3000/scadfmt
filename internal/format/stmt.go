package format

import (
	"strings"

	"github.com/schorsch3000/scadfmt/internal/doc"
	"github.com/schorsch3000/scadfmt/internal/lexer"
	"github.com/schorsch3000/scadfmt/internal/syntax"
)

// file prints a whole file; the result ends with a line break unless empty.
func (p *printer) file(f *syntax.File) doc.Doc {
	body := p.stmts(f.Stmts)
	eof := p.info[f.EOF]
	var parts []doc.Doc
	parts = append(parts, body)
	if len(eof.Leading) > 0 {
		for i, c := range eof.Leading {
			if i > 0 || len(f.Stmts) > 0 {
				parts = append(parts, breaks(max(c.NewlinesBefore, 1)))
			}
			parts = append(parts, commentDoc(c))
		}
	}
	if len(f.Stmts) == 0 && len(eof.Leading) == 0 {
		return doc.Nil
	}
	parts = append(parts, doc.Hardline)
	return doc.Concat(parts...)
}

// stmts prints statements one per line, keeping at most one blank line
// between them.
func (p *printer) stmts(ss []syntax.Stmt) doc.Doc {
	p.alignStmts(ss)
	var parts []doc.Doc
	for i, s := range ss {
		if i > 0 {
			parts = append(parts, doc.Hardline)
			if p.blankBefore(s.First()) {
				parts = append(parts, doc.Hardline)
			}
		}
		parts = append(parts, p.stmt(s))
	}
	return doc.Concat(parts...)
}

func (p *printer) stmt(s syntax.Stmt) doc.Doc {
	switch n := s.(type) {
	case *syntax.Empty:
		p.stmtStart = n.Semi
		d := p.tok(n.Semi)
		p.stmtStart = nil
		return d
	case *syntax.Block:
		return p.block(n)
	case *syntax.Include:
		return p.include(n)
	case *syntax.Assignment:
		return p.assignment(n)
	case *syntax.ModuleDef:
		return doc.Concat(
			p.tok(n.Module), doc.Space, p.tok(n.Name),
			p.args(n.LParen, n.Params, n.RParen, false),
			p.afterHead(n.Body),
		)
	case *syntax.FunctionDef:
		return doc.Concat(
			p.tok(n.Function), doc.Space, p.tok(n.Name),
			p.args(n.LParen, n.Params, n.RParen, false),
			doc.Space, p.tok(n.Eq), p.rhs(n.Body), p.tok(n.Semi),
		)
	case *syntax.ModuleCall:
		return p.chain(n)
	case *syntax.If:
		return p.ifStmt(n)
	}
	panic("unreachable: unknown statement")
}

// include prints `include <...>` and `use <...>` with normalized spacing
// between the keyword and the path.
func (p *printer) include(n *syntax.Include) doc.Doc {
	t := n.Tok
	i := strings.IndexByte(t.Text, '<')
	kw := strings.TrimRight(t.Text[:i], " \t\r\n")
	return doc.Concat(p.leading(t), raw(kw+" "+t.Text[i:]), p.trailing(t))
}

func (p *printer) block(b *syntax.Block) doc.Doc {
	closeInfo := p.info[b.RBrace]
	if len(b.Stmts) == 0 && len(closeInfo.Leading) == 0 && len(p.info[b.LBrace].Trailing) == 0 {
		return doc.Concat(p.tok(b.LBrace), p.bare(b.RBrace))
	}
	p.depth++
	inner := []doc.Doc{doc.Hardline, p.stmts(b.Stmts)}
	p.depth--
	if len(b.Stmts) == 0 {
		inner = nil
	}
	if len(closeInfo.Leading) > 0 {
		if len(b.Stmts) == 0 {
			// No blank line right after `{`.
			for i, c := range closeInfo.Leading {
				n := 1
				if i > 0 {
					n = max(c.NewlinesBefore, 1)
				}
				inner = append(inner, breaks(n), commentDoc(c))
			}
		} else {
			inner = append(inner, p.danglingComments(closeInfo))
		}
	}
	return doc.Concat(p.tok(b.LBrace), doc.Nest(doc.Concat(inner...)), doc.Hardline, p.bare(b.RBrace))
}

// afterHead prints the body of a module definition after its header.
func (p *printer) afterHead(body syntax.Stmt) doc.Doc {
	if e, ok := body.(*syntax.Empty); ok {
		return p.tok(e.Semi)
	}
	return doc.Concat(doc.Space, p.stmt(body))
}

// rhs prints the right-hand side of an assignment or function definition,
// including the space after `=`. Operator expressions may move to the next line.
func (p *printer) rhs(e syntax.Expr) doc.Doc {
	switch e.(type) {
	case *syntax.Binary, *syntax.Ternary, *syntax.Keyword:
		return doc.NewGroup(doc.Nest(doc.Concat(doc.LineBr, p.expr(e))))
	}
	return doc.Concat(doc.Space, p.expr(e))
}

// ---- module calls and if ----

func (p *printer) mods(ms []tok) doc.Doc {
	var parts []doc.Doc
	for _, m := range ms {
		parts = append(parts, p.tok(m))
	}
	return doc.Concat(parts...)
}

// callHead prints `[modifiers] name(args)`.
func (p *printer) callHead(n *syntax.ModuleCall) doc.Doc {
	var sp doc.Doc = doc.Nil
	if n.Name.Kind == lexer.For || n.Name.Kind == lexer.Let {
		sp = doc.Space
	}
	return doc.Concat(p.mods(n.Modifiers), p.tok(n.Name), sp, p.args(n.LParen, n.Args, n.RParen, true))
}

// tail is what ends a chain: `;`, a block, or an if.
type tail struct {
	d    doc.Doc // the text after the last call; for an if, the if itself
	isIf bool    // the if needs a separator from the last call
}

// chain prints a module call together with the calls it controls:
// `translate(a) rotate(b) cube(c);`.
func (p *printer) chain(n *syntax.ModuleCall) doc.Doc {
	links := []*syntax.ModuleCall{n}
	for {
		next, ok := links[len(links)-1].Child.(*syntax.ModuleCall)
		if !ok {
			break
		}
		links = append(links, next)
	}
	last := links[len(links)-1]

	heads := make([]doc.Doc, len(links))
	for i, l := range links {
		heads[i] = p.callHead(l)
	}
	var t tail
	switch c := last.Child.(type) {
	case *syntax.Empty:
		t.d = p.tok(c.Semi)
	case *syntax.Block:
		// Blocks and ifs contain forced breaks, which must not break the chain.
		t.d = doc.Detach{D: doc.Concat(doc.Space, p.block(c))}
	case *syntax.If:
		t = tail{d: doc.Detach{D: p.ifStmt(c)}, isIf: true}
	}
	return p.join(heads, t, p.hasComments(n.First(), last.RParen))
}

func nest(levels int, d doc.Doc) doc.Doc {
	for ; levels > 0; levels-- {
		d = doc.Nest(d)
	}
	return d
}

// join lays out calls (each controlling the next) followed by tail, according
// to the chain option. When the chain breaks, each call is indented one step
// deeper than the one before, and the tail goes with the last call. When it
// stays on one line, the tail is not indented.
func (p *printer) join(heads []doc.Doc, t tail, commented bool) doc.Doc {
	n := len(heads)
	same := p.opts.ChainIndent == ChainIndentSame
	// Indent levels of the last call and of an if that follows it.
	lastLevel, ifLevel := n-1, n
	if same {
		lastLevel, ifLevel = min(n-1, 1), 1
	}
	brokenTail := func() doc.Doc {
		if t.isIf {
			return nest(ifLevel, doc.Concat(doc.Hardline, t.d))
		}
		return nest(lastLevel, t.d)
	}
	flatTail := func() doc.Doc {
		if t.isIf {
			return doc.Concat(doc.Space, t.d)
		}
		return t.d
	}
	build := func(sep doc.Doc) doc.Doc {
		if same {
			rest := make([]doc.Doc, 0, 2*n)
			for _, h := range heads[1:] {
				rest = append(rest, sep, h)
			}
			return doc.Concat(heads[0], doc.Nest(doc.Concat(rest...)))
		}
		d := heads[n-1]
		for i := n - 2; i >= 0; i-- {
			d = doc.Concat(heads[i], doc.Nest(doc.Concat(sep, d)))
		}
		return d
	}
	mode := p.opts.Chain
	if commented && mode == ChainFlat {
		// A line comment must end its line, so a flat chain cannot hold one.
		mode = ChainBreak
	}
	if n == 1 && !t.isIf {
		return doc.Concat(heads[0], t.d)
	}
	switch mode {
	case ChainFlat:
		d := heads[n-1]
		for i := n - 2; i >= 0; i-- {
			d = doc.Concat(heads[i], doc.Space, d)
		}
		return doc.Concat(d, flatTail())
	case ChainBreak:
		return doc.Concat(build(doc.Hardline), brokenTail())
	}
	return doc.NewGroup(build(doc.LineBr), doc.IfBreak{Broken: brokenTail(), Flat: flatTail()})
}

// attach prints head followed by the statement child it controls.
func (p *printer) attach(head doc.Doc, child syntax.Stmt) doc.Doc {
	switch c := child.(type) {
	case *syntax.Empty:
		return doc.Concat(head, p.tok(c.Semi))
	case *syntax.Block:
		return doc.Concat(head, doc.Space, p.block(c))
	}
	return p.join([]doc.Doc{head}, tail{d: doc.Detach{D: p.stmt(child)}, isIf: true}, false)
}

func (p *printer) ifStmt(n *syntax.If) doc.Doc {
	head := doc.Concat(p.mods(n.Modifiers), p.tok(n.If), doc.Space, p.parenExpr(n.LParen, n.Cond, n.RParen))
	out := p.attach(head, n.Then)
	if n.Else == nil {
		return out
	}
	var sep doc.Doc = doc.Hardline
	if _, isBlock := n.Then.(*syntax.Block); isBlock && !p.hasLeading(n.Else) {
		sep = doc.Space
	}
	elseTok := p.tok(n.Else)
	if inner, ok := n.ElseBody.(*syntax.If); ok {
		return doc.Concat(out, sep, elseTok, doc.Space, p.ifStmt(inner))
	}
	return doc.Concat(out, sep, p.attach(elseTok, n.ElseBody))
}
