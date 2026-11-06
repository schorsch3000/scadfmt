package lexer

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// LexError is a lexing error with its position.
type LexError struct {
	Pos Pos
	Msg string
}

func (e *LexError) Error() string { return fmt.Sprintf("%d:%d: %s", e.Pos.Line, e.Pos.Col, e.Msg) }

// Non-ASCII characters the OpenSCAD lexer treats as whitespace.
const (
	nbsp = "\u00a0"
	bom  = "\ufeff"
)

type lexer struct {
	src       string
	off       int
	line, col int
	toks      []Token
	trivia    []Trivia
	err       error
}

// Lex tokenizes src. The returned slice always ends with an EOF token, which
// carries any trailing trivia. On an unterminated string, comment or include,
// the offending text becomes an Error token and the first such error is
// returned along with the tokens lexed so far.
func Lex(src []byte) ([]Token, error) {
	l := &lexer{src: string(src), line: 1, col: 1}
	for l.off < len(l.src) {
		l.next()
	}
	l.emit(EOF, l.pos(), "")
	return l.toks, l.err
}

func (l *lexer) pos() Pos { return Pos{l.off, l.line, l.col} }

// advance consumes n bytes, tracking line and column.
func (l *lexer) advance(n int) {
	for _, c := range []byte(l.src[l.off : l.off+n]) {
		if c == '\n' {
			l.line++
			l.col = 1
		} else {
			l.col++
		}
	}
	l.off += n
}

func (l *lexer) emit(k Kind, p Pos, text string) {
	l.toks = append(l.toks, Token{Kind: k, Text: text, Pos: p, Leading: l.trivia})
	l.trivia = nil
}

func (l *lexer) addTrivia(k TriviaKind, n int) {
	p := l.pos()
	t := l.src[l.off : l.off+n]
	l.advance(n)
	l.trivia = append(l.trivia, Trivia{Kind: k, Text: t, Pos: p})
}

func (l *lexer) token(k Kind, n int) {
	p := l.pos()
	t := l.src[l.off : l.off+n]
	l.advance(n)
	l.emit(k, p, t)
}

func (l *lexer) fail(p Pos, n int, msg string) {
	if l.err == nil {
		l.err = &LexError{p, msg}
	}
	l.token(Error, n)
}

func (l *lexer) rest() string { return l.src[l.off:] }

func (l *lexer) next() {
	r := l.rest()
	c := r[0]
	switch {
	case c == '\n':
		l.addTrivia(Newline, 1)
	case c == '\r' && strings.HasPrefix(r, "\r\n"):
		l.addTrivia(Newline, 2)
	case c == ' ' || c == '\t' || c == '\r':
		l.addTrivia(Whitespace, 1)
	case c == 0xa0: // lone Latin-1 NBSP, accepted by lexer.l
		l.addTrivia(Whitespace, 1)
	case strings.HasPrefix(r, nbsp):
		l.addTrivia(Whitespace, len(nbsp))
	case strings.HasPrefix(r, bom):
		l.addTrivia(Whitespace, len(bom))
	case strings.HasPrefix(r, "//"):
		n := strings.IndexByte(r, '\n')
		if n < 0 {
			n = len(r)
		}
		// Keep a trailing \r with the newline, not the comment.
		if n > 0 && r[n-1] == '\r' {
			n--
		}
		l.addTrivia(LineComment, n)
	case strings.HasPrefix(r, "/*"):
		n := strings.Index(r[2:], "*/")
		if n < 0 {
			l.fail(l.pos(), len(r), "Unterminated comment")
			return
		}
		l.addTrivia(BlockComment, n+4)
	case c == '"':
		l.lexString()
	case c >= '0' && c <= '9', c == '.' && len(r) > 1 && isDigit(r[1]):
		l.lexNumberOrIdent()
	case isIdentStart(c):
		l.lexIdent()
	case c >= 0x80:
		ru, size := utf8.DecodeRuneInString(r)
		if ru != utf8.RuneError && (unicode.IsLetter(ru) || unicode.IsMark(ru) || unicode.IsDigit(ru)) {
			l.lexIdent()
		} else {
			l.fail(l.pos(), size, fmt.Sprintf("Unexpected character %q", ru))
		}
	default:
		l.lexOperator()
	}
}
