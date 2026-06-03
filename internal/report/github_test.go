package report

import (
	"bytes"
	"strings"
	"testing"

	"github.com/get-felipe/pg-contract/internal/check"
	"github.com/get-felipe/pg-contract/internal/query"
)

func TestWriteGitHubBreakingAnnotation(t *testing.T) {
	report := &check.Report{Results: []check.Result{
		{
			Query:  query.Query{Name: "customers.find", File: "queries/find.sql", StartLine: 2},
			Before: check.Outcome{OK: true},
			After:  check.Outcome{Error: &check.DBError{Code: "42703", Message: "column \"email\" does not exist"}},
		},
	}}

	var out bytes.Buffer
	WriteGitHub(&out, report)

	got := out.String()
	for _, want := range []string{
		"::error file=queries/find.sql,line=2,title=pg-contract%3A breaking%3A customers.find::",
		"A column referenced by this query does not exist in the target schema.",
		"SQLSTATE: 42703.",
		"column \"email\" does not exist",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, got)
		}
	}
}

func TestWriteGitHubShapeChangeAnnotation(t *testing.T) {
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
	WriteGitHub(&out, report)

	got := out.String()
	for _, want := range []string{
		"::error file=queries/list.sql,line=2,title=pg-contract%3A breaking%3A customers.list::",
		"The query result columns changed between the before and after schemas.",
		"column 1 \"email\" type changed from text to character varying(320)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, got)
		}
	}
	if strings.Contains(got, "SQLSTATE") {
		t.Fatalf("shape change annotation should not include SQLSTATE, got:\n%s", got)
	}
}

func TestWriteGitHubInvalidBeforeAnnotation(t *testing.T) {
	report := &check.Report{Results: []check.Result{
		{
			Query:  query.Query{Name: "broken", File: "queries/broken.sql", StartLine: 1},
			Before: check.Outcome{Error: &check.DBError{Code: "42P01", Message: "relation does not exist"}},
			After:  check.Outcome{Error: &check.DBError{Code: "42P01", Message: "relation does not exist"}},
		},
	}}

	var out bytes.Buffer
	WriteGitHub(&out, report)

	got := out.String()
	if !strings.Contains(got, "title=pg-contract%3A invalid before%3A broken") {
		t.Fatalf("expected invalid-before title, got:\n%s", got)
	}
	if !strings.Contains(got, "A table or view referenced by this query does not exist in the target schema.") {
		t.Fatalf("expected reason, got:\n%s", got)
	}
}

func TestWriteGitHubNoticeWhenClean(t *testing.T) {
	report := &check.Report{Results: []check.Result{
		{
			Query:  query.Query{Name: "customers.find", File: "queries/find.sql", StartLine: 2},
			Before: check.Outcome{OK: true},
			After:  check.Outcome{OK: true},
		},
	}}

	var out bytes.Buffer
	WriteGitHub(&out, report)

	got := out.String()
	if !strings.Contains(got, "::notice::1 query checked.") {
		t.Fatalf("expected notice command, got:\n%s", got)
	}
}

func TestGitHubEscaping(t *testing.T) {
	property := escapeGitHubProperty("path:with,chars%and\nnewline")
	if property != "path%3Awith%2Cchars%25and%0Anewline" {
		t.Fatalf("unexpected escaped property: %q", property)
	}

	data := escapeGitHubData("message%with\nnewline")
	if data != "message%25with%0Anewline" {
		t.Fatalf("unexpected escaped data: %q", data)
	}
}
