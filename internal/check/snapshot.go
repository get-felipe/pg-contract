package check

import (
	"context"
	"fmt"
	"strings"

	"github.com/get-felipe/pg-contract/internal/config"
	contractfile "github.com/get-felipe/pg-contract/internal/contract"
	"github.com/get-felipe/pg-contract/internal/query"
	"github.com/jackc/pgx/v5"
)

type SnapshotOptions struct {
	BeforeURL    string
	SchemaBefore string
	QueriesPath  string
	ConfigPath   string
	QuerySets    []string
	Tags         []string
	ToolVersion  string
}

func Snapshot(ctx context.Context, opts SnapshotOptions) (contractfile.Contract, error) {
	if strings.TrimSpace(opts.BeforeURL) == "" {
		return contractfile.Contract{}, fmt.Errorf("missing required --before-url")
	}

	cfg, err := config.Load(opts.ConfigPath)
	if err != nil {
		return contractfile.Contract{}, err
	}
	if cfg.IsManifest() {
		return snapshotManifest(ctx, opts, cfg)
	}

	if len(opts.QuerySets) > 0 || len(opts.Tags) > 0 {
		return contractfile.Contract{}, fmt.Errorf("--query-set/--tag require config version 0.2 query_sets")
	}
	if strings.TrimSpace(opts.QueriesPath) == "" {
		return contractfile.Contract{}, fmt.Errorf("missing required --queries")
	}

	queries, err := query.LoadDir(opts.QueriesPath)
	if err != nil {
		return contractfile.Contract{}, err
	}
	if err := cfg.ValidateQueryNames(queryNames(queries)); err != nil {
		return contractfile.Contract{}, err
	}

	conn, err := pgx.Connect(ctx, opts.BeforeURL)
	if err != nil {
		return contractfile.Contract{}, fmt.Errorf("connect before database: %w", err)
	}
	defer conn.Close(context.Background())

	if err := applySchema(ctx, conn, opts.SchemaBefore); err != nil {
		return contractfile.Contract{}, fmt.Errorf("apply before schema: %w", err)
	}

	typeNames := map[typeKey]string{}
	contractQueries := make([]contractfile.Query, 0, len(queries))
	for i, q := range queries {
		params := cfg.Params(q.Name)
		outcome := prepareQuery(ctx, conn, fmt.Sprintf("pg_contract_snapshot_%d", i+1), q, params, typeNames)
		if outcome.Error != nil {
			return contractfile.Contract{}, fmt.Errorf("snapshot query %q invalid before: %w", q.Name, outcome.Error)
		}
		contractQueries = append(contractQueries, newContractQuery(q, "", nil, params, outcome.ResultShape))
	}

	return contractfile.New(
		contractfile.Source{Config: opts.ConfigPath, Mode: contractfile.SourceModeLegacy},
		contractfile.Scope{Complete: true},
		contractQueries,
		opts.ToolVersion,
	), nil
}

func snapshotManifest(ctx context.Context, opts SnapshotOptions, cfg *config.Config) (contractfile.Contract, error) {
	if strings.TrimSpace(opts.QueriesPath) != "" {
		return contractfile.Contract{}, fmt.Errorf("--queries cannot be used with config version 0.2 query_sets")
	}
	if strings.TrimSpace(opts.SchemaBefore) != "" {
		return contractfile.Contract{}, fmt.Errorf("--schema-before cannot be used with config version 0.2 query_sets")
	}

	querySets, err := selectQuerySets(cfg.QuerySets, opts.QuerySets)
	if err != nil {
		return contractfile.Contract{}, err
	}
	selectedTags, err := selectTags(opts.Tags)
	if err != nil {
		return contractfile.Contract{}, err
	}

	loaded := make([]loadedQuerySet, 0, len(querySets))
	known := map[string]struct{}{}
	filesByName := map[string]string{}
	availableTags := map[string]struct{}{}
	total := 0
	for _, querySet := range querySets {
		queries, err := query.LoadPaths(cfg.ResolvePaths([]string(querySet.Queries)))
		if err != nil {
			return contractfile.Contract{}, fmt.Errorf("load query set %q: %w", querySet.Name, err)
		}
		filteredQueries := make([]query.Query, 0, len(queries))
		tagsByName := make(map[string][]string, len(queries))
		for _, q := range queries {
			if previous, ok := filesByName[q.Name]; ok {
				return contractfile.Contract{}, fmt.Errorf("duplicate query name %q in %s and %s", q.Name, previous, q.File)
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
		return contractfile.Contract{}, err
	}
	if len(opts.QuerySets) == 0 && len(opts.Tags) == 0 {
		if err := cfg.ValidateQueryNames(known); err != nil {
			return contractfile.Contract{}, err
		}
	}

	conn, err := pgx.Connect(ctx, opts.BeforeURL)
	if err != nil {
		return contractfile.Contract{}, fmt.Errorf("connect before database: %w", err)
	}
	defer conn.Close(context.Background())

	typeNames := map[typeKey]string{}
	contractQueries := make([]contractfile.Query, 0, total)
	for setIndex, loadedSet := range loaded {
		querySet := loadedSet.set
		if err := applySchemaFiles(ctx, conn, cfg.ResolvePaths([]string(querySet.Schema.Before))); err != nil {
			return contractfile.Contract{}, fmt.Errorf("apply before schema for query set %q: %w", querySet.Name, err)
		}
		if err := configureSearchPath(ctx, conn, cfg.SearchPath(querySet)); err != nil {
			return contractfile.Contract{}, fmt.Errorf("configure before search_path for query set %q: %w", querySet.Name, err)
		}

		for queryIndex, q := range loadedSet.queries {
			params := cfg.Params(q.Name)
			outcome := prepareQuery(ctx, conn, fmt.Sprintf("pg_contract_snapshot_%d_%d", setIndex+1, queryIndex+1), q, params, typeNames)
			if outcome.Error != nil {
				return contractfile.Contract{}, fmt.Errorf("snapshot query %q invalid before: %w", q.Name, outcome.Error)
			}
			contractQueries = append(contractQueries, newContractQuery(q, querySet.Name, loadedSet.tags[q.Name], params, outcome.ResultShape))
		}
	}

	return contractfile.New(
		contractfile.Source{Config: opts.ConfigPath, Mode: contractfile.SourceModeManifestV02},
		contractfile.Scope{
			Complete:  len(opts.QuerySets) == 0 && len(opts.Tags) == 0,
			QuerySets: append([]string(nil), opts.QuerySets...),
			Tags:      append([]string(nil), opts.Tags...),
		},
		contractQueries,
		opts.ToolVersion,
	), nil
}

func newContractQuery(q query.Query, querySet string, tags []string, params []string, columns []ResultColumn) contractfile.Query {
	return contractfile.Query{
		Name:        q.Name,
		QuerySet:    querySet,
		Tags:        append([]string(nil), tags...),
		File:        q.File,
		Line:        q.StartLine,
		SQLSHA256:   query.SQLSHA256(q.SQL),
		Params:      append([]string(nil), params...),
		ResultShape: newContractColumns(columns),
	}
}

func newContractColumns(columns []ResultColumn) []contractfile.Column {
	out := make([]contractfile.Column, 0, len(columns))
	for i, column := range columns {
		out = append(out, contractfile.Column{
			Position: i + 1,
			Name:     column.Name,
			Type:     column.DataType,
		})
	}
	return out
}
