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

`unfold` changes layout only. Run your formatter afterwards (`gofmt`,
`gofumpt` or `golangci-lint fmt`).

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

Unfold every Go file changed against `origin/main`, committed or not, from
the repository root:

```sh
unfold -changed -w
```

List the changed files that need unfolding without touching them; the exit
status is 1 when any does, which suits CI:

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
| `-base` | the revision `-changed` compares with (default `origin/main`) |
| `-equivalent` | check that the second file is the first with only layout changed |

Exit status: 0 when nothing needs unfolding (or after `-w`), 1 when files
need unfolding or `-equivalent` finds a difference, 2 on errors.

## License

MIT
