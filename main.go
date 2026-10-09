// Command unfold puts every element of a composite literal (struct, map and
// slice values), every field of a struct type and every statement of a
// function literal on its own line.
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
)

var generated = regexp.MustCompile(`(?m)^// Code generated .* DO NOT EDIT\.$`)

func main() {
	write := flag.Bool("w", false, "rewrite the files instead of listing them")
	changed := flag.Bool("changed", false, "the Go files changed against -base, in the working tree too")
	base := flag.String("base", "origin/main", "the revision -changed compares with")
	equivalent := flag.Bool("equivalent", false, "check that the second file is the first with only layout changed")

	flag.Parse()

	if *equivalent {
		os.Exit(checkEquivalent(flag.Args()))
	}

	files, err := targets(*changed, *base, flag.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, "unfold:", err)
		os.Exit(2)
	}

	var pending []string

	for _, name := range files {
		src, err := os.ReadFile(name) //nolint:gosec // G304: the files asked for
		if err != nil {
			fmt.Fprintln(os.Stderr, "unfold:", err)
			os.Exit(2)
		}

		if generated.Match(src) {
			continue
		}

		out, err := Unfold(src)
		if err != nil {
			fmt.Fprintf(os.Stderr, "unfold: %s: %v\n", name, err)
			os.Exit(2)
		}

		if bytes.Equal(out, src) {
			continue
		}

		pending = append(pending, name)

		if *write {
			//nolint:gosec // G306: source files are world-readable
			if err := os.WriteFile(name, out, 0o644); err != nil {
				fmt.Fprintln(os.Stderr, "unfold:", err)
				os.Exit(2)
			}
		}
	}

	if *write || len(pending) == 0 {
		return
	}

	fmt.Fprintln(
		os.Stderr,
		"unfold: inline literals, struct types or function literals; run unfold -w on:",
	)

	for _, name := range pending {
		fmt.Fprintln(os.Stderr, "  "+name)
	}

	os.Exit(1)
}

func checkEquivalent(args []string) int {
	if len(args) != 2 {
		fmt.Fprintln(os.Stderr, "unfold: -equivalent takes two files")
		return 2
	}

	src, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "unfold:", err)
		return 2
	}

	out, err := os.ReadFile(args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "unfold:", err)
		return 2
	}

	if err := Equivalent(src, out); err != nil {
		fmt.Fprintf(os.Stderr, "unfold: %s: %v\n", args[1], err)
		return 1
	}

	return 0
}

func targets(changed bool, base string, args []string) ([]string, error) {
	if changed {
		return changedFiles(base)
	}

	if len(args) == 0 {
		return nil, errors.New("name files or directories, or pass -changed")
	}

	var files []string

	for _, arg := range args {
		dir, recursive := strings.CutSuffix(arg, "/...")
		if !recursive && strings.HasSuffix(arg, ".go") {
			files = append(files, arg)
			continue
		}

		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			switch {
			case err != nil:
				return err
			case d.IsDir() && path != dir && (!recursive || skipDir(d.Name())):
				return filepath.SkipDir
			case !d.IsDir() && strings.HasSuffix(path, ".go"):
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

func skipDir(name string) bool {
	return name == "node_modules" || name == "vendor" || name == "testdata" || strings.HasPrefix(name, ".")
}

func changedFiles(base string) ([]string, error) {
	diff, err := git("diff", "--name-only", "--diff-filter=ACMR", base, "--", "*.go")
	if err != nil {
		return nil, err
	}

	untracked, err := git("ls-files", "--others", "--exclude-standard", "--", "*.go")
	if err != nil {
		return nil, err
	}

	var files []string

	for name := range strings.FieldsSeq(diff + "\n" + untracked) {
		if !strings.Contains("/"+name, "/node_modules/") {
			files = append(files, name)
		}
	}

	return files, nil
}

func git(args ...string) (string, error) {
	//nolint:gosec // G204: fixed git subcommands
	out, err := exec.CommandContext(context.Background(), "git", args...).Output()
	if err != nil {
		if exit, ok := errors.AsType[*exec.ExitError](err); ok {
			return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, exit.Stderr)
		}

		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}

	return string(out), nil
}

// Unfold returns src with every non-empty composite literal, struct type
// and function literal spread one element, field or statement per line.
func Unfold(src []byte) ([]byte, error) {
	f, err := decorator.Parse(src)
	if err != nil {
		return nil, err
	}

	dst.Inspect(f, func(n dst.Node) bool {
		switch n.(type) {
		case dst.Stmt, *dst.GenDecl:
			if _, block := n.(*dst.BlockStmt); !block && folded(n) {
				hoistNolint(n)
			}
		}

		return true
	})

	dst.Inspect(f, func(n dst.Node) bool {
		switch x := n.(type) {
		case *dst.CompositeLit:
			spread(len(x.Elts), func(i int) *dst.NodeDecs {
				return x.Elts[i].Decorations()
			})
		case *dst.StructType:
			spread(len(x.Fields.List), func(i int) *dst.NodeDecs {
				return &x.Fields.List[i].Decs.NodeDecs
			})
		case *dst.FuncLit:
			spread(len(x.Body.List), func(i int) *dst.NodeDecs {
				return x.Body.List[i].Decorations()
			})
		}

		return true
	})

	var buf bytes.Buffer
	if err := decorator.Fprint(&buf, f); err != nil {
		return nil, err
	}

	out, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, err
	}

	if err := Equivalent(src, out); err != nil {
		return nil, fmt.Errorf("rewrite refused, it would change more than layout: %w", err)
	}

	return out, nil
}

func within(n dst.Node, visit func(dst.Node)) {
	dst.Inspect(n, func(c dst.Node) bool {
		if _, block := c.(*dst.BlockStmt); block && c != n {
			return false
		}

		if c != nil {
			visit(c)
		}

		return true
	})
}

func folded(n dst.Node) bool {
	found := false

	within(n, func(c dst.Node) {
		switch x := c.(type) {
		case *dst.CompositeLit:
			found = found || inline(len(x.Elts), func(i int) *dst.NodeDecs {
				return x.Elts[i].Decorations()
			})
		case *dst.StructType:
			found = found || inline(len(x.Fields.List), func(i int) *dst.NodeDecs {
				return &x.Fields.List[i].Decs.NodeDecs
			})
		case *dst.FuncLit:
			found = found || inline(len(x.Body.List), func(i int) *dst.NodeDecs {
				return x.Body.List[i].Decorations()
			})
		}
	})

	return found
}

func inline(n int, decs func(int) *dst.NodeDecs) bool {
	for i := range n {
		if decs(i).Before == dst.None {
			return true
		}
	}

	return n > 0 && decs(n-1).After == dst.None
}

func hoistNolint(n dst.Node) {
	var hoisted []string

	within(n, func(c dst.Node) {
		d := c.Decorations()

		var kept dst.Decorations

		for _, comment := range d.End {
			if strings.HasPrefix(comment, "//nolint") {
				hoisted = append(hoisted, comment)
			} else {
				kept = append(kept, comment)
			}
		}

		d.End = kept
	})

	if len(hoisted) > 0 {
		d := n.Decorations()
		d.Start = append(d.Start, hoisted...)
		d.Before = dst.NewLine
	}
}

func spread(n int, decs func(int) *dst.NodeDecs) {
	for i := range n {
		if d := decs(i); d.Before == dst.None {
			d.Before = dst.NewLine
		}
	}

	if n > 0 {
		if d := decs(n - 1); d.After == dst.None {
			d.After = dst.NewLine
		}
	}
}
