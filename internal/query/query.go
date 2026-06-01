package query

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var namePattern = regexp.MustCompile(`(?i)^\s*--\s*name:\s*([A-Za-z][A-Za-z0-9_.-]*)(?:\s+:[A-Za-z][A-Za-z0-9_-]*)?\s*$`)

type Query struct {
	Name      string
	File      string
	SQL       string
	StartLine int
}

func LoadDir(root string) ([]Query, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("queries path is required")
	}

	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("stat queries path: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("queries path must be a directory: %s", root)
	}

	var files []string
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".sql") {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("walk queries path: %w", err)
	}

	sort.Strings(files)

	queries := make([]Query, 0, len(files))
	seen := map[string]string{}
	for _, file := range files {
		q, err := loadFile(root, file)
		if err != nil {
			return nil, err
		}
		if previous, exists := seen[q.Name]; exists {
			return nil, fmt.Errorf("duplicate query name %q in %s and %s", q.Name, previous, q.File)
		}
		seen[q.Name] = q.File
		queries = append(queries, q)
	}

	if len(queries) == 0 {
		return nil, fmt.Errorf("no .sql query files found in %s", root)
	}

	return queries, nil
}

func loadFile(root string, file string) (Query, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return Query{}, fmt.Errorf("read query file %s: %w", file, err)
	}

	sql := strings.TrimSpace(string(data))
	if sql == "" {
		return Query{}, fmt.Errorf("query file is empty: %s", file)
	}

	name := ""
	startLine := 1
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if name == "" {
			if matches := namePattern.FindStringSubmatch(line); matches != nil {
				name = matches[1]
			}
		}
		if trimmed != "" && !strings.HasPrefix(trimmed, "--") {
			startLine = i + 1
			break
		}
	}

	if name == "" {
		name = fallbackName(root, file)
	}

	return Query{
		Name:      name,
		File:      file,
		SQL:       sql,
		StartLine: startLine,
	}, nil
}

func fallbackName(root string, file string) string {
	rel, err := filepath.Rel(root, file)
	if err != nil {
		rel = filepath.Base(file)
	}
	rel = filepath.ToSlash(rel)
	rel = strings.TrimSuffix(rel, filepath.Ext(rel))
	rel = strings.Trim(rel, "/")
	rel = strings.ReplaceAll(rel, "/", ".")
	rel = strings.ReplaceAll(rel, " ", "_")
	if rel == "" {
		return "query"
	}
	return rel
}
