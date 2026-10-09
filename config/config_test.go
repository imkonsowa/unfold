package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/imkonsowa/unfold/layout"
)

func writeSettings(t *testing.T, dir, content string) string {
	t.Helper()

	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}

func TestLoad(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		content   string
		wantRules layout.Rules
		wantBase  string
		wantFmt   string
		wantErr   string
	}{
		{
			name:      "empty file keeps the defaults",
			content:   "",
			wantRules: layout.DefaultRules(),
			wantBase:  "origin/main",
			wantFmt:   "gofmt",
		},
		{
			name:    "every setting",
			content: "base: main\nformat: gofumpt\ncomposite-literals: false\nstruct-types: false\nfunction-literals: false\nmin-elements: 2\n",
			wantRules: layout.Rules{
				MinElements: 2,
			},
			wantBase: "main",
			wantFmt:  "gofumpt",
		},
		{
			name:    "an unknown key",
			content: "min-element: 2\n",
			wantErr: "field min-element not found",
		},
		{
			name:    "min-elements below one",
			content: "min-elements: 0\n",
			wantErr: "must be 1 or more",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, err := Load(writeSettings(t, t.TempDir(), tt.content))

			switch {
			case tt.wantErr != "":
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Load = %v, want an error containing %q", err, tt.wantErr)
				}

				return
			case err != nil:
				t.Fatal(err)
			}

			if !reflect.DeepEqual(c.Rules, tt.wantRules) || c.Base != tt.wantBase || c.Format != tt.wantFmt {
				t.Errorf("Load = %+v %q %q, want %+v %q %q", c.Rules, c.Base, c.Format, tt.wantRules, tt.wantBase, tt.wantFmt)
			}
		})
	}
}

func TestFind(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	nested := filepath.Join(root, "a", "b")

	if err := os.MkdirAll(nested, 0o750); err != nil {
		t.Fatal(err)
	}

	writeSettings(t, root, "min-elements: 3\n")

	c, err := Find(nested)
	if err != nil {
		t.Fatal(err)
	}

	if c.Rules.MinElements != 3 || c.Dir != root {
		t.Errorf("Find = min-elements %d in %q, want 3 in %q", c.Rules.MinElements, c.Dir, root)
	}
}

func TestExcludes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	c, err := Load(writeSettings(t, root, "exclude:\n  - \"**/*_mock.go\"\n  - \"internal/legacy/**\"\n  - \"gen?.go\"\n"))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		path string
		want bool
	}{
		{
			path: "store_mock.go",
			want: true,
		},
		{
			path: "a/b/store_mock.go",
			want: true,
		},
		{
			path: "internal/legacy/x/old.go",
			want: true,
		},
		{
			path: "internal/legacyish.go",
			want: false,
		},
		{
			path: "gen1.go",
			want: true,
		},
		{
			path: "a/gen1.go",
			want: false,
		},
		{
			path: "../outside_mock.go",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()

			if got := c.Excludes(filepath.Join(root, tt.path)); got != tt.want {
				t.Errorf("Excludes(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}
