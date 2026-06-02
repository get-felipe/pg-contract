package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	path := writeConfig(t, `
queries:
  customers.find:
    params:
      - uuid
      - timestamp with time zone
      - text[]
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	params := cfg.Params("customers.find")
	if len(params) != 3 {
		t.Fatalf("expected 3 params, got %d", len(params))
	}
	if params[1] != "timestamp with time zone" {
		t.Fatalf("expected normalized timestamp type, got %q", params[1])
	}
	if params[2] != "text[]" {
		t.Fatalf("expected array type, got %q", params[2])
	}
}

func TestLoadConfigAllowsEmptyParams(t *testing.T) {
	path := writeConfig(t, `
queries:
  customers.find:
    params: []
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if params := cfg.Params("customers.find"); len(params) != 0 {
		t.Fatalf("expected empty params, got %#v", params)
	}
}

func TestLoadManifestV02(t *testing.T) {
	path := writeConfig(t, `
version: "0.2"
defaults:
  prepare:
    search_path:
      - public
query_sets:
  - name: app
    queries:
      - queries
      - more.sql
    schema:
      before:
        - before.sql
      after: after.sql
    tags:
      - ci
queries:
  customers.find:
    params:
      - uuid
    tags:
      - customer-facing
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if !cfg.IsManifest() {
		t.Fatal("expected v0.2 manifest")
	}
	if len(cfg.QuerySets) != 1 {
		t.Fatalf("expected 1 query set, got %d", len(cfg.QuerySets))
	}
	set := cfg.QuerySets[0]
	if set.Name != "app" {
		t.Fatalf("expected query set app, got %q", set.Name)
	}
	if got := cfg.ResolvePath(set.Queries[0]); got != filepath.Join(filepath.Dir(path), "queries") {
		t.Fatalf("expected query path relative to manifest, got %q", got)
	}
	if got := cfg.ResolvePath(set.Schema.After[0]); got != filepath.Join(filepath.Dir(path), "after.sql") {
		t.Fatalf("expected schema path relative to manifest, got %q", got)
	}
	if params := cfg.Params("customers.find"); len(params) != 1 || params[0] != "uuid" {
		t.Fatalf("unexpected params: %#v", params)
	}
	tags := cfg.Tags(set, "customers.find")
	if len(tags) != 2 || tags[0] != "ci" || tags[1] != "customer-facing" {
		t.Fatalf("unexpected tags: %#v", tags)
	}
	searchPath := cfg.SearchPath(set)
	if len(searchPath) != 1 || searchPath[0] != "public" {
		t.Fatalf("unexpected search path: %#v", searchPath)
	}
}

func TestLoadManifestV02AcceptsSingleQueryPath(t *testing.T) {
	path := writeConfig(t, `
version: "0.2"
query_sets:
  - name: app
    queries: queries
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if got := []string(cfg.QuerySets[0].Queries); len(got) != 1 || got[0] != "queries" {
		t.Fatalf("unexpected query paths: %#v", got)
	}
}

func TestLoadManifestV02PerSetSearchPathOverridesDefault(t *testing.T) {
	path := writeConfig(t, `
version: "0.2"
defaults:
  prepare:
    search_path:
      - public
query_sets:
  - name: reporting
    queries: queries
    prepare:
      search_path:
        - reporting
        - public
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	searchPath := cfg.SearchPath(cfg.QuerySets[0])
	if len(searchPath) != 2 || searchPath[0] != "reporting" || searchPath[1] != "public" {
		t.Fatalf("unexpected search path: %#v", searchPath)
	}
}

func TestLoadManifestV02RejectsMissingQuerySets(t *testing.T) {
	path := writeConfig(t, `
version: "0.2"
`)

	if _, err := Load(path); err == nil {
		t.Fatal("expected missing query_sets error")
	}
}

func TestLoadManifestV02RejectsDuplicateQuerySetNames(t *testing.T) {
	path := writeConfig(t, `
version: "0.2"
query_sets:
  - name: app
    queries: queries
  - name: app
    queries: more
`)

	if _, err := Load(path); err == nil {
		t.Fatal("expected duplicate query set error")
	}
}

func TestLoadManifestV02RejectsUnsupportedVersion(t *testing.T) {
	path := writeConfig(t, `
version: "0.3"
query_sets:
  - name: app
    queries: queries
`)

	if _, err := Load(path); err == nil {
		t.Fatal("expected unsupported version error")
	}
}

func TestLoadConfigRejectsQuerySetsWithoutVersion(t *testing.T) {
	path := writeConfig(t, `
query_sets:
  - name: app
    queries: queries
`)

	if _, err := Load(path); err == nil {
		t.Fatal("expected query_sets without version error")
	}
}

func TestLoadConfigRejectsUnknownFields(t *testing.T) {
	path := writeConfig(t, `
queries:
  customers.find:
    args:
      - uuid
`)

	if _, err := Load(path); err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestLoadConfigRejectsUnsafeTypeName(t *testing.T) {
	path := writeConfig(t, `
queries:
  customers.find:
    params:
      - "uuid); drop table users; --"
`)

	if _, err := Load(path); err == nil {
		t.Fatal("expected unsafe type name error")
	}
}

func TestValidateQueryNames(t *testing.T) {
	path := writeConfig(t, `
queries:
  missing.query:
    params:
      - uuid
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	err = cfg.ValidateQueryNames(map[string]struct{}{"customers.find": {}})
	if err == nil {
		t.Fatal("expected unknown query error")
	}
}

func TestGenerate(t *testing.T) {
	got := string(Generate([]string{"billing.list", "customers.find"}))
	want := `# Generated by pg-contract init.
# Fill params only when Postgres cannot infer $1, $2, ... types during PREPARE.
queries:
  billing.list:
    params: []
  customers.find:
    params: []
`
	if got != want {
		t.Fatalf("unexpected generated config:\n%s", got)
	}
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "pg-contract.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
