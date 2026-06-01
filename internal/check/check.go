package check

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/get-felipe/pg-contract/internal/config"
	"github.com/get-felipe/pg-contract/internal/query"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type Options struct {
	BeforeURL    string
	AfterURL     string
	SchemaBefore string
	SchemaAfter  string
	QueriesPath  string
	ConfigPath   string
}

type Report struct {
	Results []Result
}

type Result struct {
	Query  query.Query
	Before Outcome
	After  Outcome
}

type Outcome struct {
	OK    bool
	Error *DBError
}

type DBError struct {
	Code           string
	Message        string
	Detail         string
	Hint           string
	Position       int32
	SchemaName     string
	TableName      string
	ColumnName     string
	DataTypeName   string
	ConstraintName string
}

type Diagnostic struct {
	Reason     string
	Suggestion string
}

var diagnosticsBySQLState = map[string]Diagnostic{
	"22P02": {
		Reason:     "A SQL literal or cast cannot be represented by the target Postgres type.",
		Suggestion: "Keep the old accepted type values during this deploy, or update this query before changing the type contract.",
	},
	"3F000": {
		Reason:     "A schema referenced by this query does not exist in the target database.",
		Suggestion: "Keep the schema available, restore the expected search path, or update the query to use the new schema name.",
	},
	"42601": {
		Reason:     "The query is not valid SQL for this Postgres database.",
		Suggestion: "Fix the SQL syntax before relying on this query contract.",
	},
	"42702": {
		Reason:     "A column reference is ambiguous in the target schema.",
		Suggestion: "Qualify the ambiguous column with a table name or alias.",
	},
	"42703": {
		Reason:     "A column referenced by this query does not exist in the target schema.",
		Suggestion: "Keep the old column until deployed application code no longer reads it, or update this query before removing the column.",
	},
	"42704": {
		Reason:     "An object referenced by this query does not exist in the target schema.",
		Suggestion: "Keep the referenced object available through the deploy, or update the query to reference its replacement.",
	},
	"42725": {
		Reason:     "A function call in this query is ambiguous in the target schema.",
		Suggestion: "Add explicit casts or qualify the function call so Postgres can choose the intended overload.",
	},
	"42804": {
		Reason:     "The query has a datatype mismatch against the target schema.",
		Suggestion: "Align the query expression types with the target schema, or keep the previous column or function type until callers are updated.",
	},
	"42809": {
		Reason:     "An object referenced by this query has the wrong kind in the target schema.",
		Suggestion: "Keep the previous object kind available, or update the query to use the replacement table, view, function, or type.",
	},
	"42846": {
		Reason:     "A cast or coercion used by this query is not valid for the target schema.",
		Suggestion: "Keep the old type contract, add an explicit compatible cast, or update the query for the new type.",
	},
	"42883": {
		Reason:     "A function or operator used by this query does not exist for the inferred argument types.",
		Suggestion: "Keep a compatible function/operator signature, add explicit casts, or update this query before changing the callable contract.",
	},
	"42P01": {
		Reason:     "A table or view referenced by this query does not exist in the target schema.",
		Suggestion: "Keep the table or view during this deploy, restore the expected search path, or update this query before removing or renaming it.",
	},
	"42P02": {
		Reason:     "A parameter referenced by this query is not defined for the prepared statement.",
		Suggestion: "Check positional parameters and explicit parameter type config for this query.",
	},
	"42P10": {
		Reason:     "A column reference or constraint inference clause is not valid for the target schema.",
		Suggestion: "Update the query to match the target indexes, constraints, or grouping rules.",
	},
	"42P18": {
		Reason:     "Postgres could not infer a parameter type for this query.",
		Suggestion: "Add explicit casts in the query or configure parameter types in pg-contract.yaml.",
	},
}

func (e *DBError) Error() string {
	if e == nil {
		return ""
	}
	if e.Code != "" && e.Message != "" {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Code != "" {
		return e.Code
	}
	return "postgres error"
}

func Run(ctx context.Context, opts Options) (*Report, error) {
	if strings.TrimSpace(opts.BeforeURL) == "" {
		return nil, fmt.Errorf("missing required --before-url")
	}
	if strings.TrimSpace(opts.AfterURL) == "" {
		return nil, fmt.Errorf("missing required --after-url")
	}
	if strings.TrimSpace(opts.QueriesPath) == "" {
		return nil, fmt.Errorf("missing required --queries")
	}
	if opts.BeforeURL == opts.AfterURL && (strings.TrimSpace(opts.SchemaBefore) != "" || strings.TrimSpace(opts.SchemaAfter) != "") {
		return nil, fmt.Errorf("--schema-before/--schema-after require distinct --before-url and --after-url values")
	}

	queries, err := query.LoadDir(opts.QueriesPath)
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(opts.ConfigPath)
	if err != nil {
		return nil, err
	}
	if err := cfg.ValidateQueryNames(queryNames(queries)); err != nil {
		return nil, err
	}

	beforeConn, err := pgx.Connect(ctx, opts.BeforeURL)
	if err != nil {
		return nil, fmt.Errorf("connect before database: %w", err)
	}
	defer beforeConn.Close(context.Background())

	afterConn, err := pgx.Connect(ctx, opts.AfterURL)
	if err != nil {
		return nil, fmt.Errorf("connect after database: %w", err)
	}
	defer afterConn.Close(context.Background())

	if err := applySchema(ctx, beforeConn, opts.SchemaBefore); err != nil {
		return nil, fmt.Errorf("apply before schema: %w", err)
	}
	if err := applySchema(ctx, afterConn, opts.SchemaAfter); err != nil {
		return nil, fmt.Errorf("apply after schema: %w", err)
	}

	report := &Report{Results: make([]Result, 0, len(queries))}
	for i, q := range queries {
		result := Result{Query: q}
		params := cfg.Params(q.Name)
		result.Before = prepareQuery(ctx, beforeConn, fmt.Sprintf("pg_contract_before_%d", i+1), q, params)
		result.After = prepareQuery(ctx, afterConn, fmt.Sprintf("pg_contract_after_%d", i+1), q, params)
		report.Results = append(report.Results, result)
	}

	return report, nil
}

func queryNames(queries []query.Query) map[string]struct{} {
	names := make(map[string]struct{}, len(queries))
	for _, q := range queries {
		names[q.Name] = struct{}{}
	}
	return names
}

func applySchema(ctx context.Context, conn *pgx.Conn, path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}

	sql, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if strings.TrimSpace(string(sql)) == "" {
		return nil
	}

	if _, err := conn.PgConn().Exec(ctx, string(sql)).ReadAll(); err != nil {
		return pgError(err)
	}

	return nil
}

func prepareQuery(ctx context.Context, conn *pgx.Conn, preparedName string, q query.Query, params []string) Outcome {
	if len(params) > 0 {
		statement := fmt.Sprintf("PREPARE %s (%s) AS %s", preparedName, strings.Join(params, ", "), q.SQL)
		if _, err := conn.PgConn().Exec(ctx, statement).ReadAll(); err != nil {
			return Outcome{Error: pgError(err)}
		}
		if err := conn.Deallocate(ctx, preparedName); err != nil {
			return Outcome{Error: pgError(err)}
		}
		return Outcome{OK: true}
	}

	if _, err := conn.Prepare(ctx, preparedName, q.SQL); err != nil {
		return Outcome{Error: pgError(err)}
	}
	if err := conn.Deallocate(ctx, preparedName); err != nil {
		return Outcome{Error: pgError(err)}
	}
	return Outcome{OK: true}
}

func pgError(err error) *DBError {
	if err == nil {
		return nil
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return &DBError{
			Code:           pgErr.SQLState(),
			Message:        pgErr.Message,
			Detail:         pgErr.Detail,
			Hint:           pgErr.Hint,
			Position:       pgErr.Position,
			SchemaName:     pgErr.SchemaName,
			TableName:      pgErr.TableName,
			ColumnName:     pgErr.ColumnName,
			DataTypeName:   pgErr.DataTypeName,
			ConstraintName: pgErr.ConstraintName,
		}
	}

	return &DBError{Message: err.Error()}
}

func (r *Report) Breaking() []Result {
	if r == nil {
		return nil
	}

	var breaking []Result
	for _, result := range r.Results {
		if result.Before.OK && !result.After.OK {
			breaking = append(breaking, result)
		}
	}
	return breaking
}

func (r *Report) InvalidBefore() []Result {
	if r == nil {
		return nil
	}

	var invalid []Result
	for _, result := range r.Results {
		if !result.Before.OK {
			invalid = append(invalid, result)
		}
	}
	return invalid
}

func ExitCode(report *Report) int {
	if len(report.Breaking()) > 0 {
		return 1
	}
	if len(report.InvalidBefore()) > 0 {
		return 2
	}
	return 0
}

func Reason(err *DBError) string {
	return Classify(err).Reason
}

func Suggestion(err *DBError) string {
	return Classify(err).Suggestion
}

func Classify(err *DBError) Diagnostic {
	if err == nil {
		return Diagnostic{
			Reason:     "Unknown validation failure.",
			Suggestion: "Inspect the Postgres error and update either the schema change or the query contract.",
		}
	}

	if diagnostic, ok := diagnosticsBySQLState[err.Code]; ok {
		return diagnostic
	}

	reason := "Postgres rejected this query for the target schema."
	if err.Message != "" {
		reason = err.Message
	}
	return Diagnostic{
		Reason:     reason,
		Suggestion: "Keep the old database contract until deployed application code no longer depends on it.",
	}
}
