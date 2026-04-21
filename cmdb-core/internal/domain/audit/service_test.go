package audit

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// The unit tests here cover Service.Record's client-side validation only —
// they don't need Postgres because the rules are enforced before the query
// is issued. The DB CHECK constraint (migration 000051) is a backstop we
// exercise in integration tests.

func TestRecord_UserRequiresOperatorID(t *testing.T) {
	t.Parallel()
	s := &Service{} // queries nil — we never reach it
	err := s.Record(
		context.Background(),
		uuid.New(),
		"asset.created", "asset", "asset", uuid.New(),
		OperatorTypeUser, nil,
		nil, "api",
	)
	if err == nil {
		t.Fatal("expected validation error for OperatorTypeUser + nil operatorID")
	}
}

func TestRecord_NonUserRejectsOperatorID(t *testing.T) {
	t.Parallel()
	s := &Service{}
	opID := uuid.New()
	for _, opType := range []OperatorType{
		OperatorTypeSystem,
		OperatorTypeIntegration,
		OperatorTypeSync,
		OperatorTypeAnonymous,
	} {
		err := s.Record(
			context.Background(),
			uuid.New(),
			"workflow.tick", "workflow", "asset", uuid.New(),
			opType, &opID,
			nil, "workflow",
		)
		if err == nil {
			t.Errorf("operator_type=%q with non-nil operatorID: expected validation error, got nil", opType)
		}
	}
}
