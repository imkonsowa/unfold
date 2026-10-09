// Command unfold-vet runs unfold's analyzer on packages, with -fix to apply
// its fixes, or as go vet -vettool.
package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"github.com/imkonsowa/unfold/analyzer"
)

func main() {
	singlechecker.Main(analyzer.Analyzer)
}
