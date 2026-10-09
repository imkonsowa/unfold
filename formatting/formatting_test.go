package formatting

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setting string
		wantErr string
	}{
		{
			name:    "default",
			setting: "",
		},
		{
			name:    "gofmt",
			setting: "gofmt",
		},
		{
			name:    "gofumpt",
			setting: "gofumpt",
		},
		{
			name:    "a command",
			setting: "cat -u",
		},
		{
			name:    "a missing command",
			setting: "no-such-formatter-command --stdin",
			wantErr: "no-such-formatter-command",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := Parse(tt.setting)

			switch {
			case tt.wantErr == "" && err != nil:
				t.Errorf("Parse(%q) = %v, want nil", tt.setting, err)
			case tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)):
				t.Errorf("Parse(%q) = %v, want an error containing %q", tt.setting, err, tt.wantErr)
			}
		})
	}
}

func TestFormat(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "p.go")
	src := "package p\n\nconst mode   = 0755\n"

	tests := []struct {
		name    string
		setting string
		want    string
	}{
		{
			name:    "gofmt",
			setting: "gofmt",
			want:    "package p\n\nconst mode = 0755\n",
		},
		{
			name:    "gofumpt with the module's language version",
			setting: "gofumpt",
			want:    "package p\n\nconst mode = 0o755\n",
		},
		{
			name:    "a command after gofmt",
			setting: "tr 7 8",
			want:    "package p\n\nconst mode = 0855\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f, err := Parse(tt.setting)
			if err != nil {
				t.Fatal(err)
			}

			got, err := f.Format(path, []byte(src))
			if err != nil {
				t.Fatal(err)
			}

			if string(got) != tt.want {
				t.Errorf("Format = %q, want %q", got, tt.want)
			}
		})
	}
}
