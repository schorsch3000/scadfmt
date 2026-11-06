// Package format turns OpenSCAD source into canonically formatted source.
package format

import (
	"fmt"

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

// How the calls of a broken chain are indented.
const (
	ChainIndentStep = "step" // each call one step deeper than the one before
	ChainIndentSame = "same" // all calls after the first at the same depth
)

// Matrix layouts for vectors and argument lists written over several lines.
const (
	MatrixPreserve = "preserve" // keep the source's line structure
	MatrixReflow   = "reflow"   // lay out by width only
	MatrixAlign    = "align"    // preserve, and align columns (not yet implemented)
)

// Options controls formatting output.
type Options struct {
	MaxWidth int
	Indent   int
	UseTabs  bool
	Chain    string // ChainAuto, ChainFlat or ChainBreak

	// ChainIndent is ChainIndentStep or ChainIndentSame. It only matters when a
	// chain breaks over several lines.
	ChainIndent string
	Matrix      string // MatrixPreserve, MatrixReflow or MatrixAlign

	// Alignment. All off by default. Each pads with spaces only and never
	// changes the text of a token. AlignDecimal needs AlignAssignments.
	AlignAssignments      bool // align `=` in consecutive assignments
	AlignDecimal          bool // align numeric values on the decimal point
	AlignParams           bool // align `=` in broken parameter/argument lists
	AlignTrailingComments bool // align `//` comments after consecutive statements
	AlignMaxGap           int  // split an alignment group when widths differ by more than this; <= 0 means no limit

	// NormalizeNumbers rewrites number literals: `.5` becomes `0.5`, `1.50`
	// becomes `1.5` and `1.` becomes `1.0`. Hex and exponent parts are kept.
	NormalizeNumbers bool
}

// DefaultOptions returns the default formatting options.
func DefaultOptions() Options {
	return Options{MaxWidth: 100, Indent: 2, Chain: ChainAuto, ChainIndent: ChainIndentStep, Matrix: MatrixPreserve, AlignMaxGap: 8}
}

// Format formats src. It fails if src does not lex and parse. It also checks
// its own output: if the result is not the same program as src, or a comment
// was lost, it returns an error instead of the result.
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
	if err := verify(toks, f, []byte(out), opts); err != nil {
		return nil, fmt.Errorf("internal error, refusing to change the file (please report this): %w", err)
	}
	return []byte(out), nil
}
