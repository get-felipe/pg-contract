package check

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

func TestDiagnostics(t *testing.T) {
	tests := []struct {
		code           string
		message        string
		reasonContains string
		fixContains    string
	}{
		{code: "22P02", message: "invalid input value for enum", reasonContains: "literal or cast", fixContains: "accepted type values"},
		{code: "3F000", message: "schema does not exist", reasonContains: "schema", fixContains: "search path"},
		{code: "42601", message: "syntax error", reasonContains: "not valid SQL", fixContains: "syntax"},
		{code: "42702", message: "column reference is ambiguous", reasonContains: "ambiguous", fixContains: "Qualify"},
		{code: "42703", message: "column does not exist", reasonContains: "column", fixContains: "old column"},
		{code: "42704", message: "object does not exist", reasonContains: "object", fixContains: "referenced object"},
		{code: "42725", message: "function is not unique", reasonContains: "function call", fixContains: "explicit casts"},
		{code: "42804", message: "datatype mismatch", reasonContains: "datatype mismatch", fixContains: "expression types"},
		{code: "42809", message: "wrong object type", reasonContains: "wrong kind", fixContains: "object kind"},
		{code: "42846", message: "cannot cast type", reasonContains: "cast or coercion", fixContains: "compatible cast"},
		{code: "42883", message: "function does not exist", reasonContains: "function or operator", fixContains: "function/operator signature"},
		{code: "42P01", message: "relation does not exist", reasonContains: "table or view", fixContains: "search path"},
		{code: "42P02", message: "there is no parameter $1", reasonContains: "parameter", fixContains: "parameter type config"},
		{code: "42P10", message: "invalid column reference", reasonContains: "column reference", fixContains: "constraints"},
		{code: "42P18", message: "could not determine data type", reasonContains: "infer a parameter type", fixContains: "pg-contract.yaml"},
	}

	for _, test := range tests {
		t.Run(test.code, func(t *testing.T) {
			err := &DBError{Code: test.code, Message: test.message}
			diagnostic := Classify(err)
			if diagnostic.Reason == test.message {
				t.Fatal("expected SQLSTATE-specific reason, got raw message")
			}
			if !strings.Contains(diagnostic.Reason, test.reasonContains) {
				t.Fatalf("expected reason to contain %q, got %q", test.reasonContains, diagnostic.Reason)
			}
			if !strings.Contains(diagnostic.Suggestion, test.fixContains) {
				t.Fatalf("expected suggestion to contain %q, got %q", test.fixContains, diagnostic.Suggestion)
			}
		})
	}
}

func TestUnknownDiagnosticFallsBackToPostgresMessage(t *testing.T) {
	err := &DBError{Code: "99999", Message: "custom extension error"}

	diagnostic := Classify(err)
	if diagnostic.Reason != err.Message {
		t.Fatalf("expected fallback reason %q, got %q", err.Message, diagnostic.Reason)
	}
	if diagnostic.Suggestion == "" {
		t.Fatal("expected fallback suggestion")
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

func TestRunManifestRejectsCLIQueryInputs(t *testing.T) {
	root := t.TempDir()
	queriesDir := filepath.Join(root, "queries")
	if err := os.MkdirAll(queriesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configFile := filepath.Join(root, "pg-contract.yaml")
	if err := os.WriteFile(configFile, []byte(`version: "0.2"
query_sets:
  - name: app
    queries: queries
`), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := Run(ctx, Options{
		BeforeURL:   "postgres://%zz",
		AfterURL:    "postgres://%zz",
		QueriesPath: queriesDir,
		ConfigPath:  configFile,
	})
	if err == nil || !strings.Contains(err.Error(), "--queries cannot be used with config version 0.2") {
		t.Fatalf("expected manifest/queries conflict before connecting, got %v", err)
	}
}

func TestRunRejectsQuerySetSelectionOutsideManifest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := Run(ctx, Options{
		BeforeURL: "postgres://%zz",
		AfterURL:  "postgres://%zz",
		QuerySets: []string{"app"},
	})
	if err == nil || !strings.Contains(err.Error(), "--query-set requires config version 0.2") {
		t.Fatalf("expected query-set manifest requirement before legacy validation, got %v", err)
	}
}

func TestRunManifestRejectsUnknownQuerySetBeforeConnecting(t *testing.T) {
	root := t.TempDir()
	queriesDir := filepath.Join(root, "queries")
	if err := os.MkdirAll(queriesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(queriesDir, "find.sql"), []byte("-- name: customers.find\nselect 1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	configFile := filepath.Join(root, "pg-contract.yaml")
	if err := os.WriteFile(configFile, []byte(`version: "0.2"
query_sets:
  - name: app
    queries: queries
`), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := Run(ctx, Options{
		BeforeURL:  "postgres://%zz",
		AfterURL:   "postgres://%zz",
		ConfigPath: configFile,
		QuerySets:  []string{"reporting"},
	})
	if err == nil || !strings.Contains(err.Error(), "unknown query set \"reporting\"") {
		t.Fatalf("expected unknown query set before connecting, got %v", err)
	}
}

func TestRunManifestSkipsUnselectedQuerySetsBeforeConnecting(t *testing.T) {
	root := t.TempDir()
	queriesDir := filepath.Join(root, "queries")
	if err := os.MkdirAll(queriesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(queriesDir, "find.sql"), []byte("-- name: customers.find\nselect 1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	configFile := filepath.Join(root, "pg-contract.yaml")
	if err := os.WriteFile(configFile, []byte(`version: "0.2"
query_sets:
  - name: app
    queries: queries
  - name: reporting
    queries: missing-reporting-dir
queries:
  reporting.list:
    params: []
`), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := Run(ctx, Options{
		BeforeURL:  "postgres://%zz",
		AfterURL:   "postgres://%zz",
		ConfigPath: configFile,
		QuerySets:  []string{"app"},
	})
	if err == nil {
		t.Fatal("expected connection failure after loading only the selected query set")
	}
	if strings.Contains(err.Error(), "missing-reporting-dir") {
		t.Fatalf("expected unselected query set to be skipped, got %v", err)
	}
	if !strings.Contains(err.Error(), "connect before database") {
		t.Fatalf("expected connection failure after selected query loading, got %v", err)
	}
}

func TestRunManifestDetectsDuplicateNamesBeforeConnecting(t *testing.T) {
	root := t.TempDir()
	left := filepath.Join(root, "left")
	right := filepath.Join(root, "right")
	if err := os.MkdirAll(left, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(right, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(left, "find.sql"), []byte("-- name: duplicate\nselect 1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(right, "find.sql"), []byte("-- name: duplicate\nselect 2;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	configFile := filepath.Join(root, "pg-contract.yaml")
	if err := os.WriteFile(configFile, []byte(`version: "0.2"
query_sets:
  - name: app
    queries:
      - left
      - right
`), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := Run(ctx, Options{
		BeforeURL:  "postgres://%zz",
		AfterURL:   "postgres://%zz",
		ConfigPath: configFile,
	})
	if err == nil || !strings.Contains(err.Error(), "duplicate query name") {
		t.Fatalf("expected duplicate query name before connecting, got %v", err)
	}
}

func TestRunWithPostgresManifest(t *testing.T) {
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

	schemaName := fmt.Sprintf("contract_it_%d", time.Now().UnixNano())
	beforeSchema := filepath.Join(root, "before.sql")
	afterSchema := filepath.Join(root, "after.sql")
	queryFile := filepath.Join(queriesDir, "find_customer.sql")
	configFile := filepath.Join(root, "pg-contract.yaml")
	defer dropIntegrationSchema(t, beforeURL, schemaName)
	defer dropIntegrationSchema(t, afterURL, schemaName)

	if err := os.WriteFile(beforeSchema, []byte(fmt.Sprintf(`
drop schema if exists %[1]s cascade;
create schema %[1]s;
create table %[1]s.customers (
  id uuid primary key,
  email text not null
);
`, schemaName)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(afterSchema, []byte(fmt.Sprintf(`
drop schema if exists %[1]s cascade;
create schema %[1]s;
create table %[1]s.customers (
  id uuid primary key
);
`, schemaName)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(queryFile, []byte(`-- name: customers.find
select id, email
from customers
where id = $1;
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configFile, []byte(fmt.Sprintf(`version: "0.2"
query_sets:
  - name: app
    queries: queries
    schema:
      before: before.sql
      after: after.sql
    prepare:
      search_path:
        - %[1]s
        - public
    tags:
      - app
queries:
  customers.find:
    tags:
      - customer-facing
`, schemaName)), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	report, err := Run(ctx, Options{
		BeforeURL:  beforeURL,
		AfterURL:   afterURL,
		ConfigPath: configFile,
	})
	if err != nil {
		t.Fatal(err)
	}

	breaking := report.Breaking()
	if len(breaking) != 1 {
		t.Fatalf("expected 1 breaking query, got %d", len(breaking))
	}
	if breaking[0].QuerySet != "app" {
		t.Fatalf("expected query set app, got %q", breaking[0].QuerySet)
	}
	if len(breaking[0].Tags) != 2 || breaking[0].Tags[0] != "app" || breaking[0].Tags[1] != "customer-facing" {
		t.Fatalf("unexpected tags: %#v", breaking[0].Tags)
	}
	if breaking[0].After.Error == nil || breaking[0].After.Error.Code != "42703" {
		t.Fatalf("expected undefined_column 42703, got %#v", breaking[0].After.Error)
	}
}

func TestRunWithPostgresExampleFixtures(t *testing.T) {
	beforeURL := os.Getenv("PG_CONTRACT_TEST_BEFORE_URL")
	afterURL := os.Getenv("PG_CONTRACT_TEST_AFTER_URL")
	if beforeURL == "" || afterURL == "" {
		t.Skip("set PG_CONTRACT_TEST_BEFORE_URL and PG_CONTRACT_TEST_AFTER_URL to run integration test")
	}
	if beforeURL == afterURL {
		t.Skip("integration test requires separate before and after databases")
	}

	tests := []struct {
		name     string
		config   string
		exitCode int
		sqlstate string
	}{
		{name: "basic", exitCode: 1, sqlstate: "42703"},
		{name: "missing-table", exitCode: 1, sqlstate: "42P01"},
		{name: "ambiguous-column", exitCode: 1, sqlstate: "42702"},
		{name: "typed-params", config: "pg-contract.yaml", exitCode: 0},
		{name: "view-changed", exitCode: 1, sqlstate: "42703"},
		{name: "function-signature", exitCode: 1, sqlstate: "42883"},
		{name: "enum-value", exitCode: 1, sqlstate: "22P02"},
		{name: "search-path", exitCode: 1, sqlstate: "42P01"},
	}

	repoRoot := testRepoRoot(t)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			exampleDir := filepath.Join(repoRoot, "examples", test.name)
			opts := Options{
				BeforeURL:    beforeURL,
				AfterURL:     afterURL,
				SchemaBefore: filepath.Join(exampleDir, "schema-before.sql"),
				SchemaAfter:  filepath.Join(exampleDir, "schema-after.sql"),
				QueriesPath:  filepath.Join(exampleDir, "queries"),
			}
			if test.config != "" {
				opts.ConfigPath = filepath.Join(exampleDir, test.config)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			report, err := Run(ctx, opts)
			if err != nil {
				t.Fatal(err)
			}

			if got := ExitCode(report); got != test.exitCode {
				t.Fatalf("expected exit code %d, got %d with report %#v", test.exitCode, got, report)
			}
			if test.sqlstate == "" {
				return
			}

			breaking := report.Breaking()
			if len(breaking) != 1 {
				t.Fatalf("expected 1 breaking query, got %d", len(breaking))
			}
			if breaking[0].After.Error == nil || breaking[0].After.Error.Code != test.sqlstate {
				t.Fatalf("expected SQLSTATE %s, got %#v", test.sqlstate, breaking[0].After.Error)
			}
		})
	}
}

func dropIntegrationSchema(t *testing.T, url string, schema string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Logf("cleanup connect failed: %v", err)
		return
	}
	defer conn.Close(context.Background())

	_, err = conn.Exec(ctx, fmt.Sprintf("drop schema if exists %s cascade", schema))
	if err != nil {
		t.Logf("cleanup drop schema failed: %v", err)
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

func testRepoRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not resolve test file path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
