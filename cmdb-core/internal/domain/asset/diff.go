package asset

import (
	"bytes"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// DiffSnapshots computes the ordered list of per-field changes between
// two point-in-time asset snapshots. Pure function — no DB, no context —
// so the diff rules are easy to unit-test. Fields are emitted in a
// stable, human-readable order (same order the API response and the UI
// render); a test pins the shape so a column reshuffle does not silently
// change the diff output.
//
// nil-vs-empty handling: pgtype.Text with Valid=false and tags=nil are
// reported as nil; "" and []string{} are reported as their zero values.
// The behaviour matches what a JSON caller sees in the point-in-time
// response, so a diff never disagrees with "look at both snapshots side
// by side".
func DiffSnapshots(from, to dbgen.AssetSnapshot) []FieldChange {
	var changes []FieldChange

	cmpString := func(field string, a, b string) {
		if a != b {
			changes = append(changes, FieldChange{Field: field, From: a, To: b})
		}
	}
	cmpText := func(field string, a, b pgtype.Text) {
		if textEq(a, b) {
			return
		}
		changes = append(changes, FieldChange{Field: field, From: textVal(a), To: textVal(b)})
	}
	cmpUUID := func(field string, a, b pgtype.UUID) {
		if uuidEq(a, b) {
			return
		}
		changes = append(changes, FieldChange{Field: field, From: uuidVal(a), To: uuidVal(b)})
	}
	cmpTags := func(field string, a, b []string) {
		if tagsEq(a, b) {
			return
		}
		changes = append(changes, FieldChange{Field: field, From: tagsVal(a), To: tagsVal(b)})
	}
	cmpJSON := func(field string, a, b []byte) {
		// JSONB columns are persisted in canonical form by Postgres,
		// so a byte-level compare is sufficient for equality and
		// cheaper than re-parsing into map[string]any on every diff.
		if bytes.Equal(a, b) {
			return
		}
		changes = append(changes, FieldChange{Field: field, From: jsonRaw(a), To: jsonRaw(b)})
	}

	cmpString("name", from.Name, to.Name)
	cmpString("asset_tag", from.AssetTag, to.AssetTag)
	cmpString("status", from.Status, to.Status)
	cmpString("bia_level", from.BiaLevel, to.BiaLevel)
	cmpUUID("location_id", from.LocationID, to.LocationID)
	cmpUUID("rack_id", from.RackID, to.RackID)
	cmpText("vendor", from.Vendor, to.Vendor)
	cmpText("model", from.Model, to.Model)
	cmpText("serial_number", from.SerialNumber, to.SerialNumber)
	cmpJSON("attributes", from.Attributes, to.Attributes)
	cmpTags("tags", []string(from.Tags), []string(to.Tags))
	cmpText("owner_team", from.OwnerTeam, to.OwnerTeam)

	return changes
}

func textEq(a, b pgtype.Text) bool {
	if a.Valid != b.Valid {
		return false
	}
	if !a.Valid {
		return true
	}
	return a.String == b.String
}

func textVal(t pgtype.Text) any {
	if !t.Valid {
		return nil
	}
	return t.String
}

func uuidEq(a, b pgtype.UUID) bool {
	if a.Valid != b.Valid {
		return false
	}
	if !a.Valid {
		return true
	}
	return a.Bytes == b.Bytes
}

func uuidVal(u pgtype.UUID) any {
	if !u.Valid {
		return nil
	}
	id := uuid.UUID(u.Bytes)
	return id
}

func tagsEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func tagsVal(t []string) any {
	if t == nil {
		return nil
	}
	return t
}

// jsonRaw keeps []byte out of the API layer; the handler re-decodes so
// the response contains a JSON object / array rather than a base64
// string. An empty / nil attributes column is normalised to nil so the
// diff reports "unset" consistently.
func jsonRaw(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}
