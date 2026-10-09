package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), "git", append([]string{
		"-C",
		dir,
	}, args...)...)
	cmd.Env = append(
		os.Environ(),
		"GIT_AUTHOR_NAME=t",
		"GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t",
		"GIT_COMMITTER_EMAIL=t@example.com",
	)

	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestRunChangedFromASubdirectory(t *testing.T) {
	repo := t.TempDir()

	writeFile(t, filepath.Join(repo, "go.mod"), "module example.com/m\n\ngo 1.26\n")
	writeFile(t, filepath.Join(repo, "sub", "kept.go"), "package sub\n\nvar kept = 1\n")
	writeFile(t, filepath.Join(repo, "other", "other.go"), "package other\n\nvar other = 1\n")
	runGit(t, repo, "init", "-q")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-q", "-m", "start")

	writeFile(t, filepath.Join(repo, ".unfold.yml"), "format: gofumpt\nexclude:\n  - \"**/skipped.go\"\n")
	writeFile(t, filepath.Join(repo, "sub", "kept.go"), "package sub\n\nvar kept = []int{1, 2}\n\nconst mode = 0755\n")
	writeFile(t, filepath.Join(repo, "sub", "added.go"), "package sub\n\nvar added = map[string]int{\"a\": 1}\n")
	writeFile(t, filepath.Join(repo, "sub", "skipped.go"), "package sub\n\nvar skipped = []int{1, 2}\n")
	writeFile(t, filepath.Join(repo, "other", "other.go"), "package other\n\nvar other = []int{1}\n")

	t.Chdir(filepath.Join(repo, "sub"))

	files, err := changedFiles("HEAD")
	if err != nil {
		t.Fatal(err)
	}

	slices.Sort(files)

	if want := []string{
		"../other/other.go",
		"added.go",
		"kept.go",
		"skipped.go",
	}; !slices.Equal(files, want) {
		t.Fatalf("changedFiles = %q, want %q", files, want)
	}

	check := options{
		changed: true,
		base:    "HEAD",
	}
	if code := check.run(); code != 1 {
		t.Errorf("run without -w = %d, want 1", code)
	}

	write := check
	write.write = true

	if code := write.run(); code != 0 {
		t.Fatalf("run -w = %d, want 0", code)
	}

	for name, want := range map[string]string{
		"kept.go":           "package sub\n\nvar kept = []int{\n\t1,\n\t2,\n}\n\nconst mode = 0o755\n",
		"added.go":          "package sub\n\nvar added = map[string]int{\n\t\"a\": 1,\n}\n",
		"skipped.go":        "package sub\n\nvar skipped = []int{1, 2}\n",
		"../other/other.go": "package other\n\nvar other = []int{\n\t1,\n}\n",
	} {
		got, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}

		if string(got) != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}

	if code := check.run(); code != 0 {
		t.Errorf("run without -w after -w = %d, want 0", code)
	}
}
