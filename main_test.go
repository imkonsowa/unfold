package main

import (
	"strings"
	"testing"
)

func TestUnfold(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "struct literal",
			in:   "package p\n\nvar v = T{A: 1, B: 2}\n",
			want: "package p\n\nvar v = T{\n\tA: 1,\n\tB: 2,\n}\n",
		},
		{
			name: "one field",
			in:   "package p\n\nvar v = T{A: 1}\n",
			want: "package p\n\nvar v = T{\n\tA: 1,\n}\n",
		},
		{
			name: "map and slice",
			in:   "package p\n\nvar m = map[string]int{\"a\": 1}\nvar s = []string{\"a\", \"b\"}\n",
			want: "package p\n\nvar m = map[string]int{\n\t\"a\": 1,\n}\nvar s = []string{\n\t\"a\",\n\t\"b\",\n}\n",
		},
		{
			name: "nested",
			in:   "package p\n\nvar v = T{A: U{B: 1}}\n",
			want: "package p\n\nvar v = T{\n\tA: U{\n\t\tB: 1,\n\t},\n}\n",
		},
		{
			name: "struct type",
			in:   "package p\n\nvar v struct{ Name, Repo string }\n",
			want: "package p\n\nvar v struct {\n\tName, Repo string\n}\n",
		},
		{
			name: "function literal",
			in:   "package p\n\nfunc f() {\n\tdefer func() { _ = g() }()\n}\n",
			want: "package p\n\nfunc f() {\n\tdefer func() {\n\t\t_ = g()\n\t}()\n}\n",
		},
		{
			name: "a //nolint on a split line moves above it",
			in:   "package p\n\nfunc f() *T {\n\treturn &T{A: md5.New()} //nolint:gosec // wire checksums\n}\n",
			want: "package p\n\nfunc f() *T {\n\t//nolint:gosec // wire checksums\n\treturn &T{\n\t\tA: md5.New(),\n\t}\n}\n",
		},
		{
			name: "a //nolint on a line left alone stays",
			in:   "package p\n\nfunc f() {\n\tg() //nolint:errcheck // why\n}\n",
			want: "package p\n\nfunc f() {\n\tg() //nolint:errcheck // why\n}\n",
		},
		{
			name: "empty ones stay",
			in:   "package p\n\nvar v = T{}\nvar e struct{}\nvar f = func() {}\n",
			want: "package p\n\nvar v = T{}\nvar e struct{}\nvar f = func() {}\n",
		},
		{
			name: "unfolded code is unchanged, comments and blank lines kept",
			in:   "package p\n\nvar v = T{\n\t// A is first.\n\tA: 1,\n\n\tB: 2, // B\n}\n",
			want: "package p\n\nvar v = T{\n\t// A is first.\n\tA: 1,\n\n\tB: 2, // B\n}\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := Unfold([]byte(tt.in))
			if err != nil {
				t.Fatal(err)
			}

			if string(got) != tt.want {
				t.Errorf("Unfold:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestEquivalentSeparators(t *testing.T) {
	t.Parallel()

	src := "package p\n\nvar f = func() int { a := 1; return a }\n"
	out := "package p\n\nvar f = func() int {\n\ta := 1\n\treturn a\n}\n"

	if err := Equivalent([]byte(src), []byte(out)); err != nil {
		t.Errorf("Equivalent = %v, want nil", err)
	}

	if got, err := Unfold([]byte(src)); err != nil || string(got) != out {
		t.Errorf("Unfold = %q, %v; want %q", got, err, out)
	}
}

func TestEquivalent(t *testing.T) {
	t.Parallel()

	src := "package p\n\n// V is a value.\nvar v = T{A: 1, B: 2} // trailing\n"

	tests := []struct {
		name    string
		out     string
		wantErr string
	}{
		{
			name: "unfolded",
			out:  "package p\n\n// V is a value.\nvar v = T{\n\tA: 1,\n\tB: 2,\n} // trailing\n",
		},
		{
			name: "statement separators became line breaks",
			out:  "package p\n\n// V is a value.\nvar v = T{A: 1, B: 2} // trailing\n",
		},
		{
			name:    "a comment lost",
			out:     "package p\n\nvar v = T{\n\tA: 1,\n\tB: 2,\n} // trailing\n",
			wantErr: "comments differ",
		},
		{
			name:    "a comment changed",
			out:     "package p\n\n// V is a value!\nvar v = T{\n\tA: 1,\n\tB: 2,\n} // trailing\n",
			wantErr: "comments differ",
		},
		{
			name:    "a value changed",
			out:     "package p\n\n// V is a value.\nvar v = T{\n\tA: 1,\n\tB: 3,\n} // trailing\n",
			wantErr: "code differs",
		},
		{
			name:    "an element lost",
			out:     "package p\n\n// V is a value.\nvar v = T{\n\tA: 1,\n} // trailing\n",
			wantErr: "code differs",
		},
		{
			name:    "a comma that is not layout",
			out:     "package p\n\n// V is a value.\nvar v = T{A: 1 B: 2} // trailing\n",
			wantErr: "code differs",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := Equivalent([]byte(src), []byte(tt.out))

			switch {
			case tt.wantErr == "" && err != nil:
				t.Errorf("Equivalent = %v, want nil", err)
			case tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)):
				t.Errorf("Equivalent = %v, want %q", err, tt.wantErr)
			}
		})
	}
}
