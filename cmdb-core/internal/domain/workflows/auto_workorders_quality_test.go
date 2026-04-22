package workflows

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

// routingLabelFor is the D9-P1 routing hint baked into auto-created
// low-quality work orders. Three branches matter:
//   - a non-empty owner_team is surfaced so operators can filter by team
//   - an empty string is treated as unassigned (NULL was stored as '')
//   - an invalid / NULL pgtype.Text is also unassigned
//
// A silent fallback to the empty string would make unassigned WOs
// invisible to a "team-less queue" view, so the explicit sentinel matters.
func TestRoutingLabelFor(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input pgtype.Text
		want  string
	}{
		{"valid team", pgtype.Text{String: "infra-sre", Valid: true}, "team infra-sre"},
		{"valid but empty", pgtype.Text{String: "", Valid: true}, "unassigned team"},
		{"null column", pgtype.Text{Valid: false}, "unassigned team"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := routingLabelFor(tc.input); got != tc.want {
				t.Errorf("routingLabelFor(%+v) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
