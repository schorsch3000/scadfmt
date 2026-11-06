// Package format turns OpenSCAD source into canonically formatted source.
package format

// Options controls formatting output.
type Options struct {
	MaxWidth int
	Indent   int
	UseTabs  bool
}

// DefaultOptions returns the default formatting options.
func DefaultOptions() Options {
	return Options{MaxWidth: 100, Indent: 2}
}

// Format formats src. Until the parser exists it returns src unchanged.
func Format(src []byte, opts Options) ([]byte, error) {
	return src, nil
}
