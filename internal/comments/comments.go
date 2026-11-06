// Package comments decides which token each comment belongs to. The lexer
// leaves comments in the Leading trivia of the token that follows them; Attach
// reassigns them:
//
//   - A comment on the same line as a token, with a line break after it
//     (`x = 1; // note`), is a Trailing comment of that token.
//   - Every other comment is a Leading comment of the token after it. That
//     includes comments before `}`, `)` and end of file, which is where a
//     "dangling" comment would otherwise sit.
//
// Line breaks are kept as counts so the printer can preserve single blank
// lines and tell own-line comments from inline ones.
package comments

import "github.com/schorsch3000/scadfmt/internal/lexer"

// Comment is one `//` or `/* */` comment.
type Comment struct {
	Text  string // raw text; line comments exclude the line break
	Block bool   // /* */ comment, which may span several lines
	Pos   lexer.Pos

	// NewlinesBefore is the number of line breaks between the previous item
	// (the token or comment before this one) and this comment. 0 means they
	// share a line; 2 or more means a blank line precedes the comment.
	NewlinesBefore int
}

// Info describes the comments around one token.
type Info struct {
	// Leading comments come before the token, in source order.
	Leading []Comment

	// NewlinesBefore is the number of line breaks between the last item before
	// the token (previous token, its trailing comments, or the last leading
	// comment) and the token itself. 0 means the token continues that line;
	// 2 or more means a blank line was there.
	NewlinesBefore int

	// Trailing comments follow the token on its line and are followed by a
	// line break. The first one's NewlinesBefore is 0.
	Trailing []Comment
}

// Map holds the Info of every token.
type Map map[*lexer.Token]Info

// Attach classifies the comments of all tokens. toks must be the slice
// returned by lexer.Lex, which ends with an EOF token.
func Attach(toks []lexer.Token) Map {
	m := make(Map, len(toks))
	for i := range toks {
		t := &toks[i]
		trailing, leading, nl := classify(t.Leading, i > 0, t.Kind == lexer.EOF)
		if len(trailing) > 0 {
			prev := &toks[i-1]
			info := m[prev]
			info.Trailing = trailing
			m[prev] = info
		}
		info := m[t]
		info.Leading, info.NewlinesBefore = leading, nl
		m[t] = info
	}
	return m
}

// classify splits one token's leading trivia. nl is the line-break count
// between the last comment (or the previous token when there are none) and
// the token.
func classify(trivia []lexer.Trivia, hasPrev, atEOF bool) (trailing, leading []Comment, nl int) {
	var all []Comment
	for _, t := range trivia {
		switch t.Kind {
		case lexer.Newline:
			nl++
		case lexer.LineComment, lexer.BlockComment:
			all = append(all, Comment{
				Text:           t.Text,
				Block:          t.Kind == lexer.BlockComment,
				Pos:            t.Pos,
				NewlinesBefore: nl,
			})
			nl = 0
		}
	}
	if len(all) == 0 {
		return nil, nil, nl
	}

	// run is the number of comments on the previous token's line.
	run := 1
	for run < len(all) && all[run].NewlinesBefore == 0 {
		run++
	}
	if all[0].NewlinesBefore != 0 {
		run = 0
	}
	lineBreakAfterRun := atEOF || nl > 0 || run < len(all)
	if hasPrev && run > 0 && lineBreakAfterRun {
		return all[:run], all[run:], nl
	}
	return nil, all, nl
}
