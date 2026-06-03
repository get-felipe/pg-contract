package report

import (
	"fmt"
	"io"

	"github.com/get-felipe/pg-contract/internal/check"
)

func WriteText(w io.Writer, report *check.Report) {
	breaking := report.Breaking()
	invalidBefore := report.InvalidBefore()

	for _, result := range invalidBefore {
		writeInvalidBefore(w, result)
	}
	for _, result := range breaking {
		writeBreaking(w, result)
	}

	switch {
	case len(breaking) > 0:
		fmt.Fprintf(w, "Summary: %d query checked, %d breaking change found.\n", len(report.Results), len(breaking))
	case len(invalidBefore) > 0:
		fmt.Fprintf(w, "Summary: %d query checked, %d query invalid against the before schema.\n", len(report.Results), len(invalidBefore))
	default:
		fmt.Fprintf(w, "OK: %d query checked. No valid-before/fail-after breakages found.\n", len(report.Results))
	}
}

func writeInvalidBefore(w io.Writer, result check.Result) {
	fmt.Fprintf(w, "WARN %s\n", result.Query.Name)
	fmt.Fprintf(w, "File: %s:%d\n\n", result.Query.File, result.Query.StartLine)
	fmt.Fprintln(w, "Reason:")
	fmt.Fprintf(w, "  Query is already invalid against the before schema: %s\n\n", check.Reason(result.Before.Error))
	writePostgresError(w, result.Before.Error)
	fmt.Fprintln(w)
}

func writeBreaking(w io.Writer, result check.Result) {
	fmt.Fprintf(w, "FAIL %s\n", result.Query.Name)
	fmt.Fprintf(w, "File: %s:%d\n\n", result.Query.File, result.Query.StartLine)
	fmt.Fprintln(w, "Reason:")
	fmt.Fprintf(w, "  %s\n\n", result.BreakingReason())
	if result.ShapeChange != nil && result.After.OK {
		writeShapeChange(w, result.ShapeChange)
	} else {
		writePostgresError(w, result.BreakingError())
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Impact:")
	if result.ShapeChange != nil && result.After.OK {
		fmt.Fprintln(w, "  This query works before and after the schema change, but its returned columns changed.")
	} else {
		fmt.Fprintln(w, "  This query worked before the schema change and fails after it.")
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Suggested fix:")
	fmt.Fprintf(w, "  %s\n", result.BreakingSuggestion())
	fmt.Fprintln(w)
}

func writeShapeChange(w io.Writer, change *check.ShapeChange) {
	if change == nil || len(change.Differences) == 0 {
		return
	}

	fmt.Fprintln(w, "Result shape change:")
	for _, difference := range change.Differences {
		fmt.Fprintf(w, "  - %s\n", difference.Message)
	}
}

func writePostgresError(w io.Writer, err *check.DBError) {
	if err == nil {
		return
	}

	fmt.Fprintln(w, "Postgres error:")
	if err.Message != "" {
		fmt.Fprintf(w, "  ERROR: %s\n", err.Message)
	}
	if err.Code != "" {
		fmt.Fprintf(w, "  SQLSTATE: %s\n", err.Code)
	}
	if err.Detail != "" {
		fmt.Fprintf(w, "  DETAIL: %s\n", err.Detail)
	}
	if err.Hint != "" {
		fmt.Fprintf(w, "  HINT: %s\n", err.Hint)
	}
	if err.Position > 0 {
		fmt.Fprintf(w, "  POSITION: %d\n", err.Position)
	}
	writeField(w, "SCHEMA", err.SchemaName)
	writeField(w, "TABLE", err.TableName)
	writeField(w, "COLUMN", err.ColumnName)
	writeField(w, "DATA TYPE", err.DataTypeName)
	writeField(w, "CONSTRAINT", err.ConstraintName)
}

func writeField(w io.Writer, label string, value string) {
	if value != "" {
		fmt.Fprintf(w, "  %s: %s\n", label, value)
	}
}
