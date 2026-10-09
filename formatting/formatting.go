// Package formatting formats Go source once unfold has rewritten it.
package formatting

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"
	gofumpt "mvdan.cc/gofumpt/format"
)

// Formatter formats Go source as gofmt does, then optionally as gofumpt
// does or as a command does.
type Formatter struct {
	gofumpt bool
	command []string
}

// Parse reads a format setting: "gofmt", "gofumpt", or a command and its
// arguments, separated by spaces, that reads Go source on standard input and
// writes it formatted to standard output.
func Parse(setting string) (Formatter, error) {
	fields := strings.Fields(setting)

	switch {
	case len(fields) == 0 || setting == "gofmt":
		return Formatter{}, nil
	case setting == "gofumpt":
		return Formatter{
			gofumpt: true,
		}, nil
	}

	if _, err := exec.LookPath(fields[0]); err != nil {
		return Formatter{}, fmt.Errorf("format command %q: %w", fields[0], err)
	}

	return Formatter{
		command: fields,
	}, nil
}

// Format formats src, the content of the file at path.
func (f Formatter) Format(path string, src []byte) ([]byte, error) {
	out, err := format.Source(src)
	if err != nil {
		return nil, err
	}

	switch {
	case f.gofumpt:
		return gofumptSource(path, out)
	case len(f.command) > 0:
		return f.runCommand(out)
	}

	return out, nil
}

func (f Formatter) runCommand(src []byte) ([]byte, error) {
	//nolint:gosec // G204: the command the settings name
	cmd := exec.CommandContext(context.Background(), f.command[0], f.command[1:]...)
	cmd.Stdin = bytes.NewReader(src)

	var stderr bytes.Buffer

	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %s", strings.Join(f.command, " "), err, strings.TrimSpace(stderr.String()))
	}

	return out, nil
}

func gofumptSource(path string, src []byte) ([]byte, error) {
	var options gofumpt.Options

	modPath, err := nearestGoMod(filepath.Dir(path))
	if err != nil {
		return nil, err
	}

	if modPath != "" {
		raw, err := os.ReadFile(modPath) //nolint:gosec // G304: the module's go.mod
		if err != nil {
			return nil, err
		}

		mod, err := modfile.ParseLax(modPath, raw, nil)
		if err != nil {
			return nil, err
		}

		if mod.Go != nil {
			options.LangVersion = "go" + mod.Go.Version
		}

		if mod.Module != nil {
			options.ModulePath = mod.Module.Mod.Path
		}
	}

	return gofumpt.Source(src, options)
}

func nearestGoMod(dir string) (string, error) {
	current, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}

	for {
		path := filepath.Join(current, "go.mod")

		_, err := os.Stat(path)

		switch {
		case err == nil:
			return path, nil
		case !errors.Is(err, fs.ErrNotExist):
			return "", err
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", nil
		}

		current = parent
	}
}
