// Package config reads .unfold.yml, the settings the unfold command and its
// analyzer share.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/imkonsowa/unfold/layout"
)

// FileName is the settings file, looked up from a directory upwards.
const FileName = ".unfold.yml"

// Config is unfold's settings; Dir is the settings file's directory, which
// Exclude patterns are relative to.
type Config struct {
	Base     string
	Format   string
	Rules    layout.Rules
	Exclude  []string
	Dir      string
	excluded []*regexp.Regexp
}

type settingsFile struct {
	Base              string   `yaml:"base"`
	Format            string   `yaml:"format"`
	CompositeLiterals *bool    `yaml:"composite-literals"`
	StructTypes       *bool    `yaml:"struct-types"`
	FunctionLiterals  *bool    `yaml:"function-literals"`
	MinElements       *int     `yaml:"min-elements"`
	Exclude           []string `yaml:"exclude"`
}

// Default is the configuration when no settings file is found.
func Default() Config {
	return Config{
		Base:   "origin/main",
		Format: "gofmt",
		Rules:  layout.DefaultRules(),
	}
}

// Find reads the nearest settings file in dir or one of its parents, or
// returns Default when there is none.
func Find(dir string) (Config, error) {
	current, err := filepath.Abs(dir)
	if err != nil {
		return Config{}, err
	}

	for {
		path := filepath.Join(current, FileName)

		_, err := os.Stat(path)

		switch {
		case err == nil:
			return Load(path)
		case !errors.Is(err, fs.ErrNotExist):
			return Config{}, err
		}

		parent := filepath.Dir(current)
		if parent == current {
			return Default(), nil
		}

		current = parent
	}
}

// Load reads the settings file at path.
func Load(path string) (Config, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return Config{}, err
	}

	raw, err := os.ReadFile(abs) //nolint:gosec // G304: the settings file asked for
	if err != nil {
		return Config{}, err
	}

	var settings settingsFile

	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)

	if err := decoder.Decode(&settings); err != nil && !errors.Is(err, io.EOF) {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}

	c := Default()
	c.Dir = filepath.Dir(abs)

	if settings.Base != "" {
		c.Base = settings.Base
	}

	if settings.Format != "" {
		c.Format = settings.Format
	}

	if settings.CompositeLiterals != nil {
		c.Rules.CompositeLiterals = *settings.CompositeLiterals
	}

	if settings.StructTypes != nil {
		c.Rules.StructTypes = *settings.StructTypes
	}

	if settings.FunctionLiterals != nil {
		c.Rules.FunctionLiterals = *settings.FunctionLiterals
	}

	if settings.MinElements != nil {
		if *settings.MinElements < 1 {
			return Config{}, fmt.Errorf("%s: min-elements is %d, it must be 1 or more", path, *settings.MinElements)
		}

		c.Rules.MinElements = *settings.MinElements
	}

	for _, pattern := range settings.Exclude {
		excluded, err := globRegexp(pattern)
		if err != nil {
			return Config{}, fmt.Errorf("%s: exclude %q: %w", path, pattern, err)
		}

		c.Exclude = append(c.Exclude, pattern)
		c.excluded = append(c.excluded, excluded)
	}

	return c, nil
}

// Excludes reports whether path matches one of the exclude patterns.
func (c Config) Excludes(path string) bool {
	if len(c.excluded) == 0 {
		return false
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	rel, err := filepath.Rel(c.Dir, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}

	rel = filepath.ToSlash(rel)

	for _, excluded := range c.excluded {
		if excluded.MatchString(rel) {
			return true
		}
	}

	return false
}

func globRegexp(pattern string) (*regexp.Regexp, error) {
	var expr strings.Builder

	expr.WriteString("^")

	for i := 0; i < len(pattern); i++ {
		switch rest := pattern[i:]; {
		case strings.HasPrefix(rest, "**/"):
			expr.WriteString("(?:.*/)?")
			i += 2
		case strings.HasPrefix(rest, "**"):
			expr.WriteString(".*")
			i++
		case rest[0] == '*':
			expr.WriteString("[^/]*")
		case rest[0] == '?':
			expr.WriteString("[^/]")
		default:
			expr.WriteString(regexp.QuoteMeta(rest[:1]))
		}
	}

	expr.WriteString("$")

	return regexp.Compile(expr.String())
}
