// Package golangci registers unfold's analyzer as a golangci-lint module
// plugin; its rules come from .unfold.yml.
package golangci

import (
	"errors"
	"reflect"

	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"

	"github.com/imkonsowa/unfold/analyzer"
)

func init() {
	register.Plugin("unfold", New)
}

type plugin struct{}

// New returns the plugin; it takes no settings, since .unfold.yml holds them.
func New(settings any) (register.LinterPlugin, error) {
	if settings != nil && reflect.ValueOf(settings).Len() > 0 {
		return nil, errors.New("unfold takes no settings in .golangci.yml: put them in .unfold.yml")
	}

	return plugin{}, nil
}

func (plugin) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	return []*analysis.Analyzer{
		analyzer.Analyzer,
	}, nil
}

func (plugin) GetLoadMode() string {
	return register.LoadModeSyntax
}
