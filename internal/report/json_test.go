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

func TestNewJSONReportShapeChange(t *testing.T) {
	report := &check.Report{Results: []check.Result{
		{
			Query: query.Query{Name: "customers.list", File: "queries/list.sql", StartLine: 2},
			Before: check.Outcome{OK: true, ResultShape: []check.ResultColumn{
				{Name: "email", DataType: "text", DataTypeOID: 25, TypeModifier: -1},
			}},
			After: check.Outcome{OK: true, ResultShape: []check.ResultColumn{
				{Name: "email", DataType: "character varying(320)", DataTypeOID: 1043, TypeModifier: 324},
			}},
			ShapeChange: &check.ShapeChange{Differences: []check.ShapeDifference{
				{
					Kind:     "column_type",
					Position: 1,
					Before:   &check.ResultColumn{Name: "email", DataType: "text", DataTypeOID: 25, TypeModifier: -1},
					After:    &check.ResultColumn{Name: "email", DataType: "character varying(320)", DataTypeOID: 1043, TypeModifier: 324},
					Message:  "column 1 \"email\" type changed from text to character varying(320)",
				},
			}},
		},
	}}

	got := NewJSONReport(report)
	if got.Status != "breaking" || got.Summary.Breaking != 1 {
		t.Fatalf("expected breaking shape report, got %#v", got)
	}
	if got.Results[0].After.Error != nil || !got.Results[0].After.OK {
		t.Fatalf("expected after outcome to remain OK, got %#v", got.Results[0].After)
	}
	if len(got.Results[0].After.ResultShape) != 1 || got.Results[0].After.ResultShape[0].Type != "character varying(320)" {
		t.Fatalf("expected after result shape, got %#v", got.Results[0].After.ResultShape)
	}
	if got.Results[0].ShapeChange == nil || got.Results[0].ShapeChange.Differences[0].Kind != "column_type" {
		t.Fatalf("expected shape_change details, got %#v", got.Results[0].ShapeChange)
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
