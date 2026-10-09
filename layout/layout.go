// Package layout puts every element of a composite literal, every field of a
// struct type and every statement of a function literal on its own line.
package layout

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
)

// Kind is a construct the rules unfold.
type Kind string

// The constructs the rules unfold.
const (
	CompositeLiteral Kind = "composite literal"
	StructType       Kind = "struct type"
	FunctionLiteral  Kind = "function literal"
)

// ElementName is what a construct of this kind holds, one per line.
func (k Kind) ElementName() string {
	switch k {
	case StructType:
		return "field"
	case FunctionLiteral:
		return "statement"
	default:
		return "element"
	}
}

// Rules says which constructs are unfolded: MinElements counts elements,
// fields or statements.
type Rules struct {
	CompositeLiterals bool
	StructTypes       bool
	FunctionLiterals  bool
	MinElements       int
}

// DefaultRules unfolds every non-empty composite literal, struct type and
// function literal.
func DefaultRules() Rules {
	return Rules{
		CompositeLiterals: true,
		StructTypes:       true,
		FunctionLiterals:  true,
		MinElements:       1,
	}
}

// Finding is an inlined construct the rules cover.
type Finding struct {
	Kind Kind
	Node ast.Node
}

type construct struct {
	kind     Kind
	count    int
	elements func(int) *dst.NodeDecs
}

func (r Rules) covered(n dst.Node) (construct, bool) {
	var (
		found   construct
		enabled bool
	)

	switch x := n.(type) {
	case *dst.CompositeLit:
		found = construct{
			kind:  CompositeLiteral,
			count: len(x.Elts),
			elements: func(i int) *dst.NodeDecs {
				return x.Elts[i].Decorations()
			},
		}
		enabled = r.CompositeLiterals
	case *dst.StructType:
		found = construct{
			kind:  StructType,
			count: len(x.Fields.List),
			elements: func(i int) *dst.NodeDecs {
				return &x.Fields.List[i].Decs.NodeDecs
			},
		}
		enabled = r.StructTypes
	case *dst.FuncLit:
		found = construct{
			kind:  FunctionLiteral,
			count: len(x.Body.List),
			elements: func(i int) *dst.NodeDecs {
				return x.Body.List[i].Decorations()
			},
		}
		enabled = r.FunctionLiterals
	default:
		return construct{}, false
	}

	return found, enabled && found.count >= max(r.MinElements, 1)
}

func (c construct) inlined() bool {
	for i := range c.count {
		if c.elements(i).Before == dst.None {
			return true
		}
	}

	return c.count > 0 && c.elements(c.count-1).After == dst.None
}

func (c construct) spread() {
	for i := range c.count {
		if d := c.elements(i); d.Before == dst.None {
			d.Before = dst.NewLine
		}
	}

	if c.count > 0 {
		if d := c.elements(c.count - 1); d.After == dst.None {
			d.After = dst.NewLine
		}
	}
}

// Unfold returns src with every construct the rules cover spread one
// element, field or statement per line, formatted as gofmt does.
func (r Rules) Unfold(src []byte) ([]byte, error) {
	f, err := decorator.Parse(src)
	if err != nil {
		return nil, err
	}

	dst.Inspect(f, func(n dst.Node) bool {
		switch n.(type) {
		case dst.Stmt, *dst.GenDecl:
			if _, block := n.(*dst.BlockStmt); !block && r.folded(n) {
				hoistNolint(n)
			}
		}

		return true
	})

	dst.Inspect(f, func(n dst.Node) bool {
		if c, ok := r.covered(n); ok {
			c.spread()
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

// Findings returns the inlined constructs of file, in source order; fset
// holds file, parsed with its comments.
func (r Rules) Findings(fset *token.FileSet, file *ast.File) ([]Finding, error) {
	d := decorator.NewDecorator(fset)

	f, err := d.DecorateFile(file)
	if err != nil {
		return nil, err
	}

	var findings []Finding

	dst.Inspect(f, func(n dst.Node) bool {
		if c, ok := r.covered(n); ok && c.inlined() {
			findings = append(findings, Finding{
				Kind: c.kind,
				Node: d.Ast.Nodes[n],
			})
		}

		return true
	})

	return findings, nil
}

func (r Rules) folded(n dst.Node) bool {
	found := false

	within(n, func(c dst.Node) {
		if covered, ok := r.covered(c); ok && covered.inlined() {
			found = true
		}
	})

	return found
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
