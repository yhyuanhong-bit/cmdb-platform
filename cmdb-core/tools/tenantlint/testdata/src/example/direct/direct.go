package direct

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// HandlerDirect demonstrates the exact pattern we want to flag: a handler
// reaching straight into the pool and adding a tenant predicate by hand.
func HandlerDirect(ctx context.Context, pool *pgxpool.Pool, tenantID, id string) error {
	_, err := pool.Exec(ctx, "DELETE FROM assets WHERE tenant_id=$1 AND id=$2", tenantID, id) // want `direct pool.Exec call`
	if err != nil {
		return err
	}
	_, err = pool.Query(ctx, "SELECT id FROM assets WHERE tenant_id=$1", tenantID) // want `direct pool.Query call`
	if err != nil {
		return err
	}
	_ = pool.QueryRow(ctx, "SELECT id FROM assets WHERE tenant_id=$1", tenantID) // want `direct pool.QueryRow call`
	return nil
}
