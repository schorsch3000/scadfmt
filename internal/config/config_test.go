package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"scadfmt/internal/format"
)

func TestParseFull(t *testing.T) {
	c, err := Parse([]byte(`
max_width: 80
indent: 4
use_tabs: true
chain: break
chain_indent: same
matrix: reflow
align:
  assignments: true
  params: true
  decimal: true
  trailing_comments: true
  max_gap: 3
numbers:
  normalize: true
`))
	if err != nil {
		t.Fatal(err)
	}
	o, err := Options(c, "")
	if err != nil {
		t.Fatal(err)
	}
	want := format.Options{
		MaxWidth: 80, Indent: 4, UseTabs: true,
		Chain: format.ChainBreak, ChainIndent: format.ChainIndentSame, Matrix: format.MatrixReflow,
		AlignAssignments: true, AlignParams: true, AlignDecimal: true, AlignTrailingComments: true,
		AlignMaxGap: 3, NormalizeNumbers: true,
	}
	if o != want {
		t.Errorf("got %+v\nwant %+v", o, want)
	}
}

func TestEmptyAndUnsetKeepDefaults(t *testing.T) {
	for _, src := range []string{"", "# only a comment\n", "indent: 3\n"} {
		c, err := Parse([]byte(src))
		if err != nil {
			t.Fatalf("%q: %v", src, err)
		}
		o, _ := Options(c, "")
		def := format.DefaultOptions()
		if strings.HasPrefix(src, "indent") {
			def.Indent = 3
		}
		if o != def {
			t.Errorf("%q: got %+v want %+v", src, o, def)
		}
	}
}

func TestParseErrors(t *testing.T) {
	for src, frag := range map[string]string{
		"max_widht: 80\n":         "max_widht",
		"max_width: 0\n":          "max_width",
		"indent: 40\n":            "indent",
		"chain: sideways\n":       "chain",
		"matrix: nope\n":          "matrix",
		"chain_indent: x\n":       "chain_indent",
		"preset: fancy\n":         "preset",
		"align:\n  max_gap: -1\n": "max_gap",
		"align:\n  nope: true\n":  "nope",
		"max_width: wide\n":       "line 1",
	} {
		_, err := Parse([]byte(src))
		if err == nil {
			t.Errorf("%q: want error", src)
		} else if !strings.Contains(err.Error(), frag) {
			t.Errorf("%q: error %q should mention %q", src, err, frag)
		}
	}
}

func TestPresetPrecedence(t *testing.T) {
	o, err := Options(nil, "pretty")
	if err != nil {
		t.Fatal(err)
	}
	if !o.AlignAssignments || !o.AlignDecimal || !o.AlignParams || !o.AlignTrailingComments || o.Matrix != format.MatrixAlign {
		t.Errorf("pretty preset incomplete: %+v", o)
	}
	if o.NormalizeNumbers {
		t.Error("number normalization must stay opt-in")
	}

	// A key in the file beats the preset; the file's preset can be replaced.
	c, _ := Parse([]byte("preset: pretty\nalign:\n  decimal: false\n"))
	o, _ = Options(c, "")
	if !o.AlignAssignments || o.AlignDecimal {
		t.Errorf("file key must override preset: %+v", o)
	}
	o, _ = Options(c, "default")
	if o.AlignAssignments {
		t.Errorf("explicit preset must replace the file's: %+v", o)
	}
	if _, err := Options(nil, "bogus"); err == nil {
		t.Error("unknown preset must fail")
	}
}

func TestFind(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, ok := Find(sub); ok {
		// A config above the temp dir would make this test meaningless.
		t.Skip("a config file exists above the temp directory")
	}
	top := filepath.Join(root, FileName)
	os.WriteFile(top, []byte("indent: 3\n"), 0o644)
	if got, ok := Find(sub); !ok || got != top {
		t.Errorf("got %q %v want %q", got, ok, top)
	}
	nearer := filepath.Join(root, "a", FileName)
	os.WriteFile(nearer, []byte("indent: 4\n"), 0o644)
	if got, _ := Find(sub); got != nearer {
		t.Errorf("nearest file must win: got %q", got)
	}
	c, err := Load(nearer)
	if err != nil || *c.Indent != 4 {
		t.Errorf("load: %v %+v", err, c)
	}
}

func TestLoadErrorNamesFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), FileName)
	os.WriteFile(p, []byte("bogus: 1\n"), 0o644)
	if _, err := Load(p); err == nil || !strings.Contains(err.Error(), p) {
		t.Errorf("error should name the file: %v", err)
	}
}
