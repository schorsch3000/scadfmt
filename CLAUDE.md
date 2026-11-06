# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project goal

`scadfmt` = code formatter for OpenSCAD (`.scad`) files. Existing formatters use string/regex matching and produce broken output. This one must build a real AST and re-emit the file deterministically. See `idea.md`.

Repo state: formatter itself not written yet. Only `idea.md` and a vendored OpenSCAD checkout in `openscad/` (upstream commit `a81e863`, has own `.git`, untracked by the outer repo). `.az/` and `.idea/` are tooling dirs, ignore.

## OpenSCAD source as reference (`openscad/`)

Treat as read-only reference for the grammar and AST. Do not modify unless explicitly asked. Build docs: `openscad/README.md` (CMake, many deps).

Files that matter for a formatter:
- `src/core/lexer.l` — flex lexer. **Comments (`//`, `/* */`) and whitespace are discarded here** (lines ~220-240). Comments never reach the AST. A formatter must capture them separately (own lexer/trivia pass, or patched lexer) and reattach them to nodes by source location.
- `src/core/parser.y` — bison grammar, authoritative syntax definition. Builds `SourceFile`, `LocalScope`, `ModuleInstantiation`, `Expression` nodes.
- `src/core/AST.h` — `ASTNode` base, source `Location`, virtual `print(ostream, indent)`. Existing `print` is the `--export-format ast` dump (tests: `add_cmdline_test(astdump ...)` in `tests/CMakeLists.txt`). It is a debugging printer: normalizes and drops comments, not usable as formatter as-is.
- `src/core/Expression.*`, `ModuleInstantiation.*`, `LocalScope.*`, `Assignment.*`, `SourceFile.*` — node types.
- `src/core/customizer/comment_lexer.l`, `comment_parser.y`, `CommentParser.cc` — existing comment parsing for Customizer annotations (`/*[Group]*/`, `// [min:max]` after parameters). Formatter must preserve these comments and their position.
- Syntax test inputs: `tests/data/scad/**` (e.g. `misc/allexpressions.scad`, `misc/ifelse-ast-dump.scad`) — good corpus for round-trip/idempotency tests.

Inspect AST of a file with a built OpenSCAD binary: `openscad -o - --export-format ast file.scad`.

## Design constraints (from idea.md)

- AST-based, never regex/string-pattern rewriting.
- Deterministic and idempotent: `fmt(fmt(x)) == fmt(x)`.
- Must not change semantics; must keep all comments.

## Design decisions

- Language: Go. Single static binary, no cgo (so no tree-sitter).
- Own lossless lexer + recursive-descent parser producing a CST. Grammar taken from `openscad/src/core/parser.y`. Lexer emits trivia (comments, newlines, blank lines); comments attach to nodes as leading/trailing/dangling. Must keep `#` `%` `!` `*` modifiers, parens, and raw `include <...>`/`use <...>`.
- Output via Wadler-style doc IR (text, line, softline, indent, group), written in-house. Alignment uses a `Table` node (rows of cells, per-column max width, pad with spaces only). Alignment must never influence the preserve-layout decision, or idempotency breaks.
- Style: prettier-like, configurable. Config file `.scadfmt.yaml` (YAML, snake_case keys), CLI flags override:

```yaml
max_width: 100
indent: 2
use_tabs: false
chain: auto            # break | flat | auto  (long transform chains)
matrix: preserve       # preserve | reflow | align
align:
  assignments: false   # align `=` in consecutive assignments
  params: false
  decimal: false       # align numbers on the decimal point; spaces only, never change digits
  trailing_comments: false
  max_gap: 8
numbers:
  normalize: false     # e.g. 1.50 -> 1.5; opt-in only
```

- Defaults: matrix `preserve`, all align options off. `--preset pretty` turns the align options on.
- Alignment groups end at a blank line, a non-assignment statement, or a block boundary. Alignment falls back to unaligned if the group exceeds `max_width`.

## Verification plan

- Idempotency: `fmt(fmt(x)) == fmt(x)`.
- Semantics: `openscad -o - --export-format ast` of input vs. output must be equal.
- Corpus: `openscad/tests/data/scad/**`, `openscad/examples/**`.

## Milestones

Tracked in git-bug (`git bug bug`, close with `git bug bug status close <id>`).

## Commands

Go lives in `/usr/local/go/bin`, not on PATH by default: `export PATH=$PATH:/usr/local/go/bin`.

- Build: `go build -o scadfmt ./cmd/scadfmt`
- Test all: `go test ./...`
- Single test: `go test ./internal/format -run TestGolden/simple`
- Run: `scadfmt [-w] [-check] [files...]` (stdin when no files)
- Golden tests: `testdata/golden/<name>.in.scad` + `<name>.out.scad`; the runner also asserts idempotency.
