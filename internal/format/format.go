// Package format turns OpenSCAD source into canonically formatted source.
package format

import (
	"scadfmt/internal/doc"
	"scadfmt/internal/lexer"
	"scadfmt/internal/syntax"
)

// Chain layouts for transform chains such as `translate(a) rotate(b) cube(c);`.
const (
	ChainAuto  = "auto"  // one line if it fits, else one call per line
	ChainFlat  = "flat"  // always one line; only arguments wrap
	ChainBreak = "break" // always one call per line
)

// Options controls formatting output.
type Options struct {
	MaxWidth int
	Indent   int
	UseTabs  bool
	Chain    string // ChainAuto, ChainFlat or ChainBreak
}

// DefaultOptions returns the default formatting options.
func DefaultOptions() Options {
	return Options{MaxWidth: 100, Indent: 2, Chain: ChainAuto}
}

// Format formats src. It fails if src does not lex and parse.
func Format(src []byte, opts Options) ([]byte, error) {
	toks, err := lexer.Lex(src)
	if err != nil {
		return nil, err
	}
	f, err := syntax.Parse(toks)
	if err != nil {
		return nil, err
	}
	p := newPrinter(toks, opts)
	out := doc.Print(p.file(f), doc.Options{
		MaxWidth: opts.MaxWidth,
		Indent:   opts.Indent,
		UseTabs:  opts.UseTabs,
	})
	return []byte(out), nil
}
