package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(nil, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Fatalf("expected help output to contain Usage, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestRunVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"version"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	want := "pg-contract " + Version
	if !strings.Contains(stdout.String(), want) {
		t.Fatalf("expected version output to contain %q, got %q", want, stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestRunVersionUsesInjectedVersion(t *testing.T) {
	original := Version
	Version = "1.2.3-test"
	t.Cleanup(func() {
		Version = original
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"version"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if got := strings.TrimSpace(stdout.String()); got != "pg-contract 1.2.3-test" {
		t.Fatalf("expected injected version, got %q", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestNormalizeBuildVersion(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "devel", in: "(devel)", want: ""},
		{name: "module tag", in: "v0.1.0-alpha.2", want: "0.1.0-alpha.2"},
		{name: "plain version", in: "0.1.0-alpha.2", want: "0.1.0-alpha.2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeBuildVersion(tt.in); got != tt.want {
				t.Fatalf("normalizeBuildVersion(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestRunCheckMissingFlags(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"check"}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "missing required --before-url") {
		t.Fatalf("expected missing flag message, got %q", stderr.String())
	}
}

func TestRunCheckRejectsUnknownFormat(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"check", "--format", "xml"}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "invalid --format") {
		t.Fatalf("expected invalid format message, got %q", stderr.String())
	}
}

func TestRunCheckAutoLoadsDefaultConfig(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	queriesDir := filepath.Join(root, "queries")
	if err := os.MkdirAll(queriesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(queriesDir, "find.sql"), []byte("-- name: customers.find\nselect 1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "pg-contract.yaml"), []byte("queries:\n  missing.query:\n    params: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"check", "--before-url", "postgres://%zz", "--after-url", "postgres://%zz", "--queries", queriesDir}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "config references unknown query") {
		t.Fatalf("expected autodetected config error, got %q", stderr.String())
	}
}

func TestRunCheckNoConfigDisablesAutoLoad(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	queriesDir := filepath.Join(root, "queries")
	if err := os.MkdirAll(queriesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(queriesDir, "find.sql"), []byte("-- name: customers.find\nselect 1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "pg-contract.yaml"), []byte("queries:\n  missing.query:\n    params: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"check", "--before-url", "postgres://%zz", "--after-url", "postgres://%zz", "--queries", queriesDir, "--no-config"}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if strings.Contains(stderr.String(), "config references unknown query") {
		t.Fatalf("expected --no-config to skip autodetected config, got %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "connect before database") {
		t.Fatalf("expected connection failure after skipping config, got %q", stderr.String())
	}
}

func TestRunCheckRejectsConfigWithNoConfig(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"check", "--config", "pg-contract.yaml", "--no-config"}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "cannot be used together") {
		t.Fatalf("expected conflict error, got %q", stderr.String())
	}
}

func TestRunCheckRejectsEmptyConfigFlag(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"check", "--config", ""}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "--config cannot be empty") {
		t.Fatalf("expected empty config error, got %q", stderr.String())
	}
}

func TestRunInitWritesConfig(t *testing.T) {
	root := t.TempDir()
	queriesDir := filepath.Join(root, "queries")
	if err := os.MkdirAll(queriesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(queriesDir, "find.sql"), []byte("-- name: customers.find\nselect 1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(root, "pg-contract.yaml")

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"init", "--queries", queriesDir, "--out", outPath}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "customers.find:") {
		t.Fatalf("expected generated config to include query name, got:\n%s", string(data))
	}
	if !strings.Contains(stdout.String(), "Wrote ") {
		t.Fatalf("expected success message, got %q", stdout.String())
	}
}

func TestRunInitRefusesOverwrite(t *testing.T) {
	root := t.TempDir()
	queriesDir := filepath.Join(root, "queries")
	if err := os.MkdirAll(queriesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(queriesDir, "find.sql"), []byte("select 1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(root, "pg-contract.yaml")
	if err := os.WriteFile(outPath, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"init", "--queries", queriesDir, "--out", outPath}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "already exists") {
		t.Fatalf("expected already exists error, got %q", stderr.String())
	}
}

func TestRunInitStdout(t *testing.T) {
	root := t.TempDir()
	queriesDir := filepath.Join(root, "queries")
	if err := os.MkdirAll(queriesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(queriesDir, "find.sql"), []byte("select 1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"init", "--queries", queriesDir, "--out", "-"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "find:") {
		t.Fatalf("expected generated config on stdout, got:\n%s", stdout.String())
	}
}
