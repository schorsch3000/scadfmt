// Command scadfmt formats OpenSCAD files.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/schorsch3000/scadfmt/internal/completion"
	"github.com/schorsch3000/scadfmt/internal/config"
	"github.com/schorsch3000/scadfmt/internal/format"
)

const usage = `usage: scadfmt [flags] [file or directory ...]

Formats OpenSCAD files. With no arguments it reads standard input and writes
standard output. Directories are searched for *.scad files.

Options come from .scadfmt.yaml (found in the file's directory or a parent),
and can be overridden with the flags below.

`

// version is set at build time: -ldflags "-X main.version=v1.2.3".
var version = "dev"

// versionString returns the stamped version, or for a binary built by
// `go install module@version` the module version Go recorded.
func versionString() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return version
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// flags holds the command line. Option flags only override the config file when
// they were given, which is tracked in set.
type flags struct {
	write, check, noConfig bool
	showVersion            bool
	completion             string
	configPath, preset     string
	stdinPath              string

	opts format.Options
	set  map[string]bool
}

// newFlagSet declares every command-line flag, storing the values in fl.
func newFlagSet(fl *flags, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet("scadfmt", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprint(stderr, usage)
		fs.PrintDefaults()
	}
	fs.BoolVar(&fl.write, "w", false, "write result to the source file instead of stdout")
	fs.BoolVar(&fl.check, "check", false, "list files whose formatting differs; exit 1 if any")
	fs.BoolVar(&fl.showVersion, "version", false, "print the version and exit")
	fs.StringVar(&fl.completion, "completion", "", "print a shell completion script ("+strings.Join(completion.Shells(), ", ")+") and exit")
	fs.StringVar(&fl.configPath, "config", "", "use this config file instead of searching for "+config.FileName)
	fs.BoolVar(&fl.noConfig, "no-config", false, "ignore config files")
	fs.StringVar(&fl.preset, "preset", "", "start from a preset: default or pretty")
	fs.StringVar(&fl.stdinPath, "stdin-filepath", "", "path to use for config lookup when reading standard input")

	o := &fl.opts
	fs.IntVar(&o.MaxWidth, "max-width", o.MaxWidth, "maximum line width")
	fs.IntVar(&o.Indent, "indent", o.Indent, "spaces per indent level")
	fs.BoolVar(&o.UseTabs, "use-tabs", o.UseTabs, "indent with tabs")
	fs.StringVar(&o.Chain, "chain", o.Chain, "transform chains: auto, flat or break")
	fs.StringVar(&o.ChainIndent, "chain-indent", o.ChainIndent, "indent of broken chains: step or same")
	fs.StringVar(&o.Matrix, "matrix", o.Matrix, "multi-line vectors: preserve, reflow or align")
	fs.BoolVar(&o.AlignAssignments, "align-assignments", false, "align = in consecutive assignments")
	fs.BoolVar(&o.AlignParams, "align-params", false, "align = in broken parameter and argument lists")
	fs.BoolVar(&o.AlignDecimal, "align-decimal", false, "align numbers on the decimal point (needs -align-assignments)")
	fs.BoolVar(&o.AlignTrailingComments, "align-trailing-comments", false, "align // comments after consecutive statements")
	fs.IntVar(&o.AlignMaxGap, "align-max-gap", o.AlignMaxGap, "split an alignment group when widths differ by more (0: no limit)")
	fs.BoolVar(&o.NormalizeNumbers, "normalize-numbers", false, "rewrite .5 as 0.5 and 1.50 as 1.5")
	return fs
}

func parseFlags(args []string, stderr io.Writer) (*flags, []string, error) {
	fl := &flags{opts: format.DefaultOptions(), set: map[string]bool{}}
	fs := newFlagSet(fl, stderr)
	if err := fs.Parse(args); err != nil {
		return nil, nil, err
	}
	fs.Visit(func(f *flag.Flag) { fl.set[f.Name] = true })
	return fl, fs.Args(), nil
}

// flagChoices lists the valid values of the flags that take one of a fixed set
// of words. They come from the same constants the options are validated against.
var flagChoices = map[string][]string{
	"chain":        {format.ChainAuto, format.ChainFlat, format.ChainBreak},
	"chain-indent": {format.ChainIndentStep, format.ChainIndentSame},
	"matrix":       {format.MatrixPreserve, format.MatrixReflow, format.MatrixAlign},
	"preset":       config.PresetNames,
	"completion":   completion.Shells(),
}

// fileFlags take a file name.
var fileFlags = map[string]bool{"config": true, "stdin-filepath": true}

// completionFlags describes the command line for the completion generator.
func completionFlags() []completion.Flag {
	var out []completion.Flag
	newFlagSet(&flags{}, io.Discard).VisitAll(func(f *flag.Flag) {
		isBool := false
		if b, ok := f.Value.(interface{ IsBoolFlag() bool }); ok {
			isBool = b.IsBoolFlag()
		}
		out = append(out, completion.Flag{
			Name: f.Name, Usage: f.Usage, Bool: isBool,
			Values: flagChoices[f.Name], File: fileFlags[f.Name],
		})
	})
	return out
}

// optionsFor resolves the options for a file in dir: defaults, preset, config
// file, then flags.
func (fl *flags) optionsFor(dir string, cache map[string]format.Options) (format.Options, error) {
	if o, ok := cache[dir]; ok {
		return o, nil
	}
	var cfg *config.Config
	if !fl.noConfig {
		path := fl.configPath
		if path == "" {
			path, _ = config.Find(dir)
		}
		if path != "" {
			c, err := config.Load(path)
			if err != nil {
				return format.Options{}, err
			}
			cfg = c
		}
	}
	opts, err := config.Options(cfg, fl.preset)
	if err != nil {
		return opts, err
	}
	fl.applyFlags(&opts)
	if err := config.Check(opts); err != nil {
		return opts, err
	}
	cache[dir] = opts
	return opts, nil
}

// applyFlags overrides opts with every option flag that was given.
func (fl *flags) applyFlags(opts *format.Options) {
	s, f := fl.set, fl.opts
	if s["max-width"] {
		opts.MaxWidth = f.MaxWidth
	}
	if s["indent"] {
		opts.Indent = f.Indent
	}
	if s["use-tabs"] {
		opts.UseTabs = f.UseTabs
	}
	if s["chain"] {
		opts.Chain = f.Chain
	}
	if s["chain-indent"] {
		opts.ChainIndent = f.ChainIndent
	}
	if s["matrix"] {
		opts.Matrix = f.Matrix
	}
	if s["align-assignments"] {
		opts.AlignAssignments = f.AlignAssignments
	}
	if s["align-params"] {
		opts.AlignParams = f.AlignParams
	}
	if s["align-decimal"] {
		opts.AlignDecimal = f.AlignDecimal
	}
	if s["align-trailing-comments"] {
		opts.AlignTrailingComments = f.AlignTrailingComments
	}
	if s["align-max-gap"] {
		opts.AlignMaxGap = f.AlignMaxGap
	}
	if s["normalize-numbers"] {
		opts.NormalizeNumbers = f.NormalizeNumbers
	}
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fl, paths, err := parseFlags(args, stderr)
	if err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if fl.completion != "" {
		script, err := completion.Generate(fl.completion, "scadfmt", completionFlags())
		if err != nil {
			fmt.Fprintln(stderr, "scadfmt:", err)
			return 2
		}
		fmt.Fprint(stdout, script)
		return 0
	}
	if fl.showVersion {
		fmt.Fprintln(stdout, "scadfmt", versionString())
		return 0
	}
	cache := map[string]format.Options{}

	if len(paths) == 0 {
		return fl.stdin(stdin, stdout, stderr, cache)
	}
	files, err := expand(paths)
	if err != nil {
		fmt.Fprintln(stderr, "scadfmt:", err)
		return 2
	}
	status := 0
	for _, path := range files {
		if code := fl.file(path, stdout, stderr, cache); code > status {
			status = code
		}
	}
	return status
}

// expand replaces directories by the .scad files below them.
func expand(paths []string) ([]string, error) {
	var files []string
	for _, p := range paths {
		st, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if !st.IsDir() {
			files = append(files, p)
			continue
		}
		err = filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.HasSuffix(path, ".scad") {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}

func (fl *flags) stdin(stdin io.Reader, stdout, stderr io.Writer, cache map[string]format.Options) int {
	src, err := io.ReadAll(stdin)
	if err != nil {
		fmt.Fprintln(stderr, "scadfmt:", err)
		return 2
	}
	dir := "."
	if fl.stdinPath != "" {
		dir = filepath.Dir(fl.stdinPath)
	}
	opts, err := fl.optionsFor(dir, cache)
	if err != nil {
		fmt.Fprintln(stderr, "scadfmt:", err)
		return 2
	}
	out, err := format.Format(src, opts)
	if err != nil {
		fmt.Fprintln(stderr, "scadfmt: <stdin>:", err)
		return 2
	}
	if fl.check {
		if !bytes.Equal(src, out) {
			fmt.Fprintln(stdout, "<stdin>")
			return 1
		}
		return 0
	}
	stdout.Write(out)
	return 0
}

func (fl *flags) file(path string, stdout, stderr io.Writer, cache map[string]format.Options) int {
	src, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(stderr, "scadfmt:", err)
		return 2
	}
	opts, err := fl.optionsFor(filepath.Dir(path), cache)
	if err != nil {
		fmt.Fprintln(stderr, "scadfmt:", err)
		return 2
	}
	out, err := format.Format(src, opts)
	if err != nil {
		fmt.Fprintf(stderr, "scadfmt: %s: %v\n", path, err)
		return 2
	}
	changed := !bytes.Equal(src, out)
	switch {
	case fl.check:
		if changed {
			fmt.Fprintln(stdout, path)
			return 1
		}
	case fl.write:
		if changed {
			if err := os.WriteFile(path, out, 0o644); err != nil {
				fmt.Fprintln(stderr, "scadfmt:", err)
				return 2
			}
		}
	default:
		stdout.Write(out)
	}
	return 0
}
