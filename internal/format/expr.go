package format

import (
	"scadfmt/internal/doc"
	"scadfmt/internal/lexer"
	"scadfmt/internal/syntax"
)

// delimited prints `open item, item, ... close` as a group: on one line if it
// fits, else one item per line. A trailing comma present in the source is kept
// only when the list is broken. With hug, a single bracketed item stays
// attached to the delimiters instead of moving to its own line.
func (p *printer) delimited(open tok, items []doc.Doc, commas []tok, close tok, hug bool) doc.Doc {
	inside, closeDoc := p.closer(close)
	if len(items) == 0 {
		if isNil(inside) {
			return doc.Concat(p.tok(open), closeDoc)
		}
		return doc.NewGroup(p.tok(open), doc.Nest(inside), doc.Hardline, closeDoc)
	}
	if hug && len(items) == 1 && len(commas) == 0 && isNil(inside) {
		return doc.Concat(p.tok(open), items[0], closeDoc)
	}
	var body []doc.Doc
	for i, it := range items {
		body = append(body, it)
		switch {
		case i < len(items)-1:
			body = append(body, p.tok(commas[i]), doc.LineBr)
		case len(commas) == len(items):
			c := commas[i]
			if len(p.info[c].Leading) > 0 || len(p.info[c].Trailing) > 0 {
				body = append(body, p.tok(c))
			} else {
				body = append(body, doc.IfBreak{Broken: doc.Text(",")})
			}
		}
	}
	return doc.NewGroup(
		p.tok(open),
		doc.Nest(doc.Concat(doc.Softline, doc.Concat(body...), inside)),
		doc.Softline,
		closeDoc,
	)
}

func isNil(d doc.Doc) bool {
	c, ok := d.(doc.Cat)
	return ok && len(c) == 0
}

// args prints a parameter or argument list including its parentheses.
func (p *printer) args(lp tok, l syntax.List, rp tok, hug bool) doc.Doc {
	items := make([]doc.Doc, len(l.Items))
	for i, a := range l.Items {
		items[i] = p.arg(a)
	}
	hugOK := hug && len(l.Items) == 1 && l.Items[0].Name == nil && isVector(l.Items[0].Value)
	return p.delimited(lp, items, l.Commas, rp, hugOK)
}

func isVector(e syntax.Expr) bool {
	_, ok := e.(*syntax.Vector)
	return ok
}

func (p *printer) arg(a *syntax.Arg) doc.Doc {
	switch {
	case a.Name != nil && a.Value != nil:
		return doc.Concat(p.tok(a.Name), doc.Space, p.tok(a.Eq), doc.Space, p.expr(a.Value))
	case a.Name != nil:
		return p.tok(a.Name)
	}
	return p.expr(a.Value)
}

// parenExpr prints `( expr )`, breaking inside the parentheses if needed.
func (p *printer) parenExpr(lp tok, e syntax.Expr, rp tok) doc.Doc {
	inside, closeDoc := p.closer(rp)
	return doc.NewGroup(p.tok(lp), doc.Nest(doc.Concat(doc.Softline, p.expr(e), inside)), doc.Softline, closeDoc)
}

// ---- expressions ----

func (p *printer) expr(e syntax.Expr) doc.Doc {
	switch n := e.(type) {
	case *syntax.Literal:
		return p.tok(n.Tok)
	case *syntax.Ident:
		return p.tok(n.Tok)
	case *syntax.Paren:
		return p.parenExpr(n.LParen, n.X, n.RParen)
	case *syntax.Unary:
		return doc.Concat(p.tok(n.Op), p.expr(n.X))
	case *syntax.Binary:
		return p.binary(n)
	case *syntax.Ternary:
		return p.ternary(n)
	case *syntax.Call:
		return doc.Concat(p.expr(n.Fn), p.args(n.LParen, n.Args, n.RParen, true))
	case *syntax.Index:
		return doc.Concat(p.expr(n.X), p.tok(n.LBracket), p.expr(n.Idx), p.tok(n.RBracket))
	case *syntax.Member:
		return doc.Concat(p.expr(n.X), p.tok(n.Dot), p.tok(n.Name))
	case *syntax.Vector:
		items := make([]doc.Doc, len(n.Elems))
		for i, x := range n.Elems {
			items[i] = p.expr(x)
		}
		return p.delimited(n.LBracket, items, n.Commas, n.RBracket, false)
	case *syntax.Range:
		return p.rangeExpr(n)
	case *syntax.FuncLit:
		return doc.Concat(
			p.tok(n.Function), p.args(n.LParen, n.Params, n.RParen, false),
			doc.Space, p.expr(n.Body),
		)
	case *syntax.Keyword:
		return p.keyword(n.Kw, n.LParen, n.Args, n.RParen, n.Body)
	case *syntax.LcLet:
		return p.keyword(n.Let, n.LParen, n.Args, n.RParen, n.Body)
	case *syntax.LcEach:
		return doc.Concat(p.tok(n.Each), doc.Space, p.expr(n.X))
	case *syntax.LcFor:
		return p.lcFor(n)
	case *syntax.LcIf:
		return p.lcIf(n)
	}
	panic("unreachable: unknown expression")
}

// keyword prints let/assert/echo with their body.
func (p *printer) keyword(kw, lp tok, args syntax.List, rp tok, body syntax.Expr) doc.Doc {
	var sp doc.Doc = doc.Nil
	if kw.Kind == lexer.Let {
		sp = doc.Space
	}
	head := doc.Concat(p.tok(kw), sp, p.args(lp, args, rp, false))
	if body == nil {
		return head
	}
	return doc.NewGroup(head, doc.LineBr, p.expr(body))
}

func prec(k lexer.Kind) int {
	switch k {
	case lexer.Or:
		return 1
	case lexer.And:
		return 2
	case lexer.EQ, lexer.NEQ:
		return 3
	case lexer.Gt, lexer.GE, lexer.Lt, lexer.LE:
		return 4
	case lexer.Pipe:
		return 5
	case lexer.Amp:
		return 6
	case lexer.Shl, lexer.Shr:
		return 7
	case lexer.Plus, lexer.Minus:
		return 8
	case lexer.Star, lexer.Slash, lexer.Percent:
		return 9
	}
	return 10 // ^
}

// binary prints a chain of left-associative operators of equal precedence as
// one group, so it breaks after every operator or none.
func (p *printer) binary(n *syntax.Binary) doc.Doc {
	if n.Op.Kind == lexer.Caret {
		return doc.Concat(p.expr(n.L), doc.Space, p.tok(n.Op), doc.Space, p.expr(n.R))
	}
	var operands []syntax.Expr
	var ops []tok
	var walk func(b *syntax.Binary)
	walk = func(b *syntax.Binary) {
		if l, ok := b.L.(*syntax.Binary); ok && l.Op.Kind != lexer.Caret && prec(l.Op.Kind) == prec(b.Op.Kind) {
			walk(l)
		} else {
			operands = append(operands, b.L)
		}
		ops = append(ops, b.Op)
		operands = append(operands, b.R)
	}
	walk(n)
	rest := make([]doc.Doc, 0, 4*len(ops))
	for i, op := range ops {
		rest = append(rest, doc.Space, p.tok(op), doc.LineBr, p.expr(operands[i+1]))
	}
	return doc.NewGroup(p.expr(operands[0]), doc.Nest(doc.Concat(rest...)))
}

// ternary prints `c ? a : b`, putting `?` and `:` at the start of the
// continuation lines when it breaks. Chained else-branches stay flat.
func (p *printer) ternary(n *syntax.Ternary) doc.Doc {
	parts := []doc.Doc{p.expr(n.Cond)}
	var rest []doc.Doc
	for {
		rest = append(rest, doc.LineBr, p.tok(n.Question), doc.Space, p.expr(n.Then), doc.LineBr, p.tok(n.Colon), doc.Space)
		next, ok := n.Else.(*syntax.Ternary)
		if !ok {
			rest = append(rest, p.expr(n.Else))
			break
		}
		rest = append(rest, p.expr(next.Cond))
		n = next
	}
	return doc.NewGroup(parts[0], doc.Nest(doc.Concat(rest...)))
}

// rangeExpr prints `[a:b]` / `[a:s:b]`. Simple parts stay tight, `[0:10]`;
// anything else gets spaces around the colons, `[0 : n - 1]`.
func (p *printer) rangeExpr(n *syntax.Range) doc.Doc {
	simple := true
	for _, x := range n.Parts {
		if !isSimple(x) {
			simple = false
		}
	}
	var sep doc.Doc = doc.Nil
	if !simple {
		sep = doc.Space
	}
	parts := []doc.Doc{p.tok(n.LBracket), p.expr(n.Parts[0])}
	for i, c := range n.Colons {
		parts = append(parts, sep, p.tok(c), sep, p.expr(n.Parts[i+1]))
	}
	parts = append(parts, p.tok(n.RBracket))
	return doc.Concat(parts...)
}

func isSimple(e syntax.Expr) bool {
	switch x := e.(type) {
	case *syntax.Literal, *syntax.Ident:
		return true
	case *syntax.Unary:
		return isSimple(x.X)
	}
	return false
}

func (p *printer) lcFor(n *syntax.LcFor) doc.Doc {
	var head doc.Doc
	if n.Semi1 == nil {
		head = p.args(n.LParen, n.Args, n.RParen, false)
	} else {
		// C-style: `for (init; cond; step)`, kept on one line.
		head = doc.Concat(
			p.tok(n.LParen), p.flatList(n.Args), p.tokBreak(n.Semi1), doc.Space,
			p.expr(n.Cond), p.tokBreak(n.Semi2), doc.Space, p.flatList(n.Step), p.tok(n.RParen),
		)
	}
	return doc.NewGroup(p.tok(n.For), doc.Space, head, doc.Nest(doc.Concat(doc.LineBr, p.expr(n.Body))))
}

func (p *printer) flatList(l syntax.List) doc.Doc {
	var parts []doc.Doc
	for i, a := range l.Items {
		parts = append(parts, p.arg(a))
		if i < len(l.Commas) {
			parts = append(parts, p.tok(l.Commas[i]))
			if i < len(l.Items)-1 {
				parts = append(parts, doc.Space)
			}
		}
	}
	return doc.Concat(parts...)
}

func (p *printer) lcIf(n *syntax.LcIf) doc.Doc {
	parts := []doc.Doc{
		p.tok(n.If), doc.Space, p.parenExpr(n.LParen, n.Cond, n.RParen),
		doc.Nest(doc.Concat(doc.LineBr, p.expr(n.Then))),
	}
	if n.Else != nil {
		parts = append(parts, doc.LineBr, p.tok(n.Else), doc.Nest(doc.Concat(doc.LineBr, p.expr(n.ElseBody))))
	}
	return doc.NewGroup(parts...)
}
