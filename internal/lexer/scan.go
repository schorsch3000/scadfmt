package lexer

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

func isDigit(c byte) bool      { return c >= '0' && c <= '9' }
func isHex(c byte) bool        { return isDigit(c) || (c|0x20 >= 'a' && c|0x20 <= 'f') }
func isIdentStart(c byte) bool { return c == '_' || c == '$' || (c|0x20 >= 'a' && c|0x20 <= 'z') }

// identLen returns the length of the longest run at the start of s made of
// identifier-rest characters (ASCII alphanumerics, '_', and non-ASCII letters,
// marks and digits).
func identLen(s string) int {
	n := 0
	for n < len(s) {
		c := s[n]
		if c < 0x80 {
			if c == '_' || isDigit(c) || c|0x20 >= 'a' && c|0x20 <= 'z' {
				n++
				continue
			}
			break
		}
		r, size := utf8.DecodeRuneInString(s[n:])
		if r == utf8.RuneError || r == '\u00a0' || r == '\ufeff' ||
			!(unicode.IsLetter(r) || unicode.IsMark(r) || unicode.IsDigit(r)) {
			break
		}
		n += size
	}
	return n
}

func (l *lexer) lexIdent() {
	r := l.rest()
	// '$' is only a valid first character; the rest follows identifier rules.
	n := 1
	if r[0] >= 0x80 {
		_, n = utf8.DecodeRuneInString(r)
	}
	n += identLen(r[n:])
	if k, ok := keywords[r[:n]]; ok {
		l.token(k, n)
		return
	}
	if (r[:n] == "include" || r[:n] == "use") && l.lexIncludeUse(r, n) {
		return
	}
	l.token(Ident, n)
}

// lexIncludeUse handles `include <...>` and `use <...>`, which lexer.l scans
// as a single unit. The whole construct, whitespace included, becomes one
// token so the path is never re-interpreted. Returns false if the keyword is
// not followed by '<' (then it is a plain identifier).
func (l *lexer) lexIncludeUse(r string, n int) bool {
	i := n
	for i < len(r) && strings.IndexByte(" \t\r\n", r[i]) >= 0 {
		i++
	}
	if i >= len(r) || r[i] != '<' {
		return false
	}
	k := Include
	if r[:n] == "use" {
		k = Use
	}
	end := strings.IndexByte(r[i:], '>')
	if end < 0 {
		l.fail(l.pos(), len(r), "Unterminated "+r[:n]+" statement")
		return true
	}
	l.token(k, i+end+1)
	return true
}

// numberLen returns the length of the longest number literal at the start of
// s, or 0. Forms: 0x1F, 12, 1., .5, 1.5e-3, 1e3.
func numberLen(s string) int {
	if len(s) > 2 && s[0] == '0' && s[1]|0x20 == 'x' && isHex(s[2]) {
		n := 2
		for n < len(s) && isHex(s[n]) {
			n++
		}
		return n
	}
	n := 0
	for n < len(s) && isDigit(s[n]) {
		n++
	}
	intDigits := n
	if n < len(s) && s[n] == '.' {
		m := n + 1
		for m < len(s) && isDigit(s[m]) {
			m++
		}
		if intDigits > 0 || m > n+1 {
			n = m
		}
	}
	if n == 0 {
		return 0
	}
	if n < len(s) && s[n]|0x20 == 'e' {
		m := n + 1
		if m < len(s) && (s[m] == '+' || s[m] == '-') {
			m++
		}
		if m < len(s) && isDigit(s[m]) {
			for m < len(s) && isDigit(s[m]) {
				m++
			}
			n = m
		}
	}
	return n
}

// lexNumberOrIdent mirrors flex's longest-match between the number rules and
// the legacy `{D}{IDREST}*` identifier rule (e.g. `2d`). A tie goes to the
// number.
func (l *lexer) lexNumberOrIdent() {
	r := l.rest()
	num := numberLen(r)
	id := 0
	if isDigit(r[0]) {
		id = identLen(r)
	}
	if id > num {
		l.token(Ident, id)
		return
	}
	l.token(Number, num)
}

func (l *lexer) lexString() {
	r := l.rest()
	for i := 1; i < len(r); i++ {
		switch r[i] {
		case '\\':
			i++ // skip the escaped byte, including a quote
		case '"':
			l.token(String, i+1)
			return
		}
	}
	l.fail(l.pos(), len(r), "Unterminated string")
}

var twoChar = map[string]Kind{
	"<=": LE, ">=": GE, "==": EQ, "!=": NEQ, "&&": And, "||": Or, "<<": Shl, ">>": Shr,
}

func (l *lexer) lexOperator() {
	r := l.rest()
	if len(r) >= 2 {
		if k, ok := twoChar[r[:2]]; ok {
			l.token(k, 2)
			return
		}
	}
	if k, ok := punct[r[0]]; ok {
		l.token(k, 1)
		return
	}
	l.fail(l.pos(), 1, "Unexpected character "+string(rune(r[0])))
}
