// check-api-routes verifies that the routes registered in the running
// server exactly match the operations documented in api/openapi.yaml.
//
// It is intended to run in CI. Exits 0 when spec and code are aligned,
// 1 (with a human-readable diff) when they drift.
//
// Three inputs:
//  1. api/openapi.yaml                     — declared contract
//  2. cmdb-core/cmd/server/main.go         — manually registered (Track B)
//  3. cmdb-core/internal/api/generated.go  — auto-registered (Track A)
//
// Two classes of drift are reported:
//  - Spec ∖ Code: spec documents a route but no handler is registered.
//                 Always an error — consumers will hit a 404.
//  - Code ∖ Spec: a route is registered but not documented. Error unless
//                 the route appears in UndocumentedAllowlist (intentional
//                 infrastructure endpoints like /ws).
package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

// Routes registered in code but intentionally not declared in the OpenAPI
// spec. Keep this list short and obvious — any addition should be
// infrastructure or a one-off admin utility, not a user-facing API.
var UndocumentedAllowlist = map[string]bool{
	"GET /ws":                     true, // WebSocket upgrade, not REST
	"POST /admin/migrate-statuses": true, // one-off status migration utility
}

type route struct {
	Method string
	Path   string // normalized to gin form: /foo/:id
}

func (r route) String() string { return r.Method + " " + r.Path }

func main() {
	specPath := envOr("OPENAPI_SPEC", "api/openapi.yaml")
	mainPath := envOr("SERVER_MAIN", "cmdb-core/cmd/server/main.go")
	genPath := envOr("GENERATED_GO", "cmdb-core/internal/api/generated.go")

	spec, err := parseSpecRoutes(specPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to parse spec %s: %v\n", specPath, err)
		os.Exit(2)
	}
	manual, err := parseMainGoRoutes(mainPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to parse %s: %v\n", mainPath, err)
		os.Exit(2)
	}
	generated, err := parseGeneratedGoRoutes(genPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to parse %s: %v\n", genPath, err)
		os.Exit(2)
	}

	code := merge(manual, generated)

	specOnly := diff(spec, code)
	codeOnly := diff(code, spec)

	var unexpected []string
	for _, r := range codeOnly {
		if !UndocumentedAllowlist[r.String()] {
			unexpected = append(unexpected, r.String())
		}
	}

	fmt.Printf("OpenAPI route health check\n")
	fmt.Printf("  spec operations:        %d (%s)\n", len(spec), specPath)
	fmt.Printf("  code routes (manual):   %d (%s)\n", len(manual), mainPath)
	fmt.Printf("  code routes (generated):%d (%s)\n", len(generated), genPath)
	fmt.Printf("  code routes (merged):   %d\n", len(code))
	fmt.Printf("  allowlisted undoc:      %d\n", len(UndocumentedAllowlist))

	hasError := false

	if len(specOnly) > 0 {
		hasError = true
		fmt.Printf("\nERROR: %d operation(s) in spec but NOT registered in code:\n", len(specOnly))
		for _, r := range specOnly {
			fmt.Printf("  - %s\n", r)
		}
		fmt.Printf("  fix: implement the handler and register it, or remove the operation from %s\n", specPath)
	}

	if len(unexpected) > 0 {
		hasError = true
		sort.Strings(unexpected)
		fmt.Printf("\nERROR: %d route(s) registered in code but NOT documented in spec:\n", len(unexpected))
		for _, r := range unexpected {
			fmt.Printf("  - %s\n", r)
		}
		fmt.Printf("  fix: add the operation to %s, or\n", specPath)
		fmt.Printf("       add it to UndocumentedAllowlist in cmdb-core/cmd/check-api-routes/main.go\n")
		fmt.Printf("       (only for infrastructure endpoints that MUST NOT appear in the public API)\n")
	}

	if hasError {
		os.Exit(1)
	}
	fmt.Println("\nOK — spec and code are aligned.")
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// parseSpecRoutes reads an OpenAPI 3.x YAML file and extracts every
// (method, path) pair declared under `paths:`.
//
// The parser is hand-rolled to avoid adding a YAML dependency. It relies
// on the repo's convention that:
//   - the top-level `paths:` key is at column 0
//   - each path key lives at exactly 2-space indent and starts with "/"
//   - each method key lives at exactly 4-space indent and is one of
//     get/put/post/delete/patch/options/head/trace
//
// Path params in OpenAPI form ({id}) are rewritten to gin form (:id).
func parseSpecRoutes(path string) ([]route, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	methodRE := regexp.MustCompile(`^    (get|put|post|delete|patch|options|head|trace):\s*$`)
	pathRE := regexp.MustCompile(`^  (/[^:]*):\s*$`)

	var out []route
	inPaths := false
	curPath := ""

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		// Track entry into the top-level `paths:` block.
		if !inPaths {
			if strings.HasPrefix(line, "paths:") {
				inPaths = true
			}
			continue
		}
		// Exit the paths block on any new top-level key.
		if len(line) > 0 && line[0] != ' ' && line[0] != '#' {
			break
		}
		if m := pathRE.FindStringSubmatch(line); m != nil {
			curPath = openapiPathToGin(m[1])
			continue
		}
		if curPath == "" {
			continue
		}
		if m := methodRE.FindStringSubmatch(line); m != nil {
			out = append(out, route{
				Method: strings.ToUpper(m[1]),
				Path:   curPath,
			})
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// openapiPathToGin converts "/foo/{id}/bar/{itemId}" to "/foo/:id/bar/:itemId".
func openapiPathToGin(p string) string {
	var b strings.Builder
	b.Grow(len(p))
	i := 0
	for i < len(p) {
		if p[i] == '{' {
			end := strings.IndexByte(p[i:], '}')
			if end < 0 {
				b.WriteByte(p[i])
				i++
				continue
			}
			name := p[i+1 : i+end]
			b.WriteByte(':')
			b.WriteString(name)
			i += end + 1
			continue
		}
		b.WriteByte(p[i])
		i++
	}
	return b.String()
}

// parseMainGoRoutes extracts v1.GET("/path", ...) / v1.POST(...) / etc.
// from cmd/server/main.go. These are the Track-B manual registrations.
var manualRE = regexp.MustCompile(`^\s*v1\.(GET|POST|PUT|DELETE|PATCH)\("([^"]+)"`)

func parseMainGoRoutes(path string) ([]route, error) {
	return parseGoFileRoutes(path, manualRE, nil)
}

// parseGeneratedGoRoutes extracts router.GET(options.BaseURL+"/path", ...)
// from internal/api/generated.go. These are Track-A auto-registrations.
var generatedRE = regexp.MustCompile(`^\s*router\.(GET|POST|PUT|DELETE|PATCH)\(options\.BaseURL\+"([^"]+)"`)

func parseGeneratedGoRoutes(path string) ([]route, error) {
	// generated.go uses gin-style params ({id}) in a handful of places —
	// mostly they're already :id form. Convert defensively.
	return parseGoFileRoutes(path, generatedRE, openapiPathToGin)
}

func parseGoFileRoutes(path string, re *regexp.Regexp, normalize func(string) string) ([]route, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var out []route
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		m := re.FindStringSubmatch(sc.Text())
		if m == nil {
			continue
		}
		p := m[2]
		if normalize != nil {
			p = normalize(p)
		}
		out = append(out, route{Method: m[1], Path: p})
	}
	return out, sc.Err()
}

// merge returns the set union (by method+path) of two route slices.
func merge(a, b []route) []route {
	seen := make(map[string]route, len(a)+len(b))
	for _, r := range a {
		seen[r.String()] = r
	}
	for _, r := range b {
		seen[r.String()] = r
	}
	out := make([]route, 0, len(seen))
	for _, r := range seen {
		out = append(out, r)
	}
	return out
}

// diff returns elements of a that are not in b (by method+path).
func diff(a, b []route) []route {
	have := make(map[string]struct{}, len(b))
	for _, r := range b {
		have[r.String()] = struct{}{}
	}
	var out []route
	for _, r := range a {
		if _, ok := have[r.String()]; !ok {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}
