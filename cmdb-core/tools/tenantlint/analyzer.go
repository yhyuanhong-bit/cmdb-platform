// Package tenantlint provides a go/analysis Analyzer that flags direct
// use of *pgxpool.Pool.Exec/Query/QueryRow inside handler/domain code.
//
// Handlers and domain services must go through database.TenantScoped so
// the multi-tenant predicate cannot be forgotten. Truly cross-tenant work
// (background schedulers, cross-tenant reports) can opt out per file with
// the top-of-file comment:
//
//	//tenantlint:allow-direct-pool
package tenantlint

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// poolTypeName is the fully qualified type whose direct Exec/Query/QueryRow
// calls we want to flag. We compare against the pointer form because pgxpool
// methods are defined on *Pool.
const poolTypeName = "*github.com/jackc/pgx/v5/pgxpool.Pool"

// flaggedMethods is the set of methods that bypass tenant scoping.
var flaggedMethods = map[string]struct{}{
	"Exec":     {},
	"Query":    {},
	"QueryRow": {},
}

// allowMarker is the per-file escape hatch. Any comment in the file whose
// text contains this marker disables the analyzer for that file.
const allowMarker = "tenantlint:allow-direct-pool"

// Analyzer is the exported go/analysis Analyzer.
var Analyzer = &analysis.Analyzer{
	Name: "tenantlint",
	Doc:  "flags direct *pgxpool.Pool.Exec/Query/QueryRow in handler/domain code",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		if fileHasAllowMarker(file) {
			continue
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			method := sel.Sel.Name
			if _, hit := flaggedMethods[method]; !hit {
				return true
			}

			recvType := pass.TypesInfo.TypeOf(sel.X)
			if recvType == nil {
				return true
			}
			if types.TypeString(recvType, nil) != poolTypeName {
				return true
			}

			pass.Reportf(
				sel.Sel.Pos(),
				"direct pool.%s call — use database.TenantScoped or add //tenantlint:allow-direct-pool",
				method,
			)
			return true
		})
	}
	return nil, nil
}

// fileHasAllowMarker returns true if any comment in the file contains the
// escape-hatch marker. We scan all comment groups (not just the first) so
// the comment can sit near the package doc, an import block, or wherever
// reads naturally in a given file.
func fileHasAllowMarker(file *ast.File) bool {
	for _, cg := range file.Comments {
		for _, c := range cg.List {
			if strings.Contains(c.Text, allowMarker) {
				return true
			}
		}
	}
	return false
}
