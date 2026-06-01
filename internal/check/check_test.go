package check

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestExitCode(t *testing.T) {
	breaking := &Report{Results: []Result{{Before: Outcome{OK: true}, After: Outcome{Error: &DBError{Code: "42703"}}}}}
	if got := ExitCode(breaking); got != 1 {
		t.Fatalf("expected breaking exit code 1, got %d", got)
	}

	invalidBefore := &Report{Results: []Result{{Before: Outcome{Error: &DBError{Code: "42P01"}}, After: Outcome{Error: &DBError{Code: "42P01"}}}}}
	if got := ExitCode(invalidBefore); got != 2 {
		t.Fatalf("expected invalid-before exit code 2, got %d", got)
	}

	clean := &Report{Results: []Result{{Before: Outcome{OK: true}, After: Outcome{OK: true}}}}
	if got := ExitCode(clean); got != 0 {
		t.Fatalf("expected clean exit code 0, got %d", got)
	}
}

func TestReason(t *testing.T) {
	reason := Reason(&DBError{Code: "42703", Message: "column does not exist"})
	if reason == "column does not exist" {
		t.Fatal("expected SQLSTATE-specific reason, got raw message")
	}
}

func TestRunWithPostgres(t *testing.T) {
	beforeURL := os.Getenv("PG_CONTRACT_TEST_BEFORE_URL")
	afterURL := os.Getenv("PG_CONTRACT_TEST_AFTER_URL")
	if beforeURL == "" || afterURL == "" {
		t.Skip("set PG_CONTRACT_TEST_BEFORE_URL and PG_CONTRACT_TEST_AFTER_URL to run integration test")
	}
	if beforeURL == afterURL {
		t.Skip("integration test requires separate before and after databases")
	}

	root := t.TempDir()
	queriesDir := filepath.Join(root, "queries")
	if err := os.MkdirAll(queriesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	table := fmt.Sprintf("pg_contract_it_%d", time.Now().UnixNano())
	beforeSchema := filepath.Join(root, "before.sql")
	afterSchema := filepath.Join(root, "after.sql")
	queryFile := filepath.Join(queriesDir, "find_customer.sql")
	defer dropIntegrationTable(t, beforeURL, table)
	defer dropIntegrationTable(t, afterURL, table)

	if err := os.WriteFile(beforeSchema, []byte(fmt.Sprintf(`
drop table if exists %[1]s;
create table %[1]s (
  id uuid primary key,
  email text not null
);
`, table)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(afterSchema, []byte(fmt.Sprintf(`
drop table if exists %[1]s;
create table %[1]s (
  id uuid primary key
);
`, table)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(queryFile, []byte(fmt.Sprintf(`-- name: customers.find
select id, email
from %s
where id = $1;
`, table)), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	report, err := Run(ctx, Options{
		BeforeURL:    beforeURL,
		AfterURL:     afterURL,
		SchemaBefore: beforeSchema,
		SchemaAfter:  afterSchema,
		QueriesPath:  queriesDir,
	})
	if err != nil {
		t.Fatal(err)
	}

	breaking := report.Breaking()
	if len(breaking) != 1 {
		t.Fatalf("expected 1 breaking query, got %d", len(breaking))
	}
	if breaking[0].After.Error == nil || breaking[0].After.Error.Code != "42703" {
		t.Fatalf("expected undefined_column 42703, got %#v", breaking[0].After.Error)
	}
}

func TestRunWithPostgresTypedParams(t *testing.T) {
	beforeURL := os.Getenv("PG_CONTRACT_TEST_BEFORE_URL")
	afterURL := os.Getenv("PG_CONTRACT_TEST_AFTER_URL")
	if beforeURL == "" || afterURL == "" {
		t.Skip("set PG_CONTRACT_TEST_BEFORE_URL and PG_CONTRACT_TEST_AFTER_URL to run integration test")
	}

	root := t.TempDir()
	queriesDir := filepath.Join(root, "queries")
	if err := os.MkdirAll(queriesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	queryFile := filepath.Join(queriesDir, "count_tags.sql")
	configFile := filepath.Join(root, "pg-contract.yaml")

	if err := os.WriteFile(queryFile, []byte(`-- name: search.count_tags
select array_length($1, 1) as tag_count;
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configFile, []byte(`queries:
  search.count_tags:
    params:
      - text[]
`), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	report, err := Run(ctx, Options{
		BeforeURL:   beforeURL,
		AfterURL:    afterURL,
		QueriesPath: queriesDir,
		ConfigPath:  configFile,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := ExitCode(report); got != 0 {
		t.Fatalf("expected clean exit code 0, got %d with report %#v", got, report)
	}
}

func dropIntegrationTable(t *testing.T, url string, table string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Logf("cleanup connect failed: %v", err)
		return
	}
	defer conn.Close(context.Background())

	_, err = conn.Exec(ctx, fmt.Sprintf("drop table if exists %s", table))
	if err != nil {
		t.Logf("cleanup drop failed: %v", err)
	}
}
