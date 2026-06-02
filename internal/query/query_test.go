package query

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDirParsesNamedQueries(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "customers", "find.sql")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("-- name: customers.find\nselect id from customers where id = $1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	queries, err := LoadDir(root)
	if err != nil {
		t.Fatal(err)
	}

	if len(queries) != 1 {
		t.Fatalf("expected 1 query, got %d", len(queries))
	}
	if queries[0].Name != "customers.find" {
		t.Fatalf("expected query name customers.find, got %q", queries[0].Name)
	}
	if queries[0].StartLine != 2 {
		t.Fatalf("expected start line 2, got %d", queries[0].StartLine)
	}
}

func TestLoadDirParsesSQLCCommandSuffix(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "customers.sql")
	if err := os.WriteFile(path, []byte("-- name: customers.find :one\nselect id from customers where id = $1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	queries, err := LoadDir(root)
	if err != nil {
		t.Fatal(err)
	}

	if got := queries[0].Name; got != "customers.find" {
		t.Fatalf("expected query name customers.find, got %q", got)
	}
}

func TestLoadDirFallsBackToFileName(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "billing", "list_invoices.sql")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("select id from invoices;\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	queries, err := LoadDir(root)
	if err != nil {
		t.Fatal(err)
	}

	if got := queries[0].Name; got != "billing.list_invoices" {
		t.Fatalf("expected fallback name billing.list_invoices, got %q", got)
	}
}

func TestLoadDirRejectsDuplicateNames(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.sql"), []byte("-- name: duplicate\nselect 1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.sql"), []byte("-- name: duplicate\nselect 2;\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadDir(root); err == nil {
		t.Fatal("expected duplicate query name error")
	}
}

func TestLoadPathsAcceptsSQLFilesAndDirectories(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "queries")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "find.sql"), []byte("-- name: customers.find\nselect 1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "count.sql")
	if err := os.WriteFile(file, []byte("-- name: reporting.count\nselect 2;\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	queries, err := LoadPaths([]string{dir, file})
	if err != nil {
		t.Fatal(err)
	}

	if len(queries) != 2 {
		t.Fatalf("expected 2 queries, got %d", len(queries))
	}
	if queries[0].Name != "reporting.count" {
		t.Fatalf("expected sorted file query first, got %q", queries[0].Name)
	}
	if queries[1].Name != "customers.find" {
		t.Fatalf("expected directory query second, got %q", queries[1].Name)
	}
}

func TestLoadPathsUsesFileBaseNameFallback(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "count_tags.sql")
	if err := os.WriteFile(file, []byte("select 1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	queries, err := LoadPaths([]string{file})
	if err != nil {
		t.Fatal(err)
	}

	if got := queries[0].Name; got != "count_tags" {
		t.Fatalf("expected file basename fallback, got %q", got)
	}
}

func TestLoadPathsRejectsDuplicateNamesAcrossRoots(t *testing.T) {
	root := t.TempDir()
	left := filepath.Join(root, "left")
	right := filepath.Join(root, "right")
	if err := os.MkdirAll(left, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(right, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(left, "a.sql"), []byte("-- name: duplicate\nselect 1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(right, "b.sql"), []byte("-- name: duplicate\nselect 2;\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadPaths([]string{left, right}); err == nil {
		t.Fatal("expected duplicate query name error")
	}
}
