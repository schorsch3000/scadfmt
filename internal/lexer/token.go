// Package lexer is a lossless OpenSCAD lexer. Every byte of the input ends up
// in exactly one token or trivia piece, so concatenating them reproduces the
// source. The token set mirrors openscad/src/core/lexer.l.
package lexer

// Kind identifies a token type.
type Kind int

const (
	EOF Kind = iota
	Error

	Ident
	Number
	String  // raw text including quotes and escapes
	Include // raw text: `include <path>`
	Use     // raw text: `use <path>`

	// Keywords.
	Module
	Function
	If
	Else
	Let
	Assert
	Echo
	For
	Each
	True
	False
	Undef

	// Multi-character operators.
	LE  // <=
	GE  // >=
	EQ  // ==
	NEQ // !=
	And // &&
	Or  // ||
	Shl // <<
	Shr // >>

	// Single-character punctuation and operators.
	LParen
	RParen
	LBrace
	RBrace
	LBracket
	RBracket
	Semicolon
	Comma
	Dot
	Colon
	Question
	Assign
	Lt
	Gt
	Plus
	Minus
	Star
	Slash
	Percent
	Caret
	Bang
	Tilde
	Hash
	Amp
	Pipe
	Dollar
)

var kindNames = map[Kind]string{
	EOF: "EOF", Error: "Error", Ident: "Ident", Number: "Number", String: "String",
	Include: "Include", Use: "Use", Module: "module", Function: "function", If: "if",
	Else: "else", Let: "let", Assert: "assert", Echo: "echo", For: "for", Each: "each",
	True: "true", False: "false", Undef: "undef", LE: "<=", GE: ">=", EQ: "==",
	NEQ: "!=", And: "&&", Or: "||", Shl: "<<", Shr: ">>", LParen: "(", RParen: ")",
	LBrace: "{", RBrace: "}", LBracket: "[", RBracket: "]", Semicolon: ";",
	Comma: ",", Dot: ".", Colon: ":", Question: "?", Assign: "=", Lt: "<", Gt: ">",
	Plus: "+", Minus: "-", Star: "*", Slash: "/", Percent: "%", Caret: "^",
	Bang: "!", Tilde: "~", Hash: "#", Amp: "&", Pipe: "|", Dollar: "$",
}

func (k Kind) String() string {
	if s, ok := kindNames[k]; ok {
		return s
	}
	return "Kind(?)"
}

var keywords = map[string]Kind{
	"module": Module, "function": Function, "if": If, "else": Else, "let": Let,
	"assert": Assert, "echo": Echo, "for": For, "each": Each,
	"true": True, "false": False, "undef": Undef,
}

var punct = map[byte]Kind{
	'(': LParen, ')': RParen, '{': LBrace, '}': RBrace, '[': LBracket, ']': RBracket,
	';': Semicolon, ',': Comma, '.': Dot, ':': Colon, '?': Question, '=': Assign,
	'<': Lt, '>': Gt, '+': Plus, '-': Minus, '*': Star, '/': Slash, '%': Percent,
	'^': Caret, '!': Bang, '~': Tilde, '#': Hash, '&': Amp, '|': Pipe, '$': Dollar,
}

// TriviaKind identifies a piece of non-token source text.
type TriviaKind int

const (
	Whitespace   TriviaKind = iota // spaces, tabs, lone \r, NBSP, BOM
	Newline                        // "\n" or "\r\n"
	LineComment                    // // ... (without the newline)
	BlockComment                   // /* ... */
)

// Trivia is source text between tokens.
type Trivia struct {
	Kind TriviaKind
	Text string
	Pos  Pos
}

// Pos is a source position. Offset is a byte offset; Line and Col are 1-based
// and Col counts bytes.
type Pos struct {
	Offset, Line, Col int
}

// Token is a lexeme plus the trivia that precedes it.
type Token struct {
	Kind    Kind
	Text    string
	Pos     Pos
	Leading []Trivia
}
