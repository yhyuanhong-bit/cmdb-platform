// Command tenantlint runs the tenantlint analyzer as a standalone vet tool.
//
//	go vet -vettool=$(which tenantlint) ./...
package main

import (
	"github.com/cmdb-platform/cmdb-core/tools/tenantlint"

	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(tenantlint.Analyzer)
}
