package config

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

var typeNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*|\s+[A-Za-z_][A-Za-z0-9_]*)*(?:\[\])?$`)

type Config struct {
	Version   string           `yaml:"version"`
	Defaults  Defaults         `yaml:"defaults"`
	QuerySets []QuerySet       `yaml:"query_sets"`
	Queries   map[string]Query `yaml:"queries"`

	baseDir string
}

type Defaults struct {
	Prepare Prepare `yaml:"prepare"`
}

type QuerySet struct {
	Name    string   `yaml:"name"`
	Queries PathList `yaml:"queries"`
	Schema  Schema   `yaml:"schema"`
	Prepare Prepare  `yaml:"prepare"`
	Tags    []string `yaml:"tags"`
}

type Schema struct {
	Before PathList `yaml:"before"`
	After  PathList `yaml:"after"`
}

type Prepare struct {
	SearchPath []string `yaml:"search_path"`
}

type Query struct {
	Params []string `yaml:"params"`
	Tags   []string `yaml:"tags"`
}

type PathList []string

func (p *PathList) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		if value.Tag != "!!str" {
			return fmt.Errorf("path must be a string")
		}
		*p = PathList{value.Value}
		return nil
	case yaml.SequenceNode:
		out := make([]string, 0, len(value.Content))
		for _, item := range value.Content {
			if item.Kind != yaml.ScalarNode || item.Tag != "!!str" {
				return fmt.Errorf("paths must be strings")
			}
			out = append(out, item.Value)
		}
		*p = out
		return nil
	default:
		return fmt.Errorf("path must be a string or list of strings")
	}
}

func Load(path string) (*Config, error) {
	if strings.TrimSpace(path) == "" {
		return &Config{Queries: map[string]Query{}}, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config %s: %w", path, err)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)

	var cfg Config
	if err := decoder.Decode(&cfg); err != nil {
		if err == io.EOF {
			return &Config{Queries: map[string]Query{}}, nil
		}
		return nil, fmt.Errorf("decode config %s: %w", path, err)
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("decode config %s: multiple YAML documents are not supported", path)
		}
		return nil, fmt.Errorf("decode config %s: %w", path, err)
	}

	if cfg.Queries == nil {
		cfg.Queries = map[string]Query{}
	}
	cfg.baseDir = filepath.Dir(path)

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("validate config %s: %w", path, err)
	}

	return &cfg, nil
}

func (c *Config) IsManifest() bool {
	return c != nil && c.Version == "0.2"
}

func (c *Config) ResolvePath(path string) string {
	if c == nil || c.baseDir == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Clean(filepath.Join(c.baseDir, path))
}

func (c *Config) ResolvePaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		out = append(out, c.ResolvePath(path))
	}
	return out
}

func (c *Config) Params(queryName string) []string {
	if c == nil {
		return nil
	}

	query, ok := c.Queries[queryName]
	if !ok {
		return nil
	}

	params := make([]string, len(query.Params))
	copy(params, query.Params)
	return params
}

func (c *Config) SearchPath(querySet QuerySet) []string {
	if c == nil {
		return nil
	}

	searchPath := querySet.Prepare.SearchPath
	if len(searchPath) == 0 {
		searchPath = c.Defaults.Prepare.SearchPath
	}

	out := make([]string, len(searchPath))
	copy(out, searchPath)
	return out
}

func (c *Config) Tags(querySet QuerySet, queryName string) []string {
	if c == nil {
		return nil
	}

	seen := map[string]struct{}{}
	var tags []string
	for _, tag := range querySet.Tags {
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		tags = append(tags, tag)
	}
	if query, ok := c.Queries[queryName]; ok {
		for _, tag := range query.Tags {
			if _, ok := seen[tag]; ok {
				continue
			}
			seen[tag] = struct{}{}
			tags = append(tags, tag)
		}
	}
	return tags
}

func (c *Config) ValidateQueryNames(known map[string]struct{}) error {
	if c == nil {
		return nil
	}

	var unknown []string
	for queryName := range c.Queries {
		if _, ok := known[queryName]; !ok {
			unknown = append(unknown, queryName)
		}
	}
	sort.Strings(unknown)

	if len(unknown) > 0 {
		return fmt.Errorf("config references unknown query %q", unknown[0])
	}

	return nil
}

func (c *Config) validate() error {
	c.Version = strings.TrimSpace(c.Version)
	if c.Version != "" && c.Version != "0.2" {
		return fmt.Errorf("unsupported config version %q", c.Version)
	}
	if c.Version == "" && len(c.QuerySets) > 0 {
		return fmt.Errorf("query_sets requires version \"0.2\"")
	}
	if c.Version == "" && len(c.Defaults.Prepare.SearchPath) > 0 {
		return fmt.Errorf("defaults requires version \"0.2\"")
	}
	if c.Version == "0.2" {
		if err := c.validateManifest(); err != nil {
			return err
		}
	}

	for queryName, query := range c.Queries {
		if strings.TrimSpace(queryName) == "" {
			return fmt.Errorf("query name cannot be empty")
		}
		for i, param := range query.Params {
			normalized, err := normalizeTypeName(param)
			if err != nil {
				return fmt.Errorf("query %q param %d: %w", queryName, i+1, err)
			}
			query.Params[i] = normalized
		}
		tags, err := normalizeList(query.Tags, fmt.Sprintf("query %q tag", queryName))
		if err != nil {
			return err
		}
		query.Tags = tags
		c.Queries[queryName] = query
	}
	return nil
}

func (c *Config) validateManifest() error {
	if len(c.QuerySets) == 0 {
		return fmt.Errorf("version 0.2 requires query_sets")
	}

	searchPath, err := normalizeList(c.Defaults.Prepare.SearchPath, "defaults.prepare.search_path")
	if err != nil {
		return err
	}
	c.Defaults.Prepare.SearchPath = searchPath

	seen := map[string]struct{}{}
	for i, querySet := range c.QuerySets {
		label := fmt.Sprintf("query_sets[%d]", i)
		querySet.Name = strings.TrimSpace(querySet.Name)
		if querySet.Name == "" {
			return fmt.Errorf("%s.name is required", label)
		}
		if _, ok := seen[querySet.Name]; ok {
			return fmt.Errorf("duplicate query set name %q", querySet.Name)
		}
		seen[querySet.Name] = struct{}{}

		queries, err := normalizeList([]string(querySet.Queries), label+".queries")
		if err != nil {
			return err
		}
		if len(queries) == 0 {
			return fmt.Errorf("%s.queries is required", label)
		}
		querySet.Queries = PathList(queries)

		before, err := normalizeList([]string(querySet.Schema.Before), label+".schema.before")
		if err != nil {
			return err
		}
		querySet.Schema.Before = PathList(before)

		after, err := normalizeList([]string(querySet.Schema.After), label+".schema.after")
		if err != nil {
			return err
		}
		querySet.Schema.After = PathList(after)

		searchPath, err := normalizeList(querySet.Prepare.SearchPath, label+".prepare.search_path")
		if err != nil {
			return err
		}
		querySet.Prepare.SearchPath = searchPath

		tags, err := normalizeList(querySet.Tags, label+".tags")
		if err != nil {
			return err
		}
		querySet.Tags = tags

		c.QuerySets[i] = querySet
	}

	return nil
}

func normalizeList(values []string, label string) ([]string, error) {
	out := make([]string, 0, len(values))
	for i, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			return nil, fmt.Errorf("%s %d cannot be empty", label, i+1)
		}
		out = append(out, normalized)
	}
	return out, nil
}

func normalizeTypeName(value string) (string, error) {
	normalized := strings.Join(strings.Fields(value), " ")
	if normalized == "" {
		return "", fmt.Errorf("type cannot be empty")
	}
	if !typeNamePattern.MatchString(normalized) {
		return "", fmt.Errorf("unsupported type name %q", value)
	}
	return normalized, nil
}

func Generate(queryNames []string) []byte {
	names := append([]string(nil), queryNames...)
	sort.Strings(names)

	var out bytes.Buffer
	out.WriteString("# Generated by pg-contract init.\n")
	out.WriteString("# Fill params only when Postgres cannot infer $1, $2, ... types during PREPARE.\n")
	out.WriteString("queries:\n")
	for _, name := range names {
		out.WriteString("  ")
		out.WriteString(name)
		out.WriteString(":\n")
		out.WriteString("    params: []\n")
	}
	return out.Bytes()
}
