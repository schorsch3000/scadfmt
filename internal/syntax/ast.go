// Package syntax parses OpenSCAD token streams into a lossless concrete
// syntax tree. Every token of the input is referenced by exactly one node
// field, so comments (which live in token trivia) and original line breaks
// are never lost. The grammar follows openscad/src/core/parser.y.
package syntax

import "scadfmt/internal/lexer"

// Tok is a token in the tree. Pointers refer into the slice returned by
// lexer.Lex and are stable.
type Tok = *lexer.Token

// Node is any tree node. First and Last return the first and last source
// token the node covers.
type Node interface {
	First() Tok
	Last() Tok
}

// Span records the token range of a node. Embedded in every node.
type Span struct{ FirstTok, LastTok Tok }

func (s Span) First() Tok { return s.FirstTok }
func (s Span) Last() Tok  { return s.LastTok }

// Stmt is a statement.
type Stmt interface {
	Node
	stmt()
}

// Expr is an expression, including list-comprehension elements.
type Expr interface {
	Node
	expr()
}

// File is a whole source file.
type File struct {
	Span
	Stmts []Stmt
	EOF   Tok // carries trailing trivia
}

// ---- Statements ----

// Empty is a lone `;`.
type Empty struct {
	Span
	Semi Tok
}

// Block is `{ stmts }`.
type Block struct {
	Span
	LBrace Tok
	Stmts  []Stmt
	RBrace Tok
}

// Include is a whole `include <...>` or `use <...>` token.
type Include struct {
	Span
	Tok Tok
}

// Assignment is `name = value;`.
type Assignment struct {
	Span
	Name  Tok
	Eq    Tok
	Value Expr
	Semi  Tok
}

// ModuleDef is `module name(params) body`.
type ModuleDef struct {
	Span
	Module Tok
	Name   Tok
	LParen Tok
	Params List
	RParen Tok
	Body   Stmt
}

// FunctionDef is `function name(params) = body;`.
type FunctionDef struct {
	Span
	Function Tok
	Name     Tok
	LParen   Tok
	Params   List
	RParen   Tok
	Eq       Tok
	Body     Expr
	Semi     Tok
}

// ModuleCall is `[modifiers] name(args) child`. Name is an identifier or one
// of the keywords for, let, assert, echo, each. Child is Empty, Block or
// another ModuleCall/If.
type ModuleCall struct {
	Span
	Modifiers []Tok // ! # % *
	Name      Tok
	LParen    Tok
	Args      List
	RParen    Tok
	Child     Stmt
}

// If is `[modifiers] if (cond) then [else other]`.
type If struct {
	Span
	Modifiers []Tok
	If        Tok
	LParen    Tok
	Cond      Expr
	RParen    Tok
	Then      Stmt
	Else      Tok // nil if absent
	ElseBody  Stmt
}

// ---- Lists ----

// Arg is a call argument or a parameter. For parameters Name is always set and
// Value is the optional default.
type Arg struct {
	Span
	Name  Tok // nil for positional arguments
	Eq    Tok
	Value Expr // nil only for a parameter without default
}

// List is a comma separated list of arguments or parameters. Commas[i] follows
// Items[i]; len(Commas) == len(Items) means a trailing comma.
type List struct {
	Items  []*Arg
	Commas []Tok
}

// ---- Expressions ----

type (
	// Literal is a number, string, true, false or undef.
	Literal struct {
		Span
		Tok Tok
	}
	// Ident is a variable reference.
	Ident struct {
		Span
		Tok Tok
	}
	// Paren is `( x )`.
	Paren struct {
		Span
		LParen Tok
		X      Expr
		RParen Tok
	}
	// Unary is a prefix operator: + - ! ~.
	Unary struct {
		Span
		Op Tok
		X  Expr
	}
	// Binary is an infix operator, including ^.
	Binary struct {
		Span
		L  Expr
		Op Tok
		R  Expr
	}
	// Ternary is `cond ? then : else`.
	Ternary struct {
		Span
		Cond     Expr
		Question Tok
		Then     Expr
		Colon    Tok
		Else     Expr
	}
	// Call is `fn(args)`.
	Call struct {
		Span
		Fn     Expr
		LParen Tok
		Args   List
		RParen Tok
	}
	// Index is `x[i]`.
	Index struct {
		Span
		X        Expr
		LBracket Tok
		Idx      Expr
		RBracket Tok
	}
	// Member is `x.name`.
	Member struct {
		Span
		X    Expr
		Dot  Tok
		Name Tok
	}
	// Vector is `[a, b, c]`. Elems may be list-comprehension nodes. Commas
	// follows the List convention.
	Vector struct {
		Span
		LBracket Tok
		Elems    []Expr
		Commas   []Tok
		RBracket Tok
	}
	// Range is `[a:b]` or `[a:s:b]`; Parts has 2 or 3 entries and Colons one
	// fewer.
	Range struct {
		Span
		LBracket Tok
		Parts    []Expr
		Colons   []Tok
		RBracket Tok
	}
	// FuncLit is an anonymous `function(params) body`.
	FuncLit struct {
		Span
		Function Tok
		LParen   Tok
		Params   List
		RParen   Tok
		Body     Expr
	}
	// Keyword is the expression forms `let(args) body`, `assert(args) [body]`
	// and `echo(args) [body]`. Body is nil if absent (assert/echo only).
	Keyword struct {
		Span
		Kw     Tok
		LParen Tok
		Args   List
		RParen Tok
		Body   Expr
	}
	// LcLet is `let(args) element` inside a vector.
	LcLet struct {
		Span
		Let    Tok
		LParen Tok
		Args   List
		RParen Tok
		Body   Expr
	}
	// LcEach is `each element`.
	LcEach struct {
		Span
		Each Tok
		X    Expr
	}
	// LcFor is `for (args) element`, or the C-style
	// `for (init; cond; step) element` when Semi1 is set.
	LcFor struct {
		Span
		For    Tok
		LParen Tok
		Args   List // loop variables, or init for the C-style form
		Semi1  Tok
		Cond   Expr
		Semi2  Tok
		Step   List
		RParen Tok
		Body   Expr
	}
	// LcIf is `if (cond) then [else other]` inside a vector.
	LcIf struct {
		Span
		If       Tok
		LParen   Tok
		Cond     Expr
		RParen   Tok
		Then     Expr
		Else     Tok
		ElseBody Expr
	}
)

func (*Empty) stmt()       {}
func (*Block) stmt()       {}
func (*Include) stmt()     {}
func (*Assignment) stmt()  {}
func (*ModuleDef) stmt()   {}
func (*FunctionDef) stmt() {}
func (*ModuleCall) stmt()  {}
func (*If) stmt()          {}

func (*Literal) expr() {}
func (*Ident) expr()   {}
func (*Paren) expr()   {}
func (*Unary) expr()   {}
func (*Binary) expr()  {}
func (*Ternary) expr() {}
func (*Call) expr()    {}
func (*Index) expr()   {}
func (*Member) expr()  {}
func (*Vector) expr()  {}
func (*Range) expr()   {}
func (*FuncLit) expr() {}
func (*Keyword) expr() {}
func (*LcLet) expr()   {}
func (*LcEach) expr()  {}
func (*LcFor) expr()   {}
func (*LcIf) expr()    {}
