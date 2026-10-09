// Package analyzer reports inlined composite literals, struct types and
// function literals, following .unfold.yml, with fixes that unfold them.
package analyzer

import (
	"fmt"
	"go/ast"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/tools/go/analysis"

	"github.com/imkonsowa/unfold/config"
	"github.com/imkonsowa/unfold/layout"
)

// Analyzer reports the constructs unfold would spread one element, field or
// statement per line.
var Analyzer = &analysis.Analyzer{
	Name: "unfold",
	Doc:  "reports inlined composite literals, struct types and function literals: each element, field or statement belongs on its own line",
	URL:  "https://github.com/imkonsowa/unfold",
	Run:  run,
}

var configsByDir sync.Map

func configFor(dir string) (config.Config, error) {
	if cached, ok := configsByDir.Load(dir); ok {
		return cached.(config.Config), nil //nolint:forcetypeassert // only Config values are stored
	}

	c, err := config.Find(dir)
	if err != nil {
		return config.Config{}, err
	}

	configsByDir.Store(dir, c)

	return c, nil
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		name := pass.Fset.File(file.Pos()).Name()
		if ast.IsGenerated(file) || !strings.HasSuffix(name, ".go") {
			continue
		}

		c, err := configFor(filepath.Dir(name))
		if err != nil {
			return nil, err
		}

		if c.Excludes(name) {
			continue
		}

		findings, err := c.Rules.Findings(pass.Fset, file)
		if err != nil {
			return nil, err
		}

		if len(findings) == 0 {
			continue
		}

		fixes := unfoldingFixes(pass, c.Rules, name, findings)

		for i, f := range findings {
			pass.Report(analysis.Diagnostic{
				Pos:            f.Node.Pos(),
				End:            f.Node.End(),
				Message:        fmt.Sprintf("%s is inlined: put each %s on its own line", f.Kind, f.Kind.ElementName()),
				SuggestedFixes: fixes[i],
			})
		}
	}

	return nil, nil //nolint:nilnil // the analyzer has no result
}

func unfoldingFixes(
	pass *analysis.Pass,
	rules layout.Rules,
	name string,
	findings []layout.Finding,
) [][]analysis.SuggestedFix {
	fixes := make([][]analysis.SuggestedFix, len(findings))

	src, err := pass.ReadFile(name)
	if err != nil {
		return fixes
	}

	out, err := rules.Unfold(src)
	if err != nil {
		return fixes
	}

	correspondence, err := layout.NewCorrespondence(src, out)
	if err != nil {
		return fixes
	}

	tokens := pass.Fset.File(findings[0].Node.Pos())

	for i, f := range findings {
		if enclosed(findings, i) {
			continue
		}

		outStart, outEnd, err := correspondence.Span(tokens.Offset(f.Node.Pos()), tokens.Offset(f.Node.End()))
		if err != nil {
			continue
		}

		fixes[i] = []analysis.SuggestedFix{
			{
				Message: "unfold the " + string(f.Kind),
				TextEdits: []analysis.TextEdit{
					{
						Pos:     f.Node.Pos(),
						End:     f.Node.End(),
						NewText: out[outStart:outEnd],
					},
				},
			},
		}
	}

	return fixes
}

func enclosed(findings []layout.Finding, i int) bool {
	inner := findings[i].Node

	for j, f := range findings {
		if j != i && f.Node.Pos() <= inner.Pos() && inner.End() <= f.Node.End() {
			return true
		}
	}

	return false
}
