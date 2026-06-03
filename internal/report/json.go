package report

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/get-felipe/pg-contract/internal/check"
)

type JSONReport struct {
	Version string       `json:"version"`
	Status  string       `json:"status"`
	Summary JSONSummary  `json:"summary"`
	Results []JSONResult `json:"results"`
}

type JSONSummary struct {
	Checked       int `json:"checked"`
	Breaking      int `json:"breaking"`
	InvalidBefore int `json:"invalid_before"`
}

type JSONResult struct {
	QuerySet    string           `json:"query_set,omitempty"`
	Tags        []string         `json:"tags,omitempty"`
	Query       JSONQuery        `json:"query"`
	Status      string           `json:"status"`
	Before      JSONOutcome      `json:"before"`
	After       JSONOutcome      `json:"after"`
	ShapeChange *JSONShapeChange `json:"shape_change,omitempty"`
}

type JSONQuery struct {
	Name string `json:"name"`
	File string `json:"file"`
	Line int    `json:"line"`
}

type JSONOutcome struct {
	OK          bool               `json:"ok"`
	ResultShape []JSONResultColumn `json:"result_shape,omitempty"`
	Reason      string             `json:"reason,omitempty"`
	Suggestion  string             `json:"suggestion,omitempty"`
	Error       *JSONDBError       `json:"error,omitempty"`
}

type JSONResultColumn struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	DataTypeOID uint32 `json:"type_oid,omitempty"`
}

type JSONShapeChange struct {
	Reason      string                `json:"reason"`
	Suggestion  string                `json:"suggestion"`
	Differences []JSONShapeDifference `json:"differences"`
}

type JSONShapeDifference struct {
	Kind     string            `json:"kind"`
	Position int               `json:"position"`
	Before   *JSONResultColumn `json:"before,omitempty"`
	After    *JSONResultColumn `json:"after,omitempty"`
	Message  string            `json:"message"`
}

type JSONDBError struct {
	Code           string `json:"code,omitempty"`
	Message        string `json:"message,omitempty"`
	Detail         string `json:"detail,omitempty"`
	Hint           string `json:"hint,omitempty"`
	Position       int32  `json:"position,omitempty"`
	SchemaName     string `json:"schema_name,omitempty"`
	TableName      string `json:"table_name,omitempty"`
	ColumnName     string `json:"column_name,omitempty"`
	DataTypeName   string `json:"data_type_name,omitempty"`
	ConstraintName string `json:"constraint_name,omitempty"`
}

func WriteJSON(w io.Writer, report *check.Report) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(NewJSONReport(report)); err != nil {
		return fmt.Errorf("write json report: %w", err)
	}
	return nil
}

func NewJSONReport(report *check.Report) JSONReport {
	breaking := report.Breaking()
	invalidBefore := report.InvalidBefore()

	status := "ok"
	if len(breaking) > 0 {
		status = "breaking"
	} else if len(invalidBefore) > 0 {
		status = "invalid_before"
	}

	out := JSONReport{
		Version: "0.1",
		Status:  status,
		Summary: JSONSummary{
			Checked:       len(report.Results),
			Breaking:      len(breaking),
			InvalidBefore: len(invalidBefore),
		},
		Results: make([]JSONResult, 0, len(report.Results)),
	}

	for _, result := range report.Results {
		out.Results = append(out.Results, newJSONResult(result))
	}

	return out
}

func newJSONResult(result check.Result) JSONResult {
	status := "ok"
	if result.IsBreaking() {
		status = "breaking"
	} else if result.IsInvalidBefore() {
		status = "invalid_before"
	}

	return JSONResult{
		QuerySet: result.QuerySet,
		Tags:     result.Tags,
		Query: JSONQuery{
			Name: result.Query.Name,
			File: result.Query.File,
			Line: result.Query.StartLine,
		},
		Status:      status,
		Before:      newJSONOutcome(result.Before),
		After:       newJSONOutcome(result.After),
		ShapeChange: newJSONShapeChange(result.ShapeChange),
	}
}

func newJSONOutcome(outcome check.Outcome) JSONOutcome {
	if outcome.OK {
		return JSONOutcome{OK: true, ResultShape: newJSONResultColumns(outcome.ResultShape)}
	}

	return JSONOutcome{
		OK:         false,
		Reason:     check.Reason(outcome.Error),
		Suggestion: check.Suggestion(outcome.Error),
		Error:      newJSONDBError(outcome.Error),
	}
}

func newJSONResultColumns(columns []check.ResultColumn) []JSONResultColumn {
	if len(columns) == 0 {
		return nil
	}

	out := make([]JSONResultColumn, 0, len(columns))
	for _, column := range columns {
		out = append(out, newJSONResultColumn(column))
	}
	return out
}

func newJSONResultColumn(column check.ResultColumn) JSONResultColumn {
	return JSONResultColumn{
		Name:        column.Name,
		Type:        column.DataType,
		DataTypeOID: column.DataTypeOID,
	}
}

func newJSONShapeChange(change *check.ShapeChange) *JSONShapeChange {
	if change == nil {
		return nil
	}

	out := &JSONShapeChange{
		Reason:      check.ShapeReason(change),
		Suggestion:  check.ShapeSuggestion(change),
		Differences: make([]JSONShapeDifference, 0, len(change.Differences)),
	}
	for _, difference := range change.Differences {
		out.Differences = append(out.Differences, newJSONShapeDifference(difference))
	}
	return out
}

func newJSONShapeDifference(difference check.ShapeDifference) JSONShapeDifference {
	return JSONShapeDifference{
		Kind:     difference.Kind,
		Position: difference.Position,
		Before:   newJSONResultColumnPtr(difference.Before),
		After:    newJSONResultColumnPtr(difference.After),
		Message:  difference.Message,
	}
}

func newJSONResultColumnPtr(column *check.ResultColumn) *JSONResultColumn {
	if column == nil {
		return nil
	}
	out := newJSONResultColumn(*column)
	return &out
}

func newJSONDBError(err *check.DBError) *JSONDBError {
	if err == nil {
		return nil
	}

	return &JSONDBError{
		Code:           err.Code,
		Message:        err.Message,
		Detail:         err.Detail,
		Hint:           err.Hint,
		Position:       err.Position,
		SchemaName:     err.SchemaName,
		TableName:      err.TableName,
		ColumnName:     err.ColumnName,
		DataTypeName:   err.DataTypeName,
		ConstraintName: err.ConstraintName,
	}
}
