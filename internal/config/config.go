// Package config reads .scadfmt.yaml and turns it into format options.
//
// Precedence, lowest to highest: built-in defaults, preset, config file,
// command-line flags. The caller applies flags after calling Options.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/schorsch3000/scadfmt/internal/format"
)

// FileName is the config file looked for in each directory up the tree.
const FileName = ".scadfmt.yaml"

// Config is the content of a config file. Pointers distinguish "not set" from
// zero values, so unset keys keep the default (or the preset's value).
type Config struct {
	Preset   string `yaml:"preset"`
	MaxWidth *int   `yaml:"max_width"`
	Indent   *int   `yaml:"indent"`
	UseTabs  *bool  `yaml:"use_tabs"`

	Chain       string `yaml:"chain"`
	ChainIndent string `yaml:"chain_indent"`
	Matrix      string `yaml:"matrix"`

	Align struct {
		Assignments      *bool `yaml:"assignments"`
		Params           *bool `yaml:"params"`
		Decimal          *bool `yaml:"decimal"`
		TrailingComments *bool `yaml:"trailing_comments"`
		MaxGap           *int  `yaml:"max_gap"`
	} `yaml:"align"`

	Numbers struct {
		Normalize *bool `yaml:"normalize"`
	} `yaml:"numbers"`
}

// Parse reads a config file. Unknown keys and invalid values are errors.
func Parse(data []byte) (*Config, error) {
	var c Config
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil && !errors.Is(err, io.EOF) { // io.EOF: empty file
		return nil, err
	}
	if err := c.validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

// Load reads and parses the file at path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	c, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return c, nil
}

// Find looks for FileName in dir and each of its parents, and returns the path
// of the first one found.
func Find(dir string) (string, bool) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", false
	}
	for {
		p := filepath.Join(dir, FileName)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func oneOf(key, val string, allowed ...string) error {
	for _, a := range allowed {
		if val == a {
			return nil
		}
	}
	return fmt.Errorf("%s: invalid value %q, want one of %v", key, val, allowed)
}

// Check reports the first invalid value in o. Messages use the config file's key
// names, which are the command-line flag names with underscores.
func Check(o format.Options) error {
	if o.MaxWidth < 1 {
		return fmt.Errorf("max_width: must be at least 1, got %d", o.MaxWidth)
	}
	if o.Indent < 1 || o.Indent > 16 {
		return fmt.Errorf("indent: must be between 1 and 16, got %d", o.Indent)
	}
	if o.AlignMaxGap < 0 {
		return fmt.Errorf("align.max_gap: must not be negative, got %d", o.AlignMaxGap)
	}
	if err := oneOf("chain", o.Chain, format.ChainAuto, format.ChainFlat, format.ChainBreak); err != nil {
		return err
	}
	if err := oneOf("chain_indent", o.ChainIndent, format.ChainIndentStep, format.ChainIndentSame); err != nil {
		return err
	}
	return oneOf("matrix", o.Matrix, format.MatrixPreserve, format.MatrixReflow, format.MatrixAlign)
}

func (c *Config) validate() error {
	o, err := Options(c, "")
	if err != nil {
		return fmt.Errorf("preset: %w", err)
	}
	return Check(o)
}

// PresetNames lists the names Preset accepts.
var PresetNames = []string{"default", "pretty"}

// Preset returns the changes a named preset makes to the default options.
//
//	pretty: turn on all alignment options, including matrix column alignment.
func Preset(name string) (func(*format.Options), error) {
	switch name {
	case "pretty":
		return func(o *format.Options) {
			o.Matrix = format.MatrixAlign
			o.AlignAssignments = true
			o.AlignDecimal = true
			o.AlignParams = true
			o.AlignTrailingComments = true
		}, nil
	case "default":
		return func(*format.Options) {}, nil
	}
	return nil, fmt.Errorf("unknown preset %q (have: %s)", name, strings.Join(PresetNames, ", "))
}

// Options builds format options from the defaults, then the preset, then c.
// preset, if not empty, replaces the preset named in c. c may be nil.
func Options(c *Config, preset string) (format.Options, error) {
	opts := format.DefaultOptions()
	if preset == "" && c != nil {
		preset = c.Preset
	}
	if preset != "" {
		apply, err := Preset(preset)
		if err != nil {
			return opts, err
		}
		apply(&opts)
	}
	if c == nil {
		return opts, nil
	}
	setInt(&opts.MaxWidth, c.MaxWidth)
	setInt(&opts.Indent, c.Indent)
	setBool(&opts.UseTabs, c.UseTabs)
	setStr(&opts.Chain, c.Chain)
	setStr(&opts.ChainIndent, c.ChainIndent)
	setStr(&opts.Matrix, c.Matrix)
	setBool(&opts.AlignAssignments, c.Align.Assignments)
	setBool(&opts.AlignParams, c.Align.Params)
	setBool(&opts.AlignDecimal, c.Align.Decimal)
	setBool(&opts.AlignTrailingComments, c.Align.TrailingComments)
	setInt(&opts.AlignMaxGap, c.Align.MaxGap)
	setBool(&opts.NormalizeNumbers, c.Numbers.Normalize)
	return opts, nil
}

func setInt(dst *int, v *int) {
	if v != nil {
		*dst = *v
	}
}

func setBool(dst *bool, v *bool) {
	if v != nil {
		*dst = *v
	}
}

func setStr(dst *string, v string) {
	if v != "" {
		*dst = v
	}
}
