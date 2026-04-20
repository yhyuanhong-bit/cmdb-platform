// Package errcheck provides a go/analysis Analyzer that flags the
// three most common "silently swallowed error" patterns in this
// codebase:
//
//  1. *pgxpool.Pool.Exec/Query/QueryRow whose returned error is
//     blank-assigned or simply dropped on the floor as an expression
//     statement — e.g. `_, _ = pool.Exec(ctx, …)` or `pool.Exec(…)`.
//     Every Exec/Query must either propagate or Warn-log the error;
//     silent discard is the concrete failure mode that let a stalled
//     retention sweep go unnoticed for weeks in production.
//
//  2. json.Unmarshal where the returned error is assigned to `_` or
//     to a variable that is never subsequently referenced. A dropped
//     Unmarshal means we parse garbage into a zero value and proceed
//     as if nothing happened, which the audit traced directly to the
//     adapter_zabbix.go empty-response bug.
//
//  3. rows.Scan where the returned error is not checked within three
//     statements of the call. Scan failures typically mean a column
//     shape mismatch; dropping that error hides schema drift.
//
// Escape hatch: a `//errcheck:allow` comment on the line immediately
// preceding a flagged call silences just that call. Prefer a real
// fix — Warn-log, metric, and continue the loop — over the allow
// directive. The directive exists for genuinely cross-cutting code
// paths where the discard is part of the design (rollback in defer,
// best-effort cache warmup, etc.).
package errcheck

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// Type strings we compare against via types.TypeString. The pointer
// form is required because pgxpool methods are defined on *Pool.
const (
	pgxpoolPoolType = "*github.com/jackc/pgx/v5/pgxpool.Pool"
	pgxRowsType     = "github.com/jackc/pgx/v5.Rows"
)

// poolMethods are the pool methods whose error return must not be
// silently dropped. QueryRow returns a Row (no error), so we watch
// Exec and Query only.
var poolMethods = map[string]struct{}{
	"Exec":  {},
	"Query": {},
}

// allowMarker is the per-line escape hatch. A `//errcheck:allow`
// comment on the line immediately preceding the flagged call
// silences just that diagnostic.
const allowMarker = "errcheck:allow"

// Analyzer is the exported go/analysis Analyzer.
var Analyzer = &analysis.Analyzer{
	Name: "errcheck",
	Doc:  "flags silently-swallowed errors from pool.Exec/Query, json.Unmarshal, and rows.Scan",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	// Collect the set of line numbers (per file) carrying a leading
	// //errcheck:allow directive so we can short-circuit diagnostics
	// for those lines without re-scanning the comment list on every
	// report. Keyed by (filename, line-of-the-guarded-call).
	allowLines := collectAllowLines(pass)

	// testdata/** and *_test.go are excluded by design: the linter is
	// aimed at production code paths. Test files legitimately discard
	// errors to keep assertion code readable, and testdata/** is
	// itself the fixture tree for other analyzers.
	for _, file := range pass.Files {
		filename := pass.Fset.Position(file.Pos()).Filename
		if isExcludedFile(filename) {
			continue
		}
		// Two-pass sweep per file:
		//   1) ast.Inspect for the call-expression patterns (pool
		//      Exec/Query bare or blank-assigned, Unmarshal
		//      likewise, Scan in expr-stmt or blank-assign form).
		//   2) a function-level walk for the tricky cases where
		//      Unmarshal/Scan's err IS assigned to a name but
		//      never subsequently referenced.
		scanFile(pass, file, allowLines)
	}
	return nil, nil
}

// isExcludedFile returns true for paths we never want to scan.
// Test files have relaxed error-handling conventions (table-driven
// setup code often drops errors intentionally), so we skip them.
// testdata trees are excluded by the Go build system itself — go
// vet never hands them to the analyzer in production runs — so the
// analyzer doesn't need a redundant testdata exclusion. Omitting
// it also keeps analysistest fixtures reachable, which is required
// for the analyzer's own test suite to execute against them.
func isExcludedFile(filename string) bool {
	return strings.HasSuffix(filename, "_test.go")
}

// allowLineKey uniquely identifies a line in a file. Using the
// filename rather than the *token.File pointer keeps the map stable
// across analyzer passes that share state.
type allowLineKey struct {
	filename string
	line     int
}

// collectAllowLines walks every comment group in every file and
// records the set of (filename, line) pairs that follow a leading
// //errcheck:allow comment. A directive on line N applies to the
// call on line N+1.
func collectAllowLines(pass *analysis.Pass) map[allowLineKey]struct{} {
	out := make(map[allowLineKey]struct{})
	for _, file := range pass.Files {
		for _, cg := range file.Comments {
			for _, c := range cg.List {
				if !strings.Contains(c.Text, allowMarker) {
					continue
				}
				pos := pass.Fset.Position(c.Slash)
				out[allowLineKey{filename: pos.Filename, line: pos.Line + 1}] = struct{}{}
			}
		}
	}
	return out
}

// scanFile runs the per-file AST traversal. We walk at the
// statement level so we can distinguish ExprStmt (bare call) from
// AssignStmt (captured result), which the three checks need to tell
// apart.
func scanFile(pass *analysis.Pass, file *ast.File, allowLines map[allowLineKey]struct{}) {
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl:
			if node.Body != nil {
				scanBlock(pass, node.Body, allowLines)
			}
			return false // body already scanned
		case *ast.FuncLit:
			if node.Body != nil {
				scanBlock(pass, node.Body, allowLines)
			}
			return false
		}
		return true
	})
}

// scanBlock handles a single block of statements. Order matters for
// the Unmarshal/Scan follow-up checks: we must look N statements
// ahead in the same block to decide whether the assigned err was
// checked.
func scanBlock(pass *analysis.Pass, block *ast.BlockStmt, allowLines map[allowLineKey]struct{}) {
	for i, stmt := range block.List {
		checkStmt(pass, block.List, i, stmt, allowLines)
		// Recurse into nested blocks so for/if/switch bodies are
		// also scanned.
		ast.Inspect(stmt, func(n ast.Node) bool {
			inner, ok := n.(*ast.BlockStmt)
			if !ok || inner == block {
				return true
			}
			scanBlock(pass, inner, allowLines)
			return false
		})
	}
}

// checkStmt routes a single statement to the appropriate check.
// The full slice + index lets scanning-ahead checks (Unmarshal
// assignment lookahead, Scan lookahead) peek at subsequent
// statements in the same block.
func checkStmt(
	pass *analysis.Pass,
	stmts []ast.Stmt,
	idx int,
	stmt ast.Stmt,
	allowLines map[allowLineKey]struct{},
) {
	switch s := stmt.(type) {
	case *ast.ExprStmt:
		checkBareCall(pass, s, allowLines)
	case *ast.AssignStmt:
		checkAssignCall(pass, stmts, idx, s, allowLines)
	}
}

// checkBareCall handles `pool.Exec(...)` / `json.Unmarshal(...)` /
// `rows.Scan(...)` used as standalone expression statements. By
// construction the returned error is thrown away.
func checkBareCall(pass *analysis.Pass, stmt *ast.ExprStmt, allowLines map[allowLineKey]struct{}) {
	call, ok := stmt.X.(*ast.CallExpr)
	if !ok {
		return
	}
	if allowed(pass, call.Pos(), allowLines) {
		return
	}
	if kind, ok := classifyCall(pass, call); ok {
		switch kind {
		case callKindPoolExec, callKindPoolQuery:
			pass.Reportf(call.Pos(),
				"pool.%s error is discarded — propagate or Warn-log (see docs/ERROR_HANDLING.md)",
				selectorMethodName(call),
			)
		case callKindJSONUnmarshal:
			pass.Reportf(call.Pos(),
				"json.Unmarshal error is discarded — propagate or Warn-log (see docs/ERROR_HANDLING.md)",
			)
		case callKindRowsScan:
			pass.Reportf(call.Pos(),
				"rows.Scan error is not checked — propagate or Warn-log within 3 statements",
			)
		}
	}
}

// checkAssignCall handles:
//
//	_, _ = pool.Exec(ctx, …)       // blank-assigned error
//	_ = pool.Exec(ctx, …)           // same
//	err := json.Unmarshal(…)        // assigned but possibly unchecked
//	_ = rows.Scan(&x)               // blank-assigned error
//	err := rows.Scan(&x)            // assigned but possibly unchecked
func checkAssignCall(
	pass *analysis.Pass,
	stmts []ast.Stmt,
	idx int,
	assign *ast.AssignStmt,
	allowLines map[allowLineKey]struct{},
) {
	// Only consider single-call RHS. Multi-value RHS like
	// `a, b := f(), g()` is out of scope.
	if len(assign.Rhs) != 1 {
		return
	}
	call, ok := assign.Rhs[0].(*ast.CallExpr)
	if !ok {
		return
	}
	if allowed(pass, call.Pos(), allowLines) {
		return
	}

	kind, ok := classifyCall(pass, call)
	if !ok {
		return
	}

	switch kind {
	case callKindPoolExec, callKindPoolQuery:
		// Pool Exec/Query return (CommandTag|Rows, error). The
		// error is always the last LHS slot. If that slot is `_`
		// the error has been silenced.
		if len(assign.Lhs) >= 1 && isBlankIdent(assign.Lhs[len(assign.Lhs)-1]) {
			pass.Reportf(call.Pos(),
				"pool.%s error is discarded — propagate or Warn-log (see docs/ERROR_HANDLING.md)",
				selectorMethodName(call),
			)
		}
	case callKindJSONUnmarshal:
		// Unmarshal returns a single error. Either the LHS is `_`
		// (blank-assigned, immediately suppressed) or it's a
		// named variable that must be referenced later.
		if len(assign.Lhs) != 1 {
			return
		}
		if isBlankIdent(assign.Lhs[0]) {
			pass.Reportf(call.Pos(),
				"json.Unmarshal error is discarded — propagate or Warn-log (see docs/ERROR_HANDLING.md)",
			)
			return
		}
		ident, ok := assign.Lhs[0].(*ast.Ident)
		if !ok {
			return
		}
		if !identCheckedInFunc(pass, ident, assign) {
			pass.Reportf(call.Pos(),
				"json.Unmarshal error is assigned but never checked — use `if err != nil` within 3 statements",
			)
		}
	case callKindRowsScan:
		// Scan returns a single error. Blank-assigned is always
		// suppressed. A named variable must be referenced inside
		// the next three statements (same block) for the call to
		// count as checked.
		if len(assign.Lhs) != 1 {
			return
		}
		if isBlankIdent(assign.Lhs[0]) {
			pass.Reportf(call.Pos(),
				"rows.Scan error is not checked — use `if err := rows.Scan(...); err != nil`",
			)
			return
		}
		ident, ok := assign.Lhs[0].(*ast.Ident)
		if !ok {
			return
		}
		if !identCheckedWithin(pass, ident, stmts, idx, 3) {
			pass.Reportf(call.Pos(),
				"rows.Scan error is not checked within 3 statements — use `if err != nil`",
			)
		}
	}
}

// callKind enumerates the three patterns we watch for.
type callKind int

const (
	callKindUnknown callKind = iota
	callKindPoolExec
	callKindPoolQuery
	callKindJSONUnmarshal
	callKindRowsScan
)

// classifyCall inspects the call expression and returns the kind of
// error-returning call it is (or callKindUnknown for everything
// else). We rely on type info to avoid matching look-alikes.
func classifyCall(pass *analysis.Pass, call *ast.CallExpr) (callKind, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return callKindUnknown, false
	}
	method := sel.Sel.Name

	// json.Unmarshal: match by the fully qualified function
	// object, which is immune to rename-imports and package aliases.
	if method == "Unmarshal" {
		if isJSONUnmarshal(pass, sel) {
			return callKindJSONUnmarshal, true
		}
		return callKindUnknown, false
	}

	// Receiver type tells us whether it's a pool or rows call.
	recvType := pass.TypesInfo.TypeOf(sel.X)
	if recvType == nil {
		return callKindUnknown, false
	}
	typeStr := types.TypeString(recvType, nil)

	if typeStr == pgxpoolPoolType {
		if _, hit := poolMethods[method]; hit {
			if method == "Exec" {
				return callKindPoolExec, true
			}
			return callKindPoolQuery, true
		}
	}
	if typeStr == pgxRowsType && method == "Scan" {
		return callKindRowsScan, true
	}
	return callKindUnknown, false
}

// isJSONUnmarshal returns true when the selector references
// encoding/json.Unmarshal via type/object lookup. Falls back to a
// textual comparison against the receiver expression for fixtures
// that stub out the stdlib import.
func isJSONUnmarshal(pass *analysis.Pass, sel *ast.SelectorExpr) bool {
	obj := pass.TypesInfo.ObjectOf(sel.Sel)
	if obj != nil {
		if fn, ok := obj.(*types.Func); ok && fn.Pkg() != nil {
			if fn.Pkg().Path() == "encoding/json" && fn.Name() == "Unmarshal" {
				return true
			}
		}
	}
	// Fallback: receiver is an *ast.Ident named "json" referring
	// to a package (not a variable). Useful when type info is
	// incomplete in early analysis phases.
	recv, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	if recv.Name != "json" {
		return false
	}
	if pass.TypesInfo.Uses[recv] == nil {
		return false
	}
	pkgName, ok := pass.TypesInfo.Uses[recv].(*types.PkgName)
	if !ok {
		return false
	}
	return pkgName.Imported().Path() == "encoding/json"
}

// selectorMethodName extracts the method name from a call expression
// whose fun is a selector — used for building the diagnostic
// message.
func selectorMethodName(call *ast.CallExpr) string {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "?"
	}
	return sel.Sel.Name
}

// isBlankIdent returns true if the expression is the bare
// identifier `_`. Anything else (named variable, complex expression)
// is not a blank assignment.
func isBlankIdent(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == "_"
}

// identCheckedWithin returns true if the variable bound by ident is
// referenced in any of the next `lookahead` statements in the same
// block. Conservative: ANY reference counts as "checked", even a
// simple `_ = err`, because eliminating those mistakes is outside
// this analyzer's scope.
func identCheckedWithin(
	pass *analysis.Pass,
	ident *ast.Ident,
	stmts []ast.Stmt,
	startIdx int,
	lookahead int,
) bool {
	obj := pass.TypesInfo.ObjectOf(ident)
	if obj == nil {
		return true // can't prove it's unchecked; stay quiet
	}
	end := startIdx + 1 + lookahead
	if end > len(stmts) {
		end = len(stmts)
	}
	for i := startIdx + 1; i < end; i++ {
		found := false
		ast.Inspect(stmts[i], func(n ast.Node) bool {
			if found {
				return false
			}
			id, ok := n.(*ast.Ident)
			if !ok {
				return true
			}
			if pass.TypesInfo.ObjectOf(id) == obj {
				// `_ = err` is intentionally not a real check
				// but we still treat it as one here — the
				// analyzer's remit is "is the error visible
				// to subsequent logic?", not "is it logged
				// correctly?".
				found = true
				return false
			}
			return true
		})
		if found {
			return true
		}
	}
	return false
}

// identCheckedInFunc is the looser version used for json.Unmarshal:
// the assigned err must be referenced SOMEWHERE in the enclosing
// function body, not just the next 3 statements. Unmarshal errors
// often get checked after intermediate setup so the tight window is
// too strict.
func identCheckedInFunc(pass *analysis.Pass, ident *ast.Ident, assign *ast.AssignStmt) bool {
	obj := pass.TypesInfo.ObjectOf(ident)
	if obj == nil {
		return true
	}
	// Walk the file looking for the enclosing FuncDecl/FuncLit and
	// searching its body for references to obj that are NOT the
	// assignment site itself.
	file := fileOf(pass, assign.Pos())
	if file == nil {
		return true
	}
	var fnBody *ast.BlockStmt
	ast.Inspect(file, func(n ast.Node) bool {
		if fnBody != nil {
			return false
		}
		switch node := n.(type) {
		case *ast.FuncDecl:
			if node.Body != nil && containsPos(node.Body, assign.Pos()) {
				fnBody = node.Body
			}
		case *ast.FuncLit:
			if node.Body != nil && containsPos(node.Body, assign.Pos()) {
				fnBody = node.Body
			}
		}
		return true
	})
	if fnBody == nil {
		return true
	}
	referenced := false
	ast.Inspect(fnBody, func(n ast.Node) bool {
		if referenced {
			return false
		}
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		if pass.TypesInfo.ObjectOf(id) != obj {
			return true
		}
		// Skip the assignment's own LHS ident.
		if id.Pos() == ident.Pos() {
			return true
		}
		// Skip the blank `_ = err` dead reference — its whole
		// job is to silence unused-variable errors from the Go
		// compiler, not to actually check the error.
		if isBlankCheck(n, fnBody, id) {
			return true
		}
		referenced = true
		return false
	})
	return referenced
}

// isBlankCheck reports whether the given ident is the RHS of a
// `_ = err` assignment — a dead reference that counts as unused for
// our purposes.
func isBlankCheck(n ast.Node, fnBody *ast.BlockStmt, id *ast.Ident) bool {
	var result bool
	ast.Inspect(fnBody, func(m ast.Node) bool {
		if result {
			return false
		}
		as, ok := m.(*ast.AssignStmt)
		if !ok {
			return true
		}
		if len(as.Lhs) != 1 || !isBlankIdent(as.Lhs[0]) {
			return true
		}
		if len(as.Rhs) != 1 {
			return true
		}
		rhs, ok := as.Rhs[0].(*ast.Ident)
		if !ok {
			return true
		}
		if rhs.Pos() == id.Pos() {
			result = true
			return false
		}
		return true
	})
	return result
}

// containsPos returns true if pos lies within the block's range.
func containsPos(block *ast.BlockStmt, pos token.Pos) bool {
	return block.Pos() <= pos && pos <= block.End()
}

// fileOf returns the ast.File containing pos, or nil if none.
func fileOf(pass *analysis.Pass, pos token.Pos) *ast.File {
	for _, f := range pass.Files {
		if f.Pos() <= pos && pos <= f.End() {
			return f
		}
	}
	return nil
}

// allowed reports whether a call at the given position is silenced
// by a leading //errcheck:allow directive.
func allowed(pass *analysis.Pass, pos token.Pos, allowLines map[allowLineKey]struct{}) bool {
	p := pass.Fset.Position(pos)
	_, hit := allowLines[allowLineKey{filename: p.Filename, line: p.Line}]
	return hit
}
