package format

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/schorsch3000/scadfmt/internal/doc"
	"github.com/schorsch3000/scadfmt/internal/syntax"
)

// ---- measuring ----

// flatText prints d on one line, however long, and reports whether it really
// fits on one line (no forced breaks).
func (p *printer) flatText(d doc.Doc) (string, bool) {
	s := doc.Print(d, doc.Options{MaxWidth: 1 << 20, Indent: p.opts.Indent, UseTabs: p.opts.UseTabs})
	return s, !strings.Contains(s, "\n")
}

func width(s string) int { return utf8.RuneCountInString(s) }

// innerComments reports whether a comment sits inside the statement, that is,
// anywhere except before its first token or after its last.
func (p *printer) innerComments(s syntax.Stmt) bool {
	first, last := s.First(), s.Last()
	for i := p.index[first]; i <= p.index[last]; i++ {
		t := &p.toks[i]
		info := p.info[t]
		if t != first && len(info.Leading) > 0 {
			return true
		}
		if t != last && len(info.Trailing) > 0 {
			return true
		}
	}
	return false
}

// flatStmt measures a statement printed on one line, without its outer
// comments. ok is false if it needs several lines.
func (p *printer) flatStmt(s syntax.Stmt) (w int, ok bool) {
	if p.innerComments(s) {
		return 0, false
	}
	p.measuring = true
	str, ok := p.flatText(p.stmt(s))
	p.measuring = false
	w = width(str)
	// A statement wider than the line wraps, so it is not a one-line row.
	return w, ok && p.depth*p.opts.Indent+w <= p.opts.MaxWidth
}

// segments splits widths into runs in which the largest and smallest differ by
// at most maxGap. maxGap <= 0 means no limit.
func segments(widths []int, maxGap int) [][2]int {
	var out [][2]int
	start := 0
	for i := range widths {
		lo, hi := widths[start], widths[start]
		for _, w := range widths[start : i+1] {
			lo, hi = min(lo, w), max(hi, w)
		}
		if maxGap > 0 && hi-lo > maxGap {
			out = append(out, [2]int{start, i})
			start = i
		}
	}
	if start < len(widths) {
		out = append(out, [2]int{start, len(widths)})
	}
	return out
}

// ---- statement alignment ----

type row struct {
	s    syntax.Stmt
	w    int  // flat width
	ok   bool // fits on one line, no inner comments
	asg  *syntax.Assignment
	adjN bool // no blank line before this statement, so it joins the previous row's run
}

// alignStmts prepares the padding that lines up consecutive assignments and
// trailing comments. Runs are broken by a blank line, by a statement that
// does not fit on one line, and by a statement of another kind.
func (p *printer) alignStmts(ss []syntax.Stmt) {
	if !p.opts.AlignAssignments && !p.opts.AlignTrailingComments {
		return
	}
	rows := make([]row, len(ss))
	for i, s := range ss {
		r := row{s: s, adjN: i > 0 && !p.blankBefore(s.First())}
		r.w, r.ok = p.flatStmt(s)
		r.asg, _ = s.(*syntax.Assignment)
		rows[i] = r
	}

	if p.opts.AlignAssignments {
		p.runs(rows, func(r row) bool { return r.ok && r.asg != nil }, p.alignAssignments)
	}
	if p.opts.AlignTrailingComments {
		p.runs(rows, func(r row) bool {
			return r.ok && len(p.info[r.s.Last()].Trailing) > 0
		}, p.alignComments)
	}
}

// runs calls f on every maximal run of adjacent rows satisfying in.
func (p *printer) runs(rows []row, in func(row) bool, f func([]row)) {
	start := -1
	for i, r := range rows {
		switch {
		case in(r) && start >= 0 && r.adjN:
		case in(r):
			if start >= 0 {
				f(rows[start:i])
			}
			start = i
		default:
			if start >= 0 {
				f(rows[start:i])
			}
			start = -1
		}
	}
	if start >= 0 {
		f(rows[start:])
	}
}

func (p *printer) alignAssignments(run []row) {
	widths := make([]int, len(run))
	for i, r := range run {
		widths[i] = width(r.asg.Name.Text)
	}
	for _, seg := range segments(widths, p.opts.AlignMaxGap) {
		rows := run[seg[0]:seg[1]]
		if len(rows) < 2 {
			continue
		}
		nameW := 0
		for _, r := range rows {
			nameW = max(nameW, width(r.asg.Name.Text))
		}
		// Value padding for numbers lined up on the decimal point.
		vpad := make([]int, len(rows))
		if p.opts.AlignDecimal {
			p.decimalPads(rows, vpad)
		}
		// Padding must not push a line past the width limit.
		fits := true
		for i, r := range rows {
			extra := nameW - width(r.asg.Name.Text) + vpad[i]
			if r.w <= p.opts.MaxWidth && r.w+extra > p.opts.MaxWidth {
				fits = false
			}
		}
		if !fits {
			continue
		}
		for i, r := range rows {
			p.namePad[r.asg.Name] = nameW - width(r.asg.Name.Text)
			p.valuePad[r.asg.Name] = vpad[i]
		}
	}
}

// decimalPads fills pad with the spaces that put the decimal points of
// consecutive numeric values in one column. Rows with other values are left
// alone and split the group.
func (p *printer) decimalPads(rows []row, pad []int) {
	type num struct{ i, left int }
	flush := func(group []num) {
		maxLeft := 0
		for _, n := range group {
			maxLeft = max(maxLeft, n.left)
		}
		for _, n := range group {
			pad[n.i] = maxLeft - n.left
		}
	}
	var group []num
	for i, r := range rows {
		p.measuring = true
		text, ok := p.flatText(p.expr(r.asg.Value))
		p.measuring = false
		left, _, isNum := numberParts(text)
		if ok && isNum {
			group = append(group, num{i, left})
			continue
		}
		flush(group)
		group = nil
	}
	flush(group)
}

func (p *printer) alignComments(run []row) {
	widths := make([]int, len(run))
	for i, r := range run {
		widths[i] = r.w
		if r.asg != nil {
			widths[i] += p.namePad[r.asg.Name] + p.valuePad[r.asg.Name]
		}
	}
	for _, seg := range segments(widths, p.opts.AlignMaxGap) {
		if seg[1]-seg[0] < 2 {
			continue
		}
		col := 0
		for _, w := range widths[seg[0]:seg[1]] {
			col = max(col, w)
		}
		for i := seg[0]; i < seg[1]; i++ {
			p.commentPad[run[i].s.Last()] = col - widths[i]
		}
	}
}

// ---- parameter lists ----

// paramPads returns, for each item of l, the spaces that line up its `=` with
// the neighbouring named items. All zeros unless AlignParams is set.
func (p *printer) paramPads(l syntax.List) []int {
	pads := make([]int, len(l.Items))
	if !p.opts.AlignParams {
		return pads
	}
	for i := 0; i < len(l.Items); {
		if l.Items[i].Name == nil || l.Items[i].Value == nil {
			i++
			continue
		}
		j := i
		var widths []int
		for j < len(l.Items) && l.Items[j].Name != nil && l.Items[j].Value != nil {
			widths = append(widths, width(l.Items[j].Name.Text))
			j++
		}
		for _, seg := range segments(widths, p.opts.AlignMaxGap) {
			maxW := 0
			for _, w := range widths[seg[0]:seg[1]] {
				maxW = max(maxW, w)
			}
			for k := seg[0]; k < seg[1]; k++ {
				pads[i+k] = maxW - widths[k]
			}
		}
		i = j
	}
	return pads
}

// ---- assignments ----

func (p *printer) assignment(n *syntax.Assignment) doc.Doc {
	namePad := strings.Repeat(" ", p.namePad[n.Name])
	valuePad := p.valuePad[n.Name]
	var value doc.Doc
	if valuePad > 0 {
		value = doc.Concat(doc.Space, doc.Text(strings.Repeat(" ", valuePad)), p.expr(n.Value))
	} else {
		value = p.rhs(n.Value)
	}
	return doc.Concat(p.tok(n.Name), doc.Text(namePad), doc.Space, p.tok(n.Eq), value, p.tok(n.Semi))
}

// ---- numbers ----

var numberRE = regexp.MustCompile(`^[+-]?(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?$`)

// numberParts splits a decimal number literal at its decimal point: left is the
// width before the point (the whole number if it has none), right the width
// from the point on.
func numberParts(s string) (left, right int, ok bool) {
	if !numberRE.MatchString(s) {
		return 0, 0, false
	}
	if i := strings.IndexByte(s, '.'); i >= 0 {
		return i, len(s) - i, true
	}
	return len(s), 0, true
}

// normalizeNumber returns s with a leading zero added to `.5`, trailing zeros
// dropped from `1.50`, and `1.` written as `1.0`. Hex literals and exponents
// are left alone.
func normalizeNumber(s string) string {
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		return s
	}
	mant, exp := s, ""
	if i := strings.IndexAny(s, "eE"); i >= 0 {
		mant, exp = s[:i], s[i:]
	}
	dot := strings.IndexByte(mant, '.')
	if dot < 0 {
		return s
	}
	intPart, frac := mant[:dot], strings.TrimRight(mant[dot+1:], "0")
	if intPart == "" {
		intPart = "0"
	}
	if frac == "" {
		frac = "0"
	}
	return intPart + "." + frac + exp
}

// ---- matrix columns ----

// vectorTable prints a vector of vectors as a table, one row per line, with the
// columns lined up and numbers aligned on their decimal point. It reports false
// if the vector is not a plain table (other elements, comments, multi-line
// cells), in which case the caller falls back to row preservation.
func (p *printer) vectorTable(n *syntax.Vector) (doc.Doc, bool) {
	if len(n.Elems) == 0 || p.hasComments(n.First(), n.Last()) {
		return nil, false
	}
	type cell struct {
		text        string
		left, right int
		num         bool
	}
	// Cells that span several source lines can never be aligned. Rejecting
	// them before printing anything keeps nested tables from being printed
	// twice per level.
	for _, e := range n.Elems {
		v, ok := e.(*syntax.Vector)
		if !ok {
			return nil, false
		}
		for _, ce := range v.Elems {
			if ce.First().Pos.Line != ce.Last().Pos.Line {
				return nil, false
			}
		}
	}
	grid := make([][]cell, len(n.Elems))
	rowVecs := make([]*syntax.Vector, len(n.Elems))
	for i, e := range n.Elems {
		v, ok := e.(*syntax.Vector)
		if !ok || len(v.Elems) == 0 {
			return nil, false
		}
		rowVecs[i] = v
		for _, ce := range v.Elems {
			text, flat := p.flatText(p.expr(ce))
			if !flat {
				return nil, false
			}
			l, r, isNum := numberParts(text)
			if !isNum {
				l, r = width(text), 0
			}
			grid[i] = append(grid[i], cell{text, l, r, isNum})
		}
	}

	// Per-column geometry.
	cols := 0
	for _, r := range grid {
		cols = max(cols, len(r))
	}
	leftMax, rightMax, wide := make([]int, cols), make([]int, cols), make([]int, cols)
	for _, r := range grid {
		for j, c := range r {
			if c.num {
				leftMax[j], rightMax[j] = max(leftMax[j], c.left), max(rightMax[j], c.right)
			} else {
				wide[j] = max(wide[j], width(c.text))
			}
		}
	}
	colW := make([]int, cols)
	for j := range colW {
		colW[j] = max(leftMax[j]+rightMax[j], wide[j])
	}

	var body []doc.Doc
	for i, r := range grid {
		var b strings.Builder
		b.WriteByte('[')
		for j, c := range r {
			lpad := 0
			if c.num {
				lpad = leftMax[j] - c.left
			}
			b.WriteString(strings.Repeat(" ", lpad))
			b.WriteString(c.text)
			if j < len(r)-1 {
				b.WriteByte(',')
				b.WriteString(strings.Repeat(" ", colW[j]-lpad-width(c.text)))
				b.WriteByte(' ')
			}
		}
		b.WriteByte(']')
		if i > 0 {
			body = append(body, doc.Hardline)
			if p.blankBefore(rowVecs[i].First()) {
				body = append(body, doc.Hardline)
			}
		}
		body = append(body, doc.Text(b.String()))
		if i < len(n.Commas) {
			body = append(body, doc.Text(","))
		}
	}
	return doc.Concat(
		p.tok(n.LBracket),
		doc.Nest(doc.Concat(doc.Hardline, doc.Concat(body...))),
		doc.Hardline,
		p.tok(n.RBracket),
	), true
}
