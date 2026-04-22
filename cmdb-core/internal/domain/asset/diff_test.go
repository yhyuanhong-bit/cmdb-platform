package asset

import (
	"testing"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestDiffSnapshots_Identity(t *testing.T) {
	snap := dbgen.AssetSnapshot{
		Name:     "rack-A",
		AssetTag: "ASSET-0001",
		Status:   "active",
		BiaLevel: "L2",
	}
	if changes := DiffSnapshots(snap, snap); len(changes) != 0 {
		t.Fatalf("identity diff should be empty, got %d changes: %+v", len(changes), changes)
	}
}

func TestDiffSnapshots_SingleField(t *testing.T) {
	from := dbgen.AssetSnapshot{Name: "old", AssetTag: "A-1", Status: "active", BiaLevel: "L2"}
	to := dbgen.AssetSnapshot{Name: "new", AssetTag: "A-1", Status: "active", BiaLevel: "L2"}

	changes := DiffSnapshots(from, to)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d: %+v", len(changes), changes)
	}
	got := changes[0]
	if got.Field != "name" || got.From != "old" || got.To != "new" {
		t.Fatalf("unexpected change: %+v", got)
	}
}

func TestDiffSnapshots_FieldOrder(t *testing.T) {
	// Pin the emitted order so a column reshuffle in the DTO does not
	// silently reorder the diff response.
	from := dbgen.AssetSnapshot{}
	to := dbgen.AssetSnapshot{
		Name:      "n",
		AssetTag:  "t",
		Status:    "s",
		BiaLevel:  "b",
		Vendor:    pgtype.Text{String: "v", Valid: true},
		Model:     pgtype.Text{String: "m", Valid: true},
		OwnerTeam: pgtype.Text{String: "ops", Valid: true},
	}

	changes := DiffSnapshots(from, to)
	wantOrder := []string{"name", "asset_tag", "status", "bia_level", "vendor", "model", "owner_team"}
	if len(changes) != len(wantOrder) {
		t.Fatalf("expected %d changes, got %d: %+v", len(wantOrder), len(changes), changes)
	}
	for i, want := range wantOrder {
		if changes[i].Field != want {
			t.Fatalf("change[%d]: want field %q, got %q", i, want, changes[i].Field)
		}
	}
}

func TestDiffSnapshots_NilVsEmpty(t *testing.T) {
	// A pgtype.Text with Valid=false reports as nil, an empty string "" with
	// Valid=true reports as "". These are different and must surface as a
	// change — callers rely on the distinction to tell "unset" from "blanked".
	from := dbgen.AssetSnapshot{Vendor: pgtype.Text{Valid: false}}
	to := dbgen.AssetSnapshot{Vendor: pgtype.Text{String: "", Valid: true}}

	changes := DiffSnapshots(from, to)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].Field != "vendor" {
		t.Fatalf("want field vendor, got %q", changes[0].Field)
	}
	if changes[0].From != nil {
		t.Fatalf("from should be nil, got %#v", changes[0].From)
	}
	if changes[0].To != "" {
		t.Fatalf("to should be \"\", got %#v", changes[0].To)
	}
}

func TestDiffSnapshots_TagsNilVsEmpty(t *testing.T) {
	// Tags: nil and empty slice are both "no tags" from the user's
	// perspective — treat them as equal to avoid spurious diffs on newly
	// created rows that default to an empty array.
	from := dbgen.AssetSnapshot{Tags: nil}
	to := dbgen.AssetSnapshot{Tags: []string{}}

	if changes := DiffSnapshots(from, to); len(changes) != 0 {
		t.Fatalf("nil vs empty tags should be equal, got %d changes: %+v", len(changes), changes)
	}
}

func TestDiffSnapshots_TagsReorder(t *testing.T) {
	// Tag order is semantic — re-sorting counts as a change.
	from := dbgen.AssetSnapshot{Tags: []string{"a", "b"}}
	to := dbgen.AssetSnapshot{Tags: []string{"b", "a"}}

	changes := DiffSnapshots(from, to)
	if len(changes) != 1 || changes[0].Field != "tags" {
		t.Fatalf("expected one tags change, got %+v", changes)
	}
}

func TestDiffSnapshots_UUID(t *testing.T) {
	loc1 := uuid.New()
	loc2 := uuid.New()

	from := dbgen.AssetSnapshot{LocationID: pgtype.UUID{Bytes: loc1, Valid: true}}
	to := dbgen.AssetSnapshot{LocationID: pgtype.UUID{Bytes: loc2, Valid: true}}

	changes := DiffSnapshots(from, to)
	if len(changes) != 1 || changes[0].Field != "location_id" {
		t.Fatalf("expected one location_id change, got %+v", changes)
	}
	if _, ok := changes[0].From.(uuid.UUID); !ok {
		t.Fatalf("from should be uuid.UUID, got %T", changes[0].From)
	}
}

func TestDiffSnapshots_UUIDValidToUnset(t *testing.T) {
	loc := uuid.New()
	from := dbgen.AssetSnapshot{LocationID: pgtype.UUID{Bytes: loc, Valid: true}}
	to := dbgen.AssetSnapshot{LocationID: pgtype.UUID{Valid: false}}

	changes := DiffSnapshots(from, to)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].To != nil {
		t.Fatalf("unset UUID should be nil, got %#v", changes[0].To)
	}
}

func TestDiffSnapshots_AttributesJSON(t *testing.T) {
	// JSONB columns: canonical byte comparison is authoritative —
	// identical bytes are equal even though they round-trip through
	// []byte rather than map[string]any.
	a := []byte(`{"cpu":"intel"}`)
	b := []byte(`{"cpu":"amd"}`)

	identical := DiffSnapshots(
		dbgen.AssetSnapshot{Attributes: a},
		dbgen.AssetSnapshot{Attributes: a},
	)
	if len(identical) != 0 {
		t.Fatalf("identical JSONB should be equal, got %d changes", len(identical))
	}

	different := DiffSnapshots(
		dbgen.AssetSnapshot{Attributes: a},
		dbgen.AssetSnapshot{Attributes: b},
	)
	if len(different) != 1 || different[0].Field != "attributes" {
		t.Fatalf("expected one attributes change, got %+v", different)
	}
}

func TestDiffSnapshots_AttributesEmptyNormalized(t *testing.T) {
	// An empty / nil attributes column normalises to nil so the diff
	// reports "unset" consistently regardless of whether the underlying
	// column held [] or was truly NULL.
	from := dbgen.AssetSnapshot{Attributes: nil}
	to := dbgen.AssetSnapshot{Attributes: []byte{}}

	if changes := DiffSnapshots(from, to); len(changes) != 0 {
		t.Fatalf("nil vs empty attributes should be equal, got %+v", changes)
	}
}
