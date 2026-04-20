package errcheck_test

import (
	"path/filepath"
	"testing"

	"github.com/cmdb-platform/cmdb-core/tools/errcheck"

	"golang.org/x/tools/go/analysis/analysistest"
)

// TestAnalyzer runs analysistest against the testdata tree. Each fixture
// file annotates its expected diagnostics with `// want "..."` lines;
// the allowed fixture carries no annotations and must stay clean because
// every call is guarded by the per-line //errcheck:allow directive.
func TestAnalyzer(t *testing.T) {
	testdata, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatalf("resolve testdata: %v", err)
	}
	analysistest.Run(t, testdata, errcheck.Analyzer,
		"example/bad",
		"example/good",
		"example/allowed",
	)
}
