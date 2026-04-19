//tenantlint:allow-direct-pool

// Explicit cross-tenant utility file. The escape-hatch directive above
// suppresses all diagnostics in this package; analysistest confirms this
// by the absence of any expectation annotations below, even though the
// code pattern is exactly what the analyzer flags in other packages.
package allowed

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

func CrossTenantScan(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, "VACUUM ANALYZE")
	if err != nil {
		return err
	}
	_, err = pool.Query(ctx, "SELECT id FROM tenants")
	return err
}
