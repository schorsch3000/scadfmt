package syntax

import (
	"fmt"
	"reflect"
	"strings"

	"scadfmt/internal/lexer"
)

// EqualOptions tunes Equal.
type EqualOptions struct {
	// Number, if set, rewrites every number literal before comparing, so that
	// text changes the formatter is allowed to make are not reported.
	Number func(string) string
}

// Equal reports the first difference in structure between two trees, or nil if
// they describe the same program. Whitespace and comments are not part of the
// tree and are ignored, as are things the formatter may change without
// changing meaning: the space inside `include <...>` and a trailing comma.
func Equal(a, b *File, opts EqualOptions) error {
	return equal("File", reflect.ValueOf(a), reflect.ValueOf(b), opts)
}

var tokType = reflect.TypeOf((*lexer.Token)(nil))

func equal(path string, a, b reflect.Value, opts EqualOptions) error {
	switch a.Kind() {
	case reflect.Interface:
		if a.IsNil() || b.IsNil() {
			if a.IsNil() != b.IsNil() {
				return fmt.Errorf("%s: present in only one tree", path)
			}
			return nil
		}
		if a.Elem().Type() != b.Elem().Type() {
			return fmt.Errorf("%s: %s became %s", path, a.Elem().Type(), b.Elem().Type())
		}
		return equal(path, a.Elem(), b.Elem(), opts)
	case reflect.Ptr:
		if a.IsNil() || b.IsNil() {
			if a.IsNil() != b.IsNil() {
				return fmt.Errorf("%s: present in only one tree", path)
			}
			return nil
		}
		if a.Type() == tokType {
			return equalTok(path, a.Interface().(Tok), b.Interface().(Tok), opts)
		}
		return equal(path, a.Elem(), b.Elem(), opts)
	case reflect.Struct:
		for i := 0; i < a.NumField(); i++ {
			f := a.Type().Field(i)
			// Spans only repeat tokens held elsewhere; a trailing comma is not
			// structure (the formatter drops it from lists that fit on a line).
			if f.Type == reflect.TypeOf(Span{}) || f.Name == "Commas" {
				continue
			}
			if err := equal(path+"."+f.Name, a.Field(i), b.Field(i), opts); err != nil {
				return err
			}
		}
		return nil
	case reflect.Slice:
		if a.Len() != b.Len() {
			return fmt.Errorf("%s: %d items became %d", path, a.Len(), b.Len())
		}
		for i := 0; i < a.Len(); i++ {
			if err := equal(fmt.Sprintf("%s[%d]", path, i), a.Index(i), b.Index(i), opts); err != nil {
				return err
			}
		}
		return nil
	}
	return fmt.Errorf("%s: unsupported kind %s", path, a.Kind())
}

func equalTok(path string, a, b Tok, opts EqualOptions) error {
	if a.Kind != b.Kind {
		return fmt.Errorf("%s: %q became %q", path, a.Text, b.Text)
	}
	ta, tb := a.Text, b.Text
	switch a.Kind {
	case lexer.Include, lexer.Use:
		ta, tb = squeezeInclude(ta), squeezeInclude(tb)
	case lexer.Number:
		if opts.Number != nil {
			ta, tb = opts.Number(ta), opts.Number(tb)
		}
	}
	if ta != tb {
		return fmt.Errorf("%s: %q became %q", path, a.Text, b.Text)
	}
	return nil
}

// squeezeInclude drops the whitespace between `include`/`use` and `<`.
func squeezeInclude(s string) string {
	i := strings.IndexByte(s, '<')
	if i < 0 {
		return s
	}
	return strings.Join(strings.Fields(s[:i]), "") + s[i:]
}
