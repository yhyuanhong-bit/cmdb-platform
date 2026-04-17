package main

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestOpenapiPathToGin(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"/assets", "/assets"},
		{"/assets/{id}", "/assets/:id"},
		{"/inventory/tasks/{id}/items/{itemId}", "/inventory/tasks/:id/items/:itemId"},
		{"/racks/{id}/network-connections/{connectionId}", "/racks/:id/network-connections/:connectionId"},
		{"/unclosed/{broken", "/unclosed/{broken"}, // defensive: no panic
	}
	for _, tt := range tests {
		if got := openapiPathToGin(tt.in); got != tt.want {
			t.Errorf("openapiPathToGin(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestDiff(t *testing.T) {
	a := []route{{"GET", "/a"}, {"POST", "/b"}, {"GET", "/c"}}
	b := []route{{"POST", "/b"}, {"GET", "/c"}, {"DELETE", "/d"}}

	specOnly := diff(a, b)
	want := []route{{"GET", "/a"}}
	if !reflect.DeepEqual(specOnly, want) {
		t.Errorf("diff(a,b) = %v, want %v", specOnly, want)
	}

	codeOnly := diff(b, a)
	want = []route{{"DELETE", "/d"}}
	if !reflect.DeepEqual(codeOnly, want) {
		t.Errorf("diff(b,a) = %v, want %v", codeOnly, want)
	}
}

func TestMerge(t *testing.T) {
	a := []route{{"GET", "/x"}, {"POST", "/y"}}
	b := []route{{"POST", "/y"}, {"DELETE", "/z"}}

	got := merge(a, b)
	sort.Slice(got, func(i, j int) bool { return got[i].String() < got[j].String() })
	want := []route{{"DELETE", "/z"}, {"GET", "/x"}, {"POST", "/y"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("merge = %v, want %v", got, want)
	}
}

// TestParseSpecRoutes_Fixture spot-checks the real spec: if these fail,
// the parser has regressed against the repo's actual OpenAPI structure.
func TestParseSpecRoutes_Fixture(t *testing.T) {
	spec := filepath.Join("..", "..", "..", "api", "openapi.yaml")
	if _, err := os.Stat(spec); err != nil {
		t.Skipf("spec not available at %s", spec)
	}
	routes, err := parseSpecRoutes(spec)
	if err != nil {
		t.Fatalf("parseSpecRoutes: %v", err)
	}
	if len(routes) < 100 {
		t.Fatalf("expected >=100 spec operations, got %d", len(routes))
	}
	// A few operations we know exist. If any disappears, investigate.
	mustHave := []route{
		{"GET", "/assets"},
		{"POST", "/assets"},
		{"GET", "/assets/:id"},
		{"PUT", "/assets/:id"},
		{"DELETE", "/assets/:id"},
		{"GET", "/inventory/tasks/:id/items"},
	}
	for _, want := range mustHave {
		found := false
		for _, got := range routes {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected spec to declare %s, not found", want)
		}
	}
}

// TestParseMainGoRoutes_Fixture spot-checks the real main.go.
func TestParseMainGoRoutes_Fixture(t *testing.T) {
	p := filepath.Join("..", "server", "main.go")
	routes, err := parseMainGoRoutes(p)
	if err != nil {
		t.Fatalf("parseMainGoRoutes: %v", err)
	}
	if len(routes) < 1 {
		t.Fatalf("expected at least the admin migration route, got %d", len(routes))
	}
	mustHave := route{"POST", "/admin/migrate-statuses"}
	for _, got := range routes {
		if got == mustHave {
			return
		}
	}
	t.Errorf("expected main.go to register %s", mustHave)
}
