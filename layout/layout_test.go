package layout

import (
	"go/ast"
	"go/parser"
	"go/token"
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

			got, err := DefaultRules().Unfold([]byte(tt.in))
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

	if got, err := DefaultRules().Unfold([]byte(src)); err != nil || string(got) != out {
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

func TestRulesUnfold(t *testing.T) {
	t.Parallel()

	src := "package p\n\nvar v = T{A: 1}\nvar w = T{A: 1, B: 2}\nvar s struct{ A int }\nvar f = func() { g() }\n"

	tests := []struct {
		name  string
		rules Rules
		want  string
	}{
		{
			name: "composite literals off",
			rules: Rules{
				StructTypes:      true,
				FunctionLiterals: true,
				MinElements:      1,
			},
			want: "package p\n\nvar v = T{A: 1}\nvar w = T{A: 1, B: 2}\nvar s struct {\n\tA int\n}\nvar f = func() {\n\tg()\n}\n",
		},
		{
			name: "struct types and function literals off",
			rules: Rules{
				CompositeLiterals: true,
				MinElements:       1,
			},
			want: "package p\n\nvar v = T{\n\tA: 1,\n}\nvar w = T{\n\tA: 1,\n\tB: 2,\n}\nvar s struct{ A int }\nvar f = func() { g() }\n",
		},
		{
			name: "two elements at least",
			rules: Rules{
				CompositeLiterals: true,
				StructTypes:       true,
				FunctionLiterals:  true,
				MinElements:       2,
			},
			want: "package p\n\nvar v = T{A: 1}\nvar w = T{\n\tA: 1,\n\tB: 2,\n}\nvar s struct{ A int }\nvar f = func() { g() }\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := tt.rules.Unfold([]byte(src))
			if err != nil {
				t.Fatal(err)
			}

			if string(got) != tt.want {
				t.Errorf("Unfold:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestRulesFindings(t *testing.T) {
	t.Parallel()

	src := "package p\n\nvar v = []T{{A: 1}}\n\nvar w = T{\n\tA: 1,\n}\n\nvar s struct{ A int }\n"
	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, "p.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	findings, err := DefaultRules().Findings(fset, file)
	if err != nil {
		t.Fatal(err)
	}

	var got []string
	for _, f := range findings {
		got = append(got, string(f.Kind)+" at "+fset.Position(f.Node.Pos()).String())
	}

	want := []string{
		"composite literal at p.go:3:9",
		"composite literal at p.go:3:13",
		"struct type at p.go:9:7",
	}
	if strings.Join(got, "; ") != strings.Join(want, "; ") {
		t.Errorf("Findings = %q, want %q", got, want)
	}

	if _, ok := findings[0].Node.(*ast.CompositeLit); !ok {
		t.Errorf("first finding is a %T, want *ast.CompositeLit", findings[0].Node)
	}
}

func TestCorrespondenceSpan(t *testing.T) {
	t.Parallel()

	src := []byte("package p\n\nvar v = T{A: 1, B: U{C: 2}} // v\n")

	out, err := DefaultRules().Unfold(src)
	if err != nil {
		t.Fatal(err)
	}

	c, err := NewCorrespondence(src, out)
	if err != nil {
		t.Fatal(err)
	}

	start := strings.Index(string(src), "U{")
	end := strings.Index(string(src), "}}") + 1

	outStart, outEnd, err := c.Span(start, end)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := string(out[outStart:outEnd]), "U{\n\t\tC: 2,\n\t}"; got != want {
		t.Errorf("Span = %q, want %q", got, want)
	}

	if _, _, err := c.Span(1, end); err == nil {
		t.Error("Span inside a token = nil error, want one")
	}
}
