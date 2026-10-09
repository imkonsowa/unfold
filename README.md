# unfold

`unfold` rewrites Go source so nothing is inlined:

- every element of a composite literal (struct, map and slice values, even a
  single one) goes on its own line;
- every field of a struct type goes on its own line;
- every statement of a function literal goes on its own line.

The empty forms `struct{}`, `T{}` and `func() {}` stay as they are. Files
marked `// Code generated … DO NOT EDIT.` are skipped.

Before:

```go
cfg := Config{Name: "api", Port: 8080}
type point struct{ X, Y int }
sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
```

After:

```go
cfg := Config{
	Name: "api",
	Port: 8080,
}
type point struct {
	X, Y int
}
sort.Slice(rows, func(i, j int) bool {
	return rows[i].ID < rows[j].ID
})
```

`unfold` changes layout only, refuses a rewrite that would change anything
else, and formats what it writes: as gofmt does, or with gofumpt or your own
formatter (see [Configuration](#configuration)).

It runs on its own, as a linter in golangci-lint or `go vet`, and as a
pre-commit hook, all reading the same `.unfold.yml`.

## Install

Install the command into `$(go env GOPATH)/bin` (Go 1.26 or later):

```sh
go install github.com/imkonsowa/unfold@latest
```

Or pin it as a tool of a module, so everyone on the project runs the same
version with `go tool unfold`:

```sh
go get -tool github.com/imkonsowa/unfold@latest
```

## Usage

Unfold and format every Go file changed against `origin/main`, committed or
not:

```sh
unfold -changed -w
```

List the changed files that need unfolding or formatting without touching
them; the exit status is 1 when any does, which suits CI:

```sh
unfold -changed
```

Compare with another revision:

```sh
unfold -changed -base v1.4.0
```

Unfold named files, a directory, or a tree (`/...` recurses, skipping
`vendor`, `testdata`, `node_modules` and hidden directories):

```sh
unfold -w ./internal/...
```

Check that a rewritten file differs from the original only in layout:

```sh
unfold -equivalent before.go after.go
```

| Flag | Meaning |
| --- | --- |
| `-w` | rewrite the files instead of listing them |
| `-changed` | the Go files changed against `-base`, in the working tree too |
| `-base` | the revision `-changed` compares with (default `origin/main`, or `base` in `.unfold.yml`) |
| `-format` | `gofmt`, `gofumpt`, or a formatter command (default `gofmt`, or `format` in `.unfold.yml`) |
| `-config` | the settings file (default the nearest `.unfold.yml` from the current directory up) |
| `-equivalent` | check that the second file is the first with only layout changed |

Exit status: 0 when nothing needs changing (or after `-w`), 1 when files need
unfolding or formatting, or `-equivalent` finds a difference, 2 on errors.

## Configuration

`unfold`, its analyzer and the golangci-lint plugin read the nearest
`.unfold.yml`, looked up from the current directory (the analyzer: from each
file's directory) towards the root. Every key is optional; these are the
defaults:

```yaml
base: origin/main
format: gofmt
composite-literals: true
struct-types: true
function-literals: true
min-elements: 1
exclude: []
```

- `composite-literals`, `struct-types`, `function-literals`: which constructs
  are unfolded.
- `min-elements`: unfold only constructs holding at least this many elements,
  fields or statements; `2` lets `[]string{"a"}` stay on one line.
- `exclude`: patterns of files to leave alone, relative to the settings
  file's directory. `*` and `?` stay within a directory, `**` crosses
  directories: `"**/*_mock.go"`, `"internal/legacy/**"`.
- `format`: what formats the file after unfolding, also for files `unfold`
  did not otherwise change:
  - `gofmt`: gofmt's layout, always applied first;
  - `gofumpt`: gofumpt, built in, with the language version and module path
    of the nearest `go.mod`;
  - any other value is a command and its arguments, separated by spaces, that
    reads Go source on standard input and writes it formatted to standard
    output, for example `golangci-lint fmt --stdin` or `goimports`.

## golangci-lint

The plugin reports each inlined construct and fixes it with `--fix`; it takes
its settings from `.unfold.yml`, and `//nolint:unfold` works as for any
linter. golangci-lint loads plugins into a custom build. Describe the build in
`.custom-gcl.yml`:

```yaml
version: v2.13.2
plugins:
  - module: github.com/imkonsowa/unfold
    import: github.com/imkonsowa/unfold/golangci
    version: v0.2.0
```

Build it, as `./custom-gcl`:

```sh
golangci-lint custom
```

Enable the linter in `.golangci.yml`:

```yaml
version: "2"
linters:
  enable:
    - unfold
  settings:
    custom:
      unfold:
        type: module
        description: Puts each literal element, struct field and function literal statement on its own line.
```

Then run the custom build as you run golangci-lint:

```sh
./custom-gcl run --fix ./...
```

## go vet

`unfold-vet` runs the same analyzer. Install it:

```sh
go install github.com/imkonsowa/unfold/cmd/unfold-vet@latest
```

Run it through `go vet`:

```sh
go vet -vettool="$(command -v unfold-vet)" ./...
```

Or on its own, applying the fixes:

```sh
unfold-vet -fix ./...
```

## pre-commit

Add the hook to `.pre-commit-config.yaml`; it unfolds and formats the staged
Go files, and the commit stops when it changed any:

```yaml
repos:
  - repo: https://github.com/imkonsowa/unfold
    rev: v0.2.0
    hooks:
      - id: unfold
```

## License

MIT
