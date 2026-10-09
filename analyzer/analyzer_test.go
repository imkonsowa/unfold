package analyzer

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzer(t *testing.T) {
	t.Parallel()

	analysistest.RunWithSuggestedFixes(t, analysistest.TestData(), Analyzer, "inlined")
}

func TestAnalyzerFollowsSettings(t *testing.T) {
	t.Parallel()

	analysistest.Run(t, analysistest.TestData(), Analyzer, "configured")
}
