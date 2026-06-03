package report

import (
	"bytes"
	"strings"
	"testing"

	"github.com/get-felipe/pg-contract/internal/check"
	"github.com/get-felipe/pg-contract/internal/query"
)

func TestWriteTextBreaking(t *testing.T) {
	report := &check.Report{Results: []check.Result{
		{
			Query: query.Query{Name: "customers.find", File: "queries/find.sql", StartLine: 2},
			Before: check.Outcome{
				OK: true,
			},
			After: check.Outcome{
				Error: &check.DBError{Code: "42703", Message: "column \"email\" does not exist", Position: 17},
			},
		},
	}}

	var out bytes.Buffer
	WriteText(&out, report)

	got := out.String()
	for _, want := range []string{"FAIL customers.find", "queries/find.sql:2", "SQLSTATE: 42703", "POSITION: 17", "Summary: 1 query checked, 1 breaking change found."} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, got)
		}
	}
}

func TestWriteTextShapeChange(t *testing.T) {
	report := &check.Report{Results: []check.Result{
		{
			Query:  query.Query{Name: "customers.list", File: "queries/list.sql", StartLine: 2},
			Before: check.Outcome{OK: true},
			After:  check.Outcome{OK: true},
			ShapeChange: &check.ShapeChange{Differences: []check.ShapeDifference{
				{
					Kind:     "column_type",
					Position: 1,
					Message:  "column 1 \"email\" type changed from text to character varying(320)",
				},
			}},
		},
	}}

	var out bytes.Buffer
	WriteText(&out, report)

	got := out.String()
	for _, want := range []string{
		"FAIL customers.list",
		"The query result columns changed",
		"Result shape change:",
		"column 1 \"email\" type changed from text to character varying(320)",
		"works before and after",
		"Summary: 1 query checked, 1 breaking change found.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Postgres error:") {
		t.Fatalf("shape change output should not include Postgres error section, got:\n%s", got)
	}
}
