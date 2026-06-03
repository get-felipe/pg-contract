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
	QuerySets    []string
	Tags         []string
}

type Report struct {
	Results []Result
}

type Result struct {
	QuerySet    string
	Tags        []string
	Query       query.Query
	Before      Outcome
	After       Outcome
	ShapeChange *ShapeChange
}

type Outcome struct {
	OK          bool
	Error       *DBError
	ResultShape []ResultColumn
}

type ResultColumn struct {
	Name         string
	DataType     string
	DataTypeOID  uint32
	TypeModifier int32
}

type ShapeChange struct {
	Differences []ShapeDifference
}

type ShapeDifference struct {
	Kind     string
	Position int
	Before   *ResultColumn
	After    *ResultColumn
	Message  string
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

type loadedQuerySet struct {
	set     config.QuerySet
	queries []query.Query
	tags    map[string][]string
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

	cfg, err := config.Load(opts.ConfigPath)
	if err != nil {
		return nil, err
	}
	if cfg.IsManifest() {
		return runManifest(ctx, opts, cfg)
	}

	if len(opts.QuerySets) > 0 || len(opts.Tags) > 0 {
		return nil, fmt.Errorf("--query-set/--tag require config version 0.2 query_sets")
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

	beforeTypeNames := map[typeKey]string{}
	afterTypeNames := map[typeKey]string{}

	report := &Report{Results: make([]Result, 0, len(queries))}
	for i, q := range queries {
		result := Result{Query: q}
		params := cfg.Params(q.Name)
		result.Before = prepareQuery(ctx, beforeConn, fmt.Sprintf("pg_contract_before_%d", i+1), q, params, beforeTypeNames)
		result.After = prepareQuery(ctx, afterConn, fmt.Sprintf("pg_contract_after_%d", i+1), q, params, afterTypeNames)
		result.ShapeChange = compareResultShapes(result.Before, result.After)
		report.Results = append(report.Results, result)
	}

	return report, nil
}

func runManifest(ctx context.Context, opts Options, cfg *config.Config) (*Report, error) {
	if strings.TrimSpace(opts.QueriesPath) != "" {
		return nil, fmt.Errorf("--queries cannot be used with config version 0.2 query_sets")
	}
	if strings.TrimSpace(opts.SchemaBefore) != "" || strings.TrimSpace(opts.SchemaAfter) != "" {
		return nil, fmt.Errorf("--schema-before/--schema-after cannot be used with config version 0.2 query_sets")
	}

	querySets, err := selectQuerySets(cfg.QuerySets, opts.QuerySets)
	if err != nil {
		return nil, err
	}
	selectedTags, err := selectTags(opts.Tags)
	if err != nil {
		return nil, err
	}
	loaded := make([]loadedQuerySet, 0, len(querySets))
	known := map[string]struct{}{}
	filesByName := map[string]string{}
	availableTags := map[string]struct{}{}
	total := 0
	for _, querySet := range querySets {
		queries, err := query.LoadPaths(cfg.ResolvePaths([]string(querySet.Queries)))
		if err != nil {
			return nil, fmt.Errorf("load query set %q: %w", querySet.Name, err)
		}
		filteredQueries := make([]query.Query, 0, len(queries))
		tagsByName := make(map[string][]string, len(queries))
		for _, q := range queries {
			if previous, ok := filesByName[q.Name]; ok {
				return nil, fmt.Errorf("duplicate query name %q in %s and %s", q.Name, previous, q.File)
			}
			filesByName[q.Name] = q.File
			known[q.Name] = struct{}{}
			tags := cfg.Tags(querySet, q.Name)
			tagsByName[q.Name] = tags
			for _, tag := range tags {
				availableTags[tag] = struct{}{}
			}
			if !matchesTags(tags, selectedTags) {
				continue
			}
			filteredQueries = append(filteredQueries, q)
		}
		if len(filteredQueries) == 0 {
			continue
		}
		total += len(filteredQueries)
		loaded = append(loaded, loadedQuerySet{set: querySet, queries: filteredQueries, tags: tagsByName})
	}

	if err := validateSelectedTags(selectedTags, availableTags, total); err != nil {
		return nil, err
	}
	if opts.BeforeURL == opts.AfterURL && loadedQuerySetsHaveSchema(loaded) {
		return nil, fmt.Errorf("query_sets schema files require distinct --before-url and --after-url values")
	}

	if len(opts.QuerySets) == 0 && len(opts.Tags) == 0 {
		if err := cfg.ValidateQueryNames(known); err != nil {
			return nil, err
		}
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

	beforeTypeNames := map[typeKey]string{}
	afterTypeNames := map[typeKey]string{}

	report := &Report{Results: make([]Result, 0, total)}
	for setIndex, loadedSet := range loaded {
		querySet := loadedSet.set
		if err := applySchemaFiles(ctx, beforeConn, cfg.ResolvePaths([]string(querySet.Schema.Before))); err != nil {
			return nil, fmt.Errorf("apply before schema for query set %q: %w", querySet.Name, err)
		}
		if err := applySchemaFiles(ctx, afterConn, cfg.ResolvePaths([]string(querySet.Schema.After))); err != nil {
			return nil, fmt.Errorf("apply after schema for query set %q: %w", querySet.Name, err)
		}

		if err := configureSearchPath(ctx, beforeConn, cfg.SearchPath(querySet)); err != nil {
			return nil, fmt.Errorf("configure before search_path for query set %q: %w", querySet.Name, err)
		}
		if err := configureSearchPath(ctx, afterConn, cfg.SearchPath(querySet)); err != nil {
			return nil, fmt.Errorf("configure after search_path for query set %q: %w", querySet.Name, err)
		}

		for queryIndex, q := range loadedSet.queries {
			result := Result{
				QuerySet: querySet.Name,
				Tags:     loadedSet.tags[q.Name],
				Query:    q,
			}
			params := cfg.Params(q.Name)
			result.Before = prepareQuery(ctx, beforeConn, fmt.Sprintf("pg_contract_before_%d_%d", setIndex+1, queryIndex+1), q, params, beforeTypeNames)
			result.After = prepareQuery(ctx, afterConn, fmt.Sprintf("pg_contract_after_%d_%d", setIndex+1, queryIndex+1), q, params, afterTypeNames)
			result.ShapeChange = compareResultShapes(result.Before, result.After)
			report.Results = append(report.Results, result)
		}
	}

	return report, nil
}

func selectQuerySets(querySets []config.QuerySet, selectors []string) ([]config.QuerySet, error) {
	if len(selectors) == 0 {
		return querySets, nil
	}

	selected := map[string]struct{}{}
	selectedOrder := make([]string, 0, len(selectors))
	for _, selector := range selectors {
		name := strings.TrimSpace(selector)
		if name == "" {
			return nil, fmt.Errorf("--query-set cannot be empty")
		}
		if _, ok := selected[name]; ok {
			continue
		}
		selected[name] = struct{}{}
		selectedOrder = append(selectedOrder, name)
	}

	matched := map[string]struct{}{}
	out := make([]config.QuerySet, 0, len(selected))
	for _, querySet := range querySets {
		if _, ok := selected[querySet.Name]; !ok {
			continue
		}
		out = append(out, querySet)
		matched[querySet.Name] = struct{}{}
	}

	for _, name := range selectedOrder {
		if _, ok := matched[name]; !ok {
			return nil, fmt.Errorf("unknown query set %q", name)
		}
	}

	return out, nil
}

func selectTags(selectors []string) (map[string]struct{}, error) {
	if len(selectors) == 0 {
		return nil, nil
	}

	selected := map[string]struct{}{}
	for _, selector := range selectors {
		tag := strings.TrimSpace(selector)
		if tag == "" {
			return nil, fmt.Errorf("--tag cannot be empty")
		}
		selected[tag] = struct{}{}
	}
	return selected, nil
}

func matchesTags(tags []string, selected map[string]struct{}) bool {
	if len(selected) == 0 {
		return true
	}

	for _, tag := range tags {
		if _, ok := selected[tag]; ok {
			return true
		}
	}
	return false
}

func validateSelectedTags(selected map[string]struct{}, available map[string]struct{}, total int) error {
	if len(selected) == 0 {
		return nil
	}

	for tag := range selected {
		if _, ok := available[tag]; !ok {
			return fmt.Errorf("unknown tag %q", tag)
		}
	}
	if total == 0 {
		return fmt.Errorf("no queries matched selected tags")
	}
	return nil
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

func applySchemaFiles(ctx context.Context, conn *pgx.Conn, paths []string) error {
	for _, path := range paths {
		if err := applySchema(ctx, conn, path); err != nil {
			return err
		}
	}
	return nil
}

func configureSearchPath(ctx context.Context, conn *pgx.Conn, searchPath []string) error {
	if _, err := conn.Exec(ctx, "reset search_path"); err != nil {
		return pgError(err)
	}
	if len(searchPath) == 0 {
		return nil
	}

	_, err := conn.Exec(ctx, "select set_config('search_path', $1, false)", strings.Join(searchPath, ", "))
	if err != nil {
		return pgError(err)
	}
	return nil
}

func loadedQuerySetsHaveSchema(loaded []loadedQuerySet) bool {
	for _, querySet := range loaded {
		if len(querySet.set.Schema.Before) > 0 || len(querySet.set.Schema.After) > 0 {
			return true
		}
	}
	return false
}

type typeKey struct {
	oid      uint32
	modifier int32
}

func prepareQuery(ctx context.Context, conn *pgx.Conn, preparedName string, q query.Query, params []string, typeNames map[typeKey]string) Outcome {
	paramOIDs, paramErr := resolveParamOIDs(ctx, conn, params)
	if paramErr != nil {
		return Outcome{Error: paramErr}
	}

	description, err := conn.PgConn().Prepare(ctx, preparedName, q.SQL, paramOIDs)
	if err != nil {
		_ = conn.Deallocate(ctx, preparedName)
		return Outcome{Error: pgError(err)}
	}

	resultShape, shapeErr := resultShape(ctx, conn, description.Fields, typeNames)
	deallocateErr := conn.Deallocate(ctx, preparedName)
	if shapeErr != nil {
		return Outcome{Error: shapeErr}
	}
	if deallocateErr != nil {
		return Outcome{Error: pgError(deallocateErr)}
	}

	return Outcome{OK: true, ResultShape: resultShape}
}

func resolveParamOIDs(ctx context.Context, conn *pgx.Conn, params []string) ([]uint32, *DBError) {
	if len(params) == 0 {
		return nil, nil
	}

	oids := make([]uint32, 0, len(params))
	for _, param := range params {
		var oid uint32
		if err := conn.QueryRow(ctx, "select $1::regtype::oid", param).Scan(&oid); err != nil {
			return nil, pgError(err)
		}
		oids = append(oids, oid)
	}
	return oids, nil
}

func resultShape(ctx context.Context, conn *pgx.Conn, fields []pgconn.FieldDescription, typeNames map[typeKey]string) ([]ResultColumn, *DBError) {
	shape := make([]ResultColumn, 0, len(fields))
	for _, field := range fields {
		dataType, err := formatDataType(ctx, conn, field.DataTypeOID, field.TypeModifier, typeNames)
		if err != nil {
			return nil, pgError(err)
		}
		shape = append(shape, ResultColumn{
			Name:         field.Name,
			DataType:     dataType,
			DataTypeOID:  field.DataTypeOID,
			TypeModifier: field.TypeModifier,
		})
	}
	return shape, nil
}

func formatDataType(ctx context.Context, conn *pgx.Conn, oid uint32, modifier int32, typeNames map[typeKey]string) (string, error) {
	key := typeKey{oid: oid, modifier: modifier}
	if name, ok := typeNames[key]; ok {
		return name, nil
	}

	var name string
	if err := conn.QueryRow(ctx, "select pg_catalog.format_type($1::oid, $2::integer)", oid, modifier).Scan(&name); err != nil {
		return "", err
	}
	typeNames[key] = name
	return name, nil
}

func compareResultShapes(before Outcome, after Outcome) *ShapeChange {
	if !before.OK || !after.OK {
		return nil
	}

	differences := compareColumns(before.ResultShape, after.ResultShape)
	if len(differences) == 0 {
		return nil
	}
	return &ShapeChange{Differences: differences}
}

func compareColumns(before []ResultColumn, after []ResultColumn) []ShapeDifference {
	var differences []ShapeDifference
	shared := len(before)
	if len(after) < shared {
		shared = len(after)
	}

	for i := 0; i < shared; i++ {
		beforeColumn := before[i]
		afterColumn := after[i]
		position := i + 1
		if beforeColumn.Name != afterColumn.Name {
			differences = append(differences, ShapeDifference{
				Kind:     "column_name",
				Position: position,
				Before:   &beforeColumn,
				After:    &afterColumn,
				Message:  fmt.Sprintf("column %d name changed from %q to %q", position, beforeColumn.Name, afterColumn.Name),
			})
		}
		if beforeColumn.DataType != afterColumn.DataType {
			differences = append(differences, ShapeDifference{
				Kind:     "column_type",
				Position: position,
				Before:   &beforeColumn,
				After:    &afterColumn,
				Message:  fmt.Sprintf("column %d %q type changed from %s to %s", position, afterColumn.Name, beforeColumn.DataType, afterColumn.DataType),
			})
		}
	}

	for i := shared; i < len(before); i++ {
		beforeColumn := before[i]
		position := i + 1
		differences = append(differences, ShapeDifference{
			Kind:     "column_removed",
			Position: position,
			Before:   &beforeColumn,
			Message:  fmt.Sprintf("column %d %q was removed from the result", position, beforeColumn.Name),
		})
	}

	for i := shared; i < len(after); i++ {
		afterColumn := after[i]
		position := i + 1
		differences = append(differences, ShapeDifference{
			Kind:     "column_added",
			Position: position,
			After:    &afterColumn,
			Message:  fmt.Sprintf("column %d %q was added to the result", position, afterColumn.Name),
		})
	}

	return differences
}

func ShapeReason(change *ShapeChange) string {
	if change == nil {
		return ""
	}
	return "The query result columns changed between the before and after schemas."
}

func ShapeSuggestion(change *ShapeChange) string {
	if change == nil {
		return ""
	}
	return "Keep returned column names, order, and types stable until callers are updated, or deploy the query and caller changes together."
}

func ShapeSummary(change *ShapeChange) string {
	if change == nil || len(change.Differences) == 0 {
		return ""
	}
	summary := change.Differences[0].Message
	if extra := len(change.Differences) - 1; extra > 0 {
		summary = fmt.Sprintf("%s (+%d more)", summary, extra)
	}
	return summary
}

func (result Result) IsBreaking() bool {
	return result.Before.OK && (!result.After.OK || result.ShapeChange != nil)
}

func (result Result) IsInvalidBefore() bool {
	return !result.Before.OK
}

func (result Result) BreakingReason() string {
	if result.ShapeChange != nil && result.After.OK {
		return ShapeReason(result.ShapeChange)
	}
	return Reason(result.After.Error)
}

func (result Result) BreakingSuggestion() string {
	if result.ShapeChange != nil && result.After.OK {
		return ShapeSuggestion(result.ShapeChange)
	}
	return Suggestion(result.After.Error)
}

func (result Result) BreakingSummary() string {
	if result.ShapeChange != nil && result.After.OK {
		return ShapeSummary(result.ShapeChange)
	}
	if result.After.Error != nil {
		return result.After.Error.Message
	}
	return ""
}

func (result Result) BreakingError() *DBError {
	if result.ShapeChange != nil && result.After.OK {
		return nil
	}
	return result.After.Error
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
		if result.IsBreaking() {
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
		if result.IsInvalidBefore() {
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
