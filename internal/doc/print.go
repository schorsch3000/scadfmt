package doc

import (
	"strings"
	"unicode/utf8"
)

type mode int

const (
	modeBreak mode = iota
	modeFlat
)

type cmd struct {
	indent int // indent levels
	mode   mode
	doc    Doc
}

// Print lays out d and returns the text. Trailing spaces and tabs are removed
// from every line.
func Print(d Doc, opts Options) string {
	if opts.Indent <= 0 {
		opts.Indent = 2
	}
	propagateBreaks(d)

	var out []byte
	var suffix []cmd // pending LineSuffix docs
	col := 0
	stack := []cmd{{0, modeBreak, d}}

	newline := func(indent int) {
		for len(out) > 0 && (out[len(out)-1] == ' ' || out[len(out)-1] == '\t') {
			out = out[:len(out)-1]
		}
		out = append(out, '\n')
		if opts.UseTabs {
			out = append(out, strings.Repeat("\t", indent)...)
		} else {
			out = append(out, strings.Repeat(" ", indent*opts.Indent)...)
		}
		col = indent * opts.Indent
	}

	for len(stack) > 0 {
		c := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		switch x := c.doc.(type) {
		case Text:
			out = append(out, string(x)...)
			col += utf8.RuneCountInString(string(x))
		case Cat:
			for i := len(x) - 1; i >= 0; i-- {
				stack = append(stack, cmd{c.indent, c.mode, x[i]})
			}
		case Indent:
			stack = append(stack, cmd{c.indent + 1, c.mode, x.D})
		case *Group:
			switch {
			case c.mode == modeFlat && !x.Break:
				stack = append(stack, cmd{c.indent, modeFlat, x.D})
			case !x.Break && fits(cmd{c.indent, modeFlat, x.D}, stack, opts.MaxWidth-col, opts):
				stack = append(stack, cmd{c.indent, modeFlat, x.D})
			default:
				stack = append(stack, cmd{c.indent, modeBreak, x.D})
			}
		case IfBreak:
			if c.mode == modeBreak {
				stack = append(stack, cmd{c.indent, c.mode, x.Broken})
			} else {
				stack = append(stack, cmd{c.indent, c.mode, x.Flat})
			}
		case LineSuffix:
			suffix = append(suffix, cmd{c.indent, c.mode, x.D})
		case BreakParent:
		case Detach:
			stack = append(stack, cmd{c.indent, modeBreak, x.D})
		case Fill:
			stack = fillStep(x, c, stack, opts.MaxWidth-col, opts)
		case LiteralLine:
			if len(suffix) > 0 {
				stack = append(stack, c)
				for i := len(suffix) - 1; i >= 0; i-- {
					stack = append(stack, suffix[i])
				}
				suffix = nil
				continue
			}
			out = append(out, '\n')
			col = 0
		case Line:
			if c.mode == modeFlat && !x.Hard {
				if !x.Soft {
					out = append(out, ' ')
					col++
				}
				continue
			}
			if len(suffix) > 0 {
				// Run the deferred docs first, then redo this line break.
				stack = append(stack, c)
				for i := len(suffix) - 1; i >= 0; i-- {
					stack = append(stack, suffix[i])
				}
				suffix = nil
				continue
			}
			newline(c.indent)
		}
		if len(stack) == 0 && len(suffix) > 0 {
			for i := len(suffix) - 1; i >= 0; i-- {
				stack = append(stack, suffix[i])
			}
			suffix = nil
		}
	}
	return strings.TrimRight(string(out), " \t")
}

// fillStep schedules the first content item of a Fill and the separator after
// it, then the rest of the fill. A separator is printed flat when the content
// on both sides of it fits on the current line, and broken otherwise.
func fillStep(x Fill, c cmd, stack []cmd, width int, opts Options) []cmd {
	parts := x.Parts
	if len(parts) == 0 {
		return stack
	}
	content := parts[0]
	flat := func(d Doc) cmd { return cmd{c.indent, modeFlat, d} }
	brk := func(d Doc) cmd { return cmd{c.indent, modeBreak, d} }
	contentFits := fits(flat(content), nil, width, opts)
	if len(parts) == 1 {
		if contentFits {
			return append(stack, flat(content))
		}
		return append(stack, brk(content))
	}
	sep := parts[1]
	if len(parts) == 2 {
		if contentFits {
			return append(stack, flat(sep), flat(content))
		}
		return append(stack, brk(sep), brk(content))
	}
	stack = append(stack, cmd{c.indent, c.mode, Fill{parts[2:]}})
	if fits(flat(Cat{content, sep, parts[2]}), nil, width, opts) {
		return append(stack, flat(sep), flat(content))
	}
	if contentFits {
		return append(stack, brk(sep), flat(content))
	}
	return append(stack, brk(sep), brk(content))
}

// fits reports whether next, followed by the rest of the stack, stays within
// width up to the first line break that will actually be printed as a newline.
func fits(next cmd, rest []cmd, width int, opts Options) bool {
	stack := []cmd{next}
	restIdx := len(rest)
	for width >= 0 {
		if len(stack) == 0 {
			if restIdx == 0 {
				return true
			}
			restIdx--
			stack = append(stack, rest[restIdx])
			continue
		}
		c := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		switch x := c.doc.(type) {
		case Text:
			width -= utf8.RuneCountInString(string(x))
		case Cat:
			for i := len(x) - 1; i >= 0; i-- {
				stack = append(stack, cmd{c.indent, c.mode, x[i]})
			}
		case Indent:
			stack = append(stack, cmd{c.indent + 1, c.mode, x.D})
		case *Group:
			m := c.mode
			if x.Break {
				m = modeBreak
			}
			stack = append(stack, cmd{c.indent, m, x.D})
		case IfBreak:
			if c.mode == modeBreak {
				stack = append(stack, cmd{c.indent, c.mode, x.Broken})
			} else {
				stack = append(stack, cmd{c.indent, c.mode, x.Flat})
			}
		case Detach:
			stack = append(stack, cmd{c.indent, modeBreak, x.D})
		case Fill:
			for i := len(x.Parts) - 1; i >= 0; i-- {
				stack = append(stack, cmd{c.indent, c.mode, x.Parts[i]})
			}
		case LiteralLine:
			return true
		case Line:
			if c.mode == modeBreak || x.Hard {
				return true
			}
			if !x.Soft {
				width--
			}
		}
		// LineSuffix and BreakParent take no width.
	}
	return false
}

// propagateBreaks sets Break on every group that contains a forced break and
// reports whether d contains one.
func propagateBreaks(d Doc) bool {
	switch x := d.(type) {
	case Cat:
		forced := false
		for _, c := range x {
			if propagateBreaks(c) {
				forced = true
			}
		}
		return forced
	case Indent:
		return propagateBreaks(x.D)
	case Fill:
		forced := false
		for _, c := range x.Parts {
			if propagateBreaks(c) {
				forced = true
			}
		}
		return forced
	case *Group:
		if propagateBreaks(x.D) {
			x.Break = true
		}
		return x.Break
	case IfBreak:
		// A forced break in the Broken branch only matters once the group is
		// already broken, so it must not break the group by itself.
		propagateBreaks(x.Broken)
		return propagateBreaks(x.Flat)
	case Line:
		return x.Hard
	case BreakParent, LiteralLine:
		return true
	case Detach:
		propagateBreaks(x.D)
		return false
	}
	return false
}
