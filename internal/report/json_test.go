package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/get-felipe/pg-contract/internal/check"
	"github.com/get-felipe/pg-contract/internal/query"
)

func TestWriteJSONBreaking(t *testing.T) {
	report := &check.Report{Results: []check.Result{
		{
			Query:  query.Query{Name: "customers.find", File: "queries/find.sql", StartLine: 2},
			Before: check.Outcome{OK: true},
			After:  check.Outcome{Error: &check.DBError{Code: "42703", Message: "column \"email\" does not exist", Position: 17}},
		},
	}}

	var out bytes.Buffer
	if err := WriteJSON(&out, report); err != nil {
		t.Fatal(err)
	}

	var got JSONReport
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("expected valid JSON: %v\n%s", err, out.String())
	}

	if got.Status != "breaking" {
		t.Fatalf("expected status breaking, got %q", got.Status)
	}
	if got.Summary.Checked != 1 || got.Summary.Breaking != 1 {
		t.Fatalf("unexpected summary: %#v", got.Summary)
	}
	if got.Results[0].After.Error == nil || got.Results[0].After.Error.Code != "42703" {
		t.Fatalf("expected SQLSTATE 42703, got %#v", got.Results[0].After.Error)
	}
	if strings.Contains(out.String(), "postgres://") {
		t.Fatalf("JSON output must not include database URLs: %s", out.String())
	}
}

func TestNewJSONReportInvalidBefore(t *testing.T) {
	report := &check.Report{Results: []check.Result{
		{
			Query:  query.Query{Name: "broken", File: "queries/broken.sql", StartLine: 1},
			Before: check.Outcome{Error: &check.DBError{Code: "42P01", Message: "relation does not exist"}},
			After:  check.Outcome{Error: &check.DBError{Code: "42P01", Message: "relation does not exist"}},
		},
	}}

	got := NewJSONReport(report)
	if got.Status != "invalid_before" {
		t.Fatalf("expected invalid_before status, got %q", got.Status)
	}
	if got.Results[0].Status != "invalid_before" {
		t.Fatalf("expected invalid_before result, got %q", got.Results[0].Status)
	}
}

func TestNewJSONReportIncludesManifestMetadata(t *testing.T) {
	report := &check.Report{Results: []check.Result{
		{
			QuerySet: "app",
			Tags:     []string{"ci", "customer-facing"},
			Query:    query.Query{Name: "customers.find", File: "queries/find.sql", StartLine: 2},
			Before:   check.Outcome{OK: true},
			After:    check.Outcome{OK: true},
		},
	}}

	got := NewJSONReport(report)
	if got.Results[0].QuerySet != "app" {
		t.Fatalf("expected query_set app, got %q", got.Results[0].QuerySet)
	}
	if len(got.Results[0].Tags) != 2 || got.Results[0].Tags[0] != "ci" || got.Results[0].Tags[1] != "customer-facing" {
		t.Fatalf("unexpected tags: %#v", got.Results[0].Tags)
	}
}
