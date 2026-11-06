// Package doc is a small Wadler/Prettier-style document IR and layout engine.
// Callers describe output as a tree of Docs; Print picks line breaks so that
// groups fit within the maximum width where possible. Layout depends only on
// the tree and the options, so output is deterministic.
package doc

// Doc is a node of the document tree.
type Doc interface{ isDoc() }

type (
	// Text is literal text without line breaks.
	Text string

	// Cat concatenates docs.
	Cat []Doc

	// Line is a line break. In a group laid out flat it prints as a space
	// (Soft: nothing); in a broken group it prints as a newline. Hard lines
	// always break and force every enclosing group to break.
	Line struct{ Soft, Hard bool }

	// Indent indents the line breaks inside D by one level.
	Indent struct{ D Doc }

	// Group is laid out flat if it fits in the remaining width, else broken.
	// Break is also set by Print when D contains a forced break.
	Group struct {
		D     Doc
		Break bool
	}

	// IfBreak prints Broken when the enclosing group is broken, else Flat.
	IfBreak struct{ Broken, Flat Doc }

	// LineSuffix defers D until just before the next line break (or the end of
	// output). Used for trailing comments.
	LineSuffix struct{ D Doc }

	// BreakParent is empty but forces every enclosing group to break.
	BreakParent struct{}
)

func (Text) isDoc()        {}
func (Cat) isDoc()         {}
func (Line) isDoc()        {}
func (Indent) isDoc()      {}
func (*Group) isDoc()      {}
func (IfBreak) isDoc()     {}
func (LineSuffix) isDoc()  {}
func (BreakParent) isDoc() {}

// Common constructors and singletons.
var (
	Space    Doc = Text(" ")
	Softline Doc = Line{Soft: true}
	LineBr   Doc = Line{}
	Hardline Doc = Line{Hard: true}
	Nil      Doc = Cat(nil)
)

// Concat joins docs.
func Concat(ds ...Doc) Doc { return Cat(ds) }

// Nest indents d by one level.
func Nest(d Doc) Doc { return Indent{d} }

// NewGroup wraps d in a group.
func NewGroup(ds ...Doc) *Group { return &Group{D: Cat(ds)} }

// Join places sep between the docs.
func Join(sep Doc, ds []Doc) Doc {
	var out Cat
	for i, d := range ds {
		if i > 0 {
			out = append(out, sep)
		}
		out = append(out, d)
	}
	return out
}

// Options controls layout.
type Options struct {
	MaxWidth int  // target line width in columns
	Indent   int  // columns per indent level; also the tab width when UseTabs
	UseTabs  bool // indent with tabs instead of spaces
}
