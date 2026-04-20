// Command errcheck runs the project errcheck analyzer as a
// standalone vet tool:
//
//	go vet -vettool=$(which errcheck) ./...
//
// See tools/errcheck/analyzer.go for the patterns this analyzer
// watches and the per-line //errcheck:allow escape hatch.
package main

import (
	"github.com/cmdb-platform/cmdb-core/tools/errcheck"

	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(errcheck.Analyzer)
}
