// Command unfold puts every element of a composite literal (struct, map and
// slice values), every field of a struct type and every statement of a
// function literal on its own line, then formats the file.
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/imkonsowa/unfold/config"
	"github.com/imkonsowa/unfold/formatting"
	"github.com/imkonsowa/unfold/layout"
)

var generated = regexp.MustCompile(`(?m)^// Code generated .* DO NOT EDIT\.$`)

type options struct {
	write         bool
	changed       bool
	base          string
	settingsPath  string
	formatSetting string
	args          []string
}

func main() {
	var o options

	flag.BoolVar(&o.write, "w", false, "rewrite the files instead of listing them")
	flag.BoolVar(&o.changed, "changed", false, "the Go files changed against -base, in the working tree too")
	flag.StringVar(&o.base, "base", "", "the revision -changed compares with (default origin/main, or base in "+config.FileName+")")
	flag.StringVar(&o.settingsPath, "config", "", "the settings file (default the nearest "+config.FileName+" from the current directory up)")
	flag.StringVar(
		&o.formatSetting,
		"format",
		"",
		"gofmt, gofumpt, or a command that formats Go from stdin to stdout (default gofmt, or format in "+config.FileName+")",
	)
	equivalent := flag.Bool("equivalent", false, "check that the second file is the first with only layout changed")

	flag.Parse()

	if *equivalent {
		os.Exit(checkEquivalent(flag.Args()))
	}

	o.args = flag.Args()

	os.Exit(o.run())
}

func (o options) run() int {
	c, err := o.settings()
	if err != nil {
		fmt.Fprintln(os.Stderr, "unfold:", err)
		return 2
	}

	formatter, err := formatting.Parse(c.Format)
	if err != nil {
		fmt.Fprintln(os.Stderr, "unfold:", err)
		return 2
	}

	files, err := targets(o.changed, c.Base, o.args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "unfold:", err)
		return 2
	}

	var pending []string

	for _, name := range files {
		if c.Excludes(name) {
			continue
		}

		changed, err := rewrite(c.Rules, formatter, name, o.write)
		if err != nil {
			fmt.Fprintf(os.Stderr, "unfold: %s: %v\n", name, err)
			return 2
		}

		if changed {
			pending = append(pending, name)
		}
	}

	if o.write || len(pending) == 0 {
		return 0
	}

	fmt.Fprintln(os.Stderr, "unfold: inlined literals, struct types or function literals, or unformatted code; run unfold -w on:")

	for _, name := range pending {
		fmt.Fprintln(os.Stderr, "  "+name)
	}

	return 1
}

func (o options) settings() (config.Config, error) {
	var (
		c   config.Config
		err error
	)

	if o.settingsPath != "" {
		c, err = config.Load(o.settingsPath)
	} else {
		c, err = config.Find(".")
	}

	if err != nil {
		return config.Config{}, err
	}

	if o.base != "" {
		c.Base = o.base
	}

	if o.formatSetting != "" {
		c.Format = o.formatSetting
	}

	return c, nil
}

func rewrite(rules layout.Rules, formatter formatting.Formatter, name string, write bool) (bool, error) {
	src, err := os.ReadFile(name) //nolint:gosec // G304: the files asked for
	if err != nil {
		return false, err
	}

	if generated.Match(src) {
		return false, nil
	}

	out, err := rules.Unfold(src)
	if err != nil {
		return false, err
	}

	out, err = formatter.Format(name, out)
	if err != nil {
		return false, err
	}

	if bytes.Equal(out, src) {
		return false, nil
	}

	if write {
		//nolint:gosec // G306: source files are world-readable
		if err := os.WriteFile(name, out, 0o644); err != nil {
			return false, err
		}
	}

	return true, nil
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

	if err := layout.Equivalent(src, out); err != nil {
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
	top, err := git("rev-parse", "--show-toplevel")
	if err != nil {
		return nil, err
	}

	top = strings.TrimSpace(top)

	diff, err := git("-C", top, "diff", "--name-only", "--diff-filter=ACMR", base, "--", "*.go")
	if err != nil {
		return nil, err
	}

	untracked, err := git("-C", top, "ls-files", "--others", "--exclude-standard", "--", "*.go")
	if err != nil {
		return nil, err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	var files []string

	for name := range strings.FieldsSeq(diff + "\n" + untracked) {
		if strings.Contains("/"+name, "/node_modules/") {
			continue
		}

		rel, err := filepath.Rel(cwd, filepath.Join(top, name))
		if err != nil {
			return nil, err
		}

		files = append(files, rel)
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
