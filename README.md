# scadfmt

A code formatter for OpenSCAD (`.scad`) files.

Other OpenSCAD formatters rewrite text with patterns and regular expressions, and
break on code they did not expect. scadfmt has a real lexer and parser (built from
OpenSCAD's own grammar), prints the syntax tree back out, and checks its own work:
the output must parse to the same program with every comment intact, or scadfmt
refuses to change the file.

## Install

Download an archive for Linux, macOS or Windows from the
[releases page](https://github.com/schorsch3000/scadfmt/releases), or build with Go 1.22+:

```sh
go install github.com/schorsch3000/scadfmt/cmd/scadfmt@latest
```

## Use

```sh
scadfmt file.scad            # print the formatted file
scadfmt -w src/              # rewrite all .scad files under src/ in place
scadfmt -check src/          # list files that need formatting, exit 1 if any (for CI)
scadfmt < in.scad > out.scad # filter standard input
```

Exit codes: `0` ok, `1` with `-check` when files differ, `2` on an error, such as a
syntax error (the file is left alone).

Editors that format via standard input can pass `--stdin-filepath path/to/file.scad`
so the right `.scadfmt.yaml` is found.

## Example

```openscad
width=10;height  =  20.5; // [1:100]
module bracket(w=10,t=2,center=false){
  difference(){
    cube([w,height,t],center=center);
    translate([w/2,height/2,-1]) rotate(90) cylinder(d=w/2,h=t+2);
  }
}
pts=[[0,0],[10,0],
     [10,10],[0,10]];
```

becomes

```openscad
width = 10;
height = 20.5; // [1:100]
module bracket(w = 10, t = 2, center = false) {
  difference() {
    cube([w, height, t], center = center);
    translate([w / 2, height / 2, -1]) rotate(90) cylinder(d = w / 2, h = t + 2);
  }
}
pts = [
  [0, 0], [10, 0],
  [10, 10], [0, 10]
];
```

and with `--preset pretty` the assignments and the matrix are aligned:

```openscad
width  = 10;
height = 20.5; // [1:100]
...
pts = [
  [ 0,  0],
  [10,  0],
  [10, 10],
  [ 0, 10]
];
```

## Configuration

scadfmt reads `.scadfmt.yaml` from the file's directory or the nearest parent.
Everything is optional; this shows the defaults:

```yaml
preset: default        # default | pretty
max_width: 100
indent: 2
use_tabs: false

chain: auto            # auto | flat | break: calls like translate() rotate() cube();
chain_indent: step     # step | same: indent of a broken chain
matrix: preserve       # preserve | reflow | align: vectors written over several lines

align:                 # off by default; `preset: pretty` turns all of them on
  assignments: false   # align the = of consecutive assignments
  decimal: false       # align numbers on the decimal point (needs assignments)
  params: false        # align = in parameter and argument lists that are broken up
  trailing_comments: false
  max_gap: 8           # start a new group when widths differ by more (0: no limit)

numbers:
  normalize: false     # .5 -> 0.5, 1.50 -> 1.5, 1. -> 1.0
```

- **`chain: auto`** keeps a chain on one line if it fits and otherwise puts each call
  on its own line. `flat` never breaks between calls, `break` always does.
- **`matrix: preserve`** keeps the rows of a vector as you wrote them, so tables of
  points and faces stay readable. `reflow` lays them out by width only. `align` also
  lines up the columns.
- Alignment only ever adds spaces. It never changes a number or any other token.

Every option has a command-line flag (`scadfmt -h` lists them), and flags win over
the config file, which wins over `--preset`.

## Shell completion

```sh
source <(scadfmt -complete)   # bash; the shell is taken from $SHELL
scadfmt -complete zsh         # or name it: bash, zsh, fish
```

The script comes from the binary itself, so there is nothing else to install.
For zsh save it as `_scadfmt` in a directory on your `$fpath`; for fish, as
`~/.config/fish/completions/scadfmt.fish`. Only the bash script has been tested in a real
shell so far.

## Limits

- Files with syntax errors are not formatted (the error is reported).
- Output always uses `\n` line endings.
- Comments are never lost or reordered, but a comment in an unusual spot may move to the
  nearest sensible line (for example, one in the middle of a call chain).

## Development

```sh
make check      # gofmt, vet and all tests
make corpus     # fetch the OpenSCAD source the corpus tests run against (once)
make fuzz       # fuzz the formatter
make dist       # build release archives
```

`CLAUDE.md` describes the architecture and the checks that keep the formatter safe.
Releases are made by pushing a tag such as `v1.2.3`.
