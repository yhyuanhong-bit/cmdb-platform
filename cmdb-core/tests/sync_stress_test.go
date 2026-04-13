//go:build integration

package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	shortModeRecords = 20
	fullModeRecords  = 140
)

func getTestDBURL() string {
	if url := os.Getenv("TEST_DATABASE_URL"); url != "" {
		return url
	}
	return "postgres://cmdb:changeme@localhost:5432/cmdb?sslmode=disable"
}

func getTestAPIURL() string {
	if url := os.Getenv("TEST_API_URL"); url != "" {
		return url
	}
	return "http://localhost:8080"
}

func TestSyncStress(t *testing.T) {
	ctx := context.Background()
	token := os.Getenv("TEST_API_TOKEN")
	if token == "" {
		t.Skip("Set TEST_API_TOKEN env var for stress test")
	}

	recordsPerType := fullModeRecords
	if testing.Short() {
		recordsPerType = shortModeRecords
	}

	pool, err := pgxpool.New(ctx, getTestDBURL())
	if err != nil {
		t.Fatalf("connect to DB: %v", err)
	}
	defer pool.Close()

	tenantID := "a0000000-0000-0000-0000-000000000001"

	type entityDef struct {
		table   string
		columns string
		values  func(i int) []interface{}
	}

	entities := []entityDef{
		{
			"assets",
			"id, tenant_id, asset_tag, name, type, status, sync_version",
			func(i int) []interface{} {
				return []interface{}{
					fmt.Sprintf("aaaaaaaa-0000-0000-0000-%012d", i),
					tenantID,
					fmt.Sprintf("STRESS-%d", i),
					fmt.Sprintf("Stress Asset %d", i),
					"server", "operational", i,
				}
			},
		},
		{
			"work_orders",
			"id, tenant_id, code, title, type, status, priority, execution_status, governance_status, sync_version",
			func(i int) []interface{} {
				return []interface{}{
					fmt.Sprintf("bbbbbbbb-0000-0000-0000-%012d", i),
					tenantID,
					fmt.Sprintf("WO-STRESS-%d", i),
					fmt.Sprintf("Stress WO %d", i),
					"corrective", "submitted", "medium", "pending", "submitted", i,
				}
			},
		},
	}

	// Insert test data
	t.Log("Inserting test data...")
	insertStart := time.Now()
	for _, et := range entities {
		for i := 1; i <= recordsPerType; i++ {
			vals := et.values(i)
			ph := ""
			for j := range vals {
				if j > 0 {
					ph += ", "
				}
				ph += fmt.Sprintf("$%d", j+1)
			}
			q := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT (id) DO UPDATE SET sync_version = EXCLUDED.sync_version", et.table, et.columns, ph)
			if _, err := pool.Exec(ctx, q, vals...); err != nil {
				t.Fatalf("insert %s[%d]: %v", et.table, i, err)
			}
		}
	}
	t.Logf("Inserted %d records in %v", len(entities)*recordsPerType, time.Since(insertStart))

	// Pull changes via API
	apiURL := getTestAPIURL()
	t.Log("Pulling changes via API...")
	pullStart := time.Now()

	for _, et := range entities {
		url := fmt.Sprintf("%s/api/v1/sync/changes?entity_type=%s&since_version=0&limit=1000", apiURL, et.table)
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("pull %s: %v", et.table, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != 200 {
			t.Fatalf("pull %s: status %d, body: %s", et.table, resp.StatusCode, string(body))
		}

		var result struct {
			Data struct {
				Changes []json.RawMessage `json:"changes"`
				HasMore bool              `json:"has_more"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("unmarshal %s: %v", et.table, err)
		}
		t.Logf("  %s: got %d records (has_more=%v)", et.table, len(result.Data.Changes), result.Data.HasMore)
	}

	pullDuration := time.Since(pullStart)
	t.Logf("Pull completed in %v", pullDuration)

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	t.Logf("Memory: Alloc=%dMB, Sys=%dMB", m.Alloc/1024/1024, m.Sys/1024/1024)

	if !testing.Short() && pullDuration > 30*time.Second {
		t.Errorf("Pull took %v, expected < 30s", pullDuration)
	}

	// Cleanup
	for _, et := range entities {
		pool.Exec(ctx, fmt.Sprintf("DELETE FROM %s WHERE id LIKE '%%0000-0000-0000-0000%%'", et.table))
	}
	t.Log("Cleanup complete")
}
