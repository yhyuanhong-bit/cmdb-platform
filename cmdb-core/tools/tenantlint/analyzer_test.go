package tenantlint_test

import (
	"path/filepath"
	"testing"

	"github.com/cmdb-platform/cmdb-core/tools/tenantlint"

	"golang.org/x/tools/go/analysis/analysistest"
)

// TestAnalyzer runs analysistest against the testdata tree. Each fixture
// file annotates the expected diagnostics with `// want "..."` lines.
func TestAnalyzer(t *testing.T) {
	testdata, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatalf("resolve testdata: %v", err)
	}
	analysistest.Run(t, testdata, tenantlint.Analyzer,
		"example/direct",
		"example/scoped",
		"example/allowed",
	)
}
