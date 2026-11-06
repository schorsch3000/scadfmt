package syntax

import (
	"fmt"

	"scadfmt/internal/lexer"
)

// K is shorthand for token kinds used throughout the parser.
type K = lexer.Kind

// Error is a syntax error with the position of the offending token.
type Error struct {
	Pos lexer.Pos
	Msg string
}

func (e *Error) Error() string { return fmt.Sprintf("%d:%d: %s", e.Pos.Line, e.Pos.Col, e.Msg) }

type parser struct {
	toks []lexer.Token
	pos  int
}

// Parse parses the tokens returned by lexer.Lex. Parsing stops at the first
// error: there is no recovery, because rewriting a file that was not
// understood would be unsafe.
func Parse(toks []lexer.Token) (f *File, err error) {
	p := &parser{toks: toks}
	defer func() {
		if r := recover(); r != nil {
			e, ok := r.(*Error)
			if !ok {
				panic(r)
			}
			f, err = nil, e
		}
	}()
	f = &File{}
	for p.kind() != lexer.EOF {
		f.Stmts = append(f.Stmts, p.stmt())
	}
	f.EOF = p.advance()
	f.FirstTok, f.LastTok = &toks[0], f.EOF
	return f, nil
}

// ---- token helpers ----

func (p *parser) cur() Tok { return &p.toks[p.pos] }
func (p *parser) kind() K  { return p.toks[p.pos].Kind }
func (p *parser) kindAt(n int) K {
	if p.pos+n >= len(p.toks) {
		return lexer.EOF
	}
	return p.toks[p.pos+n].Kind
}

func (p *parser) advance() Tok {
	t := p.cur()
	if t.Kind != lexer.EOF {
		p.pos++
	}
	return t
}

func (p *parser) fail(format string, args ...any) {
	panic(&Error{p.cur().Pos, fmt.Sprintf(format, args...)})
}

func (p *parser) describe() string {
	t := p.cur()
	if t.Kind == lexer.EOF {
		return "end of file"
	}
	return fmt.Sprintf("%q", t.Text)
}

func (p *parser) expect(k K) Tok {
	if p.kind() != k {
		p.fail("unexpected %s, expected %q", p.describe(), k.String())
	}
	return p.advance()
}

func (p *parser) accept(k K) Tok {
	if p.kind() == k {
		return p.advance()
	}
	return nil
}

func (s *Span) set(a, b Tok) { s.FirstTok, s.LastTok = a, b }

type spanner interface{ set(a, b Tok) }

// finish sets the span of n to the tokens consumed since start.
func finish[T spanner](p *parser, n T, start int) T {
	n.set(&p.toks[start], &p.toks[p.pos-1])
	return n
}

// try runs f and rewinds on failure.
func (p *parser) try(f func() Expr) (e Expr) {
	save := p.pos
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(*Error); !ok {
				panic(r)
			}
			p.pos, e = save, nil
		}
	}()
	return f()
}

// ---- statements ----

func (p *parser) stmt() Stmt {
	switch p.kind() {
	case lexer.Semicolon:
		s := p.pos
		return finish(p, &Empty{Semi: p.advance()}, s)
	case lexer.LBrace:
		return p.block()
	case lexer.Include, lexer.Use:
		s := p.pos
		return finish(p, &Include{Tok: p.advance()}, s)
	case lexer.Module:
		return p.moduleDef()
	case lexer.Function:
		return p.functionDef()
	case lexer.Ident:
		if p.kindAt(1) == lexer.Assign {
			return p.assignment()
		}
	}
	return p.instantiation()
}

func (p *parser) block() *Block {
	s := p.pos
	b := &Block{LBrace: p.expect(lexer.LBrace)}
	for p.kind() != lexer.RBrace {
		if p.kind() == lexer.EOF {
			p.fail("unexpected end of file, expected \"}\"")
		}
		b.Stmts = append(b.Stmts, p.stmt())
	}
	b.RBrace = p.advance()
	return finish(p, b, s)
}

func (p *parser) assignment() *Assignment {
	s := p.pos
	a := &Assignment{Name: p.expect(lexer.Ident), Eq: p.expect(lexer.Assign)}
	a.Value = p.expr()
	a.Semi = p.expect(lexer.Semicolon)
	return finish(p, a, s)
}

func (p *parser) moduleDef() *ModuleDef {
	s := p.pos
	m := &ModuleDef{Module: p.advance(), Name: p.expect(lexer.Ident)}
	m.LParen = p.expect(lexer.LParen)
	m.Params = p.params()
	m.RParen = p.expect(lexer.RParen)
	m.Body = p.stmt()
	return finish(p, m, s)
}

func (p *parser) functionDef() *FunctionDef {
	s := p.pos
	f := &FunctionDef{Function: p.advance(), Name: p.expect(lexer.Ident)}
	f.LParen = p.expect(lexer.LParen)
	f.Params = p.params()
	f.RParen = p.expect(lexer.RParen)
	f.Eq = p.expect(lexer.Assign)
	f.Body = p.expr()
	f.Semi = p.expect(lexer.Semicolon)
	return finish(p, f, s)
}

func isModifier(k K) bool {
	return k == lexer.Bang || k == lexer.Hash || k == lexer.Percent || k == lexer.Star
}

// instantiation parses a module call or if statement with optional modifiers.
func (p *parser) instantiation() Stmt {
	s := p.pos
	var mods []Tok
	for isModifier(p.kind()) {
		mods = append(mods, p.advance())
	}
	if p.kind() == lexer.If {
		n := &If{Modifiers: mods, If: p.advance()}
		n.LParen = p.expect(lexer.LParen)
		n.Cond = p.expr()
		n.RParen = p.expect(lexer.RParen)
		n.Then = p.child()
		if p.kind() == lexer.Else {
			n.Else = p.advance()
			n.ElseBody = p.child()
		}
		return finish(p, n, s)
	}
	switch p.kind() {
	case lexer.Ident, lexer.For, lexer.Let, lexer.Assert, lexer.Echo, lexer.Each:
	default:
		p.fail("unexpected %s, expected a statement", p.describe())
	}
	n := &ModuleCall{Modifiers: mods, Name: p.advance()}
	n.LParen = p.expect(lexer.LParen)
	n.Args = p.args()
	n.RParen = p.expect(lexer.RParen)
	n.Child = p.child()
	return finish(p, n, s)
}

// child parses the statement controlled by a module call or if.
func (p *parser) child() Stmt {
	switch p.kind() {
	case lexer.Semicolon:
		s := p.pos
		return finish(p, &Empty{Semi: p.advance()}, s)
	case lexer.LBrace:
		return p.block()
	}
	return p.instantiation()
}

// ---- lists ----

func (p *parser) params() List {
	var l List
	for p.kind() == lexer.Ident {
		s := p.pos
		a := &Arg{Name: p.advance()}
		if p.kind() == lexer.Assign {
			a.Eq = p.advance()
			a.Value = p.expr()
		}
		l.Items = append(l.Items, finish(p, a, s))
		c := p.accept(lexer.Comma)
		if c == nil {
			break
		}
		l.Commas = append(l.Commas, c)
	}
	return l
}

func (p *parser) args() List {
	var l List
	for p.kind() != lexer.RParen && p.kind() != lexer.Semicolon && p.kind() != lexer.EOF {
		s := p.pos
		a := &Arg{}
		if p.kind() == lexer.Ident && p.kindAt(1) == lexer.Assign {
			a.Name, a.Eq = p.advance(), p.advance()
		}
		a.Value = p.expr()
		l.Items = append(l.Items, finish(p, a, s))
		c := p.accept(lexer.Comma)
		if c == nil {
			break
		}
		l.Commas = append(l.Commas, c)
	}
	return l
}

// ---- expressions ----

func canStartExpr(k K) bool {
	switch k {
	case lexer.Ident, lexer.Number, lexer.String, lexer.True, lexer.False, lexer.Undef,
		lexer.LParen, lexer.LBracket, lexer.Plus, lexer.Minus, lexer.Bang, lexer.Tilde,
		lexer.Function, lexer.Let, lexer.Assert, lexer.Echo:
		return true
	}
	return false
}

func (p *parser) expr() Expr {
	s := p.pos
	switch p.kind() {
	case lexer.Function:
		if p.kindAt(1) != lexer.LParen {
			p.fail("unexpected %s", p.describe())
		}
		n := &FuncLit{Function: p.advance(), LParen: p.advance()}
		n.Params = p.params()
		n.RParen = p.expect(lexer.RParen)
		n.Body = p.expr()
		return finish(p, n, s)
	case lexer.Let, lexer.Assert, lexer.Echo:
		n := &Keyword{Kw: p.advance()}
		n.LParen = p.expect(lexer.LParen)
		n.Args = p.args()
		n.RParen = p.expect(lexer.RParen)
		if n.Kw.Kind == lexer.Let || canStartExpr(p.kind()) {
			n.Body = p.expr()
		}
		return finish(p, n, s)
	}
	c := p.binary(0)
	if p.kind() == lexer.Question {
		t := &Ternary{Cond: c, Question: p.advance()}
		t.Then = p.expr()
		t.Colon = p.expect(lexer.Colon)
		t.Else = p.expr()
		return finish(p, t, s)
	}
	return c
}

// Binary operator levels, loosest first. All are left associative.
var levels = [][]K{
	{lexer.Or},
	{lexer.And},
	{lexer.EQ, lexer.NEQ},
	{lexer.Gt, lexer.GE, lexer.Lt, lexer.LE},
	{lexer.Pipe},
	{lexer.Amp},
	{lexer.Shl, lexer.Shr},
	{lexer.Plus, lexer.Minus},
	{lexer.Star, lexer.Slash, lexer.Percent},
}

func (p *parser) binary(level int) Expr {
	if level == len(levels) {
		return p.unary()
	}
	s := p.pos
	l := p.binary(level + 1)
	for isOneOf(p.kind(), levels[level]) {
		op := p.advance()
		r := p.binary(level + 1)
		l = finish(p, &Binary{L: l, Op: op, R: r}, s)
	}
	return l
}

func isOneOf(k K, ks []K) bool {
	for _, x := range ks {
		if k == x {
			return true
		}
	}
	return false
}

func (p *parser) unary() Expr {
	s := p.pos
	switch p.kind() {
	case lexer.Plus, lexer.Minus, lexer.Bang, lexer.Tilde:
		op := p.advance()
		return finish(p, &Unary{Op: op, X: p.unary()}, s)
	}
	c := p.call()
	if p.kind() == lexer.Caret {
		op := p.advance()
		return finish(p, &Binary{L: c, Op: op, R: p.unary()}, s)
	}
	return c
}

func (p *parser) call() Expr {
	s := p.pos
	e := p.primary()
	for {
		switch p.kind() {
		case lexer.LParen:
			n := &Call{Fn: e, LParen: p.advance()}
			n.Args = p.args()
			n.RParen = p.expect(lexer.RParen)
			e = finish(p, n, s)
		case lexer.LBracket:
			n := &Index{X: e, LBracket: p.advance()}
			n.Idx = p.expr()
			n.RBracket = p.expect(lexer.RBracket)
			e = finish(p, n, s)
		case lexer.Dot:
			n := &Member{X: e, Dot: p.advance()}
			n.Name = p.expect(lexer.Ident)
			e = finish(p, n, s)
		default:
			return e
		}
	}
}

func (p *parser) primary() Expr {
	s := p.pos
	switch p.kind() {
	case lexer.True, lexer.False, lexer.Undef, lexer.Number, lexer.String:
		return finish(p, &Literal{Tok: p.advance()}, s)
	case lexer.Ident:
		return finish(p, &Ident{Tok: p.advance()}, s)
	case lexer.LParen:
		n := &Paren{LParen: p.advance()}
		n.X = p.expr()
		n.RParen = p.expect(lexer.RParen)
		return finish(p, n, s)
	case lexer.LBracket:
		return p.bracket()
	}
	p.fail("unexpected %s, expected an expression", p.describe())
	return nil
}

// bracket parses a vector or range.
func (p *parser) bracket() Expr {
	s := p.pos
	lb := p.advance()
	if p.kind() == lexer.RBracket {
		return finish(p, &Vector{LBracket: lb, RBracket: p.advance()}, s)
	}
	first := p.vectorElement()
	if p.kind() == lexer.Colon && !isLc(first) {
		r := &Range{LBracket: lb, Parts: []Expr{first}}
		for p.kind() == lexer.Colon && len(r.Parts) < 3 {
			r.Colons = append(r.Colons, p.advance())
			r.Parts = append(r.Parts, p.expr())
		}
		if len(r.Parts) < 2 {
			p.fail("incomplete range")
		}
		r.RBracket = p.expect(lexer.RBracket)
		return finish(p, r, s)
	}
	v := &Vector{LBracket: lb, Elems: []Expr{first}}
	for p.kind() == lexer.Comma {
		v.Commas = append(v.Commas, p.advance())
		if p.kind() == lexer.RBracket {
			break
		}
		v.Elems = append(v.Elems, p.vectorElement())
	}
	v.RBracket = p.expect(lexer.RBracket)
	return finish(p, v, s)
}

// ---- list comprehensions ----

func isLc(e Expr) bool {
	switch e.(type) {
	case *LcLet, *LcEach, *LcFor, *LcIf:
		return true
	}
	return false
}

func (p *parser) vectorElement() Expr {
	s := p.pos
	switch p.kind() {
	case lexer.Each:
		n := &LcEach{Each: p.advance()}
		n.X = p.vectorElement()
		return finish(p, n, s)
	case lexer.For:
		return p.lcFor()
	case lexer.If:
		n := &LcIf{If: p.advance()}
		n.LParen = p.expect(lexer.LParen)
		n.Cond = p.expr()
		n.RParen = p.expect(lexer.RParen)
		n.Then = p.vectorElement()
		if p.kind() == lexer.Else {
			n.Else = p.advance()
			n.ElseBody = p.vectorElement()
		}
		return finish(p, n, s)
	case lexer.Let:
		// `let(..) body` is a list comprehension only if its body is one;
		// otherwise it is a plain let expression.
		if e := p.try(p.lcLet); e != nil {
			return e
		}
	case lexer.LParen:
		switch p.kindAt(1) {
		case lexer.For, lexer.Each, lexer.If, lexer.Let:
			if e := p.try(p.lcParen); e != nil {
				return e
			}
		}
	}
	return p.expr()
}

func (p *parser) lcFor() Expr {
	s := p.pos
	n := &LcFor{For: p.advance()}
	n.LParen = p.expect(lexer.LParen)
	n.Args = p.args()
	if p.kind() == lexer.Semicolon {
		n.Semi1 = p.advance()
		n.Cond = p.expr()
		n.Semi2 = p.expect(lexer.Semicolon)
		n.Step = p.args()
	}
	n.RParen = p.expect(lexer.RParen)
	n.Body = p.vectorElement()
	return finish(p, n, s)
}

func (p *parser) lcLet() Expr {
	s := p.pos
	n := &LcLet{Let: p.advance()}
	n.LParen = p.expect(lexer.LParen)
	n.Args = p.args()
	n.RParen = p.expect(lexer.RParen)
	if p.kind() == lexer.LParen {
		n.Body = p.lcParen()
	} else {
		n.Body = p.vectorElement()
		if !isLc(n.Body) {
			p.fail("let body is not a list comprehension element")
		}
	}
	return finish(p, n, s)
}

func (p *parser) lcParen() Expr {
	s := p.pos
	n := &Paren{LParen: p.expect(lexer.LParen)}
	n.X = p.vectorElement()
	if !isLc(n.X) {
		p.fail("not a list comprehension element")
	}
	n.RParen = p.expect(lexer.RParen)
	return finish(p, n, s)
}
